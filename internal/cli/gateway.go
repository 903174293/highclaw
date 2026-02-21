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
	"time"

	"github.com/highclaw/highclaw/internal/agent"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/infra"
	"github.com/highclaw/highclaw/internal/infrastructure/channels/feishu"
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

	// 飞书 channel 实例（gateway 级别，供 reload 引用）
	var feishuCh *feishu.FeishuChannel

	// 启动飞书 channel（长连接模式 + bind 验证码）
	if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.AppID != "" {
		feishuCh = startFeishuChannel(ctx, cfg, runner, sessions, logger)
	}

	// 注入 channel reload 回调
	httpServer.SetReloadChannels(func(reloadCtx context.Context) (*http.ChannelReloadResult, error) {
		return reloadChannels(reloadCtx, ctx, cfg, runner, sessions, logger, &feishuCh)
	})

	// 启动 HTTP server，检测端口绑定是否成功
	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	// 等待端口绑定结果（200ms 足够检测 address already in use）
	select {
	case err := <-serverErr:
		fmt.Fprintf(os.Stderr, "\n  ❌ %s\n\n", err)
		cancel()
		return err
	case <-time.After(300 * time.Millisecond):
	}

	slog.Info("HighClaw gateway ready", "port", cfg.Gateway.Port)
	if logMgr != nil {
		slog.Info("log files", "dir", logMgr.LogDir(), "file", logMgr.CurrentLogFile())
	}

	// Wait for shutdown or reload signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var lastSig os.Signal
	for {
		lastSig = <-sigCh
		if lastSig == syscall.SIGHUP {
			slog.Info("received SIGHUP, reloading channels")
			if result, err := reloadChannels(ctx, ctx, cfg, runner, sessions, logger, &feishuCh); err != nil {
				slog.Error("SIGHUP reload failed", "error", err)
			} else {
				slog.Info("SIGHUP reload complete", "reloaded", result.Reloaded)
			}
			continue
		}
		slog.Info("received shutdown signal", "signal", lastSig)
		break
	}

	// 记录系统停止事件
	if taskStore != nil {
		_ = taskStore.Log(&tasklog.TaskRecord{
			Action: tasklog.ActionSystem,
			Module: "gateway",
			RequestBody: fmt.Sprintf("gateway shutdown by signal %v", lastSig),
			Status: "success",
		})
	}

	cancel()
	return nil
}


// startFeishuChannel 创建并启动飞书 channel
func startFeishuChannel(
	ctx context.Context,
	cfg *config.Config,
	runner *agent.Runner,
	sessions *session.Manager,
	logger *slog.Logger,
) *feishu.FeishuChannel {
	ch := feishu.NewFeishuChannel(feishu.Config{
		AppID:        cfg.Channels.Feishu.AppID,
		AppSecret:    cfg.Channels.Feishu.AppSecret,
		VerifyToken:  cfg.Channels.Feishu.VerifyToken,
		EncryptKey:   cfg.Channels.Feishu.EncryptKey,
		AllowedUsers: cfg.Channels.Feishu.AllowedUsers,
		AllowedChats: cfg.Channels.Feishu.AllowedChats,
	}, logger)

	ch.SetMessageHandler(func(msgCtx context.Context, msg *feishu.ParsedMessage) (string, error) {
		return processFeishuMsg(msgCtx, cfg, runner, sessions, logger, msg)
	})

	if err := ch.Start(ctx); err != nil {
		slog.Error("feishu channel start failed", "error", err)
		return nil
	}
	slog.Info("feishu channel active (long connection mode)")
	return ch
}

