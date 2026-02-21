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
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/security"
)

//go:embed static/*
var staticFiles embed.FS

// Server represents the HTTP/WebSocket server.
type Server struct {
	router      *gin.Engine
	cfg         *config.Config
	configCache *config.Cache
	logger      *slog.Logger
	upgrader    websocket.Upgrader
	agent       *agent.Runner
	sessions    *session.Manager
	logBuffer   *LogBuffer
	startedAt   time.Time

	pairing         *security.PairingGuard
	pairRateLimiter *security.SlidingWindowLimiter
	apiRateLimiter  *security.SlidingWindowLimiter

	idempotencyMu sync.Mutex
	idempotency   map[string]time.Time

	webSessionMu sync.RWMutex
	webSessions  map[string]webSession

	reloadChannels ReloadChannelsFunc
}

// ChannelReloadResult 描述 channel reload 的结果
type ChannelReloadResult struct {
	Reloaded []string                    `json:"reloaded"`
	Channels map[string]ChannelStatus    `json:"channels"`
}

// ChannelStatus 单个 channel 的状态
type ChannelStatus struct {
	Status   string `json:"status"`
	BindCode string `json:"bindCode,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ReloadChannelsFunc 由 gateway 注入，执行实际的 channel 增量重载
type ReloadChannelsFunc func(ctx context.Context) (*ChannelReloadResult, error)

type webSession struct {
	Username  string
	ExpiresAt time.Time
}

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, logger *slog.Logger, runner *agent.Runner, sessions *session.Manager, logBuffer *LogBuffer) *Server {
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
		router:      router,
		cfg:         cfg,
		configCache: config.NewCache(cfg, 500*time.Millisecond),
		logger:      logger,
		agent:     runner,
		sessions:  sessions,
		logBuffer: logBuffer,
		startedAt: time.Now(),
		pairing: security.NewPairingGuard(
			cfg.Gateway.Auth.Mode != "none",
			cfg.Gateway.Auth.Token,
		),
		pairRateLimiter: security.NewSlidingWindowLimiter(10, time.Minute),
		apiRateLimiter:  security.NewSlidingWindowLimiter(120, time.Minute),
		idempotency:     map[string]time.Time{},
		webSessions:     map[string]webSession{},
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

	// Web console auth (browser-native Basic Auth dialog).
	router.Use(s.webAuthMiddleware())

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
		api.POST("/pair", s.handlePair)
		api.GET("/pairing", s.handlePairingStatus)
		api.POST("/auth/login", s.handleLogin)
		api.POST("/auth/logout", s.handleLogout)
		api.GET("/auth/me", s.handleAuthMe)

		// 仅限 localhost 的内部管理端点（免认证）
		internal := api.Group("/internal")
		internal.Use(localhostOnlyMiddleware())
		{
			internal.POST("/reload", s.handleChannelsReload)
		}

		protected := api.Group("/")
		protected.Use(s.authMiddleware())
		{
			protected.GET("/status", s.handleStatus)
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
			protected.POST("/channels/reload", s.handleChannelsReload)

			// Models
			protected.GET("/models", s.handleListModels)
			protected.GET("/providers", s.handleListProviders)

			// Skills
			protected.GET("/skills", s.handleListSkills)

			// Runtime & Logs
			protected.GET("/runtime/stats", s.handleRuntimeStats)
			protected.GET("/logs", s.handleLogs)
			protected.GET("/meta", s.handleMeta)
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

	listenErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	// 等待短暂窗口检测端口绑定是否成功（address already in use 等错误会立刻返回）
	select {
	case err := <-listenErr:
		return fmt.Errorf("gateway failed to start: %w\n  → Is another HighClaw instance running on %s?", err, addr)
	case <-time.After(200 * time.Millisecond):
		// 端口绑定成功，进入正常运行
	}

	// 正常运行阶段：等待 context 取消或运行时错误
	select {
	case err := <-listenErr:
		return fmt.Errorf("gateway runtime error: %w", err)
	case <-ctx.Done():
	}

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
