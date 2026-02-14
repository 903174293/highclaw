package http

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// loggerMiddleware logs HTTP requests.
func loggerMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()

		logger.Info("http request",
			"method", method,
			"path", path,
			"status", status,
			"duration", duration,
			"ip", c.ClientIP(),
		)
	}
}

// corsMiddleware handles CORS.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// authMiddleware checks authentication.
func authMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader != "Bearer "+token {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		c.Next()
	}
}

