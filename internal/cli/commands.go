package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/highclaw/highclaw/internal/agent"
	skillapp "github.com/highclaw/highclaw/internal/application/skill"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/model"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/tui"
	"github.com/spf13/cobra"
)

var (
	agentMessage     string
	agentProvider    string
	agentModel       string
	agentTemperature float64
	agentSession     string

	cronTaskID      string
	cronTaskSpec    string
	cronTaskCommand string

	modelsShowAll bool

	logTailLines  int
	logTailFollow bool

	resetYes        bool
	resetKeepConfig bool
	uninstallYes    bool

	migrateDryRun bool

	tuiGateway string
	tuiAgent   string
	tuiSession string
	tuiModel   string

	memoryLimit    int
	memoryCategory string
)

// --- Agent Command ---

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with the AI agent",
	Long:  "Send messages to the AI agent and receive responses. Supports RPC and interactive modes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(agentMessage) != "" {
			return agentChatCmd.RunE(cmd, []string{agentMessage})
		}
		return tui.RunWithOptions(tui.Options{
			Agent:   "main",
			Session: strings.TrimSpace(agentSession),
			Model:   strings.TrimSpace(agentModel),
		})
	},
}

var agentChatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a chat message to the agent",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		runner := agent.NewRunner(cfg, logger)

		msg := strings.TrimSpace(strings.Join(args, " "))
		if msg == "" {
			return fmt.Errorf("message cannot be empty")
		}

		sessionKey := strings.TrimSpace(agentSession)
		if sessionKey == "" {
			sessionKey = fmt.Sprintf("agent:%s:%s", "main", fmt.Sprintf("cli-%d", time.Now().UnixNano()))
		} else if !strings.HasPrefix(sessionKey, "agent:") {
			sessionKey = fmt.Sprintf("agent:%s:%s", "main", sessionKey)
		}
		result, err := runner.Run(context.Background(), &agent.RunRequest{
			SessionKey:  sessionKey,
			Channel:     "cli",
			Message:     msg,
			Provider:    strings.TrimSpace(agentProvider),
			Model:       strings.TrimSpace(agentModel),
			Temperature: agentTemperature,
		})
		if err != nil {
			return err
		}
		modelName := strings.TrimSpace(agentModel)
		if modelName == "" {
			modelName = cfg.Agent.Model
		}
		_ = saveCLISession(sessionKey, modelName, msg, result.Reply)

		fmt.Println(result.Reply)
		return nil
	},
}

var agentRPCCmd = &cobra.Command{
	Use:   "rpc",
	Short: "Start agent in RPC mode (JSON I/O)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Agent RPC mode is available via `highclaw gateway` WebSocket endpoint: /ws")
		return runGateway(cmd, args)
	},
}

// --- Channels Command ---

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage messaging channels (WhatsApp/Telegram/Discord/Slack/Signal...)",
	Aliases: []string{
		"channel",
	},
}

var channelsDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run channel health diagnostics",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Println("channel doctor:")
		fmt.Printf("  telegram: %s\n", boolText(cfg.Channels.Telegram != nil && cfg.Channels.Telegram.BotToken != ""))
		fmt.Printf("  discord : %s\n", boolText(cfg.Channels.Discord != nil && cfg.Channels.Discord.Token != ""))
		fmt.Printf("  slack   : %s\n", boolText(cfg.Channels.Slack != nil && cfg.Channels.Slack.BotToken != ""))
		fmt.Printf("  signal  : %s\n", boolText(cfg.Channels.Signal != nil && cfg.Channels.Signal.Enabled))
		return nil
	},
}

var channelsLoginCmd = &cobra.Command{
	Use:   "login [channel]",
	Short: "Login to a messaging channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ch := strings.ToLower(args[0])
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		switch ch {
		case "telegram":
			if cfg.Channels.Telegram == nil || cfg.Channels.Telegram.BotToken == "" {
				return fmt.Errorf("telegram bot token is empty in config")
			}
		case "discord":
			if cfg.Channels.Discord == nil || cfg.Channels.Discord.Token == "" {
				return fmt.Errorf("discord token is empty in config")
			}
		case "slack":
			if cfg.Channels.Slack == nil || cfg.Channels.Slack.BotToken == "" {
				return fmt.Errorf("slack bot token is empty in config")
			}
		case "whatsapp":
			fmt.Println("whatsapp uses session-based login; start gateway and complete pairing/QR flow")
		default:
			return fmt.Errorf("unknown channel: %s", ch)
		}
		fmt.Printf("channel %s configuration looks valid\n", ch)
		return nil
	},
}

var channelsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("telegram: %v\n", cfg.Channels.Telegram != nil && cfg.Channels.Telegram.BotToken != "")
		fmt.Printf("discord : %v\n", cfg.Channels.Discord != nil && cfg.Channels.Discord.Token != "")
		fmt.Printf("slack   : %v\n", cfg.Channels.Slack != nil && cfg.Channels.Slack.BotToken != "")
		fmt.Printf("signal  : %v\n", cfg.Channels.Signal != nil && cfg.Channels.Signal.Enabled)
		return nil
	},
}

var channelsLogoutCmd = &cobra.Command{
	Use:   "logout [channel]",
	Short: "Logout from a messaging channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("channel %s logout is stateless in current build; clear tokens from config if needed\n", args[0])
		return nil
	},
}

var channelsSendCmd = &cobra.Command{
	Use:   "send [channel] [recipient] [message]",
	Short: "Send a message through a channel",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		channel := args[0]
		recipient := args[1]
		msg := strings.Join(args[2:], " ")
		fmt.Printf("send queued (local simulation): channel=%s recipient=%s message=%q\n", channel, recipient, msg)
		fmt.Println("for live send, run `highclaw gateway` and use /api/chat or channel adapters")
		return nil
	},
}

// --- Config Command ---

var configCmdGroup = &cobra.Command{
	Use:   "config",
	Short: "Manage HighClaw configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		if len(args) == 0 {
			// Show all config
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		val, err := lookupConfigKey(cfg, args[0])
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(val, "", "  ")
		fmt.Println(string(data))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := setConfigKey(cfg, args[0], args[1]); err != nil {
			return err
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("updated %s\n", args[0])
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show full configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal config: %w", err)
		}

		fmt.Println(string(data))
		fmt.Printf("\nConfig file: %s\n", config.ConfigPath())
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		issues := validateConfig(cfg)
		if len(issues) == 0 {
			fmt.Println("config valid")
			return nil
		}
		for _, i := range issues {
			fmt.Println("- " + i)
		}
		return fmt.Errorf("validation failed")
	},
}

