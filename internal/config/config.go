// Package config handles loading and validating the HighClaw configuration.
// Config is stored at ~/.highclaw/highclaw.json (JSON5 format), with fallback to ~/.openclaw/openclaw.json for migration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the top-level OpenClaw configuration.
type Config struct {
	Agent    AgentConfig    `json:"agent"`
	Gateway  GatewayConfig  `json:"gateway"`
	Channels ChannelsConfig `json:"channels"`
	Browser  BrowserConfig  `json:"browser"`
	Hooks    HooksConfig    `json:"hooks"`
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
	WhatsApp    WhatsAppConfig    `json:"whatsapp"`
	Telegram    TelegramConfig    `json:"telegram"`
	Discord     DiscordConfig     `json:"discord"`
	Slack       SlackConfig       `json:"slack"`
	Signal      SignalConfig      `json:"signal"`
	BlueBubbles BlueBubblesConfig `json:"bluebubbles"`
}

// WhatsAppConfig configures the WhatsApp channel.
type WhatsAppConfig struct {
	AllowFrom []string               `json:"allowFrom"`
	Groups    map[string]GroupConfig `json:"groups"`
	DMPolicy  string                 `json:"dmPolicy"` // "pairing", "open"
}

// TelegramConfig configures the Telegram channel.
type TelegramConfig struct {
	BotToken      string                 `json:"botToken"`
	AllowFrom     []string               `json:"allowFrom"`
	Groups        map[string]GroupConfig `json:"groups"`
	WebhookURL    string                 `json:"webhookUrl"`
	WebhookSecret string                 `json:"webhookSecret"`
}

// DiscordConfig configures the Discord channel.
type DiscordConfig struct {
	Token      string                 `json:"token"`
	DMPolicy   string                 `json:"dmPolicy"`
	AllowFrom  []string               `json:"allowFrom"`
	Guilds     map[string]GuildConfig `json:"guilds"`
	MediaMaxMB int                    `json:"mediaMaxMb"`
}

// GuildConfig configures a Discord guild (server).
type GuildConfig struct {
	RequireMention bool `json:"requireMention"`
}

// SlackConfig configures the Slack channel.
type SlackConfig struct {
	BotToken  string   `json:"botToken"`
	AppToken  string   `json:"appToken"`
	AllowFrom []string `json:"allowFrom"`
	DMPolicy  string   `json:"dmPolicy"`
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
	dir := ConfigDir()
	// Try highclaw.json first, fall back to openclaw.json.
	highclawPath := filepath.Join(dir, "highclaw.json")
	if _, err := os.Stat(highclawPath); err == nil {
		return highclawPath
	}
	openclawPath := filepath.Join(dir, "openclaw.json")
	if _, err := os.Stat(openclawPath); err == nil {
		return openclawPath
	}
	return highclawPath
}

func defaultWorkspaceDir() string {
	return filepath.Join(ConfigDir(), "workspace")
}

// Load reads and parses the config from disk.
// If the config file doesn't exist, it returns defaults.
func Load() (*Config, error) {
	cfg := Default()

	configPath := ConfigPath()

	// Check for env override.
	// Support both HIGHCLAW_CONFIG and legacy OPENCLAW_CONFIG.
	if envPath := os.Getenv("HIGHCLAW_CONFIG"); envPath != "" {
		configPath = envPath
	} else if envPath := os.Getenv("OPENCLAW_CONFIG"); envPath != "" {
		configPath = envPath
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	clean := preprocessJSONLike(string(data))
	if err := json.Unmarshal([]byte(clean), cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", configPath, err)
	}

	// Apply environment variable overrides.
	applyEnvOverrides(cfg)

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

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(cfg, "", "  ")
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
		cfg.Channels.Telegram.BotToken = v
	}
	if v := os.Getenv("DISCORD_BOT_TOKEN"); v != "" {
		cfg.Channels.Discord.Token = v
	}
	if v := os.Getenv("SLACK_BOT_TOKEN"); v != "" {
		cfg.Channels.Slack.BotToken = v
	}
	if v := os.Getenv("SLACK_APP_TOKEN"); v != "" {
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