// reloadChannels 重读配置，增量启停 channel（当前仅支持 Feishu）
func reloadChannels(
	reloadCtx context.Context,
	runCtx context.Context,
	cfg *config.Config,
	runner *agent.Runner,
	sessions *session.Manager,
	logger *slog.Logger,
	feishuChPtr **feishu.FeishuChannel,
) (*http.ChannelReloadResult, error) {
	newCfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("reload config: %w", err)
	}
	// 更新内存中的配置
	*cfg = *newCfg

	result := &http.ChannelReloadResult{
		Channels: make(map[string]http.ChannelStatus),
	}

	feishuCh := *feishuChPtr
	feishuCfg := newCfg.Channels.Feishu

	switch {
	case feishuCfg == nil || feishuCfg.AppID == "":
		// 配置已删除：停止现有 channel
		if feishuCh != nil {
			_ = feishuCh.Stop(reloadCtx)
			*feishuChPtr = nil
			result.Reloaded = append(result.Reloaded, "feishu:stopped")
		}
		result.Channels["feishu"] = http.ChannelStatus{Status: "disabled"}

	case feishuCh == nil:
		// 新增 channel：首次启动
		*feishuChPtr = startFeishuChannel(runCtx, cfg, runner, sessions, logger)
		ch := *feishuChPtr
		status := http.ChannelStatus{Status: "started"}
		if ch != nil && !ch.IsBound() {
			status.BindCode = ch.BindCode()
		}
		result.Reloaded = append(result.Reloaded, "feishu:started")
		result.Channels["feishu"] = status

	default:
		// 已有 channel：检查是否需要重连或仅更新白名单
		oldID := feishuCh.ID()
		needRestart := false

		// 核心连接参数变更 → 需要重启
		if feishuCfg.AppID != cfg.Channels.Feishu.AppID ||
			feishuCfg.AppSecret != cfg.Channels.Feishu.AppSecret {
			needRestart = true
		}

		if needRestart {
			_ = feishuCh.Stop(reloadCtx)
			*feishuChPtr = startFeishuChannel(runCtx, cfg, runner, sessions, logger)
			ch := *feishuChPtr
			status := http.ChannelStatus{Status: "restarted"}
			if ch != nil && !ch.IsBound() {
				status.BindCode = ch.BindCode()
			}
			result.Reloaded = append(result.Reloaded, "feishu:restarted")
			result.Channels["feishu"] = status
		} else {
			// 仅白名单变更 → 原地更新，不断连
			feishuCh.UpdateAllowlist(feishuCfg.AllowedUsers, feishuCfg.AllowedChats)
			result.Reloaded = append(result.Reloaded, "feishu:updated")
			result.Channels["feishu"] = http.ChannelStatus{Status: "running"}
			_ = oldID // suppress unused
		}
	}

	logger.Info("channel reload complete", "reloaded", result.Reloaded)
	return result, nil
}

// processFeishuMsg 处理飞书消息：会话路由 → 构建历史 → 调用 Agent → 返回回复
func processFeishuMsg(
	ctx context.Context,
	cfg *config.Config,
	runner *agent.Runner,
	sessions *session.Manager,
	logger *slog.Logger,
	msg *feishu.ParsedMessage,
) (string, error) {
	peerKind := "direct"
	groupID := ""
	if msg.ChatType == "group" {
		peerKind = "group"
		groupID = msg.ChatID
	}

	peer := session.PeerContext{
		Channel:  "feishu",
		PeerID:   msg.SenderID,
		PeerKind: peerKind,
		GroupID:  groupID,
	}
	sessionKey := session.ResolveSessionFromConfig(cfg, peer)

	var history []agent.ChatMessage
	if sessions != nil {
		sess := sessions.GetOrCreate(sessionKey, "feishu")
		sess.AddMessage(protocol.ChatMessage{
			Role:    "user",
			Content: msg.Text,
			Channel: "feishu",
		})

		allMsgs := sess.Messages()
		limit := 16
		start := 0
		if len(allMsgs) > limit {
			start = len(allMsgs) - limit
		}
		for _, m := range allMsgs[start:] {
			role := strings.ToLower(strings.TrimSpace(m.Role))
			if role != "user" && role != "assistant" && role != "system" {
				continue
			}
			content := strings.TrimSpace(m.Content)
			if content == "" {
				continue
			}
			if len([]rune(content)) > 3000 {
				content = string([]rune(content)[:3000]) + "..."
			}
			history = append(history, agent.ChatMessage{Role: role, Content: content})
		}
	}

	if runner == nil {
		return "", fmt.Errorf("agent not available")
	}

	result, err := runner.Run(ctx, &agent.RunRequest{
		SessionKey: sessionKey,
		Channel:    "feishu",
		Sender:     msg.SenderID,
		MessageID:  msg.MessageID,
		Message:    msg.Text,
		History:    history,
	})
	if err != nil {
		return "", err
	}

	if sessions != nil {
		if sess, ok := sessions.Get(sessionKey); ok {
			sess.AddMessage(protocol.ChatMessage{
				Role:    "assistant",
				Content: result.Reply,
				Channel: "feishu",
			})
		}
	}

	logger.Info("feishu message processed", "session", sessionKey)
	return result.Reply, nil
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
