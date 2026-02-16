// Package http provides the HTTP/Web interface.
package http

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/highclaw/highclaw/internal/agent"
	skillapp "github.com/highclaw/highclaw/internal/application/skill"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/security"
)

//go:embed static/*
var staticFiles embed.FS

// Server represents the HTTP/WebSocket server.
type Server struct {
	router    *gin.Engine
	cfg       *config.Config
	logger    *slog.Logger
	upgrader  websocket.Upgrader
	agent     *agent.Runner
	sessions  *session.Manager
	skills    *skillapp.Manager
	logBuffer *LogBuffer
	startedAt time.Time

	pairing         *security.PairingGuard
	pairRateLimiter *security.SlidingWindowLimiter
	apiRateLimiter  *security.SlidingWindowLimiter

	idempotencyMu sync.Mutex
	idempotency   map[string]time.Time
}

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, logger *slog.Logger, runner *agent.Runner, sessions *session.Manager, skills *skillapp.Manager, logBuffer *LogBuffer) *Server {
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
		router:    router,
		cfg:       cfg,
		logger:    logger,
		agent:     runner,
		sessions:  sessions,
		skills:    skills,
		logBuffer: logBuffer,
		startedAt: time.Now(),
		pairing: security.NewPairingGuard(
			cfg.Gateway.Auth.Mode != "none",
			cfg.Gateway.Auth.Token,
		),
		pairRateLimiter: security.NewSlidingWindowLimiter(10, time.Minute),
		apiRateLimiter:  security.NewSlidingWindowLimiter(120, time.Minute),
		idempotency:     map[string]time.Time{},
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimSpace(r.Header.Get("Origin"))
				if origin == "" {
					return true
				}
				host := strings.TrimSpace(r.Host)
				if host == "" {
					return false
				}
				if strings.Contains(origin, "://"+host) {
					return true
				}
				if strings.Contains(origin, "://127.0.0.1") || strings.Contains(origin, "://localhost") {
					return true
				}
				return false
			},
		},
	}

	s.setupRoutes()
	if code := s.pairing.PairingCode(); code != "" {
		s.logger.Warn("gateway pairing required", "code", code)
	}
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
		api.POST("/pair", s.handlePair)
		api.GET("/pairing", s.handlePairingStatus)

		protected := api.Group("/")
		protected.Use(s.authMiddleware())
		{
			protected.GET("/config", s.handleGetConfig)
			protected.PATCH("/config", s.handlePatchConfig)

			// Sessions
			protected.GET("/sessions", s.handleListSessions)
			protected.GET("/sessions/:key", s.handleGetSession)
			protected.POST("/sessions", s.handleCreateSession)
			protected.DELETE("/sessions/:key", s.handleDeleteSession)
			protected.PATCH("/sessions/:key", s.handlePatchSession)

			// Chat
			protected.POST("/chat", s.handleChat)

			// Channels
			protected.GET("/channels", s.handleListChannels)
			protected.GET("/channels/status", s.handleChannelsStatus)

			// Models
			protected.GET("/models", s.handleListModels)
			protected.GET("/providers", s.handleListProviders)

			// Skills
			protected.GET("/skills", s.handleListSkills)

			// Runtime & Logs
			protected.GET("/runtime/stats", s.handleRuntimeStats)
			protected.GET("/logs", s.handleLogs)
		}
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
		if ip := findTailnetIP(); ip != "" {
			return fmt.Sprintf("%s:%d", ip, port)
		}
		return fmt.Sprintf("0.0.0.0:%d", port)
	default:
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
}

func findTailnetIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if !strings.Contains(strings.ToLower(iface.Name), "tailscale") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}
