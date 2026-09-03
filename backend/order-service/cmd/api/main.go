package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"order-service/internal/geocoder"
	"order-service/internal/handlers"
	"order-service/internal/middleware"
	"order-service/internal/observability"
	"order-service/internal/pricing"
	"order-service/internal/routing"
	"order-service/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("order-service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	metrics := observability.NewCollector()
	observability.SetLogger(logger)
	observability.SetCollector(metrics)

	auth, err := middleware.NewAuthenticator(os.Getenv("JWT_SECRET"), env("JWT_ISSUER", "delivery-auth"))
	if err != nil {
		return err
	}

	poolConfig, err := pgxpool.ParseConfig(env("ORDER_DB_DSN", "postgres://postgres:postgres@localhost:5432/delivery?sslmode=disable"))
	if err != nil {
		return err
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2
	poolConfig.MaxConnIdleTime = 15 * time.Minute
	poolConfig.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return err
	}
	defer pool.Close()
	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()
	if err := pool.Ping(pingCtx); err != nil {
		return err
	}

	var redisClient *redis.Client
	var geoCache geocoder.GeocodeCache
	if redisAddr := strings.TrimSpace(os.Getenv("REDIS_ADDR")); redisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: redisAddr})
		cacheCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := redisClient.Ping(cacheCtx).Err(); err != nil {
			logger.Warn("redis_unavailable_geocoding_will_bypass_cache", "error", err)
			_ = redisClient.Close()
			redisClient = nil
		} else {
			geoCache = geocoder.NewRedisGeocodeCache(redisClient)
		}
		cancel()
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	priceCalc := pricing.NewCalculator(pricing.LoadConfig())
	orderRepo := storage.NewPostgresOrderRepository(pool)
	ordersHandler := handlers.NewOrdersHandler(orderRepo, priceCalc)
	courierRepo := storage.NewPostgresCourierRepository(pool)
	assignmentRepo := storage.NewPostgresAssignmentRepository(pool)
	courierHandler := handlers.NewCourierHandler(courierRepo, assignmentRepo, orderRepo)

	osm := geocoder.NewOSMProvider()
	geoHandler := geocoder.NewGeocodeHandler(geocoder.NewService(geoCache, osm, nil))
	routeHandler := routing.NewHandler(env("OSRM_BASE_URL", "https://router.project-osrm.org"))

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", observability.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		healthCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(healthCtx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/v1/geocode", geoHandler.GeocodeAddress)
	mux.HandleFunc("GET /api/v1/geocode/suggest", geoHandler.Suggest)
	mux.HandleFunc("POST /api/v1/geocode/reverse", geoHandler.ReverseGeocode)
	mux.HandleFunc("GET /api/geocode", geoHandler.GeocodeAddress)
	mux.HandleFunc("GET /api/geocode/suggest", geoHandler.Suggest)
	mux.HandleFunc("POST /api/geocode/reverse", geoHandler.ReverseGeocode)
	mux.HandleFunc("GET /api/route", routeHandler.Route)

	protect := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, auth.Require(fn))
	}
	protect("POST /orders", ordersHandler.CreateOrder)
	protect("GET /orders", ordersHandler.ListOrders)
	protect("GET /orders/{id}", ordersHandler.GetOrder)
	protect("POST /api/v1/orders", ordersHandler.CreateOrder)
	protect("GET /api/v1/orders", ordersHandler.ListOrders)
	protect("GET /api/v1/orders/{id}", ordersHandler.GetOrder)
	protect("POST /api/v1/couriers/register", courierHandler.RegisterCourier)
	protect("GET /api/v1/couriers/by-email", courierHandler.GetCourierByEmail)
	protect("POST /api/v1/couriers/availability", courierHandler.ToggleAvailability)
	protect("POST /api/v1/couriers/location", courierHandler.UpdateLocation)
	protect("POST /api/v1/orders/{orderId}/assign", courierHandler.AssignOrder)
	protect("PATCH /api/v1/orders/{orderId}/status", courierHandler.UpdateOrderStatus)
	protect("GET /api/v1/couriers/{courierId}/active-order", courierHandler.GetActiveOrder)

	server := &http.Server{
		Addr:              ":" + env("ORDER_PORT", "8080"),
		Handler:           observability.Middleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server_starting", "service", "order-service", "addr", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		logger.Info("server_shutdown_requested", "service", "order-service")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
