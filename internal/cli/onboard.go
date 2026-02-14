package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/spf13/cobra"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive onboarding wizard",
	Long:  "Step-by-step setup: auth provider, model selection, gateway config, channel login, skills installation.",
	RunE:  runOnboard,
}

func runOnboard(cmd *cobra.Command, args []string) error {
	// Print banner
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🦀 HighClaw Onboarding Wizard")

	fmt.Println()
	fmt.Println(banner)
	fmt.Println()
	fmt.Println("Welcome! Let's set up your AI assistant gateway.")
	fmt.Println()

	// Load existing config or create new
	cfg, err := config.Load()
	if err != nil {
		// Create default config
		cfg = config.DefaultConfig()
	}

	// Step 1: Risk acknowledgement
	var acceptRisk bool
	err = huh.NewConfirm().
		Title("⚠️  Security Warning").
		Description("HighClaw agents have full system access and can execute commands.\n" +
			"This is powerful but inherently risky.\n" +
			"Must read: https://docs.openclaw.ai/gateway/security").
		Value(&acceptRisk).
		Run()

	if err != nil || !acceptRisk {
		fmt.Println("\n❌ Onboarding cancelled. Risk not accepted.")
		return nil
	}

	// Step 2: Choose flow
	var flow string
	err = huh.NewSelect[string]().
		Title("Setup Flow").
		Options(
			huh.NewOption("QuickStart (recommended)", "quickstart"),
			huh.NewOption("Advanced (full configuration)", "advanced"),
		).
		Value(&flow).
		Run()

	if err != nil {
		return err
	}

	// Step 3: Workspace directory
	var workspace string
	defaultWorkspace := "~/.highclaw/workspace"
	if cfg.Agent.Workspace != "" {
		defaultWorkspace = cfg.Agent.Workspace
	}

	err = huh.NewInput().
		Title("Workspace Directory").
		Description("Where should the agent store its files?").
		Value(&workspace).
		Placeholder(defaultWorkspace).
		Run()

	if err != nil {
		return err
	}

	if workspace == "" {
		workspace = defaultWorkspace
	}
	workspace = expandPath(workspace)

	// Step 4: Model provider
	var provider string
	err = huh.NewSelect[string]().
		Title("AI Model Provider").
		Options(
			huh.NewOption("Anthropic Claude (recommended)", "anthropic"),
			huh.NewOption("OpenAI", "openai"),
			huh.NewOption("Google Gemini", "gemini"),
		).
		Value(&provider).
		Run()

	if err != nil {
		return err
	}

	// Step 5: API Key
	var apiKey string
	providerName := map[string]string{
		"anthropic": "Anthropic",
		"openai":    "OpenAI",
		"gemini":    "Google",
	}[provider]

	err = huh.NewInput().
		Title(fmt.Sprintf("%s API Key", providerName)).
		Description(fmt.Sprintf("Get your API key from %s console", providerName)).
		Value(&apiKey).
		EchoMode(huh.EchoModePassword).
		Run()

	if err != nil {
		return err
	}

	// Step 6: Model selection
	var model string
	modelOptions := map[string][]huh.Option[string]{
		"anthropic": {
			huh.NewOption("Claude Opus 4 (most capable)", "claude-opus-4"),
			huh.NewOption("Claude Sonnet 4 (balanced)", "claude-sonnet-4"),
			huh.NewOption("Claude Haiku 4 (fast)", "claude-haiku-4"),
		},
		"openai": {
			huh.NewOption("GPT-4o (recommended)", "gpt-4o"),
			huh.NewOption("GPT-4 Turbo", "gpt-4-turbo"),
			huh.NewOption("GPT-3.5 Turbo", "gpt-3.5-turbo"),
		},
		"gemini": {
			huh.NewOption("Gemini Pro", "gemini-pro"),
			huh.NewOption("Gemini Ultra", "gemini-ultra"),
		},
	}

	err = huh.NewSelect[string]().
		Title("Select Model").
		Options(modelOptions[provider]...).
		Value(&model).
		Run()

	if err != nil {
		return err
	}

	// Build final config
	cfg.Agent.Workspace = workspace
	cfg.Agent.Model = fmt.Sprintf("%s/%s", provider, model)

	// Initialize providers map if needed
	if cfg.Agent.Providers == nil {
		cfg.Agent.Providers = make(map[string]config.ProviderConfig)
	}
	cfg.Agent.Providers[provider] = config.ProviderConfig{
		APIKey: apiKey,
	}

	// Step 7: Gateway configuration (quickstart vs advanced)
	if flow == "quickstart" {
		// Use defaults
		cfg.Gateway.Port = 18790
		cfg.Gateway.Bind = "loopback"
		cfg.Gateway.Mode = "local"
		cfg.Gateway.Auth = config.GatewayAuth{
			Mode: "token",
		}
	} else {
		// Advanced: ask for gateway settings
		var portStr string
		err = huh.NewInput().
			Title("Gateway Port").
			Value(&portStr).
			Placeholder("18790").
			Run()
		if err != nil {
			return err
		}
		port := 18790
		if portStr != "" {
			fmt.Sscanf(portStr, "%d", &port)
		}
		cfg.Gateway.Port = port

		var bind string
		err = huh.NewSelect[string]().
			Title("Gateway Bind").
			Options(
				huh.NewOption("Loopback (127.0.0.1)", "loopback"),
				huh.NewOption("All interfaces (0.0.0.0)", "all"),
				huh.NewOption("Tailnet", "tailnet"),
			).
			Value(&bind).
			Run()
		if err != nil {
			return err
		}
		cfg.Gateway.Bind = bind
	}

	// Step 8: Channel setup (optional)
	var setupChannels bool
	err = huh.NewConfirm().
		Title("Setup Messaging Channels?").
		Description("Configure Telegram, WhatsApp, Discord, etc.").
		Value(&setupChannels).
		Run()

	if err != nil {
		return err
	}

	if setupChannels {
		var channelType string
		err = huh.NewSelect[string]().
			Title("Which channel?").
			Options(
				huh.NewOption("Telegram Bot", "telegram"),
				huh.NewOption("WhatsApp", "whatsapp"),
				huh.NewOption("Discord", "discord"),
				huh.NewOption("Skip for now", "skip"),
			).
			Value(&channelType).
			Run()

		if err != nil {
			return err
		}

		if channelType == "telegram" {
			var botToken string
			err = huh.NewInput().
				Title("Telegram Bot Token").
				Description("Get from @BotFather on Telegram").
				Value(&botToken).
				EchoMode(huh.EchoModePassword).
				Run()

			if err != nil {
				return err
			}

			var allowFrom string
			err = huh.NewInput().
				Title("Allowed Telegram Username").
				Description("Your Telegram username (e.g., @yourname)").
				Value(&allowFrom).
				Run()

			if err != nil {
				return err
			}

			cfg.Channels.Telegram.BotToken = botToken
			cfg.Channels.Telegram.AllowFrom = []string{allowFrom}
		}
	}

	// Save configuration
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create workspace directory
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Success message
	fmt.Println()
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42"))

	fmt.Println(successStyle.Render("✅ Onboarding complete!"))
	fmt.Println()
	fmt.Println("Configuration saved to:", config.ConfigPath())
	fmt.Println("Workspace created at:", workspace)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the gateway:  highclaw gateway")
	fmt.Println("  2. Check status:       highclaw status")
	fmt.Println("  3. Open TUI:           highclaw tui")
	fmt.Println()

	return nil
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
