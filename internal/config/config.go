// Package config handles loading and validating the HighClaw configuration.
// Config is stored at ~/.highclaw/config.yaml (YAML format), with legacy JSON fallback for migration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// Config is the top-level OpenClaw configuration.
type Config struct {
	Agent         AgentConfig         `json:"agent"`
	Gateway       GatewayConfig       `json:"gateway"`
	Channels      ChannelsConfig      `json:"channels"`
	Browser       BrowserConfig       `json:"browser"`
	Hooks         HooksConfig         `json:"hooks"`
	Web           WebConfig           `json:"web"`
	Autonomy      AutonomyConfig      `json:"autonomy"`
	Memory        MemoryConfig        `json:"memory"`
	Session       SessionConfig       `json:"session"`
	Reliability   ReliabilityConfig   `json:"reliability"`
	ModelRoutes   []ModelRouteConfig  `json:"modelRoutes"`
	Tunnel        TunnelConfig        `json:"tunnel"`
	Composio      ComposioConfig      `json:"composio"`
	Secrets       SecretsConfig       `json:"secrets"`
	Identity      IdentityConfig      `json:"identity"`
	Observability ObservabilityConfig `json:"observability"`
}

// SessionConfig 控制会话路由策略
type SessionConfig struct {
	// Scope 控制全局会话粒度: "per-sender" | "global"
	Scope string `json:"scope"`
	// DMScope 控制 DM 会话隔离级别:
	//   "main"                   — 所有 DM 共享主会话
	//   "per-peer"               — 每个用户独立会话，跨渠道合并
	//   "per-channel-peer"       — 每个渠道每个用户独立（推荐）
	//   "per-account-channel-peer" — 完全隔离（多 bot 场景）
	DMScope string `json:"dmScope"`
	// MainKey 主会话名称，默认 "main"
	MainKey string `json:"mainKey"`
	// IdentityLinks 跨渠道身份映射，同一用户在不同渠道的 ID 可合并
	IdentityLinks map[string][]string `json:"identityLinks"`
}

// AgentConfig configures the AI agent runtime.
type AgentConfig struct {
	Model     string                    `json:"model"`
	Workspace string                    `json:"workspace"`
	Sandbox   SandboxConfig             `json:"sandbox"`
	Models    ModelsConfig              `json:"models"`
	Defaults  AgentDefaults             `json:"defaults"`
	Providers map[string]ProviderConfig `json:"providers"`
}

type ObservabilityConfig struct{}

type IdentityConfig struct{}

type AutonomyConfig struct {
	Level string `json:"level"`
}

type ComposioConfig struct {
	Enabled bool   `json:"enabled"`
	APIKey  string `json:"apiKey"`
}

type SecretsConfig struct {
	Encrypt bool `json:"encrypt"`
}

type MemoryConfig struct {
	Backend                   string  `json:"backend"`
	AutoSave                  bool    `json:"autoSave"`
	HygieneEnabled            bool    `json:"hygieneEnabled"`
	ArchiveAfterDays          int     `json:"archiveAfterDays"`
	PurgeAfterDays            int     `json:"purgeAfterDays"`
	ConversationRetentionDays int     `json:"conversationRetentionDays"`
	EmbeddingProvider         string  `json:"embeddingProvider"`
	EmbeddingModel            string  `json:"embeddingModel"`
	EmbeddingDimensions       int     `json:"embeddingDimensions"`
	VectorWeight              float64 `json:"vectorWeight"`
	KeywordWeight             float64 `json:"keywordWeight"`
	EmbeddingCacheSize        int     `json:"embeddingCacheSize"`
	ChunkMaxTokens            int     `json:"chunkMaxTokens"`
}

// ReliabilityConfig controls provider retry/backoff and fallback chain.
type ReliabilityConfig struct {
	ProviderRetries   uint32   `json:"providerRetries"`
	ProviderBackoffMs uint64   `json:"providerBackoffMs"`
	FallbackProviders []string `json:"fallbackProviders"`
}

