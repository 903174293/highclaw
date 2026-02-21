package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/skills"
	"github.com/spf13/cobra"
)

var (
	onboardAPIKey       string
	onboardProvider     string
	onboardModel        string
	onboardMemory       string
	onboardInteractive  bool
	onboardChannelsOnly bool
)

const wizardBanner = `
    ⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡

    ██╗  ██╗██╗ ██████╗ ██╗  ██╗ ██████╗██╗      █████╗ ██╗    ██╗
    ██║  ██║██║██╔════╝ ██║  ██║██╔════╝██║     ██╔══██╗██║    ██║
    ███████║██║██║  ███╗███████║██║     ██║     ███████║██║ █╗ ██║
    ██╔══██║██║██║   ██║██╔══██║██║     ██║     ██╔══██║██║███╗██║
    ██║  ██║██║╚██████╔╝██║  ██║╚██████╗███████╗██║  ██║╚███╔███╔╝
    ╚═╝  ╚═╝╚═╝ ╚═════╝ ╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝ ╚══╝╚══╝

    High performance. Built for speed and reliability. 100% Go 100% Agnostic.

    ⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡
`

type projectContext struct {
	UserName           string
	Timezone           string
	AgentName          string
	CommunicationStyle string
}

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiUnder  = "\033[4m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiGray   = "\033[90m"
	ansiMagenta = "\033[38;5;99m"
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Quick setup with sensible defaults",
	RunE:  runOnboard,
}

func init() {
	onboardCmd.Flags().StringVar(&onboardAPIKey, "api-key", "", "Provider API key")
	onboardCmd.Flags().StringVar(&onboardProvider, "provider", "openrouter", "Default provider")
	onboardCmd.Flags().StringVar(&onboardModel, "model", "", "Default model")
	onboardCmd.Flags().StringVar(&onboardMemory, "memory", "sqlite", "Memory backend")
	onboardCmd.Flags().BoolVar(&onboardInteractive, "interactive", false, "Run interactive wizard")
	onboardCmd.Flags().BoolVar(&onboardChannelsOnly, "channels-only", false, "Repair channels only")
}

func runOnboard(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	if onboardInteractive && onboardChannelsOnly {
		return fmt.Errorf("use either --interactive or --channels-only, not both")
	}
	if onboardChannelsOnly && (cmd.Flags().Changed("api-key") || cmd.Flags().Changed("provider") || cmd.Flags().Changed("model") || cmd.Flags().Changed("memory")) {
		return fmt.Errorf("--channels-only does not accept --api-key, --provider, --model, or --memory")
	}

	if onboardChannelsOnly {
		return runChannelsRepairWizard(cfg)
	}
	if onboardInteractive {
		return runWizard(cfg)
	}
	return runQuickSetup(cfg)
}

func runWizard(cfg *config.Config) error {
	fmt.Print(cyan(wizardBanner))
	fmt.Println("  Welcome to HighClaw — the fastest, smallest AI assistant.")
	fmt.Println("  This wizard will configure your agent in under 60 seconds.")
	fmt.Println()

	printStep(1, 9, "Workspace Setup")
	workspace := setupWorkspace()
	cfg.Agent.Workspace = workspace

	printStep(2, 9, "AI Provider & API Key")
	provider, apiKey, model := setupProvider()
	cfg.Agent.Model = provider + "/" + model
	providerCfg := config.ProviderConfig{}
	if apiKey != "" {
		providerCfg.APIKey = apiKey
	}
	providerCfg.BaseURL = providerDefaultBaseURL(provider)
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		provider: providerCfg,
	}

	printStep(3, 9, "Channels (How You Talk to HighClaw)")
	cfg.Channels = setupChannels(cfg.Channels)

	printStep(4, 9, "Tunnel (Expose to Internet)")
	cfg.Tunnel = setupTunnel()

	printStep(5, 9, "Tool Mode & Security")
	cfg.Composio, cfg.Secrets = setupToolMode()

	printStep(6, 9, "Memory Configuration")
	cfg.Memory = setupMemory()

	printStep(7, 9, "Project Context (Personalize Your Agent)")
	ctx := setupProjectContext()

	printStep(8, 9, "Skills Configuration")
	setupSkills(workspace)

	printStep(9, 9, "Workspace Files")
	createdFiles, skippedFiles, _, ws, err := ensureWorkspaceLayout(workspace, ctx)
	if err != nil {
		return err
	}
	fmt.Printf("  %s Created %d files, skipped %d existing | 5 subdirectories\n", green("✓"), createdFiles, skippedFiles)
	printWorkspaceTree(ws)

	if cfg.Autonomy.Level == "" {
		cfg.Autonomy.Level = "supervised"
	}
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 8080
	}
	if cfg.Gateway.Auth.Mode == "" {
		cfg.Gateway.Auth.Mode = "token"
	}
	fmt.Printf("  %s Security: %s | workspace-scoped\n", green("✓"), green("Supervised"))
	fmt.Printf("  %s Memory: %s (auto-save: %s)\n", green("✓"), green(cfg.Memory.Backend), onOff(cfg.Memory.AutoSave))
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	printSummary(cfg)
	promptLaunchChannels(cfg)
	return nil
}

func runChannelsRepairWizard(cfg *config.Config) error {
	fmt.Print(cyan(wizardBanner))
	fmt.Println("  Channels Repair — update channel tokens and allowlists only")
	fmt.Println()
	printStep(1, 1, "Channels (How You Talk to HighClaw)")
	cfg.Channels = setupChannels(cfg.Channels)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("  %s Channel config saved: %s\n", green("✓"), config.ConfigPath())
	promptLaunchChannels(cfg)
	return nil
}

func runQuickSetup(cfg *config.Config) error {
	fmt.Print(cyan(wizardBanner))
	fmt.Println("  " + bold("Quick Setup — generating config with sensible defaults..."))
	fmt.Println()

	provider := strings.TrimSpace(onboardProvider)
	if provider == "" {
		provider = "openrouter"
	}
	model := strings.TrimSpace(onboardModel)
	if model == "" {
		model = defaultModelForProvider(provider)
	}
	mem := strings.TrimSpace(onboardMemory)
	if mem == "" {
		mem = "sqlite"
	}
	cfg.Agent.Workspace = filepath.Join(config.ConfigDir(), "workspace")
	cfg.Agent.Model = provider + "/" + model
	providerCfg := config.ProviderConfig{
		BaseURL: providerDefaultBaseURL(provider),
	}
	if strings.TrimSpace(onboardAPIKey) != "" {
		providerCfg.APIKey = strings.TrimSpace(onboardAPIKey)
	}
	cfg.Agent.Providers = map[string]config.ProviderConfig{
		provider: providerCfg,
	}
	cfg.Memory = memoryConfigForBackend(mem, mem != "none")
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 8080
	}
	if cfg.Autonomy.Level == "" {
		cfg.Autonomy.Level = "supervised"
	}
	cfg.Secrets.Encrypt = true
	cfg.Tunnel.Provider = "none"
	cfg.Composio.Enabled = false
	cfg.Channels.CLI = true

	defaultCtx := projectContext{
		UserName:           os.Getenv("USER"),
		Timezone:           "UTC",
		AgentName:          "HighClaw",
		CommunicationStyle: "Be warm, natural, and clear. Use occasional relevant emojis (1-2 max) and avoid robotic phrasing.",
	}
	createdFiles, skippedFiles, _, ws, err := ensureWorkspaceLayout(cfg.Agent.Workspace, defaultCtx)
	if err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	check := green("✓")
	fmt.Printf("  %s Created %s files, skipped %s existing | %s subdirectories\n", check, green(fmt.Sprintf("%d", createdFiles)), gray(fmt.Sprintf("%d", skippedFiles)), green("5"))
	printWorkspaceTree(ws)
	fmt.Printf("  %s Workspace:  %s\n", check, green(ws))
	fmt.Printf("  %s Provider:   %s\n", check, green(provider))
	fmt.Printf("  %s Model:      %s\n", check, green(model))
	if strings.TrimSpace(onboardAPIKey) == "" {
		fmt.Printf("  %s API Key:    %s\n", check, yellow("not set (use --api-key or edit config.yaml)"))
	} else {
		fmt.Printf("  %s API Key:    %s\n", check, green("set"))
	}
	fmt.Printf("  %s Security:   %s\n", check, green("Supervised (workspace-scoped)"))
	fmt.Printf("  %s Memory:     %s %s\n", check, green(cfg.Memory.Backend), gray(fmt.Sprintf("(auto-save: %s)", onOff(cfg.Memory.AutoSave))))
	fmt.Printf("  %s Secrets:    %s\n", check, green("encrypted"))
	fmt.Printf("  %s Gateway:    %s\n", check, green(fmt.Sprintf("pairing required (127.0.0.1:%d)", cfg.Gateway.Port)))
	fmt.Printf("  %s Tunnel:     %s\n", check, gray("none (local only)"))
	fmt.Printf("  %s Composio:   %s\n", check, gray("disabled (sovereign mode)"))
	fmt.Println()
	fmt.Printf("  %s %s\n", bold("Config saved:"), green(config.ConfigPath()))
	fmt.Println()
	fmt.Println("  " + bold("Next steps:"))
	fmt.Printf("    1. Set your API key:  %s\n", magenta(fmt.Sprintf("export %s=\"sk-...\"", providerEnvVar(provider))))
	fmt.Printf("    2. Or edit:           %s\n", magenta("~/.highclaw/config.yaml"))
	fmt.Printf("    3. Chat:              %s\n", magenta("highclaw agent -m \"Hello!\""))
	fmt.Printf("    4. Gateway:           %s\n", magenta("highclaw gateway"))
	fmt.Println()

	return nil
}

