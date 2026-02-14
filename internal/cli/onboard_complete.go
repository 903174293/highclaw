package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/spf13/cobra"
)

var onboardCompleteCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Complete onboarding wizard (full OpenClaw parity)",
	Long:  "Interactive setup wizard with beautiful UI, provider filtering, channels, and skills.",
	RunE:  runOnboardComplete,
}

// Color palette (inspired by OpenClaw's Lobster palette)
var (
	colorAccent       = lipgloss.Color("#FF5A2D")
	colorAccentBright = lipgloss.Color("#FF7A3D")
	colorSuccess      = lipgloss.Color("#2FBF71")
	colorWarn         = lipgloss.Color("#FFB020")
	colorError        = lipgloss.Color("#E23D2D")
	colorMuted        = lipgloss.Color("#8B7F77")
	colorInfo         = lipgloss.Color("#FF8A5B")
)

// Styles
var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginTop(1).
			MarginBottom(1)

	styleBanner = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccentBright).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	styleWarn = lipgloss.NewStyle().
			Foreground(colorWarn)

	styleError = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	styleMuted = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleInfo = lipgloss.NewStyle().
			Foreground(colorInfo)

	styleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	styleStep = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)
)

func runOnboardComplete(cmd *cobra.Command, args []string) error {
	// Print beautiful banner
	printBanner()

	// Intro message
	printIntro()

	// Check existing config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
		printNote("No existing configuration found. Starting fresh setup.", "Config")
	} else {
		printNote("Found existing configuration. You can update it or start fresh.", "Config")
	}

	// Platform-specific warnings
	if runtime.GOOS == "windows" {
		printWarning(
			"Windows detected — HighClaw runs great on WSL2!\n" +
				"Native Windows might be trickier.\n" +
				"Quick setup: wsl --install (one command, one reboot)\n" +
				"Guide: https://docs.openclaw.ai/windows",
		)
	}

	// Step 1: Risk acknowledgement
	printStep(1, "Security Acknowledgement")
	if !confirmRiskComplete() {
		printError("Onboarding cancelled. Risk not accepted.")
		return nil
	}

	// Step 2: Choose flow
	printStep(2, "Setup Mode")
	flow := selectFlowComplete()

	// Step 3: Workspace
	printStep(3, "Workspace Configuration")
	workspace := configureWorkspaceComplete(cfg, flow)
	if workspace != "" {
		cfg.Agent.Workspace = workspace
	}

	// Step 4: Model selection with provider filtering
	printStep(4, "AI Model Selection")
	provider, modelID := selectModelCompleteWithFilter(cfg)
	if provider != "" && modelID != "" {
		cfg.Agent.Model = fmt.Sprintf("%s/%s", provider, modelID)
		printSuccess(fmt.Sprintf("Selected model: %s/%s", provider, modelID))
	}

	// Step 5: API Key
	printStep(5, "API Key Configuration")
	apiKey := inputAPIKeyComplete(provider)
	if apiKey != "" {
		if cfg.Agent.Providers == nil {
			cfg.Agent.Providers = make(map[string]config.ProviderConfig)
		}
		cfg.Agent.Providers[provider] = config.ProviderConfig{APIKey: apiKey}
		printSuccess("API key configured")
	}

	// Step 6: Gateway configuration
	printStep(6, "Gateway Configuration")
	configureGatewayComplete(cfg, flow)

	// Step 7: Channel selection
	printStep(7, "Messaging Channels")
	configureChannelsComplete(cfg)

	// Step 8: Skills configuration
	printStep(8, "Skills Configuration")
	configureSkillsComplete(cfg)

	// Save configuration
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Create workspace
	if workspace != "" {
		os.MkdirAll(expandPath(workspace), 0o755)
	}

	// Success outro
	printOutro(cfg)

	return nil
}

// printBanner prints a beautiful ASCII banner
func printBanner() {
	banner := `
╔═══════════════════════════════════════════════════════════╗
║                                                           ║
║   🦀  H I G H C L A W  🦀                                ║
║                                                           ║
║   Personal AI Assistant Gateway                          ║
║   Pure Go · Single Binary · No Dependencies              ║
║                                                           ║
╚═══════════════════════════════════════════════════════════╝
`
	fmt.Println(styleBanner.Render(banner))
}

