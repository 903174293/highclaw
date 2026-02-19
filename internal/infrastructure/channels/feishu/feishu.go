// Package feishu 实现飞书/Lark channel
package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config 表示飞书 channel 配置
type Config struct {
	AppID        string
	AppSecret    string
	VerifyToken  string
	EncryptKey   string
	AllowedUsers []string
	AllowedChats []string
	WebhookURL   string
}

// FeishuChannel 实现飞书消息 channel
type FeishuChannel struct {
	config      Config
	logger      *slog.Logger
	connected   bool
	mu          sync.RWMutex
	messages    chan *channel.Message
	stopCh      chan struct{}
	accessToken string
	tokenExpiry time.Time
}

// NewFeishuChannel 创建飞书 channel 实例
func NewFeishuChannel(config Config, logger *slog.Logger) *FeishuChannel {
	return &FeishuChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID 返回 channel ID
func (f *FeishuChannel) ID() string { return "feishu" }

// Name 返回 channel 名称
func (f *FeishuChannel) Name() string { return "Feishu/Lark" }

// Type 返回 channel 类型
func (f *FeishuChannel) Type() string { return "bot" }

// Start 启动飞书 channel
func (f *FeishuChannel) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.connected {
		return fmt.Errorf("feishu channel already started")
	}
	f.logger.Info("starting feishu channel", "appId", maskToken(f.config.AppID))
	if f.config.AppID == "" || f.config.AppSecret == "" {
		return fmt.Errorf("feishu app_id and app_secret are required")
	}
	if err := f.refreshAccessToken(ctx); err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	f.connected = true
	go f.tokenRefreshLoop(ctx)
	go f.pollMessages(ctx)
	return nil
}

// Stop 停止飞书 channel
func (f *FeishuChannel) Stop(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.connected {
		return nil
	}
	f.logger.Info("stopping feishu channel")
	close(f.stopCh)
	f.connected = false
	return nil
}

// IsConnected 返回是否已连接
func (f *FeishuChannel) IsConnected() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.connected
}

// SendMessage 发送消息到飞书
func (f *FeishuChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if !f.connected {
		return fmt.Errorf("feishu channel not connected")
	}
	f.logger.Info("sending feishu message", "to", msg.To, "text", truncate(msg.Text, 50))
	// TODO: POST https://open.feishu.cn/open-apis/im/v1/messages
	return nil
}

// ReceiveMessages 返回接收消息的 channel
func (f *FeishuChannel) ReceiveMessages() <-chan *channel.Message {
	return f.messages
}

// StartTyping 飞书暂不支持原生 typing 指示
func (f *FeishuChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

// StopTyping 停止输入指示
func (f *FeishuChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

// refreshAccessToken 获取或刷新 tenant_access_token
func (f *FeishuChannel) refreshAccessToken(ctx context.Context) error {
	// TODO: POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal
	f.logger.Info("refreshing feishu access token")
	f.accessToken = "mock_access_token"
	f.tokenExpiry = time.Now().Add(2 * time.Hour)
	return nil
}

// tokenRefreshLoop 定期刷新 access token
func (f *FeishuChannel) tokenRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		case <-ticker.C:
			if err := f.refreshAccessToken(ctx); err != nil {
				f.logger.Error("failed to refresh feishu access token", "error", err)
			}
		}
	}
}

// pollMessages 轮询消息（或等待 webhook 推送）
func (f *FeishuChannel) pollMessages(ctx context.Context) {
	f.logger.Info("feishu message handler started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// isUserAllowed 检查用户是否在白名单中
func (f *FeishuChannel) isUserAllowed(userID string) bool {
	for _, u := range f.config.AllowedUsers {
		if u == "*" || u == userID {
			return true
		}
	}
	return false
}

// isChatAllowed 检查群聊是否在白名单中
func (f *FeishuChannel) isChatAllowed(chatID string) bool {
	if len(f.config.AllowedChats) == 0 {
		return true
	}
	for _, c := range f.config.AllowedChats {
		if c == "*" || c == chatID {
			return true
		}
	}
	return false
}

func maskToken(token string) string {
	if len(token) < 10 {
		return "***"
	}
	return token[:5] + "..." + token[len(token)-3:]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