func setupWorkspace() string {
	home := config.ConfigDir()
	printBullet("Default location: " + home)
	useDefault := promptYesNo("Use default workspace location?", true)
	base := home
	if !useDefault {
		base = expandPath(promptString("Enter workspace path", home))
	}
	ws := filepath.Join(base, "workspace")
	fmt.Printf("  %s Workspace: %s\n", green("✓"), green(ws))
	fmt.Println()
	return ws
}

func setupProvider() (string, string, string) {
	tiers := []string{
		"⭐ Recommended (OpenRouter, Venice, Anthropic, OpenAI, Gemini)",
		"⚡ Fast inference (Groq, Fireworks, Together AI)",
		"🌐 Gateway / proxy (Vercel AI, Cloudflare AI, Amazon Bedrock)",
		"🔬 Specialized (Moonshot/Kimi, GLM/Zhipu, MiniMax, Qianfan, Z.AI, Synthetic, OpenCode Zen, Cohere)",
		"🏠 Local / private (Ollama — no API key needed)",
		"🔧 Custom — bring your own OpenAI-compatible API",
	}
	tier := promptSelect("Select provider category", tiers, 0)
	fmt.Printf("  Select provider category: %s\n", tiers[tier])
	fmt.Println()

	type opt struct{ Key, Label string }
	var providers []opt
	switch tier {
	case 0:
		providers = []opt{
			{"openrouter", "OpenRouter — 200+ models, 1 API key (recommended)"},
			{"venice", "Venice AI — privacy-first (Llama, Opus)"},
			{"anthropic", "Anthropic — Claude Sonnet & Opus (direct)"},
			{"openai", "OpenAI — GPT-4o, o1, GPT-5 (direct)"},
			{"deepseek", "DeepSeek — V3 & R1 (affordable)"},
			{"mistral", "Mistral — Large & Codestral"},
			{"xai", "xAI — Grok 3 & 4"},
			{"perplexity", "Perplexity — search-augmented AI"},
			{"gemini", "Google Gemini — Gemini 2.0 Flash & Pro (supports CLI auth)"},
		}
	case 1:
		providers = []opt{
			{"groq", "Groq — ultra-fast LPU inference"},
			{"fireworks", "Fireworks AI — fast open-source inference"},
			{"together", "Together AI — open-source model hosting"},
		}
	case 2:
		providers = []opt{
			{"vercel", "Vercel AI Gateway"},
			{"cloudflare", "Cloudflare AI Gateway"},
			{"bedrock", "Amazon Bedrock — AWS managed models"},
		}
	case 3:
		providers = []opt{
			{"moonshot", "Moonshot — Kimi & Kimi Coding"},
			{"glm", "GLM — ChatGLM / Zhipu models"},
			{"minimax", "MiniMax — MiniMax AI models"},
			{"qianfan", "Qianfan — Baidu AI models"},
			{"zai", "Z.AI — Z.AI inference"},
			{"synthetic", "Synthetic — Synthetic AI models"},
			{"opencode", "OpenCode Zen — code-focused AI"},
			{"cohere", "Cohere — Command R+ & embeddings"},
		}
	case 4:
		providers = []opt{{"ollama", "Ollama — local models (Llama, Mistral, Phi)"}}
	default:
		fmt.Println()
		fmt.Println("  Custom Provider Setup — any OpenAI-compatible API")
		printBullet("HighClaw works with ANY API that speaks the OpenAI chat completions format.")
		printBullet("Examples: LiteLLM, LocalAI, vLLM, text-generation-webui, LM Studio, etc.")
		fmt.Println()
		baseURL := strings.TrimRight(strings.TrimSpace(promptString("API base URL (e.g. http://localhost:1234 or https://my-api.com)", "")), "/")
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		key := strings.TrimSpace(promptString("API key (or Enter to skip if not needed)", ""))
		model := strings.TrimSpace(promptString("Model name (e.g. llama3, gpt-4o, mistral)", "default"))
		if model == "" {
			model = "default"
		}
		fmt.Printf("  %s Provider: %s | Model: %s\n", green("✓"), green("custom:"+baseURL), green(model))
		fmt.Println()
		return "custom:" + baseURL, key, model
	}

	labels := make([]string, 0, len(providers))
	for _, p := range providers {
		labels = append(labels, p.Label)
	}
	providerIdx := promptSelect("Select your AI provider", labels, 0)
	provider := providers[providerIdx].Key
	fmt.Printf("  Select your AI provider: %s\n", labels[providerIdx])
	fmt.Println()

	apiKey := ""
	if provider == "ollama" {
		printBullet("Ollama runs locally — no API key needed!")
	} else if provider == "gemini" || provider == "google" || provider == "google-gemini" {
		if hasGeminiCLICredentials() {
			printBullet("✓ Gemini CLI credentials detected! You can skip the API key.")
			printBullet("HighClaw will reuse your existing Gemini CLI authentication.")
			fmt.Println()
			useCLI := promptYesNo("Use existing Gemini CLI authentication?", true)
			if useCLI {
				fmt.Printf("  %s Using Gemini CLI OAuth tokens\n", green("✓"))
				apiKey = ""
			} else {
				printBullet("Get your API key at: https://aistudio.google.com/app/apikey")
				apiKey = strings.TrimSpace(promptString("Paste your Gemini API key", ""))
			}
		} else if strings.TrimSpace(os.Getenv("GEMINI_API_KEY")) != "" {
			printBullet("✓ GEMINI_API_KEY environment variable detected!")
			apiKey = ""
		} else {
			printBullet("Get your API key at: https://aistudio.google.com/app/apikey")
			printBullet("Or run `gemini` CLI to authenticate (tokens will be reused).")
			fmt.Println()
			apiKey = strings.TrimSpace(promptString("Paste your Gemini API key (or press Enter to skip)", ""))
		}
	} else {
		if u := apiKeyURL(provider); u != "" {
			printBullet("Get your API key at: " + u)
		}
		printBullet("You can also set it later via env var or config file.")
		fmt.Println()
		apiKey = strings.TrimSpace(promptString("Paste your API key (or press Enter to skip)", ""))
		if apiKey == "" {
			printBullet("Skipped. Set " + providerEnvVar(provider) + " or edit config.yaml later.")
		}
	}

	models := modelsForProvider(provider)
	mLabels := make([]string, 0, len(models))
	for _, m := range models {
		mLabels = append(mLabels, m.Label)
	}
	modelIdx := promptSelect("Select your default model", mLabels, 0)
	model := models[modelIdx].ID
	fmt.Printf("  Select your default model: %s\n", mLabels[modelIdx])
	fmt.Printf("  %s Provider: %s | Model: %s\n", green("✓"), green(provider), green(model))
	fmt.Println()
	return provider, apiKey, model
}

