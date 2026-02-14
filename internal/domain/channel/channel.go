// Package channel defines the channel domain.
package channel

import "context"

// Channel represents a messaging channel.
type Channel interface {
	// ID returns the channel ID (e.g., "telegram", "whatsapp").
	ID() string

	// Name returns the human-readable channel name.
	Name() string

	// Type returns the channel type (e.g., "bot", "web", "cli").
	Type() string

	// Start starts the channel.
	Start(ctx context.Context) error

	// Stop stops the channel.
	Stop(ctx context.Context) error

	// IsConnected returns whether the channel is connected.
	IsConnected() bool

	// SendMessage sends a message to the channel.
	SendMessage(ctx context.Context, msg *Message) error

	// ReceiveMessages returns a channel for receiving messages.
	ReceiveMessages() <-chan *Message
}

// Message represents a message from/to a channel.
type Message struct {
	ID        string
	ChannelID string
	From      string
	To        string
	Text      string
	Images    []Image
	Files     []File
	Timestamp int64
	Metadata  map[string]any
}

// Image represents an image attachment.
type Image struct {
	URL      string
	Data     []byte
	MimeType string
	Width    int
	Height   int
}

// File represents a file attachment.
type File struct {
	Name     string
	URL      string
	Data     []byte
	MimeType string
	Size     int64
}

// ChannelInfo contains metadata about a channel.
type ChannelInfo struct {
	ID          string
	Name        string
	Type        string // "bot", "web", "cli", "plugin"
	Description string
	AuthType    string // "token", "oauth", "qr", "none"
	ConfigKeys  []string
	DocsURL     string
	Enabled     bool
}

// AllChannels returns all supported channel IDs.
var AllChannels = []string{
	"telegram",
	"whatsapp",
	"discord",
	"google-chat",
	"slack",
	"signal",
	"imessage",
	"nostr",
	"msteams",
	"mattermost",
	"nextcloud",
	"feishu",
	"matrix",
	"bluebubbles",
	"line",
	"zalo",
	"zalo-user",
	"tlon",
	"web",
}

// GetChannelInfo returns metadata for all channels.
func GetChannelInfo() map[string]ChannelInfo {
	return map[string]ChannelInfo{
		"telegram": {
			ID:          "telegram",
			Name:        "Telegram",
			Type:        "bot",
			Description: "Telegram Bot API",
			AuthType:    "token",
			ConfigKeys:  []string{"botToken", "allowFrom"},
			DocsURL:     "https://docs.openclaw.ai/channels/telegram",
		},
		"whatsapp": {
			ID:          "whatsapp",
			Name:        "WhatsApp",
			Type:        "web",
			Description: "WhatsApp via whatsmeow",
			AuthType:    "qr",
			ConfigKeys:  []string{},
			DocsURL:     "https://docs.openclaw.ai/channels/whatsapp",
		},
		"discord": {
			ID:          "discord",
			Name:        "Discord",
			Type:        "bot",
			Description: "Discord Bot API",
			AuthType:    "token",
			ConfigKeys:  []string{"token", "allowFrom"},
			DocsURL:     "https://docs.openclaw.ai/channels/discord",
		},
		"slack": {
			ID:          "slack",
			Name:        "Slack",
			Type:        "bot",
			Description: "Slack Socket Mode",
			AuthType:    "token",
			ConfigKeys:  []string{"botToken", "appToken"},
			DocsURL:     "https://docs.openclaw.ai/channels/slack",
		},
		"signal": {
			ID:          "signal",
			Name:        "Signal",
			Type:        "cli",
			Description: "Signal via signal-cli",
			AuthType:    "qr",
			ConfigKeys:  []string{"phoneNumber"},
			DocsURL:     "https://docs.openclaw.ai/channels/signal",
		},
		"imessage": {
			ID:          "imessage",
			Name:        "iMessage",
			Type:        "cli",
			Description: "iMessage via imsg",
			AuthType:    "none",
			ConfigKeys:  []string{},
			DocsURL:     "https://docs.openclaw.ai/channels/imessage",
		},
	}
}

// Registry manages all channels.
type Registry struct {
	channels map[string]Channel
}

// NewRegistry creates a new channel registry.
func NewRegistry() *Registry {
	return &Registry{
		channels: make(map[string]Channel),
	}
}

// Register registers a channel.
func (r *Registry) Register(channel Channel) {
	r.channels[channel.ID()] = channel
}

// Get returns a channel by ID.
func (r *Registry) Get(id string) (Channel, bool) {
	ch, ok := r.channels[id]
	return ch, ok
}

// All returns all registered channels.
func (r *Registry) All() []Channel {
	channels := make([]Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		channels = append(channels, ch)
	}
	return channels
}

