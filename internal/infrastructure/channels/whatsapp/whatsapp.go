// Package whatsapp implements the WhatsApp channel using whatsmeow.
package whatsapp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config represents WhatsApp channel configuration.
type Config struct {
	SessionPath string
	AllowFrom   []string
}

// WhatsAppChannel implements the WhatsApp channel.
type WhatsAppChannel struct {
	config    Config
	logger    *slog.Logger
	connected bool
	mu        sync.RWMutex
	messages  chan *channel.Message
	stopCh    chan struct{}
}

// NewWhatsAppChannel creates a new WhatsApp channel.
func NewWhatsAppChannel(config Config, logger *slog.Logger) *WhatsAppChannel {
	return &WhatsAppChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID returns the channel ID.
func (w *WhatsAppChannel) ID() string {
	return "whatsapp"
}

// Name returns the channel name.
func (w *WhatsAppChannel) Name() string {
	return "WhatsApp"
}

// Type returns the channel type.
func (w *WhatsAppChannel) Type() string {
	return "web"
}

// Start starts the WhatsApp channel.
func (w *WhatsAppChannel) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.connected {
		return fmt.Errorf("whatsapp channel already started")
	}

	w.logger.Info("starting whatsapp channel", "sessionPath", w.config.SessionPath)

	// TODO: Initialize whatsmeow client
	// TODO: Handle QR code login if not authenticated
	// For now, just mark as connected
	w.connected = true

	// Start message handler in background
	go w.handleMessages(ctx)

	return nil
}

// Stop stops the WhatsApp channel.
func (w *WhatsAppChannel) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.connected {
		return nil
	}

	w.logger.Info("stopping whatsapp channel")

	close(w.stopCh)
	w.connected = false

	return nil
}

// IsConnected returns whether the channel is connected.
func (w *WhatsAppChannel) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.connected
}

// SendMessage sends a message to WhatsApp.
func (w *WhatsAppChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !w.connected {
		return fmt.Errorf("whatsapp channel not connected")
	}

	w.logger.Info("sending whatsapp message", "to", msg.To, "text", msg.Text)

	// TODO: Send message via whatsmeow
	// For now, just log

	return nil
}

// ReceiveMessages returns a channel for receiving messages.
func (w *WhatsAppChannel) ReceiveMessages() <-chan *channel.Message {
	return w.messages
}

// handleMessages handles incoming WhatsApp messages.
func (w *WhatsAppChannel) handleMessages(ctx context.Context) {
	w.logger.Info("whatsapp message handler started")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("whatsapp handler stopped (context done)")
			return
		case <-w.stopCh:
			w.logger.Info("whatsapp handler stopped (stop signal)")
			return
		default:
			// TODO: Handle incoming messages from whatsmeow
			// For now, just wait
		}
	}
}

// GetQRCode returns the QR code for authentication.
// This should be called before Start() if not authenticated.
func (w *WhatsAppChannel) GetQRCode() (string, error) {
	// TODO: Generate QR code using whatsmeow
	return "", fmt.Errorf("not implemented")
}

