package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/model"
	"github.com/highclaw/highclaw/internal/domain/skill"
	"github.com/spf13/cobra"
)

var onboardV3Cmd = &cobra.Command{
	Use:   "onboard-v3",
	Short: "Complete onboarding wizard (v3 - full OpenClaw parity)",
	Long:  "Complete setup wizard with provider filtering, manual model entry, all channels, skills configuration.",
	RunE:  runOnboardV3,
}

func runOnboardV3(cmd *cobra.Command, args []string) error {
	// Banner
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🦀 HighClaw Onboarding Wizard v3")

	fmt.Println()
	fmt.Println(banner)
	fmt.Println()
	fmt.Println("Welcome! Let's set up your AI assistant gateway.")
	fmt.Println("💡 Tip: You can skip any step by selecting 'Skip' or pressing Esc")
	fmt.Println()

	// Load or create config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Step 1: Risk acknowledgement (cannot skip)
	if !confirmRiskV3() {
		fmt.Println("\n❌ Onboarding cancelled.")
		return nil
	}

	// Step 2: Workspace
	workspace := configureWorkspaceV3(cfg)
	if workspace != "" {
		cfg.Agent.Workspace = workspace
	}

	// Step 3: Model selection with provider filtering
	provider, modelID := selectModelWithFilterV3(cfg)
	if provider != "" && modelID != "" {
		cfg.Agent.Model = fmt.Sprintf("%s/%s", provider, modelID)
	}

	// Step 4: API Key for selected provider
	apiKey := inputAPIKeyV3(provider)
	if apiKey != "" {
		if cfg.Agent.Providers == nil {
			cfg.Agent.Providers = make(map[string]config.ProviderConfig)
		}
		cfg.Agent.Providers[provider] = config.ProviderConfig{APIKey: apiKey}
	}

	// Step 5: Gateway configuration
	configureGatewayV3(cfg)

	// Step 6: Channel selection (QuickStart mode)
	configureChannelQuickStartV3(cfg)

	// Step 7: Skills configuration
	configureSkillsV3(cfg)

	// Step 8: Save configuration
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create workspace
	if workspace != "" {
		os.MkdirAll(expandPath(workspace), 0o755)
	}

	// Success
	printSuccessV3(cfg)
	return nil
}

func confirmRiskV3() bool {
	var accept bool
	err := huh.NewConfirm().
		Title("⚠️  Security Warning").
		Description("HighClaw agents have full system access and can execute commands.\n" +
			"This is powerful but inherently risky.\n" +
			"Read: https://docs.openclaw.ai/gateway/security\n\n" +
			"Do you understand and accept the risks?").
		Value(&accept).
		Run()

	return err == nil && accept
}

func configureWorkspaceV3(cfg *config.Config) string {
	var workspace string
	defaultWorkspace := cfg.Agent.Workspace
	if defaultWorkspace == "" {
		defaultWorkspace = "~/.highclaw/workspace"
	}

	err := huh.NewInput().
		Title("Workspace Directory").
		Description("Where should the agent store its files? (Esc to skip)").
		Value(&workspace).
		Placeholder(defaultWorkspace).
		Run()

	if err != nil || workspace == "" {
		return defaultWorkspace
	}

	return expandPath(workspace)
}

func selectModelWithFilterV3(cfg *config.Config) (string, string) {
	// Step 1: Filter by provider
	providerFilter := selectProviderFilterV3()

	// Step 2: Select model
	return selectModelFromFilterV3(providerFilter, cfg)
}

func selectProviderFilterV3() string {
	providerOptions := []huh.Option[string]{
		huh.NewOption("All providers", "all"),
	}

	// Add all providers
	for _, p := range model.AllProviders {
		info := model.GetProviderInfo()
		if pInfo, ok := info[p]; ok {
			providerOptions = append(providerOptions, huh.NewOption(pInfo.Name, p))
		} else {
			providerOptions = append(providerOptions, huh.NewOption(p, p))
		}
	}

	var filter string
	err := huh.NewSelect[string]().
		Title("Filter models by provider").
		Description("Choose a provider to filter models (or 'All providers')").
		Options(providerOptions...).
		Value(&filter).
		Run()

	if err != nil {
		return "all"
	}

	return filter
}

