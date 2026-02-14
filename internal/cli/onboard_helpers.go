package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/model"
	"github.com/highclaw/highclaw/internal/domain/skill"
)

// selectModelCompleteWithFilter selects model with provider filtering
func selectModelCompleteWithFilter(cfg *config.Config) (string, string) {
	// Step 1: Filter by provider
	var filterChoice string
	providerOptions := []huh.Option[string]{
		huh.NewOption("All providers (100+ models)", "all"),
	}

	// Get provider info
	providerInfo := model.GetProviderInfo()
	for _, providerID := range model.AllProviders {
		if info, ok := providerInfo[providerID]; ok {
			providerOptions = append(providerOptions, huh.NewOption(info.Name, info.ID))
		} else {
			providerOptions = append(providerOptions, huh.NewOption(providerID, providerID))
		}
	}

	err := huh.NewSelect[string]().
		Title("Filter models by provider").
		Description("Choose a specific provider or see all models").
		Options(providerOptions...).
		Value(&filterChoice).
		Run()

	if err != nil {
		filterChoice = "all"
	}

	// Step 2: Select model
	var modelOptions []huh.Option[string]

	// Add "Keep current" option if model exists
	if cfg.Agent.Model != "" {
		modelOptions = append(modelOptions,
			huh.NewOption(fmt.Sprintf("Keep current (%s)", cfg.Agent.Model), "keep"))
	}

	// Add "Enter manually" option
	modelOptions = append(modelOptions,
		huh.NewOption("Enter model manually", "manual"))

	// Add models (filtered or all)
	allModels := model.GetAllModelsComplete()
	for _, m := range allModels {
		if filterChoice != "all" && m.Provider != filterChoice {
			continue
		}
		label := fmt.Sprintf("%s/%s", m.Provider, m.ID)
		if m.Name != "" {
			label = fmt.Sprintf("%s - %s", label, m.Name)
		}
		modelOptions = append(modelOptions, huh.NewOption(label, m.Provider+"/"+m.ID))
	}

	var modelChoice string
	err = huh.NewSelect[string]().
		Title("Select default model").
		Description(fmt.Sprintf("Found %d models", len(modelOptions)-2)).
		Options(modelOptions...).
		Value(&modelChoice).
		Run()

	if err != nil || modelChoice == "" {
		return "", ""
	}

	// Handle special cases
	if modelChoice == "keep" {
		parts := strings.SplitN(cfg.Agent.Model, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}

	if modelChoice == "manual" {
		return inputModelManually()
	}

	// Parse provider/model
	parts := strings.SplitN(modelChoice, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return "", ""
}

// inputModelManually prompts for manual model input
func inputModelManually() (string, string) {
	var modelInput string
	err := huh.NewInput().
		Title("Enter model (format: provider/model-id)").
		Description("Example: anthropic/claude-opus-4 or openai/gpt-5.1").
		Value(&modelInput).
		Placeholder("provider/model-id").
		Validate(func(s string) error {
			if !strings.Contains(s, "/") {
				return fmt.Errorf("must be in format: provider/model-id")
			}
			return nil
		}).
		Run()

	if err != nil || modelInput == "" {
		return "", ""
	}

	parts := strings.SplitN(modelInput, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return "", ""
}

// inputAPIKeyComplete prompts for API key
func inputAPIKeyComplete(provider string) string {
	// Check if already set
	envVar := strings.ToUpper(provider) + "_API_KEY"
	if os.Getenv(envVar) != "" {
		printNote(fmt.Sprintf("Using existing %s from environment", envVar), "API Key")
		return os.Getenv(envVar)
	}

	var apiKey string
	err := huh.NewInput().
		Title(fmt.Sprintf("Enter %s API key", provider)).
		Description(fmt.Sprintf("Get your key from %s's website", provider)).
		Value(&apiKey).
		EchoMode(huh.EchoModePassword).
		Placeholder("sk-...").
		Run()

	if err != nil {
		return ""
	}

	return apiKey
}

// configureGatewayComplete configures gateway settings
func configureGatewayComplete(cfg *config.Config, flow string) {
	if flow == "quickstart" {
		cfg.Gateway.Port = 18790
		cfg.Gateway.Bind = "loopback"
		cfg.Gateway.Mode = "local"
		printNote("Using default gateway settings (port 18790, loopback)", "Gateway")
		return
	}

	// Advanced: ask for port
	var portStr string
	err := huh.NewInput().
		Title("Gateway port").
		Description("Port for the gateway HTTP server").
		Value(&portStr).
		Placeholder("18790").
		Run()

	port := 18790
	if err == nil && portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}
	cfg.Gateway.Port = port

	// Ask for bind address
	var bind string
	err = huh.NewSelect[string]().
		Title("Gateway bind address").
		Options(
			huh.NewOption("Loopback (127.0.0.1) - local only", "loopback"),
			huh.NewOption("LAN (0.0.0.0) - accessible on network", "lan"),
		).
		Value(&bind).
		Run()

	if err == nil {
		cfg.Gateway.Bind = bind
	} else {
		cfg.Gateway.Bind = "loopback"
	}

	cfg.Gateway.Mode = "local"
}

// configureChannelsComplete configures messaging channels
func configureChannelsComplete(cfg *config.Config) {
	var configureChannels bool
	err := huh.NewConfirm().
		Title("Configure messaging channels now?").
		Description("Connect Telegram, WhatsApp, Discord, Slack, etc.").
		Value(&configureChannels).
		Run()

	if err != nil || !configureChannels {
		printNote("Skipping channel configuration. You can configure later.", "Channels")
		return
	}

	// Show available channels
	printNote("Available channels:\n"+
		"  📱 Telegram (Bot API)\n"+
		"  💬 WhatsApp (QR link)\n"+
		"  🎮 Discord (Bot API)\n"+
		"  💼 Slack (Socket Mode)\n"+
		"  📞 Signal\n"+
		"  💬 iMessage (macOS only)\n"+
		"  ... and 13 more", "Channels")

	// For now, just show a note
	// Full implementation would configure each channel
	printNote("Channel configuration will be available in the next version.", "Coming Soon")
}

// configureSkillsComplete configures skills
func configureSkillsComplete(cfg *config.Config) {
	var configureSkills bool
	err := huh.NewConfirm().
		Title("Configure skills now?").
		Description("Skills extend the agent's capabilities (Pure Go, no Node.js required)").
		Value(&configureSkills).
		Run()

	if err != nil || !configureSkills {
		printNote("Skipping skills configuration.", "Skills")
		return
	}

	// Show pure Go skills
	fmt.Println()
	fmt.Println(styleInfo.Render("🔍 Available Skills (Pure Go):"))
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
	box := styleBox.Render(fmt.Sprintf(
		"Skills Status\n\n"+
			"  Ready to use: %d\n"+
			"  Need API key: %d\n"+
			"  Total: %d",
		eligible, needsAPIKey, len(pureGoSkills),
	))
	fmt.Println(box)

	if needsAPIKey > 0 {
		printNote("Some skills need API keys. You can configure them later via environment variables.", "API Keys")
	} else {
		printSuccess("All skills are ready to use!")
	}
}
