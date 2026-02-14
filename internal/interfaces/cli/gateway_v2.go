package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/highclaw/highclaw/internal/agent"
	skillapp "github.com/highclaw/highclaw/internal/application/skill"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/session"
	httpserver "github.com/highclaw/highclaw/internal/interfaces/http"
	"github.com/spf13/cobra"
)

var gatewayV2Cmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the HighClaw gateway (Gin + WebSocket)",
	Long:  "Starts the HTTP/WebSocket gateway server with Web UI support.",
	RunE:  runGatewayV2,
}

func init() {
	gatewayV2Cmd.Flags().IntP("port", "p", 0, "Gateway port (default: from config or 18790)")
	gatewayV2Cmd.Flags().String("bind", "", "Bind address: loopback, all, tailnet")
	gatewayV2Cmd.Flags().Bool("dev", false, "Development mode")
}

func runGatewayV2(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w (run 'highclaw onboard' first)", err)
	}

	// Override from flags
	if port, _ := cmd.Flags().GetInt("port"); port > 0 {
		cfg.Gateway.Port = port
	}
	if bind, _ := cmd.Flags().GetString("bind"); bind != "" {
		cfg.Gateway.Bind = bind
	}
	if dev, _ := cmd.Flags().GetBool("dev"); dev {
		cfg.Gateway.Mode = "development"
	}

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Print banner
	printBanner(cfg)

	// Create agent runner, session manager, skill manager, and log buffer
	runner := agent.NewRunner(cfg, logger)
	sessions := session.NewManager()
	skills := skillapp.NewManager(cfg, logger)
	logBuffer := httpserver.NewLogBuffer(200)

	// Wrap logger with log buffer handler
	bufHandler := httpserver.NewLogBufferHandler(logger.Handler(), logBuffer)
	logger = slog.New(bufHandler)

	// Create HTTP server
	server := httpserver.NewServer(cfg, logger, runner, sessions, skills, logBuffer)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	logger.Info("🦀 HighClaw gateway ready",
		"address", getAddress(cfg),
		"web", fmt.Sprintf("http://%s", getAddress(cfg)),
	)

	// Wait for signal or error
	select {
	case <-sigCh:
		logger.Info("received shutdown signal")
		cancel()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	logger.Info("gateway stopped")
	return nil
}

func printBanner(cfg *config.Config) {
	fmt.Println()
	fmt.Println("  🦀 HighClaw — Personal AI Assistant Gateway")
	fmt.Println("     version: v2026.2.13")
	fmt.Println("     runtime: go1.23 (Gin + WebSocket)")
	fmt.Println()
}

func getAddress(cfg *config.Config) string {
	port := cfg.Gateway.Port
	if port == 0 {
		port = 18790
	}

	switch cfg.Gateway.Bind {
	case "loopback":
		return fmt.Sprintf("127.0.0.1:%d", port)
	case "all":
		return fmt.Sprintf("0.0.0.0:%d", port)
	default:
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
}