// printIntro prints intro message
func printIntro() {
	intro := styleTitle.Render("🚀 Welcome to HighClaw Onboarding")
	desc := styleMuted.Render("Let's set up your AI assistant gateway in a few simple steps.")
	fmt.Printf("\n%s\n%s\n\n", intro, desc)
}

// printOutro prints success message and next steps
func printOutro(cfg *config.Config) {
	fmt.Println()
	fmt.Println(styleSuccess.Render("✅ Onboarding Complete!"))
	fmt.Println()

	box := styleBox.Render(
		"Configuration saved successfully.\n\n" +
			"Next steps:\n" +
			"  1. Start the gateway:  " + styleInfo.Render("highclaw gateway") + "\n" +
			"  2. Open web UI:        " + styleInfo.Render(fmt.Sprintf("http://localhost:%d", cfg.Gateway.Port)) + "\n" +
			"  3. Send a message:     " + styleInfo.Render("highclaw message send \"Hello!\"") + "\n\n" +
			"Documentation: https://docs.openclaw.ai",
	)
	fmt.Println(box)
}

// printStep prints a step header
func printStep(num int, title string) {
	fmt.Println()
	step := styleStep.Render(fmt.Sprintf("Step %d:", num))
	fmt.Printf("%s %s\n\n", step, title)
}

// printNote prints an informational note
func printNote(message, title string) {
	fmt.Println()
	if title != "" {
		fmt.Println(styleInfo.Render("ℹ️  " + title))
	}
	fmt.Println(styleMuted.Render(message))
	fmt.Println()
}

// printSuccess prints a success message
func printSuccess(message string) {
	fmt.Println(styleSuccess.Render("✅ " + message))
}

// printWarning prints a warning message
func printWarning(message string) {
	fmt.Println()
	fmt.Println(styleWarn.Render("⚠️  Warning"))
	fmt.Println(styleWarn.Render(message))
	fmt.Println()
}

// printError prints an error message
func printError(message string) {
	fmt.Println()
	fmt.Println(styleError.Render("❌ " + message))
	fmt.Println()
}

// confirmRiskComplete shows security warning and gets confirmation
func confirmRiskComplete() bool {
	warning := `
⚠️  SECURITY WARNING

HighClaw agents have full system access and can execute commands.
This is powerful but inherently risky.

The agent can:
  • Execute bash commands
  • Read and write files
  • Make HTTP requests
  • Access your data

You should:
  • Review all tool executions
  • Use allowlists for sensitive operations
  • Run in a sandboxed environment if possible

Must read: https://docs.openclaw.ai/gateway/security
`
	fmt.Println(styleWarn.Render(warning))

	var accept bool
	err := huh.NewConfirm().
		Title("I understand this is powerful and inherently risky. Continue?").
		Value(&accept).
		Run()

	return err == nil && accept
}

// selectFlowComplete selects quickstart or advanced flow
func selectFlowComplete() string {
	var flow string
	err := huh.NewSelect[string]().
		Title("Choose setup mode").
		Description("QuickStart uses sensible defaults. Advanced lets you configure everything.").
		Options(
			huh.NewOption("QuickStart (recommended)", "quickstart"),
			huh.NewOption("Advanced (full control)", "advanced"),
		).
		Value(&flow).
		Run()

	if err != nil {
		return "quickstart"
	}
	return flow
}

// configureWorkspaceComplete configures workspace directory
func configureWorkspaceComplete(cfg *config.Config, flow string) string {
	if flow == "quickstart" {
		workspace := cfg.Agent.Workspace
		if workspace == "" {
			workspace = "~/Documents/highclaw-workspace"
		}
		printNote(fmt.Sprintf("Using workspace: %s", workspace), "Workspace")
		return workspace
	}

	var workspace string
	defaultWorkspace := cfg.Agent.Workspace
	if defaultWorkspace == "" {
		defaultWorkspace = "~/Documents/highclaw-workspace"
	}

	err := huh.NewInput().
		Title("Workspace directory").
		Description("Where to store agent sessions and files").
		Value(&workspace).
		Placeholder(defaultWorkspace).
		Run()

	if err != nil || workspace == "" {
		return defaultWorkspace
	}
	return workspace
}
