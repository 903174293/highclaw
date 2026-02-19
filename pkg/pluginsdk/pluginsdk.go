// Package pluginsdk defines the public API for OpenClaw plugins.
// Channel extensions implement this interface to integrate with the gateway.
package pluginsdk

import "context"

// Channel is the interface that all messaging channel plugins must implement.
type Channel interface {
	// Name returns the channel identifier (e.g., "whatsapp", "telegram").
	Name() string

	// Start initializes the channel and begins listening for messages.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the channel.
	Stop() error

	// Send sends a message through the channel.
	Send(ctx context.Context, msg OutgoingMessage) error

	// IsConnected returns whether the channel is currently connected.
	IsConnected() bool

	// StartTyping signals that the bot is processing a response.
	// Platforms show a "typing..." indicator to the user.
	StartTyping(ctx context.Context, recipient string) error

	// StopTyping cancels any active typing indicator.
	StopTyping(ctx context.Context, recipient string) error
}

// IncomingMessage represents a message received from a channel.
type IncomingMessage struct {
	ChannelName string  `json:"channelName"`
	SenderID    string  `json:"senderId"`
	SenderName  string  `json:"senderName"`
	GroupID     string  `json:"groupId,omitempty"`
	GroupName   string  `json:"groupName,omitempty"`
	Text        string  `json:"text"`
	Images      []Media `json:"images,omitempty"`
	Audio       *Media  `json:"audio,omitempty"`
	ReplyTo     string  `json:"replyTo,omitempty"`
	Timestamp   int64   `json:"timestamp"`
}

// OutgoingMessage represents a message to send through a channel.
type OutgoingMessage struct {
	RecipientID string  `json:"recipientId"`
	GroupID     string  `json:"groupId,omitempty"`
	Text        string  `json:"text"`
	Images      []Media `json:"images,omitempty"`
	Audio       *Media  `json:"audio,omitempty"`
	ReplyToID   string  `json:"replyToId,omitempty"`
}

// Media represents a media attachment.
type Media struct {
	URL      string `json:"url,omitempty"`
	Data     []byte `json:"data,omitempty"`
	MimeType string `json:"mimeType"`
	Filename string `json:"filename,omitempty"`
}

// MessageHandler is the callback for incoming messages.
type MessageHandler func(msg IncomingMessage)

// ChannelConfig provides configuration for a channel plugin.
type ChannelConfig struct {
	// ConfigJSON contains the channel-specific config section as raw JSON.
	ConfigJSON []byte

	// WorkspaceDir is the path to the workspace directory.
	WorkspaceDir string

	// OnMessage is called when the channel receives a message.
	OnMessage MessageHandler
}
