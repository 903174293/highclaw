// Package wechat 实现微信 channel (公众号/个人号)
package wechat

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/highclaw/highclaw/internal/domain/channel"
)

// Config 表示微信 channel 配置
type Config struct {
	Mode                string
	AppID               string
	AppSecret           string
	Token               string
	EncodingAESKey      string
	AllowedUsers        []string
	PersonalBridgeURL   string
	PersonalBridgeToken string
}

// WeChatChannel 实现微信消息 channel
type WeChatChannel struct {
	config      Config
	logger      *slog.Logger
	connected   bool
	mu          sync.RWMutex
	messages    chan *channel.Message
	stopCh      chan struct{}
	accessToken string
	tokenExpiry time.Time
}

// NewWeChatChannel 创建微信 channel 实例
func NewWeChatChannel(config Config, logger *slog.Logger) *WeChatChannel {
	return &WeChatChannel{
		config:   config,
		logger:   logger,
		messages: make(chan *channel.Message, 100),
		stopCh:   make(chan struct{}),
	}
}

// ID 返回 channel ID
func (w *WeChatChannel) ID() string { return "wechat" }

// Name 返回 channel 名称
func (w *WeChatChannel) Name() string {
	if w.config.Mode == "personal" {
		return "WeChat (Personal)"
	}
	return "WeChat (Official)"
}

// Type 返回 channel 类型
func (w *WeChatChannel) Type() string {
	if w.config.Mode == "personal" {
		return "bridge"
	}
	return "bot"
}

// Start 启动微信 channel
func (w *WeChatChannel) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.connected {
		return fmt.Errorf("wechat channel already started")
	}
	mode := w.config.Mode
	if mode == "" {
		mode = "official"
	}
	w.logger.Info("starting wechat channel", "mode", mode)
	if mode == "official" {
		if w.config.AppID == "" || w.config.AppSecret == "" {
			return fmt.Errorf("wechat official mode requires app_id and app_secret")
		}
		if err := w.refreshAccessToken(ctx); err != nil {
			return fmt.Errorf("failed to get access token: %w", err)
		}
		go w.tokenRefreshLoop(ctx)
	} else if mode == "personal" {
		if w.config.PersonalBridgeURL == "" {
			return fmt.Errorf("wechat personal mode requires bridge_url")
		}
		w.logger.Info("wechat personal mode using bridge", "url", w.config.PersonalBridgeURL)
	} else {
		return fmt.Errorf("invalid wechat mode: %s (use 'official' or 'personal')", mode)
	}
	w.connected = true
	go w.pollMessages(ctx)
	return nil
}

// Stop 停止微信 channel
func (w *WeChatChannel) Stop(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.connected {
		return nil
	}
	w.logger.Info("stopping wechat channel")
	close(w.stopCh)
	w.connected = false
	return nil
}

// IsConnected 返回是否已连接
func (w *WeChatChannel) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.connected
}

// SendMessage 发送消息到微信
func (w *WeChatChannel) SendMessage(ctx context.Context, msg *channel.Message) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !w.connected {
		return fmt.Errorf("wechat channel not connected")
	}
	w.logger.Info("sending wechat message", "to", msg.To, "text", truncate(msg.Text, 50), "mode", w.config.Mode)
	if w.config.Mode == "personal" {
		// TODO: POST {bridgeURL}/send
		return nil
	}
	// TODO: POST https://api.weixin.qq.com/cgi-bin/message/custom/send?access_token=ACCESS_TOKEN
	return nil
}

// ReceiveMessages 返回接收消息的 channel
func (w *WeChatChannel) ReceiveMessages() <-chan *channel.Message {
	return w.messages
}

// StartTyping 微信暂不支持原生 typing 指示
func (w *WeChatChannel) StartTyping(ctx context.Context, recipient string) error {
	return nil
}

// StopTyping 停止输入指示
func (w *WeChatChannel) StopTyping(ctx context.Context, recipient string) error {
	return nil
}

// refreshAccessToken 获取或刷新 access_token (公众号模式)
func (w *WeChatChannel) refreshAccessToken(ctx context.Context) error {
	if w.config.Mode != "official" && w.config.Mode != "" {
		return nil
	}
	// TODO: GET https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=APPID&secret=APPSECRET
	w.logger.Info("refreshing wechat access token")
	w.accessToken = "mock_access_token"
	w.tokenExpiry = time.Now().Add(2 * time.Hour)
	return nil
}

// tokenRefreshLoop 定期刷新 access token
func (w *WeChatChannel) tokenRefreshLoop(ctx context.Context) {
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
				w.logger.Error("failed to refresh wechat access token", "error", err)
			}
		}
	}
}

// pollMessages 轮询消息（或等待 webhook/bridge 推送）
func (w *WeChatChannel) pollMessages(ctx context.Context) {
	w.logger.Info("wechat message handler started", "mode", w.config.Mode)
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
func (w *WeChatChannel) isUserAllowed(userID string) bool {
	for _, u := range w.config.AllowedUsers {
		if u == "*" || u == userID {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
