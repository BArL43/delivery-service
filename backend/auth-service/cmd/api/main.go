package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"projectYandexLyceumFinal/internal/handlers"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	dsn := getEnv("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	port := getEnv("AUTH_PORT", "8081")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open db connection: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	handlers.SetDB(db)

	r := gin.Default()
	r.Use(corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/auth/register", handlers.Register)
	r.POST("/api/auth/login", handlers.Login)

	log.Printf("auth-service listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}

}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}

}
