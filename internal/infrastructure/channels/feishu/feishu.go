// Package feishu 实现飞书/Lark channel，通过 SDK 长连接收消息，通过 API 回复
package feishu

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Config 飞书 channel 配置
type Config struct {
	AppID        string
	AppSecret    string
	VerifyToken  string
	EncryptKey   string
	AllowedUsers []string
	AllowedChats []string
}

// ParsedMessage 解析后的消息，供上层路由使用
type ParsedMessage struct {
	MessageID string
	ChatID    string
	ChatType  string // "p2p" | "group"
	SenderID  string // open_id
	Text      string
}

// MessageHandler 消息处理回调：收到消息后调用，返回 AI 回复文本
type MessageHandler func(ctx context.Context, msg *ParsedMessage) (reply string, err error)

// bindState 持久化到磁盘的 bind 状态
type bindState struct {
	BoundUserID string `json:"boundUserID"`
	BoundAt     string `json:"boundAt"`
}

// FeishuChannel 飞书消息 channel，SDK 长连接 + bind 验证码模式
type FeishuChannel struct {
	config    Config
	logger    *slog.Logger
	apiClient *lark.Client
	wsClient  *larkws.Client

	mu          sync.RWMutex
	connected   bool
	bound       bool
	boundUserID string
	bindCode    string

	onMessage MessageHandler
}

// NewFeishuChannel 创建飞书 channel 实例
func NewFeishuChannel(config Config, logger *slog.Logger) *FeishuChannel {
	return &FeishuChannel{
		config: config,
		logger: logger,
	}
}

func (f *FeishuChannel) ID() string   { return "feishu" }
func (f *FeishuChannel) Name() string { return "Feishu/Lark" }
func (f *FeishuChannel) Type() string { return "bot" }

// SetMessageHandler 注册消息处理回调（gateway 启动时调用）
func (f *FeishuChannel) SetMessageHandler(h MessageHandler) {
	f.onMessage = h
}

// BindCode 返回当前 bind 验证码（供 reload API 返回给控制台）
func (f *FeishuChannel) BindCode() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bindCode
}

// UpdateAllowlist 运行时更新白名单（不断连）
func (f *FeishuChannel) UpdateAllowlist(users, chats []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config.AllowedUsers = users
	f.config.AllowedChats = chats
	f.logger.Info("feishu allowlist updated", "users", len(users), "chats", len(chats))
}

// Start 启动长连接：检查持久化状态/白名单，必要时生成 bind 验证码
func (f *FeishuChannel) Start(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.connected {
		return fmt.Errorf("feishu channel already started")
	}
	if f.config.AppID == "" || f.config.AppSecret == "" {
		return fmt.Errorf("feishu appId and appSecret are required")
	}

	// 检查是否可以跳过 bind 流程
	if f.shouldSkipBind() {
		f.bound = true
		f.logger.Info("feishu: bind skipped (allowlist configured or previous bind restored)")
	} else {
		// 尝试从磁盘恢复 bind 状态
		if state, err := f.loadBindState(); err == nil && state.BoundUserID != "" {
			f.bound = true
			f.boundUserID = state.BoundUserID
			f.logger.Info("feishu: bind restored from state", "user", maskID(state.BoundUserID))
		} else {
			// 首次绑定：生成 bind 验证码
			f.bindCode = generateBindCode(6)
			f.bound = false
		}
	}

	// 创建 API 客户端（自动管理 tenant_access_token）
	f.apiClient = lark.NewClient(f.config.AppID, f.config.AppSecret)

	// 创建事件分发器，注册消息接收回调
	eventHandler := dispatcher.NewEventDispatcher(f.config.VerifyToken, f.config.EncryptKey).
		OnP2MessageReceiveV1(f.handleMessageEvent)

	// 创建 WebSocket 长连接客户端
	f.wsClient = larkws.NewClient(f.config.AppID, f.config.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	// Start() 是阻塞式的，放后台 goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- f.wsClient.Start(ctx)
	}()

	// 等 3 秒检测是否立即失败（如凭证错误）
	select {
	case err := <-errCh:
		return fmt.Errorf("feishu connection failed: %w", err)
	case <-time.After(3 * time.Second):
	}

	f.connected = true

	// 输出连接状态
	if f.bound {
		printFeishuBanner("reconnected", "", f.boundUserID)
	} else {
		printFeishuBanner("waiting_bind", f.bindCode, "")
	}

	return nil
}

