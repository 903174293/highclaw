package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/highclaw/highclaw/internal/agent"
	skillapp "github.com/highclaw/highclaw/internal/application/skill"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/infra"
	"github.com/highclaw/highclaw/internal/interfaces/http"
	"github.com/spf13/cobra"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the HighClaw gateway server",
	Long: `Start the WebSocket + HTTP gateway control plane.

The gateway is the central hub for all messaging channels, AI agent
communication, session management, and device node connections.

Default: ws://127.0.0.1:18789`,
	RunE: runGateway,
}

var (
	gatewayPort    int
	gatewayBind    string
	gatewayVerbose bool
	gatewayForce   bool
	gatewayReset   bool
	gatewayDev     bool
)

func init() {
	gatewayCmd.Flags().IntVarP(&gatewayPort, "port", "p", 18789, "Gateway listen port")
	gatewayCmd.Flags().StringVar(&gatewayBind, "bind", "loopback", "Bind mode: loopback or all")
	gatewayCmd.Flags().BoolVarP(&gatewayVerbose, "verbose", "v", false, "Enable verbose logging")
	gatewayCmd.Flags().BoolVar(&gatewayForce, "force", false, "Force start even if port is in use")
	gatewayCmd.Flags().BoolVar(&gatewayReset, "reset", false, "Reset state before starting")
	gatewayCmd.Flags().BoolVar(&gatewayDev, "dev", false, "Enable development mode")
}

func runGateway(cmd *cobra.Command, args []string) error {
	// Setup structured logger.
	level := slog.LevelInfo
	if gatewayVerbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Print banner.
	infra.PrintBanner(version)

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("config load warning, using defaults", "error", err)
		cfg = config.Default()
	}

	// Override config with CLI flags.
	if cmd.Flags().Changed("port") {
		if gatewayPort == 0 {
			randomPort, pickErr := pickRandomPort()
			if pickErr != nil {
				return pickErr
			}
			cfg.Gateway.Port = randomPort
		} else {
			cfg.Gateway.Port = gatewayPort
		}
	}
	if cmd.Flags().Changed("bind") {
		cfg.Gateway.Bind = gatewayBind
	}

	slog.Info("starting HighClaw gateway",
		"version", version,
		"port", cfg.Gateway.Port,
		"bind", cfg.Gateway.Bind,
		"dev", gatewayDev,
	)

	// Create agent runner, session manager, skill manager, and log buffer
	runner := agent.NewRunner(cfg, logger)
	sessions := session.NewManager()
	skills := skillapp.NewManager(cfg, logger)
	logBuffer := http.NewLogBuffer(200)

	// Wrap logger with log buffer handler
	bufHandler := http.NewLogBufferHandler(logger.Handler(), logBuffer)
	logger = slog.New(bufHandler)
	slog.SetDefault(logger)

	// Create HTTP/Web server (which includes WebSocket gateway)
	httpServer := http.NewServer(cfg, logger, runner, sessions, skills, logBuffer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server in background
	go func() {
		if err := httpServer.Start(ctx); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	slog.Info("🦀 HighClaw gateway ready", "port", cfg.Gateway.Port)
	slog.Info("🌐 Web UI ready", "url", fmt.Sprintf("http://localhost:%d", cfg.Gateway.Port))

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	slog.Info("received shutdown signal", "signal", sig)
	cancel() // Cancel context to shutdown HTTP server

	return nil
}

func pickRandomPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("pick random port: %w", err)
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, fmt.Errorf("pick random port: invalid listener address")
	}
	return addr.Port, nil
}