// ModelRouteConfig maps a hint route (hint:xxx) to provider+model.
type ModelRouteConfig struct {
	Hint     string `json:"hint"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
}

type TunnelConfig struct {
	Provider   string                  `json:"provider"`
	Cloudflare *CloudflareTunnelConfig `json:"cloudflare,omitempty"`
	Tailscale  *TailscaleTunnelConfig  `json:"tailscale,omitempty"`
	Ngrok      *NgrokTunnelConfig      `json:"ngrok,omitempty"`
	Custom     *CustomTunnelConfig     `json:"custom,omitempty"`
}

type CloudflareTunnelConfig struct {
	Token string `json:"token"`
}

type TailscaleTunnelConfig struct {
	Funnel   bool   `json:"funnel"`
	Hostname string `json:"hostname,omitempty"`
}

type NgrokTunnelConfig struct {
	AuthToken string `json:"authToken"`
	Domain    string `json:"domain,omitempty"`
}

type CustomTunnelConfig struct {
	StartCommand string `json:"startCommand"`
	HealthURL    string `json:"healthUrl,omitempty"`
	URLPattern   string `json:"urlPattern,omitempty"`
}

// AgentDefaults contains default agent settings.
type AgentDefaults struct {
	ThinkingLevel string `json:"thinkingLevel"`
	VerboseLevel  string `json:"verboseLevel"`
}

// SandboxConfig controls Docker sandboxing for non-main sessions.
type SandboxConfig struct {
	Mode  string   `json:"mode"`  // "off", "non-main", "all"
	Allow []string `json:"allow"` // Allowed tool names
	Deny  []string `json:"deny"`  // Denied tool names
}

// ModelsConfig configures allowed models.
type ModelsConfig struct {
	Allowed []string `json:"allowed"`
}

// ProviderConfig represents an AI model provider.
type ProviderConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`
}

// GatewayConfig configures the gateway server.
type GatewayConfig struct {
	Port      int             `json:"port"`
	Bind      string          `json:"bind"` // "loopback" or "all"
	Mode      string          `json:"mode"` // "local" or "remote"
	Auth      GatewayAuth     `json:"auth"`
	Tailscale TailscaleConfig `json:"tailscale"`
}

// GatewayAuth configures gateway authentication.
type GatewayAuth struct {
	Mode           string `json:"mode"` // "token", "password", "none"
	Token          string `json:"token"`
	Password       string `json:"password"`
	AllowTailscale bool   `json:"allowTailscale"`
}

// AuthConfig is an alias for GatewayAuth for compatibility.
type AuthConfig = GatewayAuth

// TailscaleConfig configures Tailscale Serve/Funnel.
type TailscaleConfig struct {
	Mode        string `json:"mode"` // "off", "serve", "funnel"
	ResetOnExit bool   `json:"resetOnExit"`
}

// ChannelsConfig configures messaging channels.
type ChannelsConfig struct {
	CLI         bool               `json:"cli"`
	WhatsApp    *WhatsAppConfig    `json:"whatsapp,omitempty"`
	Telegram    *TelegramConfig    `json:"telegram,omitempty"`
	Discord     *DiscordConfig     `json:"discord,omitempty"`
	Slack       *SlackConfig       `json:"slack,omitempty"`
	Signal      *SignalConfig      `json:"signal,omitempty"`
	BlueBubbles *BlueBubblesConfig `json:"bluebubbles,omitempty"`
	Webhook     *WebhookConfig     `json:"webhook,omitempty"`
	IMessage    *IMessageConfig    `json:"imessage,omitempty"`
	Matrix      *MatrixConfig      `json:"matrix,omitempty"`
	Email       *EmailConfig       `json:"email,omitempty"`
	IRC         *IRCConfig         `json:"irc,omitempty"`
	Lark        *LarkConfig        `json:"lark,omitempty"`
}

// WhatsAppConfig configures the WhatsApp channel.
type WhatsAppConfig struct {
	AllowFrom      []string               `json:"allowFrom"`
	Groups         map[string]GroupConfig `json:"groups"`
	DMPolicy       string                 `json:"dmPolicy"` // "pairing", "open"
	AccessToken    string                 `json:"accessToken"`
	PhoneNumberID  string                 `json:"phoneNumberId"`
	VerifyToken    string                 `json:"verifyToken"`
	AppSecret      string                 `json:"appSecret,omitempty"`
	AllowedNumbers []string               `json:"allowedNumbers"`
}