// Stop 停止飞书 channel
func (f *FeishuChannel) Stop(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.connected {
		return nil
	}
	f.connected = false
	return nil
}

// IsConnected 返回连接状态
func (f *FeishuChannel) IsConnected() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.connected
}

// IsBound 返回是否已完成 bind 验证
func (f *FeishuChannel) IsBound() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bound
}

// handleMessageEvent SDK 事件回调入口：解析消息 -> bind 流程 -> 业务处理
func (f *FeishuChannel) handleMessageEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return nil
	}

	msg := event.Event.Message
	sender := event.Event.Sender

	// 只处理文本消息
	if msg.MessageType == nil || *msg.MessageType != "text" {
		return nil
	}

	// 解析文本内容（飞书 content 是 JSON: {"text":"xxx"}）
	var content struct {
		Text string `json:"text"`
	}
	if msg.Content != nil {
		_ = json.Unmarshal([]byte(*msg.Content), &content)
	}
	text := strings.TrimSpace(content.Text)
	if text == "" {
		return nil
	}

	senderID := ptrStr(sender.SenderId.OpenId)
	chatID := ptrStr(msg.ChatId)
	chatType := ptrStr(msg.ChatType)
	messageID := ptrStr(msg.MessageId)

	// bind 验证流程
	f.mu.RLock()
	bound := f.bound
	f.mu.RUnlock()

	if !bound {
		return f.handleBind(ctx, messageID, senderID, text)
	}

	// 白名单检查
	if !f.isUserAllowed(senderID) {
		f.logger.Warn("feishu: message rejected (unauthorized user)",
			"openId", maskID(senderID),
			"chatId", maskID(chatID),
			"chatType", chatType,
			"messagePreview", truncateText(text, 50),
		)
		return nil
	}
	if chatType == "group" && !f.isChatAllowed(chatID) {
		f.logger.Warn("feishu: message rejected (unauthorized chat)",
			"chatId", maskID(chatID),
			"openId", maskID(senderID),
			"chatType", chatType,
			"messagePreview", truncateText(text, 50),
		)
		return nil
	}

	if f.onMessage == nil {
		f.logger.Warn("feishu: no message handler registered")
		return nil
	}

	parsed := &ParsedMessage{
		MessageID: messageID,
		ChatID:    chatID,
		ChatType:  chatType,
		SenderID:  senderID,
		Text:      text,
	}

	// 异步处理，避免阻塞 SDK 事件循环
	go func() {
		reply, err := f.onMessage(context.Background(), parsed)
		if err != nil {
			f.logger.Error("feishu message handler error", "error", err)
			_ = f.replyText(context.Background(), messageID, "error: "+err.Error())
			return
		}
		if reply != "" {
			if err := f.replyText(context.Background(), messageID, reply); err != nil {
				f.logger.Error("feishu reply failed", "error", err)
			}
		}
	}()

	return nil
}