func setupChannels(existing config.ChannelsConfig) config.ChannelsConfig {
	printBullet("Channels let you talk to HighClaw from anywhere.")
	printBullet("CLI is always available. Connect more channels now.")
	fmt.Println()

	out := existing
	out.CLI = true
	for {
		options := []string{
			statusLineWithText("Telegram", channelState(out.Telegram != nil, "✅ connected", "— connect your bot")),
			statusLineWithText("Discord", channelState(out.Discord != nil, "✅ connected", "— connect your bot")),
			statusLineWithText("Slack", channelState(out.Slack != nil, "✅ connected", "— connect your bot")),
			statusLineWithText("iMessage", channelState(out.IMessage != nil, "✅ configured", "— macOS only")),
			statusLineWithText("Matrix", channelState(out.Matrix != nil, "✅ connected", "— self-hosted chat")),
			statusLineWithText("WhatsApp", channelState(out.WhatsApp != nil, "✅ connected", "— Business Cloud API")),
			statusLineWithText("IRC", channelState(out.IRC != nil, "✅ configured", "— IRC over TLS")),
			statusLineWithText("Webhook", channelState(out.Webhook != nil, "✅ configured", "— HTTP endpoint")),
			statusLineWithText("Feishu", channelState(out.Feishu != nil, "✅ connected", "— Enterprise collaboration platform")),
			statusLineWithText("WeCom", channelState(out.WeCom != nil, "✅ connected", "— Enterprise messaging")),
			statusLineWithText("WeChat", channelState(out.WeChat != nil, "✅ connected", "— Official Account / Personal")),
			"Done — finish setup",
		}
		choice := promptSelect("Connect a channel (or Done to continue)", options, 11)
		fmt.Printf("  Connect a channel (or Done to continue): %s\n", options[choice])
		switch choice {
		case 0:
			fmt.Println()
			fmt.Println("  Telegram Setup — talk to HighClaw from Telegram")
			printBullet("1. Open Telegram and message @BotFather")
			printBullet("2. Send /newbot and follow the prompts")
			printBullet("3. Copy the bot token and paste it below")
			fmt.Println()
			tok := strings.TrimSpace(promptString("Bot token (from @BotFather)", ""))
			if tok == "" {
				fmt.Println("  → Skipped")
				continue
			}
			fmt.Print("  ⏳ Testing connection... ")
			botName, ok := testTelegram(tok)
			if !ok {
				fmt.Println("\r  ❌ Connection failed — check your token and try again")
				continue
			}
			fmt.Printf("\r  ✅ Connected as @%s\n", botName)
			printBullet("Allowlist your own Telegram identity first (recommended for secure + fast setup).")
			printBullet("Use your @username without '@' or your numeric Telegram user ID.")
			printBullet("Use '*' only for temporary open testing.")
			users := parseCSV(promptString("Allowed Telegram identities (comma-separated, '*' for all)", ""))
			if len(users) == 0 {
				fmt.Println("  ⚠ No users allowlisted — Telegram inbound messages will be denied until you add identities or '*'.")
			}
			out.Telegram = &config.TelegramConfig{BotToken: tok, AllowedUsers: users}
			fmt.Println()
		case 1:
			fmt.Println()
			fmt.Println("  Discord Setup — talk to HighClaw from Discord")
			printBullet("1. Go to https://discord.com/developers/applications")
			printBullet("2. Create a New Application → Bot → Copy token")
			printBullet("3. Enable MESSAGE CONTENT intent under Bot settings")
			printBullet("4. Invite bot to your server with messages permission")
			fmt.Println()
			tok := strings.TrimSpace(promptString("Bot token", ""))
			if tok == "" {
				fmt.Println("  → Skipped")
				continue
			}
			fmt.Print("  ⏳ Testing connection... ")
			botName, ok := testDiscord(tok)
			if !ok {
				fmt.Println("\r  ❌ Connection failed — check your token and try again")
				continue
			}
			fmt.Printf("\r  ✅ Connected as %s\n", botName)
			gid := strings.TrimSpace(promptString("Server (guild) ID (optional, Enter to skip)", ""))
			printBullet("Allowlist your own Discord user ID first (recommended).")
			printBullet("Use '*' only for temporary open testing.")
			users := parseCSV(promptString("Allowed Discord user IDs (comma-separated, '*' for all)", ""))
			if len(users) == 0 {
				fmt.Println("  ⚠ No users allowlisted — Discord inbound messages will be denied until you add IDs or '*'.")
			}
			out.Discord = &config.DiscordConfig{Token: tok, GuildID: gid, AllowedUsers: users}
			fmt.Println()
		case 2:
			fmt.Println()
			fmt.Println("  Slack Setup — talk to HighClaw from Slack")
			printBullet("1. Go to https://api.slack.com/apps → Create New App")
			printBullet("2. Add Bot Token Scopes: chat:write, channels:history")
			printBullet("3. Install to workspace and copy the Bot Token")
			fmt.Println()
			bot := strings.TrimSpace(promptString("Bot token (xoxb-...)", ""))
			if bot == "" {
				fmt.Println("  → Skipped")
				continue
			}
			fmt.Print("  ⏳ Testing connection... ")
			workspace, ok := testSlack(bot)
			if !ok {
				fmt.Println("\r  ❌ Connection failed — check your token and try again")
				continue
			}
			fmt.Printf("\r  ✅ Connected to workspace: %s\n", workspace)
			app := strings.TrimSpace(promptString("App token (xapp-..., optional, Enter to skip)", ""))
			ch := strings.TrimSpace(promptString("Default channel ID (optional, Enter to skip)", ""))
			printBullet("Allowlist your own Slack member ID first (recommended).")
			printBullet("Use '*' only for temporary open testing.")
			users := parseCSV(promptString("Allowed Slack user IDs (comma-separated, '*' for all)", ""))
			if len(users) == 0 {
				fmt.Println("  ⚠ No users allowlisted — Slack inbound messages will be denied until you add IDs or '*'.")
			}
			out.Slack = &config.SlackConfig{BotToken: bot, AppToken: app, ChannelID: ch, AllowedUsers: users}
			fmt.Println()
		case 3:
			fmt.Println()
			fmt.Println("  iMessage Setup — macOS only, reads from Messages.app")
			if runtimeGOOS() != "darwin" {
				fmt.Println("  ⚠ iMessage is only available on macOS.")
				continue
			}
			printBullet("HighClaw reads your iMessage database and replies via AppleScript.")
			printBullet("You need to grant Full Disk Access to your terminal in System Settings.")
			fmt.Println()
			contacts := parseCSV(promptString("Allowed contacts (comma-separated phone/email, or * for all)", "*"))
			out.IMessage = &config.IMessageConfig{AllowedContacts: contacts}
			fmt.Println("  ✅ iMessage configured")
			fmt.Println()
		case 4:
			fmt.Println()
			fmt.Println("  Matrix Setup — self-hosted, federated chat")
			printBullet("You need a Matrix account and an access token.")
			printBullet("Get a token via Element → Settings → Help & About → Access Token.")
			fmt.Println()
			home := strings.TrimSpace(promptString("Homeserver URL (e.g. https://matrix.org)", ""))
			token := strings.TrimSpace(promptString("Access token", ""))
			room := strings.TrimSpace(promptString("Room ID (e.g. !abc123:matrix.org)", ""))
			users := parseCSV(promptString("Allowed users (comma-separated @user:server, or * for all)", "*"))
			if home == "" || token == "" || room == "" {
				fmt.Println("  → Skipped")
				continue
			}
			fmt.Print("  ⏳ Testing connection... ")
			userID, ok := testMatrix(home, token)
			if !ok {
				fmt.Println("\r  ❌ Connection failed — check homeserver URL and token")
				continue
			}
			fmt.Printf("\r  ✅ Connected as %s\n", userID)
			out.Matrix = &config.MatrixConfig{Homeserver: strings.TrimRight(home, "/"), AccessToken: token, RoomID: room, AllowedUsers: users}
			fmt.Println()
		case 5:
			fmt.Println()
			fmt.Println("  WhatsApp Setup — Business Cloud API")
			printBullet("1. Go to developers.facebook.com and create a WhatsApp app")
			printBullet("2. Add the WhatsApp product and get your phone number ID")
			printBullet("3. Generate a temporary access token (System User)")
			printBullet("4. Configure webhook URL to: https://your-domain/whatsapp")
			fmt.Println()
			at := strings.TrimSpace(promptString("Access token (from Meta Developers)", ""))
			pid := strings.TrimSpace(promptString("Phone number ID (from WhatsApp app settings)", ""))
			vt := strings.TrimSpace(promptString("Webhook verify token (create your own)", "highclaw-whatsapp-verify"))
			if at == "" || pid == "" {
				fmt.Println("  → Skipped")
				continue
			}
			fmt.Print("  ⏳ Testing connection... ")
			if !testWhatsApp(at, pid) {
				fmt.Println("\r  ❌ Connection failed — check access token and phone number ID")
				continue
			}
			fmt.Println("\r  ✅ Connected to WhatsApp API")
			allow := parseCSV(promptString("Allowed phone numbers (comma-separated +1234567890, or * for all)", "*"))
			out.WhatsApp = &config.WhatsAppConfig{AccessToken: at, PhoneNumberID: pid, VerifyToken: vt, AllowedNumbers: allow}
			fmt.Println()
		case 6:
			fmt.Println()
			fmt.Println("  IRC Setup — IRC over TLS")
			printBullet("IRC connects over TLS to any IRC server")
			printBullet("Supports SASL PLAIN and NickServ authentication")
			fmt.Println()
			server := strings.TrimSpace(promptString("IRC server (hostname)", ""))
			portStr := strings.TrimSpace(promptString("Port", "6697"))
			nick := strings.TrimSpace(promptString("Bot nickname", ""))
			chs := parseCSV(promptString("Channels to join (comma-separated: #channel1,#channel2)", ""))
			users := parseCSV(promptString("Allowed nicknames (comma-separated, or * for all)", ""))
			if server == "" || nick == "" {
				fmt.Println("  → Skipped")
				continue
			}
			port, _ := strconv.Atoi(portStr)
			if port == 0 {
				port = 6697
			}
			if len(users) == 0 {
				printBullet("⚠ Empty allowlist — inbound messages will be denied until you add nicknames or '*'.")
			}
			fmt.Println()
			printBullet("Optional authentication (press Enter to skip each):")
			sp := strings.TrimSpace(promptString("Server password (optional)", ""))
			np := strings.TrimSpace(promptString("NickServ password (optional)", ""))
			sasl := strings.TrimSpace(promptString("SASL PLAIN password (optional)", ""))
			verify := promptYesNo("Verify TLS certificate?", true)
			out.IRC = &config.IRCConfig{Server: server, Port: port, Nickname: nick, Channels: chs, AllowedUsers: users, ServerPassword: sp, NickservPassword: np, SASLPassword: sasl, VerifyTLS: verify}
			fmt.Printf("  ✅ IRC configured as %s@%s:%d\n\n", nick, server, port)
		case 7:
			fmt.Println()
			fmt.Println("  Webhook Setup — HTTP endpoint for custom integrations")
			portStr := strings.TrimSpace(promptString("Port", "8080"))
			sec := strings.TrimSpace(promptString("Secret (optional, Enter to skip)", ""))
			port, _ := strconv.Atoi(portStr)
			if port == 0 {
				port = 8080
			}
			out.Webhook = &config.WebhookConfig{Port: port, Secret: sec}
			fmt.Printf("  ✅ Webhook on port %d\n\n", port)
		case 8:
			fmt.Println()
			fmt.Println("  Feishu Setup — Connect to Feishu/Lark")
			printBullet("1. Visit https://open.feishu.cn/app to create a custom app")
			printBullet("2. Add Bot capability, get App ID and App Secret")
			printBullet("3. Configure event subscription Request URL and Encrypt Key")
			fmt.Println()
			appID := strings.TrimSpace(promptString("App ID", ""))
			appSecret := strings.TrimSpace(promptString("App Secret", ""))
			if appID == "" || appSecret == "" {
				fmt.Println("  → Skipped")
				continue
			}
			verifyToken := strings.TrimSpace(promptString("Verification Token (optional)", ""))
			encryptKey := strings.TrimSpace(promptString("Encrypt Key (optional)", ""))
			users := parseCSV(promptString("Allowed user IDs (comma-separated, * for all)", "*"))
			out.Feishu = &config.FeishuConfig{
				AppID:        appID,
				AppSecret:    appSecret,
				VerifyToken:  verifyToken,
				EncryptKey:   encryptKey,
				AllowedUsers: users,
			}
			fmt.Println("  ✅ Feishu configured")
			fmt.Println()
		case 9:
			fmt.Println()
			fmt.Println("  WeCom Setup — Connect to WeCom")
			printBullet("1. Visit https://work.weixin.qq.com admin console")
			printBullet("2. Create a custom app, get Corp ID, Agent ID and Secret")
			printBullet("3. Configure message server Token and EncodingAESKey")
			fmt.Println()
			corpID := strings.TrimSpace(promptString("Corp ID", ""))
			agentIDStr := strings.TrimSpace(promptString("Agent ID", ""))
			secret := strings.TrimSpace(promptString("Secret", ""))
			if corpID == "" || secret == "" {
				fmt.Println("  → Skipped")
				continue
			}
			agentID, _ := strconv.Atoi(agentIDStr)
			token := strings.TrimSpace(promptString("Token (optional)", ""))
			encodingKey := strings.TrimSpace(promptString("EncodingAESKey (optional)", ""))
			users := parseCSV(promptString("Allowed user IDs (comma-separated, * for all)", "*"))
			out.WeCom = &config.WeComConfig{
				CorpID:         corpID,
				AgentID:        agentID,
				Secret:         secret,
				Token:          token,
				EncodingAESKey: encodingKey,
				AllowedUsers:   users,
			}
			fmt.Println("  ✅ WeCom configured")
			fmt.Println()
		case 10:
			fmt.Println()
			fmt.Println("  WeChat Setup — Connect Official Account or Personal")
			printBullet("Official Account: requires verified service account")
			printBullet("Personal: requires bridge service (e.g. wechaty)")
			fmt.Println()
			modes := []string{"Official Account", "Personal (via bridge)"}
			modeIdx := promptSelect("Select WeChat mode", modes, 0)
			fmt.Printf("  Select WeChat mode: %s\n", modes[modeIdx])
			if modeIdx == 0 {
				fmt.Println()
				printBullet("1. Visit https://mp.weixin.qq.com")
				printBullet("2. Get AppID and AppSecret")
				printBullet("3. Configure server Token and EncodingAESKey")
				fmt.Println()
				appID := strings.TrimSpace(promptString("AppID", ""))
				appSecret := strings.TrimSpace(promptString("AppSecret", ""))
				if appID == "" || appSecret == "" {
					fmt.Println("  → Skipped")
					continue
				}
				token := strings.TrimSpace(promptString("Token", ""))
				encodingKey := strings.TrimSpace(promptString("EncodingAESKey (optional)", ""))
				users := parseCSV(promptString("Allowed user OpenIDs (comma-separated, * for all)", "*"))
				out.WeChat = &config.WeChatConfig{
					Mode:           "official",
					AppID:          appID,
					AppSecret:      appSecret,
					Token:          token,
					EncodingAESKey: encodingKey,
					AllowedUsers:   users,
				}
			} else {
				fmt.Println()
				printBullet("Personal mode requires wechaty or similar bridge service")
				printBullet("Bridge service provides HTTP API for HighClaw to call")
				fmt.Println()
				bridgeURL := strings.TrimSpace(promptString("Bridge URL (e.g. http://localhost:8788)", ""))
				if bridgeURL == "" {
					fmt.Println("  → Skipped")
					continue
				}
				bridgeToken := strings.TrimSpace(promptString("Bridge Token (optional)", ""))
				users := parseCSV(promptString("Allowed wxids (comma-separated, * for all)", "*"))
				out.WeChat = &config.WeChatConfig{
					Mode:                "personal",
					PersonalBridgeURL:   bridgeURL,
					PersonalBridgeToken: bridgeToken,
					AllowedUsers:        users,
				}
			}
			fmt.Println("  ✅ WeChat configured")
			fmt.Println()
		default:
			fmt.Printf("  %s Channels: %s\n", green("✓"), green(channelsSummary(out)))
			return out
		}
	}
}

