// Package telegram implements the Telegram channel.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config represents Telegram channel configuration.
type Config struct {
	BotToken  string
	AllowFrom []string
}

// TelegramChannel implements the Telegram channel.
type TelegramChannel struct {
	config    Config
	logger    *slog.Logger
	connected bool
	mu        sync.RWMutex
	messages  chan *channel.Message
	stopCh    chan struct{}
}

// NewTelegramChannel creates a new Telegram channel.
func NewTelegramChannel(config Config, logger *slog.Logger) *TelegramChannel {
	return &TelegramChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID returns the channel ID.
func (t *TelegramChannel) ID() string {
	return "telegram"
}

// Name returns the channel name.
func (t *TelegramChannel) Name() string {
	return "Telegram"
}

// Type returns the channel type.
func (t *TelegramChannel) Type() string {
	return "bot"
}

// Start starts the Telegram channel.
func (t *TelegramChannel) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("telegram channel already started")
	}

	t.logger.Info("starting telegram channel", "botToken", maskToken(t.config.BotToken))

	// TODO: Initialize Telegram Bot API client
	// For now, just mark as connected
	t.connected = true

	// Start message polling in background
	go t.pollMessages(ctx)

	return nil
}

// Stop stops the Telegram channel.
func (t *TelegramChannel) Stop(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	t.logger.Info("stopping telegram channel")

	close(t.stopCh)
	t.connected = false

	return nil
}

// IsConnected returns whether the channel is connected.
func (t *TelegramChannel) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// SendMessage sends a message to Telegram.
func (t *TelegramChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		return fmt.Errorf("telegram channel not connected")
	}

	t.logger.Info("sending telegram message", "to", msg.To, "text", msg.Text)

	// TODO: Send message via Telegram Bot API
	// For now, just log

	return nil
}

// ReceiveMessages returns a channel for receiving messages.
func (t *TelegramChannel) ReceiveMessages() <-chan *channel.Message {
	return t.messages
}

// pollMessages polls for new messages from Telegram.
func (t *TelegramChannel) pollMessages(ctx context.Context) {
	t.logger.Info("telegram message polling started")

	for {
		select {
		case <-ctx.Done():
			t.logger.Info("telegram polling stopped (context done)")
			return
		case <-t.stopCh:
			t.logger.Info("telegram polling stopped (stop signal)")
			return
		default:
			// TODO: Poll Telegram Bot API for updates
			// For now, just sleep
			// time.Sleep(1 * time.Second)
		}
	}
}

// maskToken masks a token for logging.
func maskToken(token string) string {
	if len(token) < 10 {
		return "***"
	}
	return token[:5] + "..." + token[len(token)-5:]
}

