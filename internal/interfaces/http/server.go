// Package http provides the HTTP/Web interface.
package http

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/highclaw/highclaw/internal/config"
)

//go:embed static/*
var staticFiles embed.FS

// Server represents the HTTP/WebSocket server.
type Server struct {
	router   *gin.Engine
	cfg      *config.Config
	logger   *slog.Logger
	upgrader websocket.Upgrader
}

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, logger *slog.Logger) *Server {
	// Set Gin mode
	if cfg.Gateway.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware(logger))
	router.Use(corsMiddleware())

	s := &Server{
		router: router,
		cfg:    cfg,
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // TODO: Implement proper origin checking
			},
		},
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", s.handleHealth)
	s.router.GET("/api/health", s.handleHealth)

	// API routes
	api := s.router.Group("/api")
	{
		api.GET("/status", s.handleStatus)
		api.GET("/config", s.handleGetConfig)
		api.PATCH("/config", s.handlePatchConfig)

		// Sessions
		api.GET("/sessions", s.handleListSessions)
		api.GET("/sessions/:key", s.handleGetSession)
		api.POST("/sessions", s.handleCreateSession)
		api.DELETE("/sessions/:key", s.handleDeleteSession)
		api.PATCH("/sessions/:key", s.handlePatchSession)

		// Chat
		api.POST("/chat", s.handleChat)

		// Channels
		api.GET("/channels", s.handleListChannels)
		api.GET("/channels/status", s.handleChannelsStatus)

		// Models
		api.GET("/models", s.handleListModels)
		api.GET("/providers", s.handleListProviders)
	}

	// WebSocket
	s.router.GET("/ws", s.handleWebSocket)

	// Static files (Web UI)
	s.serveStaticFiles()
}

// serveStaticFiles serves the Web UI static files.
func (s *Server) serveStaticFiles() {
	// Serve static files from embedded FS
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		s.logger.Error("failed to create static FS", "error", err)
		return
	}

	// Serve static files
	s.router.StaticFS("/static", http.FS(staticFS))

	// Serve app.html as the main UI
	s.router.GET("/", func(c *gin.Context) {
		data, err := staticFiles.ReadFile("static/app.html")
		if err != nil {
			c.String(http.StatusNotFound, "UI not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// Fallback for SPA routes
	s.router.NoRoute(func(c *gin.Context) {
		// If it's an API route, return 404
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Otherwise serve the app
		data, err := staticFiles.ReadFile("static/app.html")
		if err != nil {
			c.String(http.StatusNotFound, "UI not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

// Start starts the HTTP server.
func (s *Server) Start(ctx context.Context) error {
	addr := s.getListenAddr()
	s.logger.Info("starting HTTP server", "address", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.logger.Info("shutting down HTTP server")
	return srv.Shutdown(shutdownCtx)
}

// getListenAddr returns the listen address based on configuration.
func (s *Server) getListenAddr() string {
	port := s.cfg.Gateway.Port
	if port == 0 {
		port = 18790
	}

	switch s.cfg.Gateway.Bind {
	case "loopback":
		return fmt.Sprintf("127.0.0.1:%d", port)
	case "all":
		return fmt.Sprintf("0.0.0.0:%d", port)
	case "tailnet":
		// TODO: Get Tailscale IP
		return fmt.Sprintf("0.0.0.0:%d", port)
	default:
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
}