func setupTunnel() config.TunnelConfig {
	printBullet("A tunnel exposes your gateway to the internet securely.")
	printBullet("Skip this if you only use CLI or local channels.")
	fmt.Println()
	items := []string{
		"Skip — local only (default)",
		"Cloudflare Tunnel — Zero Trust, free tier",
		"Tailscale — private tailnet or public Funnel",
		"ngrok — instant public URLs",
		"Custom — bring your own (bore, frp, ssh, etc.)",
	}
	choice := promptSelect("Select tunnel provider", items, 0)
	fmt.Printf("  Select tunnel provider: %s\n", items[choice])
	fmt.Println()
	switch choice {
	case 1:
		fmt.Println()
		printBullet("Get your tunnel token from the Cloudflare Zero Trust dashboard.")
		token := strings.TrimSpace(promptString("Cloudflare tunnel token", ""))
		if token == "" {
			fmt.Println("  → Skipped")
			return config.TunnelConfig{Provider: "none"}
		}
		fmt.Printf("  %s Tunnel: %s\n", green("✓"), green("Cloudflare"))
		return config.TunnelConfig{Provider: "cloudflare", Cloudflare: &config.CloudflareTunnelConfig{Token: token}}
	case 2:
		fmt.Println()
		printBullet("Tailscale must be installed and authenticated (tailscale up).")
		funnel := promptYesNo("Use Funnel (public internet)? No = tailnet only", false)
		if funnel {
			fmt.Printf("  %s Tunnel: %s\n", green("✓"), green("Tailscale (Funnel — public)"))
		} else {
			fmt.Printf("  %s Tunnel: %s\n", green("✓"), green("Tailscale (Serve — tailnet only)"))
		}
		return config.TunnelConfig{Provider: "tailscale", Tailscale: &config.TailscaleTunnelConfig{Funnel: funnel}}
	case 3:
		fmt.Println()
		printBullet("Get your auth token at https://dashboard.ngrok.com/get-started/your-authtoken")
		auth := strings.TrimSpace(promptString("ngrok auth token", ""))
		if auth == "" {
			fmt.Println("  → Skipped")
			return config.TunnelConfig{Provider: "none"}
		}
		domain := strings.TrimSpace(promptString("Custom domain (optional, Enter to skip)", ""))
		fmt.Printf("  %s Tunnel: %s\n", green("✓"), green("ngrok"))
		return config.TunnelConfig{Provider: "ngrok", Ngrok: &config.NgrokTunnelConfig{AuthToken: auth, Domain: domain}}
	case 4:
		fmt.Println()
		printBullet("Enter the command to start your tunnel.")
		printBullet("Use {port} and {host} as placeholders.")
		printBullet("Example: bore local {port} --to bore.pub")
		cmd := strings.TrimSpace(promptString("Start command", ""))
		if cmd == "" {
			fmt.Println("  → Skipped")
			return config.TunnelConfig{Provider: "none"}
		}
		fmt.Printf("  %s Tunnel: %s\n", green("✓"), green(fmt.Sprintf("Custom (%s)", cmd)))
		return config.TunnelConfig{Provider: "custom", Custom: &config.CustomTunnelConfig{StartCommand: cmd}}
	default:
		fmt.Printf("  %s Tunnel: %s\n", green("✓"), green("none (local only)"))
		return config.TunnelConfig{Provider: "none"}
	}
}