// --- Doctor Command ---

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics, health checks, and migrations",
	Long:  "Comprehensive diagnostics: config validation, auth check, gateway health, sandbox status, security audit, legacy migrations, workspace integrity.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Println("doctor report:")
		issues := validateConfig(cfg)
		if len(issues) == 0 {
			fmt.Println("  config: ok")
		} else {
			fmt.Println("  config: issues")
			for _, it := range issues {
				fmt.Printf("    - %s\n", it)
			}
		}
		if _, err := os.Stat(cfg.Agent.Workspace); err != nil {
			fmt.Printf("  workspace: missing (%s)\n", cfg.Agent.Workspace)
		} else {
			fmt.Printf("  workspace: ok (%s)\n", cfg.Agent.Workspace)
		}
		url := fmt.Sprintf("http://127.0.0.1:%d/api/health", cfg.Gateway.Port)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("  gateway: unreachable (%v)\n", err)
		} else {
			_ = resp.Body.Close()
			fmt.Printf("  gateway: reachable (%s)\n", resp.Status)
		}
		if len(issues) > 0 {
			return fmt.Errorf("doctor found issues")
		}
		return nil
	},
}

// --- Onboard Command ---
// (Implemented in onboard.go)

// --- Status Command ---

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show overall system status",
	Long:  "Comprehensive status: gateway health, active sessions, channels, daemon, agent, models.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("⚠️  Config: %v\n", err)
			cfg = config.Default()
		} else {
			fmt.Printf("✅ Config loaded from: %s\n", config.ConfigPath())
		}

		fmt.Printf("\n📊 Gateway:\n")
		fmt.Printf("   Port: %d\n", cfg.Gateway.Port)
		fmt.Printf("   Bind: %s\n", cfg.Gateway.Bind)

		fmt.Printf("\n🤖 Agent:\n")
		fmt.Printf("   Model: %s\n", cfg.Agent.Model)
		fmt.Printf("   Workspace: %s\n", cfg.Agent.Workspace)

		fmt.Printf("\n📡 Channels:\n")
		if cfg.Channels.Telegram != nil && cfg.Channels.Telegram.BotToken != "" {
			fmt.Printf("   Telegram: configured\n")
		}
		if cfg.Channels.Discord != nil && cfg.Channels.Discord.Token != "" {
			fmt.Printf("   Discord: configured\n")
		}
		if cfg.Channels.Slack != nil && cfg.Channels.Slack.BotToken != "" {
			fmt.Printf("   Slack: configured\n")
		}

		url := fmt.Sprintf("http://127.0.0.1:%d/api/status", cfg.Gateway.Port)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("\n🌐 Live Gateway: unreachable (%v)\n", err)
		} else {
			defer resp.Body.Close()
			fmt.Printf("\n🌐 Live Gateway: %s\n", resp.Status)
		}
		fmt.Printf("\n💡 Tip: Run 'highclaw gateway' to start the server\n")
		return nil
	},
}

// --- Sessions Command ---

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage chat sessions",
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := session.LoadAll()
		if err != nil {
			return fmt.Errorf("load sessions: %w", err)
		}
		if len(sessions) == 0 {
			fmt.Printf("no persisted sessions found (%s)\n", session.SessionsDir())
			return nil
		}
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].LastActivityAt.After(sessions[j].LastActivityAt)
		})
		for _, s := range sessions {
			fmt.Printf("%s  channel=%s messages=%d last=%s\n",
				s.Key, s.Channel, s.MessageCount, s.LastActivityAt.Format(time.RFC3339))
		}
		return nil
	},
}

var sessionsGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := resolveSessionKey(args[0])
		if err != nil {
			return err
		}
		s, err := session.Load(key)
		if err != nil {
			return fmt.Errorf("load session %q: %w", key, err)
		}
		data, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(data))
		return nil
	},
}

var sessionsCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current active session",
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := session.Current()
		if err != nil {
			return err
		}
		if strings.TrimSpace(key) == "" {
			fmt.Println("current session: (none)")
			return nil
		}
		fmt.Printf("current session: %s\n", key)
		return nil
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete [key]",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := resolveSessionKey(args[0])
		if err != nil {
			return err
		}
		if err := session.Delete(key); err != nil {
			return err
		}
		if current, _ := session.Current(); strings.TrimSpace(current) == strings.TrimSpace(key) {
			_ = session.SetCurrent("")
		}
		fmt.Printf("deleted session: %s\n", key)
		return nil
	},
}

var sessionsSwitchCmd = &cobra.Command{
	Use:   "switch [key]",
	Short: "Switch current active session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := resolveSessionKey(args[0])
		if err != nil {
			return err
		}
		if _, err := session.Load(key); err != nil {
			return fmt.Errorf("load session %q: %w", key, err)
		}
		if err := session.SetCurrent(key); err != nil {
			return fmt.Errorf("set current session: %w", err)
		}
		fmt.Printf("current session: %s\n", key)
		return nil
	},
}

var sessionsResetCmd = &cobra.Command{
	Use:   "reset [key]",
	Short: "Reset a session's message history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := resolveSessionKey(args[0])
		if err != nil {
			return err
		}
		s, err := session.Load(key)
		if err != nil {
			return fmt.Errorf("load session %q: %w", key, err)
		}
		s.Reset()
		if err := s.Save(); err != nil {
			return fmt.Errorf("save session %q: %w", key, err)
		}
		fmt.Printf("session reset: %s\n", key)
		return nil
	},
}

var sessionsBindingsCmd = &cobra.Command{
	Use:   "bindings",
	Short: "List external channel session bindings",
	RunE: func(cmd *cobra.Command, args []string) error {
		bindings, err := session.ListBindings()
		if err != nil {
			return err
		}
		if len(bindings) == 0 {
			fmt.Printf("no bindings configured (default external session: %s)\n", session.DefaultExternalSessionKey)
			return nil
		}
		for _, b := range bindings {
			fmt.Printf("channel=%s conversation=%s -> %s\n", b.Channel, b.Conversation, b.SessionKey)
		}
		return nil
	},
}

var sessionsBindCmd = &cobra.Command{
	Use:   "bind [channel] [conversation] [sessionKey]",
	Short: "Bind an external conversation to a session key",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := session.SetBinding(args[0], args[1], args[2]); err != nil {
			return err
		}
		fmt.Printf("binding set: %s/%s -> %s\n", args[0], args[1], args[2])
		return nil
	},
}

var sessionsUnbindCmd = &cobra.Command{
	Use:   "unbind [channel] [conversation]",
	Short: "Remove an external conversation binding",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := session.RemoveBinding(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("binding removed: %s/%s\n", args[0], args[1])
		return nil
	},
}

// --- Cron Command ---

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage scheduled/cron tasks",
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := loadCronTasks()
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("no cron tasks configured")
			return nil
		}
		for _, t := range tasks {
			lastRun := "never"
			if !t.LastRunAt.IsZero() {
				lastRun = t.LastRunAt.Format(time.RFC3339)
			}
			fmt.Printf("%s  spec=%s  cmd=%q  last_run=%s\n", t.ID, t.Spec, t.Command, lastRun)
		}
		return nil
	},
}

var cronCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a scheduled task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(cronTaskSpec) == "" || strings.TrimSpace(cronTaskCommand) == "" {
			return fmt.Errorf("both --spec and --command are required")
		}
		id := strings.TrimSpace(cronTaskID)
		if id == "" {
			id = fmt.Sprintf("task-%d", time.Now().Unix())
		}
		tasks, err := loadCronTasks()
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if t.ID == id {
				return fmt.Errorf("task id already exists: %s", id)
			}
		}
		tasks = append(tasks, cronTask{
			ID:        id,
			Spec:      cronTaskSpec,
			Command:   cronTaskCommand,
			CreatedAt: time.Now(),
		})
		if err := saveCronTasks(tasks); err != nil {
			return err
		}
		fmt.Printf("created cron task: %s\n", id)
		return nil
	},
}

var cronDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := loadCronTasks()
		if err != nil {
			return err
		}
		out := tasks[:0]
		found := false
		for _, t := range tasks {
			if t.ID == args[0] {
				found = true
				continue
			}
			out = append(out, t)
		}
		if !found {
			return fmt.Errorf("task not found: %s", args[0])
		}
		if err := saveCronTasks(out); err != nil {
			return err
		}
		fmt.Printf("deleted cron task: %s\n", args[0])
		return nil
	},
}

var cronTriggerCmd = &cobra.Command{
	Use:   "trigger [id]",
	Short: "Manually trigger a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := loadCronTasks()
		if err != nil {
			return err
		}
		var target *cronTask
		for i := range tasks {
			if tasks[i].ID == args[0] {
				target = &tasks[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("task not found: %s", args[0])
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		out, err := exec.CommandContext(ctx, "bash", "-lc", target.Command).CombinedOutput()
		target.LastRunAt = time.Now()
		_ = saveCronTasks(tasks)
		if len(out) > 0 {
			fmt.Print(string(out))
		}
		if err != nil {
			return fmt.Errorf("trigger task %q: %w", target.ID, err)
		}
		return nil
	},
}

// --- Skills Command ---

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage AI skills (bundled, managed, workspace)",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		mgr := skillapp.NewManager(cfg, slog.Default())
		items, err := mgr.DiscoverSkills(context.Background())
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		for _, s := range items {
			fmt.Printf("%s  %-18s  %-16s %s\n", s.Icon, s.ID, s.Status, s.Reason)
		}
		return nil
	},
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install [name-or-url]",
	Short: "Install a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := strings.TrimSpace(args[0])
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			return fmt.Errorf("remote skill install is not supported in this build; use a local skill id")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		mgr := skillapp.NewManager(cfg, slog.Default())
		if err := mgr.InstallMissingDependencies(context.Background(), "npm", []string{target}); err != nil {
			return err
		}
		fmt.Printf("skill install completed for: %s\n", target)
		return nil
	},
}

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall [name]",
	Short: "Uninstall a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		id := strings.TrimSpace(args[0])
		if id == "" {
			return fmt.Errorf("skill name is required")
		}
		if !containsString(cfg.Agent.Sandbox.Deny, id) {
			cfg.Agent.Sandbox.Deny = append(cfg.Agent.Sandbox.Deny, id)
			sort.Strings(cfg.Agent.Sandbox.Deny)
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("skill disabled via sandbox denylist: %s\n", id)
		return nil
	},
}

var skillsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show skills status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		mgr := skillapp.NewManager(cfg, slog.Default())
		summary, err := mgr.GetSkillsSummary(context.Background())
		if err != nil {
			return err
		}
		keys := make([]string, 0, len(summary))
		for k := range summary {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s: %d\n", k, summary[k])
		}
		return nil
	},
}

// --- Plugins Command ---

var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage channel and feature plugins",
}

var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		plugins, err := loadPluginsState()
		if err != nil {
			return err
		}
		fmt.Println("builtin: telegram, discord, slack, signal, whatsapp")
		if len(plugins) == 0 {
			fmt.Println("custom: (none)")
			return nil
		}
		fmt.Println("custom:")
		for _, p := range plugins {
			fmt.Printf("- %s\n", p)
		}
		return nil
	},
}

var pluginsInstallCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Install a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		plugins, err := loadPluginsState()
		if err != nil {
			return err
		}
		name := strings.TrimSpace(args[0])
		if name == "" {
			return fmt.Errorf("plugin name is required")
		}
		if !containsString(plugins, name) {
			plugins = append(plugins, name)
		}
		sort.Strings(plugins)
		if err := savePluginsState(plugins); err != nil {
			return err
		}
		fmt.Printf("plugin registered: %s\n", name)
		return nil
	},
}

var pluginsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync plugin versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		plugins, err := loadPluginsState()
		if err != nil {
			return err
		}
		sort.Strings(plugins)
		plugins = uniqueStrings(plugins)
		if err := savePluginsState(plugins); err != nil {
			return err
		}
		fmt.Printf("plugins synced: %d custom entries\n", len(plugins))
		return nil
	},
}

// --- Nodes Command ---

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Manage device nodes (macOS/iOS/Android)",
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listEntityDir("nodes")
	},
}

// --- Devices Command ---

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "Manage paired devices",
}

var devicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List paired devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listEntityDir("devices")
	},
}

// --- Models Command ---

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Manage AI models (list, set default, scan providers)",
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models from all providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		all := model.GetAllModelsComplete()
		grouped := make(map[string][]model.Model)
		for _, m := range all {
			grouped[m.Provider] = append(grouped[m.Provider], m)
		}
		providers := make([]string, 0, len(grouped))
		for p := range grouped {
			providers = append(providers, p)
		}
		sort.Strings(providers)
		for _, p := range providers {
			ms := grouped[p]
			sort.Slice(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
			fmt.Printf("%s (%d)\n", p, len(ms))
			limit := len(ms)
			if !modelsShowAll && limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				fmt.Printf("  - %s\n", ms[i].ID)
			}
			if !modelsShowAll && len(ms) > limit {
				fmt.Printf("  ... (%d more, use --all)\n", len(ms)-limit)
			}
		}
		return nil
	},
}

var modelsSetCmd = &cobra.Command{
	Use:   "set [provider/model]",
	Short: "Set the default model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		target := strings.TrimSpace(args[0])
		modelID := target
		if strings.Contains(target, "/") {
			parts := strings.SplitN(target, "/", 2)
			modelID = parts[1]
		}
		if !modelIDExists(modelID) && !modelIDExists(target) {
			fmt.Fprintf(os.Stderr, "warning: model %q not found in built-in catalog; saving as custom value\n", target)
		}
		cfg.Agent.Model = target
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Printf("default model updated: %s\n", target)
		return nil
	},
}

var modelsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan and discover available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		info := model.GetProviderInfo()
		providers := make([]string, 0, len(info))
		for p := range info {
			providers = append(providers, p)
		}
		sort.Strings(providers)
		for _, p := range providers {
			meta := info[p]
			hasEnv := meta.EnvVar != "" && strings.TrimSpace(os.Getenv(meta.EnvVar)) != ""
			hasCfgKey := false
			if c, ok := cfg.Agent.Providers[p]; ok {
				hasCfgKey = strings.TrimSpace(c.APIKey) != ""
			}
			models := model.GetModelsByProvider(p)
			fmt.Printf("%s  auth=%s  env=%v  cfg=%v  models=%d\n", p, meta.AuthType, hasEnv, hasCfgKey, len(models))
		}
		return nil
	},
}

// --- Security Command ---

var securityCmd = &cobra.Command{
	Use:   "security",
	Short: "Security audit, fixes, and sandbox management",
}

var securityAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run security audit on config, files, and channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		var issues []string
		issues = append(issues, validateConfig(cfg)...)
		if cfg.Gateway.Auth.Mode == "token" && strings.TrimSpace(cfg.Gateway.Auth.Token) == "" {
			issues = append(issues, "gateway.auth.token is empty while auth mode is token")
		}
		if st, err := os.Stat(config.ConfigPath()); err == nil {
			if st.Mode().Perm()&0o077 != 0 {
				issues = append(issues, fmt.Sprintf("config file is too permissive: %o", st.Mode().Perm()))
			}
		}
		if len(issues) == 0 {
			fmt.Println("security audit passed")
			return nil
		}
		fmt.Println("security audit issues:")
		for _, i := range issues {
			fmt.Printf("- %s\n", i)
		}
		return fmt.Errorf("security audit failed with %d issue(s)", len(issues))
	},
}

var securityFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Auto-fix security issues found by audit",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		changed := false
		if cfg.Gateway.Auth.Mode == "none" {
			cfg.Gateway.Auth.Mode = "token"
			changed = true
		}
		if cfg.Gateway.Auth.Mode == "token" && strings.TrimSpace(cfg.Gateway.Auth.Token) == "" {
			token, err := randomHex(24)
			if err != nil {
				return err
			}
			cfg.Gateway.Auth.Token = token
			changed = true
		}
		if len(cfg.Agent.Sandbox.Allow) == 0 {
			cfg.Agent.Sandbox.Allow = []string{"bash", "read_file", "write_file", "web_search"}
			changed = true
		}
		if !changed {
			fmt.Println("no automatic security fixes were needed")
			return nil
		}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Println("security fixes applied")
		return nil
	},
}

// --- Exec Approvals Command ---

var execApprovalsCmd = &cobra.Command{
	Use:   "exec-approvals",
	Short: "Manage tool execution approvals",
}

var execApprovalsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending execution approvals",
	RunE: func(cmd *cobra.Command, args []string) error {
		items, err := loadExecApprovals()
		if err != nil {
			return err
		}
		pending := 0
		for _, it := range items {
			if it.Status == "pending" {
				pending++
				fmt.Printf("%s  requester=%s  cmd=%q  created=%s\n", it.ID, it.Requester, it.Command, it.CreatedAt.Format(time.RFC3339))
			}
		}
		if pending == 0 {
			fmt.Println("no pending approvals")
		}
		return nil
	},
}

var execApprovalsApproveCmd = &cobra.Command{
	Use:   "approve [id]",
	Short: "Approve a pending execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return updateExecApproval(args[0], "approved")
	},
}

var execApprovalsDenyCmd = &cobra.Command{
	Use:   "deny [id]",
	Short: "Deny a pending execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return updateExecApproval(args[0], "denied")
	},
}

// --- Hooks Command ---

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage event hooks (Gmail, webhooks, internal)",
}

var hooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("gmail hook: account=%q model=%q\n", cfg.Hooks.Gmail.Account, cfg.Hooks.Gmail.Model)
		fmt.Printf("internal hook enabled: %v\n", cfg.Hooks.Internal.Enabled)
		paths, err := listHookFiles()
		if err != nil {
			return err
		}
		if len(paths) == 0 {
			fmt.Println("custom hooks: (none)")
			return nil
		}
		fmt.Println("custom hooks:")
		for _, p := range paths {
			fmt.Printf("- %s\n", p)
		}
		return nil
	},
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install [hook-path]",
	Short: "Install a hook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := strings.TrimSpace(args[0])
		st, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("stat hook: %w", err)
		}
		if st.IsDir() {
			return fmt.Errorf("hook path must be a file")
		}
		dstDir := hooksDir()
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return err
		}
		dst := filepath.Join(dstDir, filepath.Base(src))
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("hook installed: %s\n", dst)
		return nil
	},
}

var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show hooks status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		paths, _ := listHookFiles()
		fmt.Printf("internal.enabled=%v\n", cfg.Hooks.Internal.Enabled)
		fmt.Printf("gmail.account=%q\n", cfg.Hooks.Gmail.Account)
		fmt.Printf("gmail.model=%q\n", cfg.Hooks.Gmail.Model)
		fmt.Printf("custom.hooks=%d\n", len(paths))
		return nil
	},
}

// --- Logs Command ---

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View and query gateway logs",
}

var logsTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Stream live logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tailLogFile(logFilePath(), logTailLines, logTailFollow)
	},
}

var logsQueryCmd = &cobra.Command{
	Use:   "query [pattern]",
	Short: "Search historical logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("pattern is required")
		}
		return queryLogFile(logFilePath(), strings.Join(args, " "))
	},
}

// --- Browser Command ---

var browserCmdGroup = &cobra.Command{
	Use:   "browser",
	Short: "Manage browser control (CDP/headless)",
}

var browserOpenCmd = &cobra.Command{
	Use:   "open [url]",
	Short: "Open a URL in the controlled browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("url is required")
		}
		return openURL(args[0])
	},
}

var browserInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect current browser state",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("browser.enabled=%v\n", cfg.Browser.Enabled)
		fmt.Printf("browser.color=%s\n", cfg.Browser.Color)
		bin := "not found"
		for _, candidate := range []string{"google-chrome", "chromium", "chrome", "safari"} {
			if p, err := exec.LookPath(candidate); err == nil {
				bin = p
				break
			}
		}
		fmt.Printf("browser.binary=%s\n", bin)
		return nil
	},
}

// --- Memory Command ---

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory backend (search/get/list/status)",
}

var memorySearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search memory for relevant context",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		query := strings.Join(args, " ")
		entries, err := searchMemoryBackend(cfg, query, memoryLimit, memoryCategory)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("No memories found matching that query.")
			return nil
		}
		fmt.Printf("Found %d memories:\n", len(entries))
		for _, e := range entries {
			fmt.Printf("- [%s] %s: %s\n", e.Category, e.Key, e.Content)
		}
		return nil
	},
}

var memoryGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get memory entry by key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		entry, err := getMemoryByKey(cfg, args[0])
		if err != nil {
			return err
		}
		if entry == nil {
			fmt.Println("memory not found")
			return nil
		}
		fmt.Printf("[%s] %s: %s\n", entry.Category, entry.Key, entry.Content)
		return nil
	},
}

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memory entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		entries, err := listMemoryBackend(cfg, memoryCategory, memoryLimit)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println("no memory entries")
			return nil
		}
		for _, e := range entries {
			fmt.Printf("[%s] %s: %s\n", e.Category, e.Key, e.Content)
		}
		return nil
	},
}

var memorySyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync memory from session files",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions, err := session.LoadAll()
		if err != nil {
			return err
		}
		records := make([]memoryRecord, 0, len(sessions))
		for _, s := range sessions {
			records = append(records, memoryRecord{
				SessionKey:   s.Key,
				Channel:      s.Channel,
				Model:        s.Model,
				MessageCount: s.MessageCount,
				LastActivity: s.LastActivityAt,
			})
		}
		if err := saveMemoryIndex(records); err != nil {
			return err
		}
		fmt.Printf("memory sync complete: %d records\n", len(records))
		return nil
	},
}

var memoryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show memory backend status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		backend := strings.ToLower(strings.TrimSpace(cfg.Memory.Backend))
		if backend == "" {
			backend = "sqlite"
		}
		if backend == "none" {
			backend = "markdown"
		}
		count, err := countMemoryBackend(cfg)
		if err != nil {
			return err
		}
		fmt.Printf("backend: %s\n", backend)
		fmt.Printf("location: %s\n", memoryBackendLocation(cfg))
		fmt.Printf("entries: %d\n", count)
		fmt.Printf("health: %v\n", memoryHealthBackend(cfg))
		return nil
	},
}

var memoryResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset memory index",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := os.Remove(memoryIndexPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Println("memory index reset")
		return nil
	},
}

// --- Daemon Command ---

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage system daemon (launchd/systemd/schtasks)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Align with ZeroClaw-style "highclaw daemon" as runtime entry.
		return runGateway(cmd, args)
	},
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install HighClaw as a system daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := loadDaemonState()
		if err != nil {
			return err
		}
		st.Installed = true
		if err := saveDaemonState(st); err != nil {
			return err
		}
		fmt.Printf("daemon metadata installed at %s\n", daemonStatePath())
		return nil
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the system daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := os.Remove(daemonStatePath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Println("daemon metadata removed")
		return nil
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return startDaemonProcess()
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return stopDaemonProcess()
	},
}

var daemonDaemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := loadDaemonState()
		if err != nil {
			return err
		}
		running := processRunning(st.PID)
		fmt.Printf("installed=%v\n", st.Installed)
		fmt.Printf("pid=%d\n", st.PID)
		fmt.Printf("running=%v\n", running)
		if !st.StartedAt.IsZero() {
			fmt.Printf("started_at=%s\n", st.StartedAt.Format(time.RFC3339))
		}
		return nil
	},
}

// --- Update Command ---

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates",
}

var updateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check for available updates",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("current version: %s\n", version)
		fmt.Printf("build date: %s\n", buildDate)
		fmt.Printf("commit: %s\n", gitCommit)
		fmt.Println("auto-update is not configured in this build")
		return nil
	},
}

var updateInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the latest update",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("self-update is not enabled; please update the binary manually")
	},
}

// --- TUI Command ---

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch terminal UI (interactive chat)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunWithOptions(tui.Options{
			GatewayURL: tuiGateway,
			Agent:      tuiAgent,
			Session:    tuiSession,
			Model:      tuiModel,
		})
	},
}

// --- Simple Commands ---

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset HighClaw state (configs, sessions, cache)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !resetYes {
			return fmt.Errorf("reset is destructive; rerun with --yes")
		}
		_ = os.RemoveAll(session.SessionsDir())
		_ = os.RemoveAll(stateDir())
		_ = os.RemoveAll(hooksDir())
		if !resetKeepConfig {
			_ = os.Remove(config.ConfigPath())
		}
		fmt.Println("reset complete")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall HighClaw and clean up",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !uninstallYes {
			return fmt.Errorf("uninstall is destructive; rerun with --yes")
		}
		if err := os.RemoveAll(config.ConfigDir()); err != nil {
			return err
		}
		fmt.Printf("removed %s\n", config.ConfigDir())
		return nil
	},
}

var messageCmd = &cobra.Command{
	Use:   "message [text]",
	Short: "Send a single message and print the response",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return agentChatCmd.RunE(cmd, args)
	},
}

var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Manage inbound/outbound webhooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("telegram.webhookUrl=%q\n", cfg.Channels.Telegram.WebhookURL)
		fmt.Printf("telegram.webhookSecret.set=%v\n", cfg.Channels.Telegram.WebhookSecret != "")
		fmt.Printf("bluebubbles.webhookPath=%q\n", cfg.Channels.BlueBubbles.WebhookPath)
		return nil
	},
}

var pairingCmd = &cobra.Command{
	Use:   "pairing",
	Short: "Manage device pairing (QR codes, tokens)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if cfg.Gateway.Auth.Mode == "none" {
			fmt.Println("Pairing disabled: gateway auth mode is 'none'")
			return nil
		}
		if strings.TrimSpace(cfg.Gateway.Auth.Token) != "" {
			fmt.Println("Pairing already completed: static auth token is configured")
			return nil
		}
		fmt.Println("Pairing is required and no static token is configured.")
		fmt.Println("Start gateway and call: POST /api/pair with X-Pairing-Code header.")
		return nil
	},
}

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS diagnostics and configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		hosts := []string{"api.anthropic.com", "api.openai.com", "slack.com", "discord.com", "api.telegram.org"}
		for _, h := range hosts {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			ips, err := net.DefaultResolver.LookupHost(ctx, h)
			cancel()
			if err != nil {
				fmt.Printf("%s -> error: %v\n", h, err)
				continue
			}
			fmt.Printf("%s -> %s\n", h, strings.Join(ips, ", "))
		}
		return nil
	},
}

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Open documentation in browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		candidates := []string{
			"https://github.com/highclaw/highclaw",
			"https://docs.openclaw.ai",
		}
		for _, c := range candidates {
			if err := openURL(c); err == nil {
				fmt.Printf("opened docs: %s\n", c)
				return nil
			}
		}
		return fmt.Errorf("failed to open docs URL")
	},
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the web dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		url := fmt.Sprintf("http://127.0.0.1:%d", cfg.Gateway.Port)
		if err := openURL(url); err != nil {
			return err
		}
		fmt.Printf("opened dashboard: %s\n", url)
		return nil
	},
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage background service (alias of daemon)",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemonInstallCmd.RunE(cmd, args)
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show background service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemonDaemonStatusCmd.RunE(cmd, args)
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemonStartCmd.RunE(cmd, args)
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop background service",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemonStopCmd.RunE(cmd, args)
	},
}

var integrationsCmd = &cobra.Command{
	Use:   "integrations",
	Short: "Integration setup and diagnostics",
}

