package http

import (
	"log/slog"
	"net/http"
	"strings"
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

// webAuthMiddleware enforces browser BasicAuth for web/API traffic when enabled.
func (s *Server) webAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.requiresWebLogin() {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		// Keep health/static endpoints unauthenticated to avoid repeated browser prompts
		// and broken icons/assets during initial page load.
		if path == "/health" || path == "/api/health" || strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}
		if !s.validateBasicAuth(c) {
			s.requestBasicAuth(c)
			c.Abort()
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

// authMiddleware enforces gateway bearer authentication when pairing/token mode is enabled.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.requiresWebLogin() {
			if !s.validateBasicAuth(c) {
				s.requestBasicAuth(c)
				c.Abort()
				return
			}
			// When web basic auth is enabled and validated, do not require extra bearer pairing token
			// for browser/API calls.
			c.Next()
			return
		}

		if s.pairing == nil || !s.pairing.RequireAuth() {
			c.Next()
			return
		}

		clientKey := c.ClientIP()
		if clientKey == "" {
			clientKey = "unknown"
		}
		if s.apiRateLimiter != nil && !s.apiRateLimiter.Allow(clientKey) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if !s.pairing.IsAuthenticated(token) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (s *Server) validateBasicAuth(c *gin.Context) bool {
	if !s.requiresWebLogin() {
		return true
	}
	user, pass, ok := c.Request.BasicAuth()
	if !ok {
		return false
	}
	cfgAuth := s.cfg.Web.Auth
	return secureEquals(strings.TrimSpace(user), strings.TrimSpace(cfgAuth.Username)) && secureEquals(pass, cfgAuth.Password)
}

func (s *Server) requestBasicAuth(c *gin.Context) {
	c.Writer.Header().Set("WWW-Authenticate", `Basic realm="HighClaw Console", charset="UTF-8"`)
	c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
}
