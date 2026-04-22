package observability

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const serviceName = "order-service"

var (
	defaultLogger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	defaultStats  = NewCollector()
)

type Collector struct {
	registry        *prometheus.Registry
	requestTotal    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	businessTotal   *prometheus.CounterVec
	inflight        prometheus.Gauge
}

func NewCollector() *Collector {
	registry := prometheus.NewRegistry()
	collector := &Collector{
		registry: registry,
		requestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "delivery_http_requests_total",
				Help: "Total HTTP requests handled by the service.",
			},
			[]string{"service", "route", "method", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "delivery_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds.",
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"service", "route", "method"},
		),
		businessTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "delivery_business_events_total",
				Help: "Business events emitted by the service.",
			},
			[]string{"service", "event", "result"},
		),
		inflight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "delivery_http_inflight_requests",
				Help: "Current number of in-flight requests.",
			},
		),
	}

	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collector.requestTotal)
	registry.MustRegister(collector.requestDuration)
	registry.MustRegister(collector.businessTotal)
	registry.MustRegister(collector.inflight)

	return collector
}

func SetLogger(logger *slog.Logger) {
	if logger != nil {
		defaultLogger = logger
	}
}

func Logger() *slog.Logger {
	return defaultLogger
}

func SetCollector(collector *Collector) {
	if collector != nil {
		defaultStats = collector
	}
}

func Stats() *Collector {
	return defaultStats
}

func Handler() http.Handler {
	return promhttp.HandlerFor(defaultStats.registry, promhttp.HandlerOpts{})
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		startedAt := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		defaultStats.inflight.Inc()
		defer defaultStats.inflight.Dec()

		next.ServeHTTP(rw, r)

		route := routeLabel(r.Method, r.URL.Path)
		statusText := statusClass(rw.statusCode)
		duration := time.Since(startedAt)

		defaultStats.requestTotal.WithLabelValues(serviceName, route, r.Method, statusText).Inc()
		defaultStats.requestDuration.WithLabelValues(serviceName, route, r.Method).Observe(duration.Seconds())

		defaultLogger.Info("http_request",
			"service", serviceName,
			"method", r.Method,
			"route", route,
			"status", rw.statusCode,
			"duration_ms", duration.Milliseconds(),
			"remote_ip", clientIP(r),
			"user_agent", r.UserAgent(),
		)
	})
}

func (c *Collector) ObserveBusiness(event, result string) {
	c.businessTotal.WithLabelValues(serviceName, event, result).Inc()
}

func statusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

func routeLabel(method, path string) string {
	switch {
	case method == http.MethodPost && path == "/orders":
		return "POST /orders"
	case method == http.MethodGet && path == "/orders":
		return "GET /orders"
	case method == http.MethodGet && strings.HasPrefix(path, "/orders/"):
		return "GET /orders/:id"
	case method == http.MethodPost && path == "/api/v1/couriers/availability":
		return "POST /api/v1/couriers/availability"
	case method == http.MethodPost && path == "/api/v1/couriers/location":
		return "POST /api/v1/couriers/location"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/v1/orders/") && strings.HasSuffix(path, "/assign"):
		return "POST /api/v1/orders/:orderId/assign"
	case method == http.MethodGet && strings.HasPrefix(path, "/api/v1/couriers/") && strings.HasSuffix(path, "/active-order"):
		return "GET /api/v1/couriers/:courierId/active-order"
	default:
		return method + " " + path
	}
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	return r.RemoteAddr
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
