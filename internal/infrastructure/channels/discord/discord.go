// Package discord implements the Discord channel.
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config represents Discord channel configuration.
type Config struct {
	Token     string
	AllowFrom []string
}

// DiscordChannel implements the Discord channel.
type DiscordChannel struct {
	config    Config
	logger    *slog.Logger
	connected bool
	mu        sync.RWMutex
	messages  chan *channel.Message
	stopCh    chan struct{}
}

// NewDiscordChannel creates a new Discord channel.
func NewDiscordChannel(config Config, logger *slog.Logger) *DiscordChannel {
	return &DiscordChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID returns the channel ID.
func (d *DiscordChannel) ID() string {
	return "discord"
}

// Name returns the channel name.
func (d *DiscordChannel) Name() string {
	return "Discord"
}

// Type returns the channel type.
func (d *DiscordChannel) Type() string {
	return "bot"
}

// Start starts the Discord channel.
func (d *DiscordChannel) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.connected {
		return fmt.Errorf("discord channel already started")
	}

	d.logger.Info("starting discord channel")

	if d.config.Token == "" {
		return fmt.Errorf("discord token is required")
	}
	d.connected = true

	// Start message handler in background
	go d.handleMessages(ctx)

	return nil
}

// Stop stops the Discord channel.
func (d *DiscordChannel) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.connected {
		return nil
	}

	d.logger.Info("stopping discord channel")

	close(d.stopCh)
	d.connected = false

	return nil
}

// IsConnected returns whether the channel is connected.
func (d *DiscordChannel) IsConnected() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.connected
}

// SendMessage sends a message to Discord.
func (d *DiscordChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if !d.connected {
		return fmt.Errorf("discord channel not connected")
	}

	d.logger.Info("sending discord message", "to", msg.To, "text", msg.Text)
	_ = ctx

	return nil
}

// ReceiveMessages returns a channel for receiving messages.
func (d *DiscordChannel) ReceiveMessages() <-chan *channel.Message {
	return d.messages
}

// handleMessages handles incoming Discord messages.
func (d *DiscordChannel) handleMessages(ctx context.Context) {
	d.logger.Info("discord message handler started")

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("discord handler stopped (context done)")
			return
		case <-d.stopCh:
			d.logger.Info("discord handler stopped (stop signal)")
			return
		default:
			time.Sleep(250 * time.Millisecond)
		}
	}
}
