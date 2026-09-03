package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"projectYandexLyceumFinal/internal/handlers"
	"projectYandexLyceumFinal/internal/middleware"
	"projectYandexLyceumFinal/internal/utils"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("auth-service stopped: %v", err)
	}
}

func run() error {
	dsn := env("AUTH_DB_DSN", "postgres://postgres:postgres@localhost:5432/delivery?sslmode=disable")
	port := env("AUTH_PORT", "8081")
	issuer := env("JWT_ISSUER", "delivery-auth")
	tokens, err := utils.NewTokenManager(os.Getenv("JWT_SECRET"), issuer, 12*time.Hour)
	if err != nil {
		return err
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(10 * time.Minute)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := db.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	authHandler := handlers.New(db, tokens)
	gin.SetMode(env("GIN_MODE", gin.ReleaseMode))
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), corsMiddleware(parseOrigins(env("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000"))))
	router.POST("/api/auth/register", authHandler.Register)
	router.POST("/api/auth/login", authHandler.Login)
	router.PATCH("/api/auth/profile", middleware.Auth(tokens), authHandler.UpdateProfile)
	router.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("auth-service listening on :%s", port)
		serverErr <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		log.Println("auth-service shutdown requested")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
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

func parseOrigins(value string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func corsMiddleware(allowed map[string]struct{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin is not allowed"})
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
