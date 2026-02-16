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
	"github.com/highclaw/highclaw/internal/domain/skill"
	"github.com/spf13/cobra"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive onboarding wizard",
	Long:  "Interactive wizard to set up the gateway, workspace, and skills.",
	RunE:  runOnboard,
}

func runOnboard(cmd *cobra.Command, args []string) error {
	// Banner
	printWizardHeader()

	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	// Step 1: Risk acknowledgement
	if !confirmRisk() {
		fmt.Println("\n❌ Onboarding cancelled.")
		return nil
	}

	// Step 2: Flow Selection
	flow := selectFlow()

	// Step 3: Mode Selection
	configureMode(cfg, flow)

	// Step 4: Workspace
	workspace := configureWorkspace(cfg)
	if workspace != "" {
		cfg.Agent.Workspace = workspace
	}

	// Step 5: Auth Choice & Model
	configureAuthAndModel(cfg)

	// Step 6: Gateway Configuration
	configureGateway(cfg, flow)

	// Step 7: Channel Setup
	configureChannels(cfg, flow)

	// Step 8: Skills Setup
	configureSkills(cfg)

	// Save configuration
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

func printWizardHeader() {
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render("🦀 OpenClaw Onboarding Wizard")

	fmt.Println()
	fmt.Println(banner)
	fmt.Println()
	fmt.Println("Welcome! Let's set up your AI assistant gateway.")
	fmt.Println("💡 Tip: You can skip any step by selecting 'Skip' or pressing Esc")
	fmt.Println()
}

func confirmRisk() bool {
	var accept bool
	err := huh.NewConfirm().
		Title("⚠️  Security Warning").
		Description("OpenClaw agents have full system access and can execute commands.\n" +
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
		Title("Onboarding mode").
		Options(
			huh.NewOption("QuickStart (Recommended)", "quickstart"),
			huh.NewOption("Manual / Advanced", "advanced"),
		).
		Value(&flow).
		Run()

	if err != nil {
		return "quickstart"
	}
	return flow
}

func configureMode(cfg *config.Config, flow string) {
	if flow == "quickstart" {
		cfg.Gateway.Mode = "local"
		return
	}

	var mode string
	huh.NewSelect[string]().
		Title("What do you want to set up?").
		Options(
			huh.NewOption("Local gateway (this machine)", "local"),
			huh.NewOption("Remote gateway (info-only)", "remote"),
		).
		Value(&mode).
		Run()

	if mode != "" {
		cfg.Gateway.Mode = mode
	}
}

func configureWorkspace(cfg *config.Config) string {
	var workspace string
	defaultWorkspace := cfg.Agent.Workspace
	if defaultWorkspace == "" {
		defaultWorkspace = "~/.openclaw/workspace"
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

func configureAuthAndModel(cfg *config.Config) {
	// Provider selection
	provider := selectAuthChoice()
	if provider == "" || provider == "skip" {
		return
	}

	// Model selection
	modelID := selectModelForProvider(provider, cfg)
	if modelID != "" {
		cfg.Agent.Model = fmt.Sprintf("%s/%s", provider, modelID)
	}

	// API Key
	apiKey := inputAPIKey(provider)
	if apiKey != "" {
		if cfg.Agent.Providers == nil {
			cfg.Agent.Providers = make(map[string]config.ProviderConfig)
		}
		cfg.Agent.Providers[provider] = config.ProviderConfig{APIKey: apiKey}
	}
}

func selectAuthChoice() string {
	var choice string

	// Common options
	options := []huh.Option[string]{
		huh.NewOption("Anthropic (Claude)", "anthropic"),
		huh.NewOption("OpenAI (GPT-4)", "openai"),
		huh.NewOption("Ollama (Local)", "ollama"),
		huh.NewOption("Google (Gemini)", "google"),
		huh.NewOption("Groq", "groq"),
		huh.NewOption("DeepSeek", "deepseek"),
		huh.NewOption("Mistral", "mistral"),
		huh.NewOption("Together AI", "together"),
		huh.NewOption("OpenRouter", "openrouter"),
		huh.NewOption("Other...", "other"),
		huh.NewOption("Skip", "skip"),
	}

	err := huh.NewSelect[string]().
		Title("Authentication Provider").
		Description("Choose your primary AI inference provider").
		Options(options...).
		Value(&choice).
		Run()

	if err != nil {
		return "skip"
	}

	if choice == "other" {
		// Show full list if "Other" selected
		var fullChoice string
		var fullOptions []huh.Option[string]
		for _, p := range model.AllProviders {
			fullOptions = append(fullOptions, huh.NewOption(p, p))
		}
		huh.NewSelect[string]().
			Title("All Providers").
			Options(fullOptions...).
			Value(&fullChoice).
			Run()
		return fullChoice
	}

	return choice
}

func selectModelForProvider(provider string, cfg *config.Config) string {
	// Use helper instead of model.GetModelsByProvider
	models := getModelsByProvider(provider)
	if len(models) == 0 {
		// If no models known for this provider (e.g. custom), ask for manual input
		return inputModelManually()
	}

	options := []huh.Option[string]{
		huh.NewOption("Enter model manually", "manual"),
	}

	for _, m := range models {
		label := fmt.Sprintf("%s - %s", m.ID, m.Description)
		options = append(options, huh.NewOption(label, m.ID))
	}

	var selection string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Select %s Model", provider)).
		Options(options...).
		Value(&selection).
		Run()

	if err != nil {
		return ""
	}

	if selection == "manual" {
		return inputModelManually()
	}

	return selection
}

func getModelsByProvider(provider string) []model.Model {
	var result []model.Model
	all := model.GetAllModels()
	for _, m := range all {
		if m.Provider == provider {
			result = append(result, m)
		}
	}
	return result
}

func inputModelManually() string {
	var modelID string
	huh.NewInput().
		Title("Enter Model ID").
		Description("e.g. claude-3-opus-20240229").
		Value(&modelID).
		Run()
	return modelID
}

func inputAPIKey(provider string) string {
	var apiKey string
	huh.NewInput().
		Title(fmt.Sprintf("%s API Key", provider)).
		Description("Enter your API key (Esc to skip)").
		Value(&apiKey).
		EchoMode(huh.EchoModePassword).
		Run()
	return apiKey
}

func configureGateway(cfg *config.Config, flow string) {
	if flow == "quickstart" {
		if cfg.Gateway.Port == 0 {
			cfg.Gateway.Port = 18790
		}
		if cfg.Gateway.Bind == "" {
			cfg.Gateway.Bind = "loopback"
		}
		return
	}

	// Advanced gateway config
	// Port
	var portStr string
	huh.NewInput().
		Title("Gateway Port").
		Value(&portStr).
		Placeholder("18790").
		Run()

	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &cfg.Gateway.Port)
	}
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 18790
	}

	// Bind
	var bind string
	huh.NewSelect[string]().
		Title("Gateway Bind Address").
		Options(
			huh.NewOption("Loopback (127.0.0.1)", "loopback"),
			huh.NewOption("All Interfaces (0.0.0.0)", "all"),
		).
		Value(&bind).
		Run()
	if bind != "" {
		cfg.Gateway.Bind = bind
	}
}

func configureChannels(cfg *config.Config, flow string) {
	if flow == "quickstart" {
		configureChannelQuickStart(cfg)
	} else {
		// Advanced: loop through channels or show menu?
		// For now, reuse QuickStart selection but maybe allow multiple?
		configureChannelQuickStart(cfg)
	}
}

func configureChannelQuickStart(cfg *config.Config) {
	channelOptions := []huh.Option[string]{
		huh.NewOption("Telegram (Bot API)", "telegram"),
		huh.NewOption("WhatsApp (QR link)", "whatsapp"),
		huh.NewOption("Discord (Bot API)", "discord"),
		huh.NewOption("Slack (Socket Mode)", "slack"),
		huh.NewOption("Skip for now", "skip"),
	}

	var channel string
	err := huh.NewSelect[string]().
		Title("Select primary channel").
		Options(channelOptions...).
		Value(&channel).
		Run()

	if err != nil || channel == "skip" {
		return
	}

	configureChannel(cfg, channel)
}

func configureChannel(cfg *config.Config, channel string) {
	switch channel {
	case "telegram":
		configureTelegram(cfg)
	case "discord":
		configureDiscord(cfg)
	case "slack":
		configureSlack(cfg)
	case "whatsapp":
		fmt.Println("\n📱 WhatsApp will be configured via QR code after gateway starts.")
	}
}

func configureTelegram(cfg *config.Config) {
	var botToken string
	huh.NewInput().
		Title("Telegram Bot Token").
		Value(&botToken).
		EchoMode(huh.EchoModePassword).
		Run()
	if botToken != "" {
		cfg.Channels.Telegram.BotToken = botToken
	}
}

func configureDiscord(cfg *config.Config) {
	var token string
	huh.NewInput().
		Title("Discord Bot Token").
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
		Value(&botToken).
		EchoMode(huh.EchoModePassword).
		Run()
	huh.NewInput().
		Title("Slack App Token").
		Value(&appToken).
		EchoMode(huh.EchoModePassword).
		Run()

	if botToken != "" {
		cfg.Channels.Slack.BotToken = botToken
		cfg.Channels.Slack.AppToken = appToken
	}
}

func configureSkills(cfg *config.Config) {
	var configure bool
	huh.NewConfirm().
		Title("Configure skills?").
		Description("Enable capabilities like file access, browser control, etc.").
		Value(&configure).
		Run()

	if !configure {
		return
	}

	// List skills
	fmt.Println("\nAvailable Skills:")
	for _, s := range skill.PureGoSkillsInfo {
		fmt.Printf("- %s %s\n", s.Icon, s.Name)
	}
	fmt.Println("\n(Skill configuration is auto-detected from environment or can be edited in config.yaml)")
}

func printSuccess(cfg *config.Config) {
	fmt.Println()
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	fmt.Println(successStyle.Render("✅ Onboarding complete!"))
	fmt.Println()
	fmt.Println("Configuration saved to:", config.ConfigPath())
	fmt.Println("Next steps:")
	fmt.Println("  1. Start the gateway:  highclaw gateway")
	fmt.Println("  2. Open Web UI:        http://localhost:18790")
}

// expandPath expands ~ to the home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
