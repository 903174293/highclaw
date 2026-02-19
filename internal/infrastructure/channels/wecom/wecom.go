// Package wecom 实现企业微信 channel
package wecom

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config 表示企业微信 channel 配置
type Config struct {
	CorpID             string
	AgentID            int
	Secret             string
	Token              string
	EncodingAESKey     string
	AllowedUsers       []string
	AllowedDepartments []int
}

// WeComChannel 实现企业微信消息 channel
type WeComChannel struct {
	config      Config
	logger      *slog.Logger
	connected   bool
	mu          sync.RWMutex
	messages    chan *channel.Message
	stopCh      chan struct{}
	accessToken string
	tokenExpiry time.Time
}

// NewWeComChannel 创建企业微信 channel 实例
func NewWeComChannel(config Config, logger *slog.Logger) *WeComChannel {
	return &WeComChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID 返回 channel ID
func (w *WeComChannel) ID() string { return "wecom" }

// Name 返回 channel 名称
func (w *WeComChannel) Name() string { return "WeCom" }

// Type 返回 channel 类型
func (w *WeComChannel) Type() string { return "bot" }

// Start 启动企业微信 channel
func (w *WeComChannel) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.connected {
		return fmt.Errorf("wecom channel already started")
	}
	w.logger.Info("starting wecom channel", "corpId", maskToken(w.config.CorpID), "agentId", w.config.AgentID)
	if w.config.CorpID == "" || w.config.Secret == "" {
		return fmt.Errorf("wecom corp_id and secret are required")
	}
	if err := w.refreshAccessToken(ctx); err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	w.connected = true
	go w.tokenRefreshLoop(ctx)
	go w.pollMessages(ctx)
	return nil
}

// Stop 停止企业微信 channel
func (w *WeComChannel) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.connected {
		return nil
	}
	w.logger.Info("stopping wecom channel")
	close(w.stopCh)
	w.connected = false
	return nil
}

// IsConnected 返回是否已连接
func (w *WeComChannel) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.connected
}

// SendMessage 发送消息到企业微信
func (w *WeComChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.connected {
		return fmt.Errorf("wecom channel not connected")
	}
	w.logger.Info("sending wecom message", "to", msg.To, "text", truncate(msg.Text, 50))
	// TODO: POST https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=ACCESS_TOKEN
	return nil
}

// ReceiveMessages 返回接收消息的 channel
func (w *WeComChannel) ReceiveMessages() <-chan *channel.Message {
	return w.messages
}

// StartTyping 企业微信暂不支持原生 typing 指示
func (w *WeComChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

// StopTyping 停止输入指示
func (w *WeComChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

// refreshAccessToken 获取或刷新 access_token
func (w *WeComChannel) refreshAccessToken(ctx context.Context) error {
	// TODO: GET https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=ID&corpsecret=SECRET
	w.logger.Info("refreshing wecom access token")
	w.accessToken = "mock_access_token"
	w.tokenExpiry = time.Now().Add(2 * time.Hour)
	return nil
}

// tokenRefreshLoop 定期刷新 access token
func (w *WeComChannel) tokenRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.refreshAccessToken(ctx); err != nil {
				w.logger.Error("failed to refresh wecom access token", "error", err)
			}
		}
	}
}

// pollMessages 轮询消息（或等待 webhook 推送）
func (w *WeComChannel) pollMessages(ctx context.Context) {
	w.logger.Info("wecom message handler started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// isUserAllowed 检查用户是否在白名单中
func (w *WeComChannel) isUserAllowed(userID string) bool {
	for _, u := range w.config.AllowedUsers {
		if u == "*" || u == userID {
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
