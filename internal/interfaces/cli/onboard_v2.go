package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/model"
	"github.com/spf13/cobra"
)

var onboardV2Cmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive onboarding wizard (v2)",
	Long:  "Complete setup wizard with all providers and channels. Every step can be skipped.",
	RunE:  runOnboardV2,
}

func runOnboardV2(cmd *cobra.Command, args []string) error {
	// Banner
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🦀 HighClaw Onboarding Wizard v2")

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
	if !confirmRisk() {
		fmt.Println("\n❌ Onboarding cancelled.")
		return nil
	}

	// Step 2: Flow selection
	flow := selectFlow()

	// Step 3: Workspace
	workspace := configureWorkspace(cfg, flow)
	if workspace != "" {
		cfg.Agent.Workspace = workspace
	}

	// Step 4: Model provider and model
	provider, modelID := selectModelProvider(cfg, flow)
	if provider != "" && modelID != "" {
		cfg.Agent.Model = fmt.Sprintf("%s/%s", provider, modelID)
	}

	// Step 5: API Key
	apiKey := inputAPIKey(provider)
	if apiKey != "" {
		if cfg.Agent.Providers == nil {
			cfg.Agent.Providers = make(map[string]config.ProviderConfig)
		}
		cfg.Agent.Providers[provider] = config.ProviderConfig{APIKey: apiKey}
	}

	// Step 6: Gateway configuration
	configureGateway(cfg, flow)

	// Step 7: Channels (can configure multiple)
	configureChannels(cfg)

	// Step 8: Save configuration
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create workspace
	if workspace != "" {
		os.MkdirAll(expandPath(workspace), 0o755)
	}

	// Success
	printSuccess(cfg)
	return nil
}

