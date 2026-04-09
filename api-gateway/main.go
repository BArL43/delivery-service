package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-gateway/internal/handler"
	"api-gateway/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	// 1. Database Connection
	connStr := "postgres://postgres:postgres@localhost:45432/postgres?sslmode=disable"
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

	// 2. Dependency Injection
	orderRepo := repository.NewPostgresOrderRepository(pool)
	ordersHandler := handler.NewOrdersHandler(orderRepo)

	// 3. Routing (using Go 1.22+ enhanced mux)
	mux := http.NewServeMux()

	// Mapping handlers to endpoints
	mux.HandleFunc("POST /orders", ordersHandler.CreateOrder)
	mux.HandleFunc("GET /orders", ordersHandler.ListOrders)
	mux.HandleFunc("GET /orders/{id}", ordersHandler.GetOrder)

	// 4. Server Setup with Graceful Shutdown
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
