// Package http provides the HTTP interface for internal gateway communication.
package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/highclaw/highclaw/internal/config"
)

// Server provides internal HTTP endpoints (health check + channel reload + channel status).
type Server struct {
	router    *gin.Engine
	cfg       *config.Config
	logger    *slog.Logger
	logBuffer *LogBuffer
	startedAt time.Time

	reloadChannels   ReloadChannelsFunc
	getChannelStatus GetChannelStatusFunc
}

// ChannelReloadResult describes the result of a channel reload.
type ChannelReloadResult struct {
	Reloaded []string                 `json:"reloaded"`
	Channels map[string]ChannelStatus `json:"channels"`
}

// ChannelStatus 描述单个 channel 的运行时状态
type ChannelStatus struct {
	Status   string `json:"status"`
	BindCode string `json:"bindCode,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ChannelStatusResult 是 channel-status API 的返回体
type ChannelStatusResult struct {
	Channels map[string]ChannelStatus `json:"channels"`
}

// ReloadChannelsFunc is injected by the gateway to perform channel reload.
type ReloadChannelsFunc func(ctx context.Context) (*ChannelReloadResult, error)

// GetChannelStatusFunc 由 gateway 注入，返回各 channel 的运行时状态
type GetChannelStatusFunc func() *ChannelStatusResult

// NewServer creates an HTTP server with health + internal endpoints.
func NewServer(cfg *config.Config, logger *slog.Logger, logBuffer *LogBuffer) *Server {
	if cfg.Gateway.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggerMiddleware(logger))

	s := &Server{
		router:    router,
		cfg:       cfg,
		logger:    logger,
		logBuffer: logBuffer,
		startedAt: time.Now(),
	}

	s.setupRoutes()
	return s
}

// setupRoutes 注册 health + internal 路由
func (s *Server) setupRoutes() {
	s.router.GET("/health", s.handleHealth)
	s.router.GET("/api/health", s.handleHealth)

	internal := s.router.Group("/api/internal")
	internal.Use(localhostOnlyMiddleware())
	{
		internal.POST("/reload", s.handleChannelsReload)
		internal.GET("/channel-status", s.handleChannelStatus)
	}
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

	select {
	case err := <-listenErr:
		return fmt.Errorf("gateway failed to start: %w\n  -> Is another HighClaw instance running on %s?", err, addr)
	case <-time.After(200 * time.Millisecond):
	}

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

// SetReloadChannels injects the channel reload callback.
func (s *Server) SetReloadChannels(fn ReloadChannelsFunc) {
	s.reloadChannels = fn
}

// SetGetChannelStatus 注入 channel 运行时状态查询回调
func (s *Server) SetGetChannelStatus(fn GetChannelStatusFunc) {
	s.getChannelStatus = fn
}
