package main

import (
	"context"
	"log"
	"net/http"

	"api-gateway/internal/handler"
	"api-gateway/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	// 1. Database Connection
	connStr := "postgres://postgres:postgres@localhost:5432/postgres"
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

	// 4. Server Start
	port := ":8080"
	log.Printf("Starting API Gateway server on %s\n", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Server failed: %v\n", err)
	}
}