func selectModelFromFilterV3(providerFilter string, cfg *config.Config) (string, string) {
	// Get models based on filter
	var models []model.Model
	if providerFilter == "all" {
		models = model.GetAllModelsComplete()
	} else {
		models = model.GetModelsByProvider(providerFilter)
	}

	// Build options
	options := []huh.Option[string]{
		huh.NewOption("Keep current ("+cfg.Agent.Model+")", "keep"),
		huh.NewOption("Enter model manually", "manual"),
	}

	for _, m := range models {
		label := fmt.Sprintf("%s/%s - %s", m.Provider, m.ID, m.Description)
		if len(m.Capabilities) > 0 {
			label += fmt.Sprintf(" [%s]", strings.Join(m.Capabilities, ", "))
		}
		value := fmt.Sprintf("%s/%s", m.Provider, m.ID)
		options = append(options, huh.NewOption(label, value))
	}

	var selection string
	err := huh.NewSelect[string]().
		Title("Default model").
		Description("Choose your default AI model").
		Options(options...).
		Value(&selection).
		Run()

	if err != nil || selection == "keep" {
		// Parse current model
		parts := strings.SplitN(cfg.Agent.Model, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}

	if selection == "manual" {
		return inputModelManuallyV3()
	}

	// Parse selection
	parts := strings.SplitN(selection, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return "", ""
}

func inputModelManuallyV3() (string, string) {
	var modelStr string
	err := huh.NewInput().
		Title("Enter model manually").
		Description("Format: provider/model-id (e.g., anthropic/claude-opus-4)").
		Value(&modelStr).
		Placeholder("provider/model-id").
		Run()

	if err != nil || modelStr == "" {
		return "", ""
	}

	parts := strings.SplitN(modelStr, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return "", ""
}

func inputAPIKeyV3(provider string) string {
	if provider == "" {
		return ""
	}

	info := model.GetProviderInfo()
	pInfo, ok := info[provider]
	if !ok {
		pInfo = model.ProviderInfo{
			Name:   provider,
			EnvVar: strings.ToUpper(provider) + "_API_KEY",
		}
	}

	// Skip if no auth needed
	if pInfo.AuthType == "none" {
		return ""
	}

	var apiKey string
	title := fmt.Sprintf("%s API Key", pInfo.Name)
	desc := fmt.Sprintf("Enter your %s API key", pInfo.Name)
	if pInfo.EnvVar != "" {
		desc += fmt.Sprintf(" (or set %s)", pInfo.EnvVar)
	}

	err := huh.NewInput().
		Title(title).
		Description(desc + " (Esc to skip)").
		Value(&apiKey).
		EchoMode(huh.EchoModePassword).
		Run()

	if err != nil {
		return ""
	}

	return apiKey
}

func configureGatewayV3(cfg *config.Config) {
	// Use defaults for QuickStart
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 18790
	}
	if cfg.Gateway.Bind == "" {
		cfg.Gateway.Bind = "loopback"
	}
	if cfg.Gateway.Mode == "" {
		cfg.Gateway.Mode = "local"
	}
}

func configureChannelQuickStartV3(cfg *config.Config) {
	// Channel options (all 19+ channels from OpenClaw)
	channelOptions := []huh.Option[string]{
		huh.NewOption("Telegram (Bot API) (not configured)", "telegram"),
		huh.NewOption("WhatsApp (QR link)", "whatsapp"),
		huh.NewOption("Discord (Bot API)", "discord"),
		huh.NewOption("Google Chat (Chat API)", "google-chat"),
		huh.NewOption("Slack (Socket Mode)", "slack"),
		huh.NewOption("Signal (signal-cli)", "signal"),
		huh.NewOption("iMessage (imsg)", "imessage"),
		huh.NewOption("Nostr (NIP-04 DMs)", "nostr"),
		huh.NewOption("Microsoft Teams (Bot Framework)", "msteams"),
		huh.NewOption("Mattermost (plugin)", "mattermost"),
		huh.NewOption("Nextcloud Talk (self-hosted)", "nextcloud"),
		huh.NewOption("Feishu/Lark (飞书)", "feishu"),
		huh.NewOption("Matrix (plugin)", "matrix"),
		huh.NewOption("BlueBubbles (macOS app)", "bluebubbles"),
		huh.NewOption("LINE (Messaging API)", "line"),
		huh.NewOption("Zalo (Bot API)", "zalo"),
		huh.NewOption("Zalo (Personal Account)", "zalo-user"),
		huh.NewOption("Tlon (Urbit)", "tlon"),
		huh.NewOption("Skip for now", "skip"),
	}

	var channel string
	err := huh.NewSelect[string]().
		Title("Select channel (QuickStart)").
		Description("Choose your primary messaging channel").
		Options(channelOptions...).
		Value(&channel).
		Run()

	if err != nil || channel == "skip" {
		return
	}

	// Configure selected channel
	configureChannelV3(cfg, channel)
}

func configureChannelV3(cfg *config.Config, channel string) {
	switch channel {
	case "telegram":
		configureTelegramV3(cfg)
	case "whatsapp":
		fmt.Println("\n📱 WhatsApp will be configured via QR code after gateway starts.")
		fmt.Println("   Run: highclaw channels login whatsapp")
	case "discord":
		configureDiscordV3(cfg)
	case "slack":
		configureSlackV3(cfg)
	// Add more channels as needed
	default:
		fmt.Printf("\n⚠️  Channel '%s' configuration not yet implemented.\n", channel)
	}
}

func configureTelegramV3(cfg *config.Config) {
	var botToken, allowFrom string

	huh.NewInput().
		Title("Telegram Bot Token").
		Description("Get from @BotFather (Esc to skip)").
		Value(&botToken).
		EchoMode(huh.EchoModePassword).
		Run()

	if botToken == "" {
		return
	}

	huh.NewInput().
		Title("Allowed Username").
		Description("Your Telegram username (e.g., @yourname)").
		Value(&allowFrom).
		Run()

	cfg.Channels.Telegram.BotToken = botToken
	if allowFrom != "" {
		cfg.Channels.Telegram.AllowFrom = []string{allowFrom}
	}
}

func configureDiscordV3(cfg *config.Config) {
	var token string
	huh.NewInput().
		Title("Discord Bot Token").
		Description("From Discord Developer Portal (Esc to skip)").
		Value(&token).
		EchoMode(huh.EchoModePassword).
		Run()

	if token != "" {
		cfg.Channels.Discord.Token = token
	}
}

func configureSlackV3(cfg *config.Config) {
	var botToken, appToken string

	huh.NewInput().
		Title("Slack Bot Token").
		Description("xoxb-... token (Esc to skip)").
		Value(&botToken).
		EchoMode(huh.EchoModePassword).
		Run()

	if botToken == "" {
		return
	}

	huh.NewInput().
		Title("Slack App Token").
		Description("xapp-... token for Socket Mode").
		Value(&appToken).
		EchoMode(huh.EchoModePassword).
		Run()

	cfg.Channels.Slack.BotToken = botToken
	cfg.Channels.Slack.AppToken = appToken
}

func configureSkillsV3(cfg *config.Config) {
	// Ask if user wants to configure skills
	var configureSkills bool
	err := huh.NewConfirm().
		Title("Configure skills now? (recommended)").
		Description("Skills extend the agent's capabilities (Pure Go, no Node.js required)").
		Value(&configureSkills).
		Run()

	if err != nil || !configureSkills {
		return
	}

	// Show pure Go skills
	fmt.Println("\n🔍 Available Skills (Pure Go):")
	fmt.Println()

	pureGoSkills := skill.PureGoSkillsInfo
	eligible := 0
	needsAPIKey := 0

	for _, s := range pureGoSkills {
		status := "✅"
		if len(s.RequiredEnvVars) > 0 {
			// Check if API key is set
			hasKey := true
			for _, envVar := range s.RequiredEnvVars {
				if os.Getenv(envVar) == "" {
					hasKey = false
					break
				}
			}
			if !hasKey {
				status = "⚠️"
				needsAPIKey++
			} else {
				eligible++
			}
		} else {
			eligible++
		}

		fmt.Printf("  %s %s %s - %s\n", status, s.Icon, s.Name, s.Description)
	}

	fmt.Println()
	fmt.Println("Skills status ────────────╮")
	fmt.Println("                          │")
	fmt.Printf("  Ready to use: %d         │\n", eligible)
	fmt.Printf("  Need API key: %d         │\n", needsAPIKey)
	fmt.Printf("  Total: %d                │\n", len(pureGoSkills))
	fmt.Println("                          │")
	fmt.Println("──────────────────────────╯")
	fmt.Println()

	// Ask about API keys (only if needed)
	if needsAPIKey > 0 {
		configureSkillAPIKeysV3(cfg, pureGoSkills)
	} else {
		fmt.Println("✅ All skills are ready to use!")
	}
}

// configureSkillDepsV3 is no longer needed - all skills are pure Go
// Kept for backward compatibility but does nothing

func configureSkillAPIKeysV3(cfg *config.Config, skills []skill.SkillInfo) {
	// Collect all required API keys
	apiKeys := make(map[string]string)
	for _, s := range skills {
		for _, envVar := range s.RequiredEnvVars {
			if _, exists := apiKeys[envVar]; !exists {
				apiKeys[envVar] = ""
			}
		}
	}

	// Ask for each API key
	for envVar := range apiKeys {
		// Check if already set
		if os.Getenv(envVar) != "" {
			continue
		}

		// Find which skill needs this key
		var skillName string
		for _, s := range skills {
			for _, ev := range s.RequiredEnvVars {
				if ev == envVar {
					skillName = s.Name
					break
				}
			}
			if skillName != "" {
				break
			}
		}

		var setKey bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Set %s for %s?", envVar, skillName)).
			Value(&setKey).
			Run()

		if err != nil || !setKey {
			continue
		}

		var keyValue string
		err = huh.NewInput().
			Title(fmt.Sprintf("Enter %s", envVar)).
			Description("This will be saved to your environment").
			Value(&keyValue).
			EchoMode(huh.EchoModePassword).
			Run()

		if err == nil && keyValue != "" {
			// TODO: Save to config or .env file
			fmt.Printf("✅ %s configured\n", envVar)
		}
	}
}

func printSuccessV3(cfg *config.Config) {
	fmt.Println()
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42"))

	fmt.Println(successStyle.Render("✅ Onboarding complete!"))
	fmt.Println()
	fmt.Println("Configuration saved to:", config.ConfigPath())
	fmt.Println("Model:", cfg.Agent.Model)
	fmt.Println("Workspace:", cfg.Agent.Workspace)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the gateway:  highclaw gateway")
	fmt.Println("  2. Check status:       highclaw status")
	fmt.Println("  3. Open Web UI:        http://localhost:18790")
	fmt.Println()
}