func setupToolMode() (config.ComposioConfig, config.SecretsConfig) {
	printBullet("Choose how HighClaw connects to external apps.")
	printBullet("You can always change this later in config.yaml.")
	fmt.Println()
	items := []string{
		"Sovereign (local only) — you manage API keys, full privacy (default)",
		"Composio (managed OAuth) — 1000+ apps via OAuth, no raw keys shared",
	}
	i := promptSelect("Select tool mode", items, 0)
	fmt.Printf("  Select tool mode: %s\n", items[i])
	fmt.Println()
	comp := config.ComposioConfig{}
	if i == 1 {
		fmt.Println()
		fmt.Println("  Composio Setup — 1000+ OAuth integrations (Gmail, Notion, GitHub, Slack, ...)")
		printBullet("Get your API key at: https://app.composio.dev/settings")
		printBullet("HighClaw uses Composio as a tool — your core agent stays local.")
		fmt.Println()
		key := strings.TrimSpace(promptString("Composio API key (or Enter to skip)", ""))
		if key != "" {
			comp.Enabled = true
			comp.APIKey = key
			fmt.Printf("  %s Composio: %s\n", green("✓"), green("enabled (1000+ OAuth tools available)"))
		} else {
			fmt.Println("  → Skipped — set composio.api_key in config.yaml later")
			fmt.Printf("  %s Tool mode: %s\n", green("✓"), green("Sovereign (local only)")+" — full privacy, you own every key")
		}
	} else {
		fmt.Printf("  %s Tool mode: %s\n", green("✓"), green("Sovereign (local only)")+" — full privacy, you own every key")
	}
	fmt.Println()
	printBullet("HighClaw can encrypt API keys stored in config.yaml.")
	printBullet("A local key file protects against plaintext exposure and accidental leaks.")
	enc := promptYesNo("Enable encrypted secret storage?", true)
	if enc {
		fmt.Printf("  %s Secrets: %s\n", green("✓"), green("encrypted")+" — keys encrypted with local key file")
	} else {
		fmt.Printf("  %s Secrets: %s\n", green("✓"), yellow("plaintext")+" — keys stored as plaintext (not recommended)")
	}
	return comp, config.SecretsConfig{Encrypt: enc}
}

func setupMemory() config.MemoryConfig {
	printBullet("Choose how HighClaw stores and searches memories.")
	printBullet("You can always change this later in config.yaml.")
	fmt.Println()
	items := []string{
		"SQLite with Vector Search (recommended) — fast, hybrid search, embeddings",
		"Markdown Files — simple, human-readable, no dependencies",
		"None — disable persistent memory",
	}
	choice := promptSelect("Select memory backend", items, 0)
	fmt.Printf("  Select memory backend: %s\n", items[choice])
	backend := "sqlite"
	switch choice {
	case 1:
		backend = "markdown"
	case 2:
		backend = "none"
	}
	autoSave := false
	if backend != "none" {
		autoSave = promptYesNo("Auto-save conversations to memory?", true)
	}
	cfg := memoryConfigForBackend(backend, autoSave)
	fmt.Printf("  %s Memory: %s (auto-save: %s)\n", green("✓"), green(cfg.Backend), onOff(cfg.AutoSave))
	return cfg
}

func setupProjectContext() projectContext {
	printBullet("Let's personalize your agent. You can always update these later.")
	printBullet("Press Enter to accept defaults.")
	fmt.Println()
	user := strings.TrimSpace(promptString("Your name", "User"))
	if user == "" {
		user = "User"
	}
	tzChoices := []string{
		"US/Eastern (EST/EDT)",
		"US/Central (CST/CDT)",
		"US/Mountain (MST/MDT)",
		"US/Pacific (PST/PDT)",
		"Europe/London (GMT/BST)",
		"Europe/Berlin (CET/CEST)",
		"Asia/Tokyo (JST)",
		"UTC",
		"Other (type manually)",
	}
	tzI := promptSelect("Your timezone", tzChoices, 0)
	tzDisplay := tzChoices[tzI]
	tz := "US/Eastern"
	if tzI == len(tzChoices)-1 {
		tzDisplay = strings.TrimSpace(promptString("Enter timezone (e.g. America/New_York)", "UTC"))
		tz = tzDisplay
	} else {
		tz = strings.TrimSpace(strings.SplitN(tzDisplay, "(", 2)[0])
	}
	if tz == "" {
		tz = "UTC"
		tzDisplay = "UTC"
	}
	agent := strings.TrimSpace(promptString("Agent name", "HighClaw"))
	if agent == "" {
		agent = "HighClaw"
	}
	styles := []string{
		"Direct & concise — skip pleasantries, get to the point",
		"Friendly & casual — warm, human, and helpful",
		"Professional & polished — calm, confident, and clear",
		"Expressive & playful — more personality + natural emojis",
		"Technical & detailed — thorough explanations, code-first",
		"Balanced — adapt to the situation",
		"Custom — write your own style guide",
	}
	si := promptSelect("Communication style", styles, 1)
	style := []string{
		"Be direct and concise. Skip pleasantries. Get to the point.",
		"Be friendly, human, and conversational. Show warmth and empathy while staying efficient. Use natural contractions.",
		"Be professional and polished. Stay calm, structured, and respectful. Use occasional tone-setting emojis only when appropriate.",
		"Be expressive and playful when appropriate. Use relevant emojis naturally (0-2 max), and keep serious topics emoji-light.",
		"Be technical and detailed. Thorough explanations, code-first.",
		"Adapt to the situation. Default to warm and clear communication; be concise when needed, thorough when it matters.",
	}[min(si, 5)]
	if si == 6 {
		style = promptString("Custom communication style", "Be warm, natural, and clear. Use occasional relevant emojis (1-2 max) and avoid robotic phrasing.")
	}
	styleLabel := styles[si]
	if si == 6 {
		styleLabel = "Custom"
	}
	fmt.Printf("  Your name: %s\n", user)
	fmt.Printf("  Your timezone: %s\n", tzDisplay)
	fmt.Printf("  Agent name: %s\n", agent)
	fmt.Printf("  Communication style: %s\n", styleLabel)
	fmt.Printf("  %s Context: %s %s %s %s %s %s %s\n",
		green("✓"),
		green(user),
		gray("|"),
		green(tz),
		gray("|"),
		green(agent),
		gray("|"),
		green(style),
	)
	return projectContext{UserName: user, Timezone: tz, AgentName: agent, CommunicationStyle: style}
}