var integrationsInfoCmd = &cobra.Command{
	Use:   "info [name]",
	Short: "Show integration setup details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		switch name {
		case "telegram":
			fmt.Println("Integration: Telegram")
			fmt.Printf("  configured: %v\n", cfg.Channels.Telegram.BotToken != "")
			fmt.Println("  required: channels.telegram.botToken")
			fmt.Println("  check: highclaw channel doctor")
		case "discord":
			fmt.Println("Integration: Discord")
			fmt.Printf("  configured: %v\n", cfg.Channels.Discord.Token != "")
			fmt.Println("  required: channels.discord.token")
		case "slack":
			fmt.Println("Integration: Slack")
			fmt.Printf("  configured: %v\n", cfg.Channels.Slack.BotToken != "")
			fmt.Println("  required: channels.slack.botToken (+ appToken optional)")
		default:
			fmt.Printf("Integration: %s\n", args[0])
			fmt.Println("  status: unknown integration in current build")
			fmt.Println("  supported examples: Telegram, Discord, Slack")
		}
		return nil
	},
}

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Data migration utilities",
}

var migrateOpenClawCmd = &cobra.Command{
	Use:   "openclaw",
	Short: "Migrate config/workspace from ~/.openclaw to ~/.highclaw",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		srcRoot := filepath.Join(home, ".openclaw")
		dstRoot := filepath.Join(home, ".highclaw")
		entries := []struct {
			src string
			dst string
		}{
			{src: filepath.Join(srcRoot, "config.yaml"), dst: filepath.Join(dstRoot, "config.yaml")},
			{src: filepath.Join(srcRoot, "openclaw.json"), dst: filepath.Join(dstRoot, "config.yaml")},
			{src: filepath.Join(srcRoot, "workspace"), dst: filepath.Join(dstRoot, "workspace")},
		}

		planned := 0
		for _, e := range entries {
			if _, statErr := os.Stat(e.src); statErr == nil {
				planned++
				fmt.Printf("%s %s -> %s\n", ternary(migrateDryRun, "plan", "migrate"), e.src, e.dst)
				if !migrateDryRun {
					if err := copyPath(e.src, e.dst); err != nil {
						return err
					}
				}
			}
		}
		if planned == 0 {
			fmt.Printf("no legacy assets found under %s\n", srcRoot)
			return nil
		}
		if migrateDryRun {
			fmt.Printf("dry-run complete, %d item(s) ready to migrate\n", planned)
		} else {
			fmt.Printf("migration complete, %d item(s) migrated\n", planned)
		}
		return nil
	},
}

func init() {
	// Agent subcommands
	agentCmd.AddCommand(agentChatCmd)
	agentCmd.AddCommand(agentRPCCmd)

	// Channels subcommands
	channelsCmd.AddCommand(channelsLoginCmd)
	channelsCmd.AddCommand(channelsStatusCmd)
	channelsCmd.AddCommand(channelsLogoutCmd)
	channelsCmd.AddCommand(channelsSendCmd)
	channelsCmd.AddCommand(channelsDoctorCmd)

	// Config subcommands
	configCmdGroup.AddCommand(configGetCmd)
	configCmdGroup.AddCommand(configSetCmd)
	configCmdGroup.AddCommand(configShowCmd)
	configCmdGroup.AddCommand(configValidateCmd)

	// Sessions subcommands
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsGetCmd)
	sessionsCmd.AddCommand(sessionsCurrentCmd)
	sessionsCmd.AddCommand(sessionsSwitchCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsResetCmd)
	sessionsCmd.AddCommand(sessionsBindingsCmd)
	sessionsCmd.AddCommand(sessionsBindCmd)
	sessionsCmd.AddCommand(sessionsUnbindCmd)

	// Cron subcommands
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronCreateCmd)
	cronCmd.AddCommand(cronDeleteCmd)
	cronCmd.AddCommand(cronTriggerCmd)

	// Skills subcommands
	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInstallCmd)
	skillsCmd.AddCommand(skillsUninstallCmd)
	skillsCmd.AddCommand(skillsStatusCmd)

	// Plugins subcommands
	pluginsCmd.AddCommand(pluginsListCmd)
	pluginsCmd.AddCommand(pluginsInstallCmd)
	pluginsCmd.AddCommand(pluginsSyncCmd)

	// Nodes/Devices subcommands
	nodesCmd.AddCommand(nodesListCmd)
	devicesCmd.AddCommand(devicesListCmd)

	// Models subcommands
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsSetCmd)
	modelsCmd.AddCommand(modelsScanCmd)

	// Security subcommands
	securityCmd.AddCommand(securityAuditCmd)
	securityCmd.AddCommand(securityFixCmd)

	// Exec-approvals subcommands
	execApprovalsCmd.AddCommand(execApprovalsListCmd)
	execApprovalsCmd.AddCommand(execApprovalsApproveCmd)
	execApprovalsCmd.AddCommand(execApprovalsDenyCmd)

	// Hooks subcommands
	hooksCmd.AddCommand(hooksListCmd)
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksStatusCmd)

	// Logs subcommands
	logsCmd.AddCommand(logsTailCmd)
	logsCmd.AddCommand(logsQueryCmd)

	// Browser subcommands
	browserCmdGroup.AddCommand(browserOpenCmd)
	browserCmdGroup.AddCommand(browserInspectCmd)

	// Memory subcommands
	memoryCmd.AddCommand(memorySearchCmd)
	memoryCmd.AddCommand(memorySyncCmd)
	memoryCmd.AddCommand(memoryStatusCmd)
	memoryCmd.AddCommand(memoryResetCmd)

	// Daemon subcommands
	daemonCmd.AddCommand(daemonInstallCmd)
	daemonCmd.AddCommand(daemonUninstallCmd)
	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonDaemonStatusCmd)

	// Service aliases
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)

	// Integrations
	integrationsCmd.AddCommand(integrationsInfoCmd)

	// Migration
	migrateCmd.AddCommand(migrateOpenClawCmd)

	// Update subcommands
	updateCmd.AddCommand(updateCheckCmd)
	updateCmd.AddCommand(updateInstallCmd)

	cronCreateCmd.Flags().StringVar(&cronTaskID, "id", "", "Task ID (auto-generated when empty)")
	cronCreateCmd.Flags().StringVar(&cronTaskSpec, "spec", "", "Cron schedule expression")
	cronCreateCmd.Flags().StringVar(&cronTaskCommand, "command", "", "Command to execute")
	agentCmd.Flags().StringVarP(&agentMessage, "message", "m", "", "Send one message and exit")
	agentCmd.Flags().StringVarP(&agentSession, "session", "s", "", "Associate message with an existing session key (default creates a new session)")
	agentCmd.Flags().StringVarP(&agentProvider, "provider", "p", "", "Provider override (e.g. openrouter, anthropic, glm)")
	agentCmd.Flags().StringVar(&agentModel, "model", "", "Model override (e.g. anthropic/claude-sonnet-4)")
	agentCmd.Flags().Float64VarP(&agentTemperature, "temperature", "t", 0.7, "Sampling temperature (0.0 - 2.0)")

	modelsListCmd.Flags().BoolVar(&modelsShowAll, "all", false, "Show all models")
	migrateOpenClawCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Preview migration actions without writing")
	tuiCmd.Flags().StringVarP(&tuiGateway, "gateway", "g", "ws://127.0.0.1:18789", "Gateway WebSocket URL")
	tuiCmd.Flags().StringVarP(&tuiAgent, "agent", "a", "main", "Agent ID")
	tuiCmd.Flags().StringVarP(&tuiSession, "session", "s", "main", "Session name or full session key")
	tuiCmd.Flags().StringVarP(&tuiModel, "model", "m", "", "Override model for this TUI run")

	logsTailCmd.Flags().IntVarP(&logTailLines, "lines", "n", 200, "Number of recent lines to show")
	logsTailCmd.Flags().BoolVarP(&logTailFollow, "follow", "f", false, "Follow log output")

	resetCmd.Flags().BoolVar(&resetYes, "yes", false, "Confirm destructive reset")
	resetCmd.Flags().BoolVar(&resetKeepConfig, "keep-config", true, "Keep config file during reset")
	uninstallCmd.Flags().BoolVar(&uninstallYes, "yes", false, "Confirm destructive uninstall")
}

