package cli

import (
	"encoding/json"
	"fmt"

	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/tui"
	"github.com/spf13/cobra"
)

// --- Agent Command ---

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Interact with the AI agent",
	Long:  "Send messages to the AI agent and receive responses. Supports RPC and interactive modes.",
}

var agentChatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a chat message to the agent",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Agent chat — not yet implemented")
		return nil
	},
}

var agentRPCCmd = &cobra.Command{
	Use:   "rpc",
	Short: "Start agent in RPC mode (JSON I/O)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Agent RPC mode — not yet implemented")
		return nil
	},
}

// --- Channels Command ---

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage messaging channels (WhatsApp/Telegram/Discord/Slack/Signal...)",
}

var channelsLoginCmd = &cobra.Command{
	Use:   "login [channel]",
	Short: "Login to a messaging channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Channels login [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var channelsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Channels status — not yet implemented")
		return nil
	},
}

var channelsLogoutCmd = &cobra.Command{
	Use:   "logout [channel]",
	Short: "Logout from a messaging channel",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Channels logout [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var channelsSendCmd = &cobra.Command{
	Use:   "send [channel] [recipient] [message]",
	Short: "Send a message through a channel",
	Args:  cobra.MinimumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Channels send — not yet implemented")
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

		// TODO: Implement key-based lookup
		fmt.Printf("Config key lookup not yet implemented: %s\n", args[0])
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Config set [%s]=[%s] — not yet implemented\n", args[0], args[1])
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
		fmt.Println("🦀 Config validate — not yet implemented")
		return nil
	},
}

// --- Doctor Command ---

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics, health checks, and migrations",
	Long:  "Comprehensive diagnostics: config validation, auth check, gateway health, sandbox status, security audit, legacy migrations, workspace integrity.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Doctor — not yet implemented")
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
		if cfg.Channels.Telegram.BotToken != "" {
			fmt.Printf("   Telegram: configured\n")
		}
		if cfg.Channels.Discord.Token != "" {
			fmt.Printf("   Discord: configured\n")
		}
		if cfg.Channels.Slack.BotToken != "" {
			fmt.Printf("   Slack: configured\n")
		}

		// TODO: Query gateway for live status
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
		fmt.Println("🦀 Sessions list — not yet implemented")
		return nil
	},
}

var sessionsGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Sessions get [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete [key]",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Sessions delete [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var sessionsResetCmd = &cobra.Command{
	Use:   "reset [key]",
	Short: "Reset a session's message history",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Sessions reset [%s] — not yet implemented\n", args[0])
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
		fmt.Println("🦀 Cron list — not yet implemented")
		return nil
	},
}

var cronCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a scheduled task",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Cron create — not yet implemented")
		return nil
	},
}

var cronDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Cron delete [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var cronTriggerCmd = &cobra.Command{
	Use:   "trigger [id]",
	Short: "Manually trigger a scheduled task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Cron trigger [%s] — not yet implemented\n", args[0])
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
		fmt.Println("🦀 Skills list — not yet implemented")
		return nil
	},
}

var skillsInstallCmd = &cobra.Command{
	Use:   "install [name-or-url]",
	Short: "Install a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Skills install [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var skillsUninstallCmd = &cobra.Command{
	Use:   "uninstall [name]",
	Short: "Uninstall a skill",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Skills uninstall [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var skillsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show skills status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Skills status — not yet implemented")
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
		fmt.Println("🦀 Plugins list — not yet implemented")
		return nil
	},
}

var pluginsInstallCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Install a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Plugins install [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var pluginsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync plugin versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Plugins sync — not yet implemented")
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
		fmt.Println("🦀 Nodes list — not yet implemented")
		return nil
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
		fmt.Println("🦀 Devices list — not yet implemented")
		return nil
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
		fmt.Println("🦀 Models list — not yet implemented")
		return nil
	},
}

var modelsSetCmd = &cobra.Command{
	Use:   "set [provider/model]",
	Short: "Set the default model",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Models set [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var modelsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan and discover available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Models scan — not yet implemented")
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
		fmt.Println("🦀 Security audit — not yet implemented")
		return nil
	},
}

var securityFixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Auto-fix security issues found by audit",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Security fix — not yet implemented")
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
		fmt.Println("🦀 Exec-approvals list — not yet implemented")
		return nil
	},
}

var execApprovalsApproveCmd = &cobra.Command{
	Use:   "approve [id]",
	Short: "Approve a pending execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Exec-approvals approve [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var execApprovalsDenyCmd = &cobra.Command{
	Use:   "deny [id]",
	Short: "Deny a pending execution",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Exec-approvals deny [%s] — not yet implemented\n", args[0])
		return nil
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
		fmt.Println("🦀 Hooks list — not yet implemented")
		return nil
	},
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install [hook-path]",
	Short: "Install a hook",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🦀 Hooks install [%s] — not yet implemented\n", args[0])
		return nil
	},
}

var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show hooks status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Hooks status — not yet implemented")
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
		fmt.Println("🦀 Logs tail — not yet implemented")
		return nil
	},
}

var logsQueryCmd = &cobra.Command{
	Use:   "query [pattern]",
	Short: "Search historical logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Logs query — not yet implemented")
		return nil
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
		fmt.Println("🦀 Browser open — not yet implemented")
		return nil
	},
}

var browserInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect current browser state",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Browser inspect — not yet implemented")
		return nil
	},
}

// --- Memory Command ---

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage RAG memory (search, sync, reindex)",
}

var memorySearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search memory for relevant context",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Memory search — not yet implemented")
		return nil
	},
}

var memorySyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync memory from session files",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Memory sync — not yet implemented")
		return nil
	},
}

var memoryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show memory backend status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Memory status — not yet implemented")
		return nil
	},
}

var memoryResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset memory index",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Memory reset — not yet implemented")
		return nil
	},
}

// --- Daemon Command ---

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage system daemon (launchd/systemd/schtasks)",
}

var daemonInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install HighClaw as a system daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Daemon install — not yet implemented")
		return nil
	},
}

var daemonUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the system daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Daemon uninstall — not yet implemented")
		return nil
	},
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Daemon start — not yet implemented")
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Daemon stop — not yet implemented")
		return nil
	},
}

var daemonDaemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Daemon status — not yet implemented")
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
		fmt.Println("🦀 Update check — not yet implemented")
		return nil
	},
}

var updateInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the latest update",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Update install — not yet implemented")
		return nil
	},
}

// --- TUI Command ---

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch terminal UI (interactive chat)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

// --- Simple Commands ---

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset HighClaw state (configs, sessions, cache)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Reset — not yet implemented")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall HighClaw and clean up",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Uninstall — not yet implemented")
		return nil
	},
}

var messageCmd = &cobra.Command{
	Use:   "message [text]",
	Short: "Send a single message and print the response",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Message — not yet implemented")
		return nil
	},
}

var webhooksCmd = &cobra.Command{
	Use:   "webhooks",
	Short: "Manage inbound/outbound webhooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Webhooks — not yet implemented")
		return nil
	},
}

var pairingCmd = &cobra.Command{
	Use:   "pairing",
	Short: "Manage device pairing (QR codes, tokens)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Pairing — not yet implemented")
		return nil
	},
}

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS diagnostics and configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 DNS — not yet implemented")
		return nil
	},
}

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Open documentation in browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Docs — not yet implemented")
		return nil
	},
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the web dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🦀 Dashboard — not yet implemented")
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

	// Config subcommands
	configCmdGroup.AddCommand(configGetCmd)
	configCmdGroup.AddCommand(configSetCmd)
	configCmdGroup.AddCommand(configShowCmd)
	configCmdGroup.AddCommand(configValidateCmd)

	// Sessions subcommands
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsGetCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
	sessionsCmd.AddCommand(sessionsResetCmd)

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

	// Update subcommands
	updateCmd.AddCommand(updateCheckCmd)
	updateCmd.AddCommand(updateInstallCmd)
}
