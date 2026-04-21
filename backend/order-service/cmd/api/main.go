package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"order-service/internal/handlers"
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
	// 1. Database Connection
	connStr := getEnv("ORDER_DB_DSN", "postgres://postgres:postgres@localhost:5432/delivery?sslmode=disable")
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Could not ping database: %v\n", err)
	}
	log.Println("Successfully connected to PostgreSQL")

	// 2. Pricing Calculator (from env vars)
	pricingCfg := pricing.LoadConfig()
	priceCalc := pricing.NewCalculator(pricingCfg)
	log.Printf("Pricing config: base=%.0f, per_km=%.0f, per_kg=%.0f",
		pricingCfg.BaseRate, pricingCfg.PerKmRate, pricingCfg.PerKgRate)

	// 3. Dependency Injection
	orderRepo := storage.NewPostgresOrderRepository(pool)
	ordersHandler := handlers.NewOrdersHandler(orderRepo, priceCalc)

	// 4. Courier dependencies
	courierRepo := storage.NewPostgresCourierRepository(pool)
	assignmentRepo := storage.NewPostgresAssignmentRepository(pool)
	courierHandler := handlers.NewCourierHandler(courierRepo, assignmentRepo)

	// 5. Routing (using Go 1.22+ enhanced mux)
	mux := http.NewServeMux()

	// Mapping handlers to endpoints
	mux.HandleFunc("POST /orders", ordersHandler.CreateOrder)
	mux.HandleFunc("GET /orders", ordersHandler.ListOrders)
	mux.HandleFunc("GET /orders/{id}", ordersHandler.GetOrder)

	// Courier routes
	mux.HandleFunc("POST /api/v1/couriers/availability", courierHandler.ToggleAvailability)
	mux.HandleFunc("POST /api/v1/couriers/location", courierHandler.UpdateLocation)
	mux.HandleFunc("POST /api/v1/orders/{orderId}/assign", courierHandler.AssignOrder)
	mux.HandleFunc("GET /api/v1/couriers/{courierId}/active-order", courierHandler.GetActiveOrder)

	// 6. Server Setup with Graceful Shutdown
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Create a channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting API Gateway server on :8080\n")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Create a context with timeout for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