type cronTask struct {
	ID        string    `json:"id"`
	Spec      string    `json:"spec"`
	Command   string    `json:"command"`
	CreatedAt time.Time `json:"createdAt"`
	LastRunAt time.Time `json:"lastRunAt"`
}

type execApproval struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Requester string    `json:"requester"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type memoryRecord struct {
	SessionKey   string    `json:"sessionKey"`
	Channel      string    `json:"channel"`
	Model        string    `json:"model"`
	MessageCount int       `json:"messageCount"`
	LastActivity time.Time `json:"lastActivity"`
}

type daemonState struct {
	Installed bool      `json:"installed"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
}

func lookupConfigKey(cfg *config.Config, key string) (any, error) {
	switch key {
	case "agent.model":
		return cfg.Agent.Model, nil
	case "agent.workspace":
		return cfg.Agent.Workspace, nil
	case "gateway.port":
		return cfg.Gateway.Port, nil
	case "gateway.bind":
		return cfg.Gateway.Bind, nil
	case "gateway.mode":
		return cfg.Gateway.Mode, nil
	case "gateway.auth.mode":
		return cfg.Gateway.Auth.Mode, nil
	case "gateway.auth.token":
		return cfg.Gateway.Auth.Token, nil
	case "memory.backend":
		return cfg.Memory.Backend, nil
	case "memory.autoSave":
		return cfg.Memory.AutoSave, nil
	case "channels.telegram.botToken":
		if cfg.Channels.Telegram == nil {
			return "", nil
		}
		return cfg.Channels.Telegram.BotToken, nil
	case "channels.discord.token":
		if cfg.Channels.Discord == nil {
			return "", nil
		}
		return cfg.Channels.Discord.Token, nil
	case "channels.slack.botToken":
		if cfg.Channels.Slack == nil {
			return "", nil
		}
		return cfg.Channels.Slack.BotToken, nil
	case "channels.signal.enabled":
		if cfg.Channels.Signal == nil {
			return false, nil
		}
		return cfg.Channels.Signal.Enabled, nil
	case "browser.enabled":
		return cfg.Browser.Enabled, nil
	case "browser.color":
		return cfg.Browser.Color, nil
	case "hooks.internal.enabled":
		return cfg.Hooks.Internal.Enabled, nil
	case "hooks.gmail.account":
		return cfg.Hooks.Gmail.Account, nil
	case "hooks.gmail.model":
		return cfg.Hooks.Gmail.Model, nil
	default:
		return nil, fmt.Errorf("unsupported config key: %s", key)
	}
}

func setConfigKey(cfg *config.Config, key, value string) error {
	switch key {
	case "agent.model":
		cfg.Agent.Model = value
	case "agent.workspace":
		cfg.Agent.Workspace = value
	case "gateway.port":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int for %s: %w", key, err)
		}
		cfg.Gateway.Port = n
	case "gateway.bind":
		cfg.Gateway.Bind = value
	case "gateway.mode":
		cfg.Gateway.Mode = value
	case "gateway.auth.mode":
		cfg.Gateway.Auth.Mode = value
	case "gateway.auth.token":
		cfg.Gateway.Auth.Token = value
	case "memory.backend":
		cfg.Memory.Backend = strings.TrimSpace(value)
	case "memory.autoSave":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for %s: %w", key, err)
		}
		cfg.Memory.AutoSave = b
	case "channels.telegram.botToken":
		if cfg.Channels.Telegram == nil {
			cfg.Channels.Telegram = &config.TelegramConfig{}
		}
		cfg.Channels.Telegram.BotToken = value
	case "channels.discord.token":
		if cfg.Channels.Discord == nil {
			cfg.Channels.Discord = &config.DiscordConfig{}
		}
		cfg.Channels.Discord.Token = value
	case "channels.slack.botToken":
		if cfg.Channels.Slack == nil {
			cfg.Channels.Slack = &config.SlackConfig{}
		}
		cfg.Channels.Slack.BotToken = value
	case "channels.signal.enabled":
		if cfg.Channels.Signal == nil {
			cfg.Channels.Signal = &config.SignalConfig{}
		}
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for %s: %w", key, err)
		}
		cfg.Channels.Signal.Enabled = b
	case "browser.enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for %s: %w", key, err)
		}
		cfg.Browser.Enabled = b
	case "browser.color":
		cfg.Browser.Color = value
	case "hooks.internal.enabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool for %s: %w", key, err)
		}
		cfg.Hooks.Internal.Enabled = b
	case "hooks.gmail.account":
		cfg.Hooks.Gmail.Account = value
	case "hooks.gmail.model":
		cfg.Hooks.Gmail.Model = value
	default:
		return fmt.Errorf("unsupported config key: %s", key)
	}
	return nil
}

func validateConfig(cfg *config.Config) []string {
	var issues []string
	if strings.TrimSpace(cfg.Agent.Model) == "" {
		issues = append(issues, "agent.model is required")
	}
	if strings.TrimSpace(cfg.Agent.Workspace) == "" {
		issues = append(issues, "agent.workspace is required")
	}
	if cfg.Gateway.Port <= 0 || cfg.Gateway.Port > 65535 {
		issues = append(issues, "gateway.port must be between 1 and 65535")
	}
	if cfg.Gateway.Bind != "" && cfg.Gateway.Bind != "loopback" && cfg.Gateway.Bind != "all" && cfg.Gateway.Bind != "tailnet" {
		issues = append(issues, "gateway.bind must be one of: loopback, all, tailnet")
	}
	if cfg.Gateway.Auth.Mode != "" && cfg.Gateway.Auth.Mode != "none" && cfg.Gateway.Auth.Mode != "token" && cfg.Gateway.Auth.Mode != "password" {
		issues = append(issues, "gateway.auth.mode must be one of: none, token, password")
	}
	if cfg.Gateway.Auth.Mode == "token" && strings.TrimSpace(cfg.Gateway.Auth.Token) == "" && cfg.Gateway.Bind != "loopback" {
		issues = append(issues, "gateway.auth.token is required when mode is token")
	}
	return issues
}

