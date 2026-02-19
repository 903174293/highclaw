package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/highclaw/highclaw/internal/agent"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/infra"
	"github.com/highclaw/highclaw/internal/interfaces/http"
	syslogger "github.com/highclaw/highclaw/internal/system/logger"
	"github.com/highclaw/highclaw/internal/system/tasklog"
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

// initFileLogger 根据配置初始化文件日志管理器，返回 slog.Logger
func initFileLogger(cfg *config.Config, verbose bool) (*slog.Logger, *syslogger.Manager) {
	level := parseSlogLevel(cfg.Log.Level)
	if verbose {
		level = slog.LevelDebug
	}

	stderrEnabled := true
	if cfg.Log.StderrEnabled != nil {
		stderrEnabled = *cfg.Log.StderrEnabled
	}

	logCfg := syslogger.Config{
		Dir:           cfg.Log.Dir,
		Level:         level,
		MaxAgeDays:    cfg.Log.MaxAgeDays,
		MaxSizeMB:     cfg.Log.MaxSizeMB,
		StderrEnabled: stderrEnabled,
	}

	mgr, err := syslogger.New(logCfg)
	if err != nil {
		fallback := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
		fallback.Warn("file logger init failed, using stderr only", "error", err)
		return fallback, nil
	}

	logger := mgr.NewLogger()
	return logger, mgr
}

// initTaskLog 根据配置初始化任务日志
func initTaskLog(cfg *config.Config) *tasklog.Store {
	enabled := true
	if cfg.TaskLog.Enabled != nil {
		enabled = *cfg.TaskLog.Enabled
	}
	if !enabled {
		return nil
	}
	tlCfg := tasklog.Config{
		Dir:        cfg.TaskLog.Dir,
		MaxAgeDays: cfg.TaskLog.MaxAgeDays,
		MaxRecords: cfg.TaskLog.MaxRecords,
		Enabled:    true,
	}
	store, err := tasklog.NewStore(tlCfg)
	if err != nil {
		slog.Warn("tasklog init failed", "error", err)
		return nil
	}
	return store
}

func parseSlogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func runGateway(cmd *cobra.Command, args []string) error {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("config load warning, using defaults", "error", err)
		cfg = config.Default()
	}

	// 初始化文件日志
	logger, logMgr := initFileLogger(cfg, gatewayVerbose)
	if logMgr != nil {
		defer logMgr.Close()
	}
	slog.SetDefault(logger)

	// 初始化任务日志
	taskStore = initTaskLog(cfg)
	if taskStore != nil {
		defer taskStore.Close()
		logger.Info("task log initialized", "db", taskStore.DBPath())
		// 记录系统启动事件
		_ = taskStore.Log(&tasklog.TaskRecord{
			Action: tasklog.ActionSystem,
			Module: "gateway",
			RequestBody: "gateway start",
			Status: "success",
		})
	}

	// Print banner.
	infra.PrintBanner(version)

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
	logBuffer := http.NewLogBuffer(200)

	// Wrap logger with log buffer handler
	bufHandler := http.NewLogBufferHandler(logger.Handler(), logBuffer)
	logger = slog.New(bufHandler)
	slog.SetDefault(logger)

	// Create HTTP/Web server (which includes WebSocket gateway)
	httpServer := http.NewServer(cfg, logger, runner, sessions, logBuffer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server in background
	go func() {
		if err := httpServer.Start(ctx); err != nil {
			slog.Error("HTTP server error", "error", err)
		}
	}()

	slog.Info("HighClaw gateway ready", "port", cfg.Gateway.Port)
	slog.Info("Web UI ready", "url", fmt.Sprintf("http://localhost:%d", cfg.Gateway.Port))
	if logMgr != nil {
		slog.Info("log files", "dir", logMgr.LogDir(), "file", logMgr.CurrentLogFile())
	}

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	slog.Info("received shutdown signal", "signal", sig)

	// 记录系统停止事件
	if taskStore != nil {
		_ = taskStore.Log(&tasklog.TaskRecord{
			Action: tasklog.ActionSystem,
			Module: "gateway",
			RequestBody: fmt.Sprintf("gateway shutdown by signal %v", sig),
			Status: "success",
		})
	}

	cancel()
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
