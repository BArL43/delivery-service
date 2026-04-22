package routes

import (
	"net/http"
	"projectYandexLyceumFinal/internal/handlers"
	"projectYandexLyceumFinal/internal/middleware"

	"github.com/gin-gonic/gin"
)

func NewRoute() *gin.Engine {
	router := gin.Default()

	router.Use(corsMiddleware())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"error": "ok"})
	})

	auth := router.Group("/api/auth")
	{
		auth.POST("api/auth/register", handlers.Register)
		auth.POST("/api/auth/login", handlers.Login)
	}

	protected := router.Group("/api/v1")
	protected.Use(middleware.AuthMiddleware())
	{

	}
	return router
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
