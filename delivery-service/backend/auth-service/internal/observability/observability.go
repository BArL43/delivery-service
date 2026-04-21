package observability

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const serviceName = "auth-service"

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

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		startedAt := time.Now()
		defaultStats.inflight.Inc()
		defer defaultStats.inflight.Dec()

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		statusText := statusClass(c.Writer.Status())
		duration := time.Since(startedAt)

		defaultStats.requestTotal.WithLabelValues(serviceName, route, c.Request.Method, statusText).Inc()
		defaultStats.requestDuration.WithLabelValues(serviceName, route, c.Request.Method).Observe(duration.Seconds())

		defaultLogger.Info("http_request",
			"service", serviceName,
			"method", c.Request.Method,
			"route", route,
			"status", c.Writer.Status(),
			"duration_ms", duration.Milliseconds(),
			"remote_ip", clientIP(c),
			"user_agent", c.Request.UserAgent(),
		)
	}
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

func clientIP(c *gin.Context) string {
	if forwarded := c.GetHeader("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := c.GetHeader("X-Real-IP"); realIP != "" {
		return realIP
	}
	return c.ClientIP()
}