func printSummary(cfg *config.Config) {
	fmt.Println()
	fmt.Println("  " + cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println("  " + cyan("⚡") + "  " + bold("HighClaw is ready!"))
	fmt.Println("  " + cyan("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"))
	fmt.Println()
	fmt.Println("  " + gray("Configuration saved to:"))
	fmt.Printf("    %s\n\n", green(config.ConfigPath()))
	fmt.Println("  " + bold("Quick summary:"))
	fmt.Printf("    🤖 Provider:      %s\n", green(modelProvider(cfg.Agent.Model)))
	fmt.Printf("    🧠 Model:         %s\n", green(modelName(cfg.Agent.Model)))
	autonomy := strings.TrimSpace(cfg.Autonomy.Level)
	if autonomy != "" {
		autonomy = strings.ToUpper(autonomy[:1]) + autonomy[1:]
	}
	fmt.Printf("    🛡️ Autonomy:      %s\n", green(autonomy))
	fmt.Printf("    🧠 Memory:        %s (auto-save: %s)\n", green(cfg.Memory.Backend), onOff(cfg.Memory.AutoSave))
	fmt.Printf("    📡 Channels:      %s\n", channelsSummary(cfg.Channels))
	if hasAPIKey(cfg, modelProvider(cfg.Agent.Model)) {
		fmt.Printf("    🔑 API Key:       %s\n", green("configured"))
	} else {
		fmt.Printf("    🔑 API Key:       %s\n", yellow("not set (set via env var or config)"))
	}
	if cfg.Tunnel.Provider == "" || cfg.Tunnel.Provider == "none" {
		fmt.Println("    🌐 Tunnel:        none (local only)")
	} else {
		fmt.Printf("    🌐 Tunnel:        %s\n", cfg.Tunnel.Provider)
	}
	if cfg.Composio.Enabled {
		fmt.Println("    🔗 Composio:      enabled (1000+ OAuth apps)")
	} else {
		fmt.Println("    🔗 Composio:      disabled (sovereign mode)")
	}
	if cfg.Secrets.Encrypt {
		fmt.Printf("    🔒 Secrets:       %s\n", green("encrypted"))
	} else {
		fmt.Println("    🔒 Secrets:       plaintext")
	}
	fmt.Printf("    🚪 Gateway:       %s\n", green("pairing required (secure)"))
	if isGatewayRunning(cfg) {
		fmt.Printf("    🟢 Process:       %s\n", green(fmt.Sprintf("running (port %d)", cfg.Gateway.Port)))
	} else {
		fmt.Printf("    ⚪ Process:       %s\n", gray("not running"))
	}
	gatewayRunning := isGatewayRunning(cfg)
	hasChannels := hasConfiguredChannels(cfg.Channels)

	fmt.Println()

	if gatewayRunning {
		fmt.Printf("  ⚠️  %s\n", bold("Gateway is running — new config will reload automatically"))
		fmt.Println()
		fmt.Println("  " + bold("Next steps:"))
		fmt.Println()
		if hasChannels {
			fmt.Printf("    %s Send a message to chat\n", cyan("1."))
		} else {
			fmt.Printf("    %s Send a bind verification code message to chat\n", cyan("1."))
		}
		fmt.Println("       📱 Telegram   💬 Discord    🔔 Slack")
		fmt.Println("       💻 iMessage   🔗 Matrix     📞 WhatsApp")
		fmt.Println("       📧 IRC        🌐 Webhook    🐦 Feishu")
		fmt.Println("       🏢 WeCom     💚 WeChat")
		fmt.Println()
		fmt.Printf("    %s Start interactive TUI mode:\n", cyan("2."))
		fmt.Printf("       %s\n", magenta("highclaw tui"))
		fmt.Println()
		fmt.Printf("    %s Check full status:\n", cyan("3."))
		fmt.Printf("       %s\n", magenta("highclaw status"))
		fmt.Println()
		fmt.Printf("    %s Key APIs %s:\n", cyan("4."), gray(fmt.Sprintf("(localhost:%d)", cfg.Gateway.Port)))
		fmt.Printf("       🔌 %s  — AI conversation\n", magenta("POST /api/chat"))
		fmt.Printf("       📊 %s — system status\n", magenta("GET  /api/status"))
		fmt.Printf("       🔄 %s — channel health\n", magenta("GET  /api/channels/status"))
		fmt.Println()
	} else {
		fmt.Println("  " + bold("Next steps:"))
		fmt.Println()
		step := 1
		if !hasAPIKey(cfg, modelProvider(cfg.Agent.Model)) {
			fmt.Printf("    %s Set your API key:\n", cyan(fmt.Sprintf("%d.", step)))
			fmt.Printf("       %s\n\n", magenta(fmt.Sprintf("export %s=\"sk-...\"", providerEnvVar(modelProvider(cfg.Agent.Model)))))
			step++
		}
		fmt.Printf("    %s Send a quick message:\n", cyan(fmt.Sprintf("%d.", step)))
		fmt.Printf("       %s\n", magenta("highclaw agent -m \"Hello, HighClaw!\""))
		fmt.Println()
		step++
		fmt.Printf("    %s Start interactive CLI mode:\n", cyan(fmt.Sprintf("%d.", step)))
		fmt.Printf("       %s\n", magenta("highclaw agent"))
		fmt.Println()
		step++
		fmt.Printf("    %s Check full status:\n", cyan(fmt.Sprintf("%d.", step)))
		fmt.Printf("       %s\n", magenta("highclaw status"))
		fmt.Println()
		step++
		fmt.Printf("    %s Start the gateway (channels + AI + API):\n", cyan(fmt.Sprintf("%d.", step)))
		fmt.Printf("       %s\n", magenta("highclaw gateway"))
		fmt.Println()
	}
	fmt.Println("  ⚡ Happy hacking! 🦀")
	fmt.Println()
}

type modelOpt struct {
	ID    string
	Label string
}

func modelsForProvider(provider string) []modelOpt {
	switch provider {
	case "openrouter":
		return []modelOpt{{"anthropic/claude-sonnet-4", "Claude Sonnet 4 (balanced, recommended)"}, {"anthropic/claude-3.5-sonnet", "Claude 3.5 Sonnet (fast, affordable)"}, {"openai/gpt-4o", "GPT-4o (OpenAI flagship)"}, {"openai/gpt-4o-mini", "GPT-4o Mini (fast, cheap)"}, {"google/gemini-2.0-flash-001", "Gemini 2.0 Flash (Google, fast)"}, {"meta-llama/llama-3.3-70b-instruct", "Llama 3.3 70B (open source)"}, {"deepseek/deepseek-chat", "DeepSeek Chat (affordable)"}}
	case "anthropic":
		return []modelOpt{{"claude-sonnet-4-20250514", "Claude Sonnet 4 (balanced, recommended)"}, {"claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet (fast)"}, {"claude-3-5-haiku-20241022", "Claude 3.5 Haiku (fastest, cheapest)"}}
	case "openai":
		return []modelOpt{{"gpt-4o", "GPT-4o (flagship)"}, {"gpt-4o-mini", "GPT-4o Mini (fast, cheap)"}, {"o1-mini", "o1-mini (reasoning)"}}
	case "venice":
		return []modelOpt{{"llama-3.3-70b", "Llama 3.3 70B (default, fast)"}, {"claude-opus-45", "Claude Opus 4.5 via Venice (strongest)"}, {"llama-3.1-405b", "Llama 3.1 405B (largest open source)"}}
	case "groq":
		return []modelOpt{{"llama-3.3-70b-versatile", "Llama 3.3 70B (fast, recommended)"}, {"llama-3.1-8b-instant", "Llama 3.1 8B (instant)"}, {"mixtral-8x7b-32768", "Mixtral 8x7B (32K context)"}}
	case "mistral":
		return []modelOpt{{"mistral-large-latest", "Mistral Large (flagship)"}, {"codestral-latest", "Codestral (code-focused)"}, {"mistral-small-latest", "Mistral Small (fast, cheap)"}}
	case "deepseek":
		return []modelOpt{{"deepseek-chat", "DeepSeek Chat (V3, recommended)"}, {"deepseek-reasoner", "DeepSeek Reasoner (R1)"}}
	case "xai":
		return []modelOpt{{"grok-3", "Grok 3 (flagship)"}, {"grok-3-mini", "Grok 3 Mini (fast)"}}
	case "perplexity":
		return []modelOpt{{"sonar-pro", "Sonar Pro (search + reasoning)"}, {"sonar", "Sonar (search, fast)"}}
	case "fireworks":
		return []modelOpt{{"accounts/fireworks/models/llama-v3p3-70b-instruct", "Llama 3.3 70B"}, {"accounts/fireworks/models/mixtral-8x22b-instruct", "Mixtral 8x22B"}}
	case "together":
		return []modelOpt{{"meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", "Llama 3.1 70B Turbo"}, {"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo", "Llama 3.1 8B Turbo"}, {"mistralai/Mixtral-8x22B-Instruct-v0.1", "Mixtral 8x22B"}}
	case "cohere":
		return []modelOpt{{"command-r-plus", "Command R+ (flagship)"}, {"command-r", "Command R (fast)"}}
	case "moonshot":
		return []modelOpt{{"moonshot-v1-128k", "Moonshot V1 128K"}, {"moonshot-v1-32k", "Moonshot V1 32K"}}
	case "glm", "zhipu", "zai", "z.ai":
		return []modelOpt{{"glm-5", "GLM-5 (latest)"}, {"glm-4-plus", "GLM-4 Plus (flagship)"}, {"glm-4-flash", "GLM-4 Flash (fast)"}}
	case "minimax":
		return []modelOpt{{"abab6.5s-chat", "ABAB 6.5s Chat"}, {"abab6.5-chat", "ABAB 6.5 Chat"}}
	case "ollama":
		return []modelOpt{{"llama3.2", "Llama 3.2 (recommended local)"}, {"mistral", "Mistral 7B"}, {"codellama", "Code Llama"}, {"phi3", "Phi-3 (small, fast)"}}
	case "gemini", "google", "google-gemini":
		return []modelOpt{{"gemini-2.0-flash", "Gemini 2.0 Flash (fast, recommended)"}, {"gemini-2.0-flash-lite", "Gemini 2.0 Flash Lite (fastest, cheapest)"}, {"gemini-1.5-pro", "Gemini 1.5 Pro (best quality)"}, {"gemini-1.5-flash", "Gemini 1.5 Flash (balanced)"}}
	default:
		return []modelOpt{{"default", "Default model"}}
	}
}

func providerEnvVar(name string) string {
	switch name {
	case "openrouter":
		return "OPENROUTER_API_KEY"
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "venice":
		return "VENICE_API_KEY"
	case "groq":
		return "GROQ_API_KEY"
	case "mistral":
		return "MISTRAL_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "xai", "grok":
		return "XAI_API_KEY"
	case "together", "together-ai":
		return "TOGETHER_API_KEY"
	case "fireworks", "fireworks-ai":
		return "FIREWORKS_API_KEY"
	case "perplexity":
		return "PERPLEXITY_API_KEY"
	case "cohere":
		return "COHERE_API_KEY"
	case "moonshot", "kimi":
		return "MOONSHOT_API_KEY"
	case "glm", "zhipu":
		return "GLM_API_KEY"
	case "minimax":
		return "MINIMAX_API_KEY"
	case "qianfan", "baidu":
		return "QIANFAN_API_KEY"
	case "zai", "z.ai":
		return "ZAI_API_KEY"
	case "synthetic":
		return "SYNTHETIC_API_KEY"
	case "opencode", "opencode-zen":
		return "OPENCODE_API_KEY"
	case "vercel", "vercel-ai":
		return "VERCEL_API_KEY"
	case "cloudflare", "cloudflare-ai":
		return "CLOUDFLARE_API_KEY"
	case "bedrock", "aws-bedrock":
		return "AWS_ACCESS_KEY_ID"
	case "gemini", "google", "google-gemini":
		return "GEMINI_API_KEY"
	default:
		return "API_KEY"
	}
}

func apiKeyURL(provider string) string {
	switch provider {
	case "openrouter":
		return "https://openrouter.ai/keys"
	case "anthropic":
		return "https://console.anthropic.com/settings/keys"
	case "openai":
		return "https://platform.openai.com/api-keys"
	case "venice":
		return "https://venice.ai/settings/api"
	case "groq":
		return "https://console.groq.com/keys"
	case "mistral":
		return "https://console.mistral.ai/api-keys"
	case "deepseek":
		return "https://platform.deepseek.com/api_keys"
	case "together":
		return "https://api.together.xyz/settings/api-keys"
	case "fireworks":
		return "https://fireworks.ai/account/api-keys"
	case "perplexity":
		return "https://www.perplexity.ai/settings/api"
	case "xai":
		return "https://console.x.ai"
	case "cohere":
		return "https://dashboard.cohere.com/api-keys"
	case "moonshot":
		return "https://platform.moonshot.cn/console/api-keys"
	case "glm", "zhipu":
		return "https://open.bigmodel.cn/usercenter/proj-mgmt/apikeys"
	case "zai", "z.ai":
		return "https://platform.z.ai/"
	case "minimax":
		return "https://www.minimaxi.com/user-center/basic-information"
	case "vercel":
		return "https://vercel.com/account/tokens"
	case "cloudflare":
		return "https://dash.cloudflare.com/profile/api-tokens"
	case "bedrock":
		return "https://console.aws.amazon.com/iam"
	case "gemini", "google", "google-gemini":
		return "https://aistudio.google.com/app/apikey"
	}
	return ""
}

func providerDefaultBaseURL(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "groq":
		return "https://api.groq.com/openai/v1"
	case "mistral":
		return "https://api.mistral.ai/v1"
	case "together":
		return "https://api.together.xyz/v1"
	case "fireworks":
		return "https://api.fireworks.ai/inference/v1"
	case "cohere":
		return "https://api.cohere.com/compatibility/v1"
	case "moonshot":
		return "https://api.moonshot.cn/v1"
	case "glm", "zhipu", "zai", "z.ai":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "xai":
		return "https://api.x.ai/v1"
	case "perplexity":
		return "https://api.perplexity.ai"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	return ""
}

func defaultModelForProvider(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	case "glm", "zhipu", "zai", "z.ai":
		return "glm-5"
	case "ollama":
		return "llama3.2"
	case "groq":
		return "llama-3.3-70b-versatile"
	case "deepseek":
		return "deepseek-chat"
	case "gemini", "google", "google-gemini":
		return "gemini-2.0-flash"
	default:
		return "anthropic/claude-sonnet-4"
	}
}

