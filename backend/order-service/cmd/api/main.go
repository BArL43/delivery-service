package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"order-service/internal/geocoder"
	"order-service/internal/handlers"
	"order-service/internal/pricing"
	"order-service/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// 1. Database Connection (Postgres)
	connStr := "postgres://postgres:postgres@localhost:45432/postgres?sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Could not ping database: %v\n", err)
	}
	log.Println("Successfully connected to PostgreSQL")

	// 1.5. Database Connection (Redis)
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Could not ping Redis: %v\n", err)
	}
	defer rdb.Close()
	log.Println("Successfully connected to Redis")

	// 2. Pricing Calculator (from env vars)
	pricingCfg := pricing.LoadConfig()
	priceCalc := pricing.NewCalculator(pricingCfg)
	log.Printf("Pricing config: base=%.0f, per_km=%.0f, per_kg=%.0f",
		pricingCfg.BaseRate, pricingCfg.PerKmRate, pricingCfg.PerKgRate)

	// 3. Dependency Injection (Orders)
	orderRepo := storage.NewPostgresOrderRepository(pool)
	ordersHandler := handlers.NewOrdersHandler(orderRepo, priceCalc)

	// 3.5. Dependency Injection (Geocoder)
	geoCache := geocoder.NewRedisGeocodeCache(rdb)
	osmProvider := geocoder.NewOSMProvider()
	geoService := geocoder.NewService(geoCache, osmProvider, osmProvider)
	geoHandler := geocoder.NewGeocodeHandler(geoService)

	// 4. Routing (using Go 1.22+ enhanced mux)
	mux := http.NewServeMux()

	// Mapping handlers to endpoints (Orders)
	// Я добавил префикс /api/v1/ как было в твоем изначальном ТЗ,
	// если хочешь оставить просто /orders — смело стирай /api/v1
	mux.HandleFunc("POST /api/v1/orders", ordersHandler.CreateOrder)
	mux.HandleFunc("GET /api/v1/orders", ordersHandler.ListOrders)
	mux.HandleFunc("GET /api/v1/orders/{id}", ordersHandler.GetOrder)
	mux.HandleFunc("PATCH /api/v1/orders/{orderId}/status", ordersHandler.UpdateOrderStatus) // ---> НОВОЕ: Смена статуса

	// Mapping handlers to endpoints (Geocoder)
	mux.HandleFunc("GET /api/v1/geocode", geoHandler.GeocodeAddress)
	mux.HandleFunc("GET /api/v1/geocode/suggest", geoHandler.Suggest)
	mux.HandleFunc("POST /api/v1/geocode/reverse", geoHandler.ReverseGeocode)

	// 5. Server Setup with Graceful Shutdown
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