func confirmRisk() bool {
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

func selectFlow() string {
	var flow string
	err := huh.NewSelect[string]().
		Title("Setup Flow").
		Description("Choose your setup experience").
		Options(
			huh.NewOption("QuickStart (recommended, minimal questions)", "quickstart"),
			huh.NewOption("Advanced (full configuration)", "advanced"),
			huh.NewOption("Skip (use defaults)", "skip"),
		).
		Value(&flow).
		Run()

	if err != nil {
		return "skip"
	}
	return flow
}

func configureWorkspace(cfg *config.Config, flow string) string {
	if flow == "quickstart" || flow == "skip" {
		return cfg.Agent.Workspace
	}

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

func selectModelProvider(cfg *config.Config, flow string) (string, string) {
	// Step 1: Select provider
	var provider string
	providerOptions := []huh.Option[string]{
		huh.NewOption("Anthropic Claude (recommended)", "anthropic"),
		huh.NewOption("OpenAI GPT", "openai"),
		huh.NewOption("Google Gemini", "google"),
		huh.NewOption("AWS Bedrock", "bedrock"),
		huh.NewOption("Azure OpenAI", "azure"),
		huh.NewOption("Ollama (local)", "ollama"),
		huh.NewOption("Groq (fast inference)", "groq"),
		huh.NewOption("Together AI", "together"),
		huh.NewOption("Fireworks AI", "fireworks"),
		huh.NewOption("Cohere", "cohere"),
		huh.NewOption("Mistral AI", "mistral"),
		huh.NewOption("Perplexity", "perplexity"),
		huh.NewOption("DeepSeek", "deepseek"),
		huh.NewOption("Skip", "skip"),
	}

	err := huh.NewSelect[string]().
		Title("AI Model Provider").
		Description("Choose your AI provider (Esc to skip)").
		Options(providerOptions...).
		Value(&provider).
		Run()

	if err != nil || provider == "skip" {
		return "", ""
	}

	// Step 2: Select model from provider
	return provider, selectModel(provider)
}

func selectModel(provider string) string {
	var models []model.Model

	switch provider {
	case "anthropic":
		models = model.AnthropicModels
	case "openai":
		models = model.OpenAIModels
	case "google":
		models = model.GoogleModels
	case "bedrock":
		models = model.BedrockModels
	case "azure":
		models = model.AzureModels
	case "ollama":
		models = model.OllamaModels
	case "groq":
		models = model.GroqModels
	case "together":
		models = model.TogetherModels
	case "fireworks":
		models = model.FireworksModels
	case "cohere":
		models = model.CohereModels
	case "mistral":
		models = model.MistralModels
	case "perplexity":
		models = model.PerplexityModels
	case "deepseek":
		models = model.DeepSeekModels
	default:
		return ""
	}

	options := make([]huh.Option[string], 0, len(models)+1)
	for _, m := range models {
		desc := m.Description
		if len(m.Capabilities) > 0 {
			desc += fmt.Sprintf(" [%s]", strings.Join(m.Capabilities, ", "))
		}
		options = append(options, huh.NewOption(m.Name+" - "+desc, m.ID))
	}
	options = append(options, huh.NewOption("Skip", "skip"))

	var modelID string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Select %s Model", strings.Title(provider))).
		Description("Choose a model (Esc to skip)").
		Options(options...).
		Value(&modelID).
		Run()

	if err != nil || modelID == "skip" {
		return ""
	}

	return modelID
}

func inputAPIKey(provider string) string {
	if provider == "" || provider == "ollama" {
		return "" // Ollama doesn't need API key
	}

	var apiKey string
	providerName := map[string]string{
		"anthropic":  "Anthropic",
		"openai":     "OpenAI",
		"google":     "Google",
		"bedrock":    "AWS",
		"azure":      "Azure",
		"groq":       "Groq",
		"together":   "Together AI",
		"fireworks":  "Fireworks AI",
		"cohere":     "Cohere",
		"mistral":    "Mistral AI",
		"perplexity": "Perplexity",
		"deepseek":   "DeepSeek",
	}[provider]

	err := huh.NewInput().
		Title(fmt.Sprintf("%s API Key", providerName)).
		Description(fmt.Sprintf("Enter your %s API key (Esc to skip)", providerName)).
		Value(&apiKey).
		EchoMode(huh.EchoModePassword).
		Run()

	if err != nil {
		return ""
	}

	return apiKey
}

func configureGateway(cfg *config.Config, flow string) {
	if flow == "quickstart" || flow == "skip" {
		cfg.Gateway.Port = 18790
		cfg.Gateway.Bind = "loopback"
		cfg.Gateway.Mode = "local"
		cfg.Gateway.Auth = config.GatewayAuth{Mode: "token"}
		return
	}

	// Advanced: ask for settings
	var portStr string
	huh.NewInput().
		Title("Gateway Port").
		Description("Port for the gateway server (Esc for default 18790)").
		Value(&portStr).
		Placeholder("18790").
		Run()

	port := 18790
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}
	cfg.Gateway.Port = port

	var bind string
	huh.NewSelect[string]().
		Title("Gateway Bind").
		Options(
			huh.NewOption("Loopback (127.0.0.1, recommended)", "loopback"),
			huh.NewOption("All interfaces (0.0.0.0)", "all"),
			huh.NewOption("Tailnet", "tailnet"),
		).
		Value(&bind).
		Run()

	if bind != "" {
		cfg.Gateway.Bind = bind
	}
}

func configureChannels(cfg *config.Config) {
	for {
		var setupMore bool
		err := huh.NewConfirm().
			Title("Setup Messaging Channels?").
			Description("Configure Telegram, WhatsApp, Discord, etc.").
			Value(&setupMore).
			Run()

		if err != nil || !setupMore {
			break
		}

		channel := selectChannel()
		if channel == "done" {
			break
		}

		switch channel {
		case "telegram":
			configureTelegram(cfg)
		case "whatsapp":
			configureWhatsApp(cfg)
		case "discord":
			configureDiscord(cfg)
		case "slack":
			configureSlack(cfg)
		}
	}
}

func selectChannel() string {
	var channel string
	huh.NewSelect[string]().
		Title("Which channel?").
		Options(
			huh.NewOption("Telegram Bot", "telegram"),
			huh.NewOption("WhatsApp", "whatsapp"),
			huh.NewOption("Discord", "discord"),
			huh.NewOption("Slack", "slack"),
			huh.NewOption("Done / Skip", "done"),
		).
		Value(&channel).
		Run()

	return channel
}

func configureTelegram(cfg *config.Config) {
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

func configureWhatsApp(cfg *config.Config) {
	fmt.Println("\n📱 WhatsApp will be configured via QR code after gateway starts.")
	fmt.Println("   Run: highclaw channels login whatsapp")
}

func configureDiscord(cfg *config.Config) {
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

func configureSlack(cfg *config.Config) {
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

func printSuccess(cfg *config.Config) {
	fmt.Println()
	successStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42"))

	fmt.Println(successStyle.Render("✅ Onboarding complete!"))
	fmt.Println()
	fmt.Println("Configuration saved to:", config.ConfigPath())
	fmt.Println("Workspace:", cfg.Agent.Workspace)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the gateway:  highclaw gateway")
	fmt.Println("  2. Check status:       highclaw status")
	fmt.Println("  3. Open TUI:           highclaw tui")
	fmt.Println("  4. Open Web UI:        http://localhost:18790")
	fmt.Println()
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
