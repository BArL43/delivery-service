package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"order-service/internal/handlers"
	"order-service/internal/observability"
	"order-service/internal/pricing"
	"order-service/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	metrics := observability.NewCollector()
	observability.SetLogger(logger)
	observability.SetCollector(metrics)
	// 1. Database Connection
	connStr := getEnv("ORDER_DB_DSN", "postgres://postgres:postgres@localhost:5432/delivery?sslmode=disable")
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		logger.Error("database_connection_failed", "service", "order-service", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		logger.Error("database_ping_failed", "service", "order-service", "error", err)
		os.Exit(1)
	}
	logger.Info("database_connected", "service", "order-service")

	// 2. Pricing Calculator (from env vars)
	pricingCfg := pricing.LoadConfig()
	priceCalc := pricing.NewCalculator(pricingCfg)
	logger.Info("pricing_config_loaded",
		"service", "order-service",
		"base_rate", pricingCfg.BaseRate,
		"per_km_rate", pricingCfg.PerKmRate,
		"per_kg_rate", pricingCfg.PerKgRate,
	)

	// 3. Dependency Injection
	orderRepo := storage.NewPostgresOrderRepository(pool)
	ordersHandler := handlers.NewOrdersHandler(orderRepo, priceCalc)

	// 4. Courier dependencies
	courierRepo := storage.NewPostgresCourierRepository(pool)
	assignmentRepo := storage.NewPostgresAssignmentRepository(pool)
	courierHandler := handlers.NewCourierHandler(courierRepo, assignmentRepo)

	// 5. Routing (using Go 1.22+ enhanced mux)
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", observability.Handler())
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Mapping handlers to endpoints
	mux.HandleFunc("POST /orders", ordersHandler.CreateOrder)
	mux.HandleFunc("GET /orders", ordersHandler.ListOrders)
	mux.HandleFunc("GET /orders/{id}", ordersHandler.GetOrder)

	// Courier routes
	mux.HandleFunc("POST /api/v1/couriers/register", courierHandler.RegisterCourier)
	mux.HandleFunc("GET /api/v1/couriers/by-email", courierHandler.GetCourierByEmail)
	mux.HandleFunc("POST /api/v1/couriers/availability", courierHandler.ToggleAvailability)
	mux.HandleFunc("POST /api/v1/couriers/location", courierHandler.UpdateLocation)
	mux.HandleFunc("POST /api/v1/orders/{orderId}/assign", courierHandler.AssignOrder)
	mux.HandleFunc("GET /api/v1/couriers/{courierId}/active-order", courierHandler.GetActiveOrder)

	// 6. Server Setup with Graceful Shutdown
	server := &http.Server{
		Addr:    ":8080",
		Handler: observability.Middleware(mux),
	}

	// Create a channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		logger.Info("server_starting", "service", "order-service", "addr", ":8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_failed", "service", "order-service", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-stop
	logger.Info("server_shutdown_start", "service", "order-service")

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server_shutdown_failed", "service", "order-service", "error", err)
		os.Exit(1)
	}

	logger.Info("server_exited", "service", "order-service")
}
