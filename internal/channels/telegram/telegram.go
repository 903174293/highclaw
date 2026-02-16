// Package telegram implements the Telegram Bot channel.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/pkg/pluginsdk"
)

// Channel implements the Telegram messaging channel.
type Channel struct {
	cfg       *config.TelegramConfig
	logger    *slog.Logger
	bot       *tgbotapi.BotAPI
	onMessage pluginsdk.MessageHandler
	stopCh    chan struct{}
	connected bool
}

// NewChannel creates a new Telegram channel.
func NewChannel(cfg *config.TelegramConfig, logger *slog.Logger, onMessage pluginsdk.MessageHandler) *Channel {
	return &Channel{
		cfg:       cfg,
		logger:    logger.With("channel", "telegram"),
		onMessage: onMessage,
		stopCh:    make(chan struct{}),
	}
}

// Name returns the channel identifier.
func (c *Channel) Name() string {
	return "telegram"
}

// Start initializes the Telegram bot and begins listening for messages.
func (c *Channel) Start(ctx context.Context) error {
	if c.cfg.BotToken == "" {
		return fmt.Errorf("Telegram bot token not configured")
	}

	bot, err := tgbotapi.NewBotAPI(c.cfg.BotToken)
	if err != nil {
		return fmt.Errorf("create bot: %w", err)
	}

	c.bot = bot
	c.connected = true

	c.logger.Info("telegram bot connected", "username", bot.Self.UserName)

	// Start message polling in background
	go c.pollMessages(ctx)

	return nil
}

// Stop gracefully shuts down the Telegram channel.
func (c *Channel) Stop() error {
	close(c.stopCh)
	c.connected = false
	if c.bot != nil {
		c.bot.StopReceivingUpdates()
	}
	return nil
}

// Send sends a message through Telegram.
func (c *Channel) Send(ctx context.Context, msg pluginsdk.OutgoingMessage) error {
	if c.bot == nil {
		return fmt.Errorf("bot not initialized")
	}

	chatID := parseChatID(msg.RecipientID)
	if msg.GroupID != "" {
		chatID = parseChatID(msg.GroupID)
	}

	tgMsg := tgbotapi.NewMessage(chatID, msg.Text)

	if msg.ReplyToID != "" {
		if replyID, ok := parseReplyMessageID(msg.ReplyToID); ok {
			tgMsg.ReplyToMessageID = replyID
		}
	}

	_, err := c.bot.Send(tgMsg)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

// IsConnected returns whether the channel is currently connected.
func (c *Channel) IsConnected() bool {
	return c.connected
}

// pollMessages polls for new messages from Telegram.
func (c *Channel) pollMessages(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := c.bot.GetUpdatesChan(u)

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			c.handleMessage(update.Message)
		}
	}
}

// handleMessage processes an incoming Telegram message.
func (c *Channel) handleMessage(msg *tgbotapi.Message) {
	// Check allowlist if configured
	if len(c.cfg.AllowFrom) > 0 {
		allowed := false
		username := msg.From.UserName
		for _, allowedUser := range c.cfg.AllowFrom {
			if allowedUser == username || allowedUser == fmt.Sprintf("@%s", username) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.logger.Debug("message from non-allowed user", "username", username)
			return
		}
	}

	// Build incoming message
	inMsg := pluginsdk.IncomingMessage{
		ChannelName: "telegram",
		SenderID:    fmt.Sprintf("%d", msg.From.ID),
		SenderName:  msg.From.FirstName,
		Text:        msg.Text,
		Timestamp:   time.Now().UnixMilli(),
	}

	// Handle group messages
	if msg.Chat.IsGroup() || msg.Chat.IsSuperGroup() {
		inMsg.GroupID = fmt.Sprintf("%d", msg.Chat.ID)
		inMsg.GroupName = msg.Chat.Title
	}

	c.onMessage(inMsg)
}

// parseChatID converts a string chat ID to int64.
func parseChatID(id string) int64 {
	var chatID int64
	fmt.Sscanf(id, "%d", &chatID)
	return chatID
}

func parseReplyMessageID(id string) (int, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, false
	}
	parts := strings.Split(id, ":")
	candidate := parts[len(parts)-1]
	n, err := strconv.Atoi(candidate)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