func ensureWorkspaceLayout(workspace string, ctx projectContext) (createdFiles, skippedFiles, createdDirs int, workspacePath string, err error) {
	workspacePath = expandPath(workspace)
	dirs := []string{"sessions", "memory", "state", "cron", "skills"}
	files := workspaceTemplates(ctx)
	if err = os.MkdirAll(workspacePath, 0o755); err != nil {
		return
	}
	for _, d := range dirs {
		p := filepath.Join(workspacePath, d)
		if _, st := os.Stat(p); os.IsNotExist(st) {
			if mk := os.MkdirAll(p, 0o755); mk != nil {
				err = mk
				return
			}
			createdDirs++
		}
	}
	for name, content := range files {
		p := filepath.Join(workspacePath, name)
		if _, st := os.Stat(p); os.IsNotExist(st) {
			if w := os.WriteFile(p, []byte(content), 0o644); w != nil {
				err = w
				return
			}
			createdFiles++
		} else {
			skippedFiles++
		}
	}
	return
}

func workspaceTemplates(ctx projectContext) map[string]string {
	agent := strings.TrimSpace(ctx.AgentName)
	if agent == "" {
		agent = "HighClaw"
	}
	user := strings.TrimSpace(ctx.UserName)
	if user == "" {
		user = "User"
	}
	tz := strings.TrimSpace(ctx.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	style := strings.TrimSpace(ctx.CommunicationStyle)
	if style == "" {
		style = "Be warm, natural, and clear. Use occasional relevant emojis (1-2 max) and avoid robotic phrasing."
	}
	return map[string]string{
		"IDENTITY.md":  fmt.Sprintf("# IDENTITY.md — Who Am I?\n\n- **Name:** %s\n- **Creature:** A Go-forged AI assistant\n- **Vibe:** Sharp, direct, resourceful\n- **Emoji:** 🦀\n", agent),
		"AGENTS.md":    fmt.Sprintf("# AGENTS.md — %s Personal Assistant\n\n## Every Session\n\n1. Read `SOUL.md`\n2. Read `USER.md`\n3. Use memory for recent context\n4. Execute with clarity and safety\n\n## Safety\n\n- Never exfiltrate secrets.\n- Ask before destructive actions.\n- Prefer recoverable operations.\n", agent),
		"HEARTBEAT.md": fmt.Sprintf("# HEARTBEAT.md\n\n# Keep this file empty (or comments only) to skip heartbeat work.\n# Add periodic checks you want %s to run.\n", agent),
		"SOUL.md":      fmt.Sprintf("# SOUL.md — Who You Are\n\nYou are **%s**.\n\n- Be genuinely helpful.\n- Be direct and grounded.\n- Keep privacy and safety first.\n- Communicate in this style: %s\n", agent, style),
		"USER.md":      fmt.Sprintf("# USER.md — Who You're Helping\n\n- **Name:** %s\n- **Timezone:** %s\n- **Preferred style:** %s\n", user, tz, style),
		"TOOLS.md":     "# TOOLS.md — Local Notes\n\nStore machine-specific tool notes here (SSH aliases, hostnames, local paths, conventions).\n",
		"BOOTSTRAP.md": fmt.Sprintf("# BOOTSTRAP.md — Hello, World\n\nYour human is **%s** (timezone: %s).\nIntroduce yourself as %s and gather practical preferences.\n", user, tz, agent),
		"MEMORY.md":    "# MEMORY.md — Long-Term Memory\n\nCurate durable memory: key facts, decisions, preferences, open loops.\nKeep concise and high-signal.\n",
	}
}

func promptYesNo(label string, def bool) bool {
	suffix := "[Y/n]"
	if !def {
		suffix = "[y/N]"
	}
	fmt.Printf("  %s %s: ", label, suffix)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	in := strings.ToLower(strings.TrimSpace(line))
	if in == "" {
		return def
	}
	switch in {
	case "y", "yes", "true", "1":
		return true
	case "n", "no", "false", "0":
		return false
	default:
		return def
	}
}

func promptString(label, def string) string {
	v := strings.TrimSpace(def)
	i := huh.NewInput().Title(label).Value(&v)
	if def != "" {
		i.Placeholder(def)
	}
	if err := i.Run(); err != nil {
		return strings.TrimSpace(def)
	}
	return strings.TrimSpace(v)
}

func promptSelect(label string, options []string, def int) int {
	if len(options) == 0 {
		return 0
	}
	if def < 0 || def >= len(options) {
		def = 0
	}
	selected := options[def]
	items := make([]huh.Option[string], 0, len(options))
	for _, opt := range options {
		items = append(items, huh.NewOption(opt, opt))
	}
	if err := huh.NewSelect[string]().Title(label).Options(items...).Value(&selected).Run(); err != nil {
		return def
	}
	for i, opt := range options {
		if opt == selected {
			return i
		}
	}
	return def
}

func printStep(current, total int, title string) {
	fmt.Println()
	fmt.Printf("  %s %s\n", cyan(bold(fmt.Sprintf("[%d/%d]", current, total))), bold(title))
	fmt.Println("  " + gray("──────────────────────────────────────────────────"))
}

func printBullet(text string) {
	fmt.Printf("  %s %s\n", cyan("›"), highlightURLs(text))
}

func highlightURLs(text string) string {
	parts := strings.Split(text, " ")
	for i, p := range parts {
		token := strings.TrimSpace(p)
		trimmed := strings.TrimRight(token, ".,;:!?")
		suffix := strings.TrimPrefix(token, trimmed)
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			parts[i] = cyanUnder(trimmed) + suffix
		}
	}
	return strings.Join(parts, " ")
}

func memoryConfigForBackend(backend string, autoSave bool) config.MemoryConfig {
	return config.MemoryConfig{
		Backend:                   backend,
		AutoSave:                  autoSave,
		HygieneEnabled:            backend == "sqlite",
		ArchiveAfterDays:          ifInt(backend == "sqlite", 7, 0),
		PurgeAfterDays:            ifInt(backend == "sqlite", 30, 0),
		ConversationRetentionDays: 30,
		EmbeddingProvider:         "none",
		EmbeddingModel:            "text-embedding-3-small",
		EmbeddingDimensions:       1536,
		VectorWeight:              0.7,
		KeywordWeight:             0.3,
		EmbeddingCacheSize:        ifInt(backend == "sqlite", 10000, 0),
		ChunkMaxTokens:            512,
	}
}

