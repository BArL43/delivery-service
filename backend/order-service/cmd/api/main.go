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
	"order-service/internal/models"
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

	if err := ensureCourierSchema(ctx, pool); err != nil {
		logger.Error("courier_schema_ensure_failed", "service", "order-service", "error", err)
		os.Exit(1)
	}
	logger.Info("courier_schema_ready", "service", "order-service")

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
	if err := seedDemoOrders(ctx, orderRepo, priceCalc); err != nil {
		logger.Error("demo_orders_seed_failed", "service", "order-service", "error", err)
		os.Exit(1)
	}
	logger.Info("demo_orders_ready", "service", "order-service")
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

func ensureCourierSchema(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,
		`CREATE TABLE IF NOT EXISTS orders (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL,
		from_address JSONB NOT NULL,
		to_address JSONB NOT NULL,
		weight DECIMAL(10,2) NOT NULL DEFAULT 0,
		price DECIMAL(12,2) NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'created',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS weight DECIMAL(10,2) NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS couriers (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id         UUID NOT NULL UNIQUE,
		email           TEXT NOT NULL UNIQUE,
		full_name       TEXT NOT NULL,
		phone           TEXT NOT NULL UNIQUE,
		transport_type  TEXT NOT NULL DEFAULT 'bicycle',
		is_online       BOOLEAN NOT NULL DEFAULT FALSE,
		active_order_id UUID REFERENCES orders(id),
		current_lat     DECIMAL(10,8),
		current_lon     DECIMAL(11,8),
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`CREATE TABLE IF NOT EXISTS courier_locations (
		id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		courier_id  UUID NOT NULL REFERENCES couriers(id) ON DELETE CASCADE,
		lat         DECIMAL(10,8) NOT NULL,
		lon         DECIMAL(11,8) NOT NULL,
		accuracy    DECIMAL(8,2),
		recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`CREATE TABLE IF NOT EXISTS courier_shifts (
		id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		courier_id  UUID NOT NULL REFERENCES couriers(id) ON DELETE CASCADE,
		started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		ended_at    TIMESTAMPTZ,
		is_active   BOOLEAN NOT NULL DEFAULT TRUE
)`,
		`CREATE TABLE IF NOT EXISTS assignments (
		id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id        UUID NOT NULL REFERENCES orders(id),
		courier_id      UUID NOT NULL REFERENCES couriers(id),
		status          TEXT NOT NULL DEFAULT 'assigned',
		eta_to_pickup   INTERVAL,
		picked_up_at    TIMESTAMPTZ,
		delivered_at    TIMESTAMPTZ,
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_assignments_order_active
		ON assignments (order_id)
		WHERE status IN ('assigned', 'accepted', 'at_pickup', 'in_progress')`,
		`CREATE INDEX IF NOT EXISTS idx_couriers_location
		ON couriers (current_lat, current_lon)
		WHERE current_lat IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_courier_active
		ON assignments (courier_id)
		WHERE status IN ('assigned', 'accepted', 'at_pickup', 'in_progress')`,
	}

	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

func seedDemoOrders(ctx context.Context, orderRepo storage.OrderRepository, priceCalc *pricing.Calculator) error {
	existing, err := orderRepo.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}

	demoOrders := []struct {
		from     models.Address
		to       models.Address
		weight   float64
		distance float64
		status   string
	}{
		{
			from:   models.Address{City: "Москва", Street: "Тверская 14"},
			to:     models.Address{City: "Москва", Street: "Ленинский проспект 30"},
			weight: 1.5, distance: 5.2, status: "created",
		},
		{
			from:   models.Address{City: "Москва", Street: "Арбат 7"},
			to:     models.Address{City: "Москва", Street: "Парк Победы 1"},
			weight: 2.1, distance: 8.4, status: "SEARCHING_COURIER",
		},
		{
			from:   models.Address{City: "Москва", Street: "Проспект Мира 102"},
			to:     models.Address{City: "Москва", Street: "Кутузовский проспект 45"},
			weight: 0.8, distance: 11.7, status: "COURIER_ASSIGNED",
		},
		{
			from:   models.Address{City: "Москва", Street: "Садовая-Самотечная 7"},
			to:     models.Address{City: "Москва", Street: "Новая Басманная 12"},
			weight: 3.0, distance: 4.6, status: "PICKED_UP",
		},
	}

	for _, item := range demoOrders {
		price := priceCalc.Calculate(item.distance, item.weight)
		order := models.NewOrder("00000000-0000-0000-0000-000000000000", item.from, item.to, item.weight, price)
		order.Status = item.status
		if err := orderRepo.Create(ctx, order); err != nil {
			return err
		}
	}

	return nil
}
