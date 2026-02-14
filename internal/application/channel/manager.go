// Package channel provides channel management services.
package channel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/channel"
	"github.com/highclaw/highclaw/internal/infrastructure/channels/discord"
	"github.com/highclaw/highclaw/internal/infrastructure/channels/slack"
	"github.com/highclaw/highclaw/internal/infrastructure/channels/telegram"
	"github.com/highclaw/highclaw/internal/infrastructure/channels/whatsapp"
)

// Manager manages all messaging channels.
type Manager struct {
	config   *config.Config
	logger   *slog.Logger
	registry *channel.Registry
	mu       sync.RWMutex
}

// NewManager creates a new channel manager.
func NewManager(cfg *config.Config, logger *slog.Logger) *Manager {
	return &Manager{
		config:   cfg,
		logger:   logger,
		registry: channel.NewRegistry(),
	}
}

// Initialize initializes all configured channels.
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("initializing channels")

	// Initialize Telegram if configured
	if m.config.Channels.Telegram.BotToken != "" {
		m.logger.Info("initializing telegram channel")
		tg := telegram.NewTelegramChannel(telegram.Config{
			BotToken:  m.config.Channels.Telegram.BotToken,
			AllowFrom: m.config.Channels.Telegram.AllowFrom,
		}, m.logger)
		m.registry.Register(tg)
	}

	// Initialize WhatsApp if configured
	// WhatsApp uses QR login, so we initialize it even without config
	m.logger.Info("initializing whatsapp channel")
	wa := whatsapp.NewWhatsAppChannel(whatsapp.Config{
		SessionPath: m.config.Agent.Workspace + "/whatsapp",
		AllowFrom:   []string{},
	}, m.logger)
	m.registry.Register(wa)

	// Initialize Discord if configured
	if m.config.Channels.Discord.Token != "" {
		m.logger.Info("initializing discord channel")
		dc := discord.NewDiscordChannel(discord.Config{
			Token:     m.config.Channels.Discord.Token,
			AllowFrom: []string{},
		}, m.logger)
		m.registry.Register(dc)
	}

	// Initialize Slack if configured
	if m.config.Channels.Slack.BotToken != "" {
		m.logger.Info("initializing slack channel")
		sl := slack.NewSlackChannel(slack.Config{
			BotToken:  m.config.Channels.Slack.BotToken,
			AppToken:  m.config.Channels.Slack.AppToken,
			AllowFrom: []string{},
		}, m.logger)
		m.registry.Register(sl)
	}

	m.logger.Info("channels initialized", "count", len(m.registry.All()))

	return nil
}

// StartAll starts all registered channels.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("starting all channels")

	for _, ch := range m.registry.All() {
		if err := ch.Start(ctx); err != nil {
			m.logger.Error("failed to start channel", "channel", ch.ID(), "error", err)
			continue
		}
		m.logger.Info("channel started", "channel", ch.ID())
	}

	return nil
}

// StopAll stops all registered channels.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.logger.Info("stopping all channels")

	for _, ch := range m.registry.All() {
		if err := ch.Stop(ctx); err != nil {
			m.logger.Error("failed to stop channel", "channel", ch.ID(), "error", err)
			continue
		}
		m.logger.Info("channel stopped", "channel", ch.ID())
	}

	return nil
}

// GetChannel returns a channel by ID.
func (m *Manager) GetChannel(id string) (channel.Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, ok := m.registry.Get(id)
	if !ok {
		return nil, fmt.Errorf("channel not found: %s", id)
	}

	return ch, nil
}

// ListChannels returns all registered channels.
func (m *Manager) ListChannels() []channel.Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.All()
}

// GetStatus returns the status of all channels.
func (m *Manager) GetStatus() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]bool)
	for _, ch := range m.registry.All() {
		status[ch.ID()] = ch.IsConnected()
	}

	return status
}