// TelegramConfig configures the Telegram channel.
type TelegramConfig struct {
	BotToken      string                 `json:"botToken"`
	AllowFrom     []string               `json:"allowFrom"`
	Groups        map[string]GroupConfig `json:"groups"`
	WebhookURL    string                 `json:"webhookUrl"`
	WebhookSecret string                 `json:"webhookSecret"`
	AllowedUsers  []string               `json:"allowedUsers"`
}

// DiscordConfig configures the Discord channel.
type DiscordConfig struct {
	Token        string                 `json:"token"`
	DMPolicy     string                 `json:"dmPolicy"`
	AllowFrom    []string               `json:"allowFrom"`
	Guilds       map[string]GuildConfig `json:"guilds"`
	MediaMaxMB   int                    `json:"mediaMaxMb"`
	GuildID      string                 `json:"guildId"`
	AllowedUsers []string               `json:"allowedUsers"`
	ListenToBots bool                   `json:"listenToBots"`
}

// GuildConfig configures a Discord guild (server).
type GuildConfig struct {
	RequireMention bool `json:"requireMention"`
}

// SlackConfig configures the Slack channel.
type SlackConfig struct {
	BotToken     string   `json:"botToken"`
	AppToken     string   `json:"appToken"`
	AllowFrom    []string `json:"allowFrom"`
	DMPolicy     string   `json:"dmPolicy"`
	ChannelID    string   `json:"channelId"`
	AllowedUsers []string `json:"allowedUsers"`
}

// SignalConfig configures the Signal channel.
type SignalConfig struct {
	Enabled   bool     `json:"enabled"`
	CLIPath   string   `json:"cliPath"`
	AllowFrom []string `json:"allowFrom"`
}

// BlueBubblesConfig configures the BlueBubbles (iMessage) channel.
type BlueBubblesConfig struct {
	ServerURL   string `json:"serverUrl"`
	Password    string `json:"password"`
	WebhookPath string `json:"webhookPath"`
}

type WebhookConfig struct {
	Port   int    `json:"port"`
	Secret string `json:"secret,omitempty"`
}

type IMessageConfig struct {
	AllowedContacts []string `json:"allowedContacts"`
}

type MatrixConfig struct {
	Homeserver   string   `json:"homeserver"`
	AccessToken  string   `json:"accessToken"`
	RoomID       string   `json:"roomId"`
	AllowedUsers []string `json:"allowedUsers"`
}

type EmailConfig struct{}

type IRCConfig struct {
	Server           string   `json:"server"`
	Port             int      `json:"port"`
	Nickname         string   `json:"nickname"`
	Username         string   `json:"username,omitempty"`
	Channels         []string `json:"channels"`
	AllowedUsers     []string `json:"allowedUsers"`
	ServerPassword   string   `json:"serverPassword,omitempty"`
	NickservPassword string   `json:"nickservPassword,omitempty"`
	SASLPassword     string   `json:"saslPassword,omitempty"`
	VerifyTLS        bool     `json:"verifyTls"`
}

type LarkConfig struct{}

// GroupConfig configures group behavior for a channel.
type GroupConfig struct {
	RequireMention bool `json:"requireMention"`
}

// BrowserConfig configures the browser control tool.
type BrowserConfig struct {
	Enabled bool   `json:"enabled"`
	Color   string `json:"color"`
}

// HooksConfig configures automation hooks.
type HooksConfig struct {
	Gmail    GmailHookConfig    `json:"gmail"`
	Internal InternalHookConfig `json:"internal"`
}

// WebConfig configures the embedded web console.
type WebConfig struct {
	Auth        WebAuthConfig        `json:"auth"`
	Preferences WebPreferencesConfig `json:"preferences"`
}