func ifInt(cond bool, a, b int) int {
	if cond {
		return a
	}
	return b
}

func testTelegram(token string) (string, bool) {
	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	result, _ := body["result"].(map[string]any)
	username, _ := result["username"].(string)
	if strings.TrimSpace(username) == "" {
		username = "unknown"
	}
	return username, true
}

func testDiscord(token string) (string, bool) {
	req, _ := http.NewRequest(http.MethodGet, "https://discord.com/api/v10/users/@me", nil)
	req.Header.Set("Authorization", "Bot "+token)
	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	name, _ := body["username"].(string)
	if strings.TrimSpace(name) == "" {
		name = "unknown"
	}
	return name, true
}

func testSlack(token string) (string, bool) {
	req, _ := http.NewRequest(http.MethodGet, "https://slack.com/api/auth.test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	ok, _ := body["ok"].(bool)
	team, _ := body["team"].(string)
	if strings.TrimSpace(team) == "" {
		team = "unknown"
	}
	return team, ok
}

func testMatrix(home, token string) (string, bool) {
	req, _ := http.NewRequest(http.MethodGet, strings.TrimRight(home, "/")+"/_matrix/client/v3/account/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", false
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	userID, _ := body["user_id"].(string)
	if strings.TrimSpace(userID) == "" {
		userID = "unknown"
	}
	return userID, true
}

func testWhatsApp(accessToken, phoneID string) bool {
	req, _ := http.NewRequest(http.MethodGet, "https://graph.facebook.com/v18.0/"+strings.TrimSpace(phoneID), nil)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	client := http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func statusLine(name string, connected bool, pending string) string {
	if connected {
		return fmt.Sprintf("%-10s %s", name, "✅ connected")
	}
	return fmt.Sprintf("%-10s %s", name, pending)
}

func statusLineWithText(name, status string) string {
	return fmt.Sprintf("%-10s %s", name, status)
}

func channelState(configured bool, configuredText, pendingText string) string {
	if configured {
		return configuredText
	}
	return pendingText
}

func parseCSV(v string) []string {
	raw := strings.TrimSpace(v)
	if raw == "" {
		return nil
	}
	if raw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func channelsSummary(c config.ChannelsConfig) string {
	x := []string{"CLI"}
	if c.Telegram != nil {
		x = append(x, "Telegram")
	}
	if c.Discord != nil {
		x = append(x, "Discord")
	}
	if c.Slack != nil {
		x = append(x, "Slack")
	}
	if c.IMessage != nil {
		x = append(x, "iMessage")
	}
	if c.Matrix != nil {
		x = append(x, "Matrix")
	}
	if c.WhatsApp != nil {
		x = append(x, "WhatsApp")
	}
	if c.Email != nil {
		x = append(x, "Email")
	}
	if c.IRC != nil {
		x = append(x, "IRC")
	}
	if c.Webhook != nil {
		x = append(x, "Webhook")
	}
	if c.Feishu != nil {
		x = append(x, "Feishu")
	}
	if c.WeCom != nil {
		x = append(x, "WeCom")
	}
	if c.WeChat != nil {
		x = append(x, "WeChat")
	}
	return strings.Join(x, ", ")
}

func hasConfiguredChannels(c config.ChannelsConfig) bool {
	return c.Telegram != nil || c.Discord != nil || c.Slack != nil ||
		c.IMessage != nil || c.Matrix != nil || c.Email != nil ||
		c.Feishu != nil || c.WeCom != nil || c.WeChat != nil ||
		c.WhatsApp != nil || c.Signal != nil || c.BlueBubbles != nil ||
		c.Webhook != nil || c.IRC != nil || c.Lark != nil
}

// promptLaunchChannels 在 gateway 运行时自动触发 channel 重载并通知用户
func promptLaunchChannels(cfg *config.Config) {
	if !hasConfiguredChannels(cfg.Channels) {
		return
	}
	if !hasAPIKey(cfg, modelProvider(cfg.Agent.Model)) {
		return
	}
	if !isGatewayRunning(cfg) {
		return
	}

	// gateway 运行中：尝试通知重载 channel 配置
	url := fmt.Sprintf("http://127.0.0.1:%d/api/internal/reload", cfg.Gateway.Port)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf("  %s Gateway detected — channels reloaded automatically\n", green("✓"))
	} else {
		fmt.Printf("  %s Gateway reload returned %d (restart gateway to apply)\n", yellow("!"), resp.StatusCode)
	}
}

func hasAPIKey(cfg *config.Config, provider string) bool {
	if cfg.Agent.Providers == nil {
		return false
	}
	p, ok := cfg.Agent.Providers[provider]
	return ok && strings.TrimSpace(p.APIKey) != ""
}

func modelProvider(model string) string {
	parts := strings.SplitN(strings.TrimSpace(model), "/", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	if len(parts) == 1 && parts[0] != "" {
		return "openrouter"
	}
	return "openrouter"
}

func modelName(model string) string {
	parts := strings.SplitN(strings.TrimSpace(model), "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return strings.TrimSpace(model)
}

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func printWorkspaceTree(workspacePath string) {
	fmt.Println()
	fmt.Println("  Workspace layout:")
	fmt.Printf("    %s/\n", workspacePath)
	fmt.Println("    ├── sessions/")
	fmt.Println("    ├── memory/")
	fmt.Println("    ├── state/")
	fmt.Println("    ├── cron/")
	fmt.Println("    ├── skills/")
	fmt.Println("    ├── IDENTITY.md")
	fmt.Println("    ├── AGENTS.md")
	fmt.Println("    ├── HEARTBEAT.md")
	fmt.Println("    ├── SOUL.md")
	fmt.Println("    ├── USER.md")
	fmt.Println("    ├── TOOLS.md")
	fmt.Println("    ├── BOOTSTRAP.md")
	fmt.Println("    └── MEMORY.md")
}

func runtimeGOOS() string {
	return runtime.GOOS
}

func hasGeminiCLICredentials() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	candidates := []string{
		filepath.Join(home, ".gemini", "oauth_creds.json"),
		filepath.Join(home, ".config", "gemini", "oauth_creds.json"),
		filepath.Join(home, ".config", "google-generativeai", "oauth_creds.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func color(c, s string) string {
	return c + s + ansiReset
}

func bold(s string) string  { return color(ansiBold, s) }
func under(s string) string { return color(ansiUnder, s) }
func cyan(s string) string  { return color(ansiCyan, s) }
func cyanUnder(s string) string {
	return ansiUnder + ansiCyan + s + ansiReset
}
func green(s string) string  { return color(ansiGreen, s) }
func yellow(s string) string { return color(ansiYellow, s) }
func gray(s string) string    { return color(ansiGray, s) }
func magenta(s string) string { return color(ansiMagenta, s) }

func expandPath(in string) string {
	v := strings.TrimSpace(in)
	if v == "" {
		return v
	}
	if v == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(v, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(v, "~/"))
	}
	return v
}

// setupSkills 在 onboard 向导中配置 skills
func setupSkills(workspace string) {
	printBullet("Skills extend your agent's capabilities with specialized knowledge.")
	printBullet("Open-skills from the community are auto-loaded from ~/open-skills/")
	printBullet("Custom skills can be placed in " + filepath.Join(workspace, "skills") + "/")
	fmt.Println()

	mgr := skills.NewManager(workspace)
	allSkills := mgr.LoadAll()

	if len(allSkills) == 0 {
		fmt.Printf("  %s Skills: %s (community skills will auto-load on first run)\n", green("✓"), yellow("none installed"))
	} else {
		fmt.Printf("  %s Skills: %d installed (open-skills + custom)\n", green("✓"), len(allSkills))
		for _, s := range allSkills {
			fmt.Printf("      • %s v%s\n", s.Name, s.Version)
		}
	}

	installMore := promptYesNo("Install additional skills from URL/path?", false)
	if !installMore {
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Println("  Skill Install — add custom skills from GitHub or local path")
	printBullet("Examples:")
	printBullet("  https://github.com/user/my-skill")
	printBullet("  /path/to/local/skill")
	fmt.Println()

	for {
		source := strings.TrimSpace(promptString("Skill URL or path (Enter to finish)", ""))
		if source == "" {
			break
		}

		fmt.Printf("  ⏳ Installing from %s...\n", source)
		if err := mgr.Install(source); err != nil {
			fmt.Printf("  %s Install failed: %v\n", yellow("⚠"), err)
			continue
		}
		fmt.Printf("  %s Skill installed successfully\n", green("✓"))
	}

	allSkills = mgr.LoadAll()
	fmt.Printf("  %s Total skills: %d\n", green("✓"), len(allSkills))
	fmt.Println()
}


// isGatewayRunning 检测 gateway 进程是否正在运行（尝试访问 /health）
func isGatewayRunning(cfg *config.Config) bool {
	if cfg.Gateway.Port == 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.Gateway.Port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