func stateDir() string {
	return filepath.Join(config.ConfigDir(), "state")
}

func cronTasksPath() string {
	return filepath.Join(stateDir(), "cron_tasks.json")
}

func pluginsPath() string {
	return filepath.Join(stateDir(), "plugins.json")
}

func execApprovalsPath() string {
	return filepath.Join(stateDir(), "exec_approvals.json")
}

func hooksDir() string {
	return filepath.Join(config.ConfigDir(), "hooks")
}

func memoryIndexPath() string {
	return filepath.Join(stateDir(), "memory_index.json")
}

func daemonStatePath() string {
	return filepath.Join(stateDir(), "daemon.json")
}

func logFilePath() string {
	return filepath.Join(config.ConfigDir(), "highclaw.log")
}

func ensureStateDir() error {
	return os.MkdirAll(stateDir(), 0o755)
}

func loadCronTasks() ([]cronTask, error) {
	var tasks []cronTask
	if err := readJSONFile(cronTasksPath(), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func saveCronTasks(tasks []cronTask) error {
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return writeJSONFile(cronTasksPath(), tasks)
}

func loadPluginsState() ([]string, error) {
	var plugins []string
	if err := readJSONFile(pluginsPath(), &plugins); err != nil {
		return nil, err
	}
	return plugins, nil
}

func savePluginsState(plugins []string) error {
	return writeJSONFile(pluginsPath(), uniqueStrings(plugins))
}

func loadExecApprovals() ([]execApproval, error) {
	var items []execApproval
	if err := readJSONFile(execApprovalsPath(), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func updateExecApproval(id, status string) error {
	items, err := loadExecApprovals()
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].ID == id {
			items[i].Status = status
			items[i].UpdatedAt = time.Now()
			if err := writeJSONFile(execApprovalsPath(), items); err != nil {
				return err
			}
			fmt.Printf("%s: %s\n", status, id)
			return nil
		}
	}
	return fmt.Errorf("approval not found: %s", id)
}

func listHookFiles() ([]string, error) {
	entries, err := os.ReadDir(hooksDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		out = append(out, filepath.Join(hooksDir(), e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func loadMemoryIndex() ([]memoryRecord, error) {
	var records []memoryRecord
	if err := readJSONFile(memoryIndexPath(), &records); err != nil {
		return nil, err
	}
	return records, nil
}

func saveMemoryIndex(records []memoryRecord) error {
	sort.Slice(records, func(i, j int) bool { return records[i].SessionKey < records[j].SessionKey })
	return writeJSONFile(memoryIndexPath(), records)
}

func loadDaemonState() (daemonState, error) {
	var st daemonState
	if err := readJSONFile(daemonStatePath(), &st); err != nil {
		return daemonState{}, err
	}
	return st, nil
}

func saveDaemonState(st daemonState) error {
	return writeJSONFile(daemonStatePath(), st)
}

func startDaemonProcess() error {
	st, err := loadDaemonState()
	if err != nil {
		return err
	}
	if !st.Installed {
		return fmt.Errorf("daemon not installed; run `highclaw daemon install` first")
	}
	if processRunning(st.PID) {
		return fmt.Errorf("daemon is already running (pid=%d)", st.PID)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		return err
	}
	logF, err := os.OpenFile(logFilePath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "gateway")
	cmd.Stdout = logF
	cmd.Stderr = logF
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return err
	}
	st.PID = cmd.Process.Pid
	st.StartedAt = time.Now()
	if err := saveDaemonState(st); err != nil {
		return err
	}
	fmt.Printf("daemon started (pid=%d)\n", st.PID)
	return nil
}

func stopDaemonProcess() error {
	st, err := loadDaemonState()
	if err != nil {
		return err
	}
	if st.PID <= 0 {
		return fmt.Errorf("no running daemon pid found")
	}
	p, err := os.FindProcess(st.PID)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	st.PID = 0
	st.StartedAt = time.Time{}
	if err := saveDaemonState(st); err != nil {
		return err
	}
	fmt.Println("daemon stop requested")
	return nil
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func tailLogFile(path string, lines int, follow bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	all := strings.Split(string(data), "\n")
	if lines <= 0 {
		lines = 200
	}
	start := len(all) - lines
	if start < 0 {
		start = 0
	}
	for i := start; i < len(all); i++ {
		if all[i] == "" {
			continue
		}
		fmt.Println(all[i])
	}
	if !follow {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	buf := make([]byte, 4096)
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		n, err := f.Read(buf)
		if n > 0 {
			fmt.Print(string(buf[:n]))
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
	}
}

func queryLogFile(path, pattern string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	q := strings.ToLower(pattern)
	matches := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(strings.ToLower(line), q) {
			fmt.Println(line)
			matches++
		}
	}
	fmt.Printf("matches: %d\n", matches)
	return nil
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func listEntityDir(name string) error {
	dir := filepath.Join(config.ConfigDir(), name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("no %s found (%s)\n", name, dir)
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		fmt.Printf("no %s found (%s)\n", name, dir)
		return nil
	}
	for _, e := range entries {
		fmt.Println(e.Name())
	}
	return nil
}

func modelIDExists(id string) bool {
	for _, m := range model.GetAllModelsComplete() {
		if m.ID == id || (m.Provider+"/"+m.ID) == id {
			return true
		}
	}
	return false
}

func containsString(items []string, item string) bool {
	for _, it := range items {
		if it == item {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it) == "" {
			continue
		}
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		out = append(out, it)
	}
	return out
}

func randomHex(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("invalid random length")
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func readJSONFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, v any) error {
	if err := ensureStateDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func saveCLISession(sessionKey, modelName, userMessage, assistantReply string) error {
	now := time.Now().UnixMilli()
	history := []protocol.ChatMessage{{
		Role:      "user",
		Content:   userMessage,
		Channel:   "cli",
		Timestamp: now,
	}, {
		Role:      "assistant",
		Content:   assistantReply,
		Channel:   "cli",
		Timestamp: time.Now().UnixMilli(),
	}}
	return session.SaveFromHistory(sessionKey, "cli", "main", modelName, history)
}

func resolveSessionKey(input string) (string, error) {
	key := strings.TrimSpace(input)
	if key == "" {
		return "", fmt.Errorf("session key cannot be empty")
	}
	if strings.HasPrefix(key, "agent:") {
		return key, nil
	}
	candidate := fmt.Sprintf("agent:%s:%s", "main", key)
	if _, err := session.Load(candidate); err == nil {
		return candidate, nil
	}
	return key, nil
}

func boolText(ok bool) string {
	if ok {
		return "ok"
	}
	return "missing"
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
