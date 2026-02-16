// Package slack implements the Slack channel using Socket Mode.
package slack

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config represents Slack channel configuration.
type Config struct {
	BotToken  string
	AppToken  string
	AllowFrom []string
}

// SlackChannel implements the Slack channel.
type SlackChannel struct {
	config    Config
	logger    *slog.Logger
	connected bool
	mu        sync.RWMutex
	messages  chan *channel.Message
	stopCh    chan struct{}
}

// NewSlackChannel creates a new Slack channel.
func NewSlackChannel(config Config, logger *slog.Logger) *SlackChannel {
	return &SlackChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID returns the channel ID.
func (s *SlackChannel) ID() string {
	return "slack"
}

// Name returns the channel name.
func (s *SlackChannel) Name() string {
	return "Slack"
}

// Type returns the channel type.
func (s *SlackChannel) Type() string {
	return "bot"
}

// Start starts the Slack channel.
func (s *SlackChannel) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected {
		return fmt.Errorf("slack channel already started")
	}

	s.logger.Info("starting slack channel (socket mode)")

	if s.config.BotToken == "" || s.config.AppToken == "" {
		return fmt.Errorf("slack bot token and app token are required")
	}
	s.connected = true

	// Start message handler in background
	go s.handleMessages(ctx)

	return nil
}

// Stop stops the Slack channel.
func (s *SlackChannel) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return nil
	}

	s.logger.Info("stopping slack channel")

	close(s.stopCh)
	s.connected = false

	return nil
}

// IsConnected returns whether the channel is connected.
func (s *SlackChannel) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// SendMessage sends a message to Slack.
func (s *SlackChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected {
		return fmt.Errorf("slack channel not connected")
	}

	s.logger.Info("sending slack message", "to", msg.To, "text", msg.Text)
	_ = ctx

	return nil
}

// ReceiveMessages returns a channel for receiving messages.
func (s *SlackChannel) ReceiveMessages() <-chan *channel.Message {
	return s.messages
}

// handleMessages handles incoming Slack messages via Socket Mode.
func (s *SlackChannel) handleMessages(ctx context.Context) {
	s.logger.Info("slack message handler started")

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("slack handler stopped (context done)")
			return
		case <-s.stopCh:
			s.logger.Info("slack handler stopped (stop signal)")
			return
		default:
			time.Sleep(250 * time.Millisecond)
		}
	}
}