// WebAuthConfig controls web login.
type WebAuthConfig struct {
	Enabled           bool   `json:"enabled"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	SessionTTLMinutes int    `json:"sessionTtlMinutes"`
}

// WebPreferencesConfig stores web UI preferences on server side.
type WebPreferencesConfig struct {
	ShowTerminalInSidebar bool `json:"showTerminalInSidebar"`
	AutoStart             bool `json:"autoStart"`
}

// GmailHookConfig configures Gmail Pub/Sub integration.
type GmailHookConfig struct {
	Account string `json:"account"`
	Model   string `json:"model"`
}

// InternalHookConfig configures internal event hooks.
type InternalHookConfig struct {
	Enabled bool `json:"enabled"`
}

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Agent: AgentConfig{
			Model:     "anthropic/claude-opus-4-6",
			Workspace: defaultWorkspaceDir(),
		},
		Gateway: GatewayConfig{
			Port: 18790,
			Bind: "loopback",
			Auth: GatewayAuth{
				Mode: "token",
			},
		},
		Channels: ChannelsConfig{
			CLI: true,
		},
		Autonomy: AutonomyConfig{
			Level: "supervised",
		},
		Memory: MemoryConfig{
			Backend:                   "sqlite",
			AutoSave:                  true,
			HygieneEnabled:            true,
			ArchiveAfterDays:          7,
			PurgeAfterDays:            30,
			ConversationRetentionDays: 30,
			EmbeddingProvider:         "none",
			EmbeddingModel:            "text-embedding-3-small",
			EmbeddingDimensions:       1536,
			VectorWeight:              0.7,
			KeywordWeight:             0.3,
			EmbeddingCacheSize:        10000,
			ChunkMaxTokens:            512,
		},
		Session: SessionConfig{
			Scope:   "per-sender",
			DMScope: "per-channel-peer",
			MainKey: "main",
		},
		Reliability: ReliabilityConfig{
			ProviderRetries:   2,
			ProviderBackoffMs: 500,
			FallbackProviders: []string{},
		},
		ModelRoutes: []ModelRouteConfig{},
		Tunnel: TunnelConfig{
			Provider: "none",
		},
		Composio: ComposioConfig{
			Enabled: false,
		},
		Secrets: SecretsConfig{
			Encrypt: true,
		},
		Web: WebConfig{
			Auth: WebAuthConfig{
				Enabled:           true,
				Username:          "admin",
				Password:          "admin",
				SessionTTLMinutes: 1440,
			},
			Preferences: WebPreferencesConfig{
				ShowTerminalInSidebar: false,
				AutoStart:             false,
			},
		},
	}
}

// ConfigDir returns the HighClaw config directory (~/.highclaw).
// Falls back to ~/.openclaw if ~/.highclaw doesn't exist (migration support).
func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".highclaw"
	}
	highclawDir := filepath.Join(home, ".highclaw")
	if _, err := os.Stat(highclawDir); err == nil {
		return highclawDir
	}
	// Fallback to legacy openclaw dir for migration.
	openclawDir := filepath.Join(home, ".openclaw")
	if _, err := os.Stat(openclawDir); err == nil {
		return openclawDir
	}
	return highclawDir
}

// ConfigPath returns the path to the main config file.
func ConfigPath() string {
	// Env override has highest priority.
	if envPath := os.Getenv("HIGHCLAW_CONFIG"); envPath != "" {
		return envPath
	}
	if envPath := os.Getenv("OPENCLAW_CONFIG"); envPath != "" {
		return envPath
	}
	return filepath.Join(ConfigDir(), "config.yaml")
}

func defaultWorkspaceDir() string {
	return filepath.Join(ConfigDir(), "workspace")
}

// Load reads and parses the config from disk.
// If the config file doesn't exist, it returns defaults.
func Load() (*Config, error) {
	cfg := Default()

	configPath := ConfigPath()
	explicitPath := os.Getenv("HIGHCLAW_CONFIG") != "" || os.Getenv("OPENCLAW_CONFIG") != ""
	loadedFromLegacyJSON := false
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if explicitPath {
				return cfg, nil
			}
			// Legacy fallback: support loading historical JSON config files.
			for _, legacyPath := range []string{
				filepath.Join(ConfigDir(), "highclaw.json"),
				filepath.Join(ConfigDir(), "openclaw.json"),
			} {
				legacyData, legacyErr := os.ReadFile(legacyPath)
				if legacyErr != nil {
					continue
				}
				data = legacyData
				configPath = legacyPath
				loadedFromLegacyJSON = true
				err = nil
				break
			}
			if err != nil {
				applyEnvOverrides(cfg)
				return cfg, nil
			}
		}
		if err != nil {
			return cfg, fmt.Errorf("read config: %w", err)
		}
	}

	if err := parseConfigData(data, configPath, cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", configPath, err)
	}

	// Apply environment variable overrides.
	applyEnvOverrides(cfg)

	// Auto-migrate old JSON config into YAML when using default pathing.
	if loadedFromLegacyJSON && !explicitPath {
		_ = Save(cfg)
	}

	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	path := ConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Marshal to YAML while preserving json-tag key names.
	data, err := marshalConfigYAML(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// DefaultConfig is an alias for Default for compatibility.
func DefaultConfig() *Config {
	return Default()
}

// applyEnvOverrides merges environment variables into configuration.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		if cfg.Channels.Telegram == nil {
			cfg.Channels.Telegram = &TelegramConfig{}
		}
		cfg.Channels.Telegram.BotToken = v
	}
	if v := os.Getenv("DISCORD_BOT_TOKEN"); v != "" {
		if cfg.Channels.Discord == nil {
			cfg.Channels.Discord = &DiscordConfig{}
		}
		cfg.Channels.Discord.Token = v
	}
	if v := os.Getenv("SLACK_BOT_TOKEN"); v != "" {
		if cfg.Channels.Slack == nil {
			cfg.Channels.Slack = &SlackConfig{}
		}
		cfg.Channels.Slack.BotToken = v
	}
	if v := os.Getenv("SLACK_APP_TOKEN"); v != "" {
		if cfg.Channels.Slack == nil {
			cfg.Channels.Slack = &SlackConfig{}
		}
		cfg.Channels.Slack.AppToken = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		if cfg.Agent.Providers == nil {
			cfg.Agent.Providers = make(map[string]ProviderConfig)
		}
		p := cfg.Agent.Providers["anthropic"]
		p.APIKey = v
		cfg.Agent.Providers["anthropic"] = p
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		if cfg.Agent.Providers == nil {
			cfg.Agent.Providers = make(map[string]ProviderConfig)
		}
		p := cfg.Agent.Providers["openai"]
		p.APIKey = v
		cfg.Agent.Providers["openai"] = p
	}
	if v := os.Getenv("HIGHCLAW_WEB_USERNAME"); v != "" {
		cfg.Web.Auth.Username = v
	}
	if v := os.Getenv("HIGHCLAW_WEB_PASSWORD"); v != "" {
		cfg.Web.Auth.Password = v
	}
}

func preprocessJSONLike(input string) string {
	s := input
	for {
		start := strings.Index(s, "/*")
		if start < 0 {
			break
		}
		end := strings.Index(s[start+2:], "*/")
		if end < 0 {
			s = s[:start]
			break
		}
		end += start + 2
		s = s[:start] + s[end+2:]
	}

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		inString := false
		escape := false
		for j := 0; j < len(line)-1; j++ {
			ch := line[j]
			if ch == '\\' && inString {
				escape = !escape
				continue
			}
			if ch == '"' && !escape {
				inString = !inString
			}
			escape = false
			if !inString && ch == '/' && line[j+1] == '/' {
				line = line[:j]
				break
			}
		}
		lines[i] = strings.TrimRight(line, " \t")
	}
	s = strings.Join(lines, "\n")
	s = strings.ReplaceAll(s, ",}", "}")
	s = strings.ReplaceAll(s, ",]", "]")
	return s
}

func parseConfigData(data []byte, path string, cfg *Config) error {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return unmarshalYAMLToConfig(data, cfg)
	}

	// Legacy JSON/JSON5 path handling.
	clean := preprocessJSONLike(string(data))
	if err := json.Unmarshal([]byte(clean), cfg); err == nil {
		return nil
	}

	// Last chance: try YAML parser even when extension is unknown.
	return unmarshalYAMLToConfig(data, cfg)
}

func unmarshalYAMLToConfig(data []byte, cfg *Config) error {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	normalized := normalizeYAMLValue(raw)
	jb, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return json.Unmarshal(jb, cfg)
}

func normalizeYAMLValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = normalizeYAMLValue(val)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalizeYAMLValue(val)
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeYAMLValue(val)
		}
		return out
	default:
		return t
	}
}

func marshalConfigYAML(cfg *Config) ([]byte, error) {
	jb, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(jb, &obj); err != nil {
		return nil, err
	}
	return yaml.Marshal(obj)
}