// handleBind 处理 bind 验证码匹配，成功后持久化状态
func (f *FeishuChannel) handleBind(ctx context.Context, messageID, senderID, text string) error {
	code := strings.TrimSpace(text)
	if strings.HasPrefix(strings.ToLower(code), "bind ") {
		code = strings.TrimSpace(code[5:])
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if strings.EqualFold(code, f.bindCode) {
		f.bound = true
		f.boundUserID = senderID
		f.bindCode = ""
		f.logger.Info("feishu bind successful", "sender", maskID(senderID))

		_ = f.saveBindState(bindState{
			BoundUserID: senderID,
			BoundAt:     time.Now().UTC().Format(time.RFC3339),
		})

		// 使用独立 context 回复，避免 SDK 回调 context 超时导致回复丢失
		if err := f.replyText(context.Background(), messageID, "Bind successful! HighClaw connected. You can start chatting now."); err != nil {
			f.logger.Error("feishu bind success reply failed", "error", err)
		}
		return nil
	}

	f.logger.Warn("feishu bind code mismatch", "got", code)
	if err := f.replyText(context.Background(), messageID, "Bind code mismatch. Please check the bind code in your terminal and send: bind <code>"); err != nil {
		f.logger.Error("feishu bind mismatch reply failed", "error", err)
	}
	return nil
}

// replyText 通过 SDK API 客户端回复文本消息
func (f *FeishuChannel) replyText(ctx context.Context, messageID, text string) error {
	if f.apiClient == nil {
		return fmt.Errorf("api client not initialized")
	}

	contentJSON, _ := json.Marshal(map[string]string{"text": text})
	resp, err := f.apiClient.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("text").
			Content(string(contentJSON)).
			Build()).
		Build())

	if err != nil {
		return fmt.Errorf("feishu reply API call: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu reply error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// ReplyMessage 公开的回复方法（供外部使用）
func (f *FeishuChannel) ReplyMessage(ctx context.Context, messageID, text string) error {
	return f.replyText(ctx, messageID, text)
}

// ============ bind 状态持久化 ============

// shouldSkipBind 如果配置了白名单，跳过 bind 流程
func (f *FeishuChannel) shouldSkipBind() bool {
	return len(f.config.AllowedUsers) > 0
}

func bindStatePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".highclaw", "state", "feishu.json")
}

func (f *FeishuChannel) loadBindState() (*bindState, error) {
	data, err := os.ReadFile(bindStatePath())
	if err != nil {
		return nil, err
	}
	var s bindState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (f *FeishuChannel) saveBindState(s bindState) error {
	p := bindStatePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(p, data, 0o644)
}

// ============ 白名单 ============

func (f *FeishuChannel) isUserAllowed(userID string) bool {
	// 配置了白名单：按白名单判断
	if len(f.config.AllowedUsers) > 0 {
		for _, u := range f.config.AllowedUsers {
			if u == "*" || u == userID {
				return true
			}
		}
		return false
	}
	// 未配置白名单：仅允许 bind 成功的用户
	if f.boundUserID != "" {
		return userID == f.boundUserID
	}
	// 既没有白名单也没有 bind 用户（理论上不会到这里，因为 bound=false 时走 bind 流程）
	return false
}

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

// ============ stdout 醒目输出 ============

// printFeishuBanner 在终端输出醒目的飞书状态信息（非日志，直接 stdout）
func printFeishuBanner(status, bindCode, boundUser string) {
	const (
		green  = "\033[32m"
		yellow = "\033[33m"
		cyan   = "\033[36m"
		bold   = "\033[1m"
		reset  = "\033[0m"
	)

	fmt.Println()
	fmt.Println(cyan + "  ╔══════════════════════════════════════════════╗" + reset)
	switch status {
	case "waiting_bind":
		fmt.Println(cyan + "  ║" + reset + bold + "  Feishu Bot Connected (waiting for bind)     " + cyan + "║" + reset)
		fmt.Println(cyan + "  ║" + reset + "                                              " + cyan + "║" + reset)
		fmt.Println(cyan + "  ║" + reset + "  Bind code: " + yellow + bold + bindCode + reset + "                              " + cyan + "║" + reset)
		fmt.Println(cyan + "  ║" + reset + "  Send to bot: " + green + "bind " + bindCode + reset + "                     " + cyan + "║" + reset)
	case "reconnected":
		fmt.Println(cyan + "  ║" + reset + bold + green + "  Feishu Bot Reconnected                      " + reset + cyan + "║" + reset)
		if boundUser != "" {
			fmt.Println(cyan + "  ║" + reset + "  Bound user: " + maskID(boundUser) + "                       " + cyan + "║" + reset)
		}
	}
	fmt.Println(cyan + "  ╚══════════════════════════════════════════════╝" + reset)
	fmt.Println()
}

// ============ 工具函数 ============

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// truncateText 截断文本用于日志预览，避免大段内容刷屏
func truncateText(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen]) + "..."
}


func maskID(s string) string {
	if len(s) < 10 {
		return "***"
	}
	return s[:5] + "..." + s[len(s)-3:]
}

func generateBindCode(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, length)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}
	return string(code)
}
