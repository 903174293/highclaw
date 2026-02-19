package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/highclaw/highclaw/internal/agent"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/model"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
	"github.com/highclaw/highclaw/internal/gateway/session"
	"github.com/highclaw/highclaw/internal/skills"
)

// handleHealth returns the health status.
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "v2026.2.13",
	})
}

// handleStatus returns the overall system status.
func (s *Server) handleStatus(c *gin.Context) {
	uptime := time.Since(s.startedAt)
	uptimeStr := formatUptime(uptime)

	sessionCount := 0
	if s.sessions != nil {
		sessionCount = s.sessions.Count()
	}

	c.JSON(http.StatusOK, gin.H{
		"gateway": gin.H{
			"status":  "running",
			"port":    s.cfg.Gateway.Port,
			"bind":    s.cfg.Gateway.Bind,
			"mode":    s.cfg.Gateway.Mode,
			"version": "v2026.2.13",
			"paired":  s.pairing != nil && s.pairing.IsPaired(),
		},
		"agent": gin.H{
			"model":     s.cfg.Agent.Model,
			"workspace": s.cfg.Agent.Workspace,
		},
		"channels": gin.H{
			"telegram": s.cfg.Channels.Telegram.BotToken != "",
			"discord":  s.cfg.Channels.Discord.Token != "",
			"slack":    s.cfg.Channels.Slack.BotToken != "",
		},
		"sessions": sessionCount,
		"uptime":   uptimeStr,
	})
}

// handlePair exchanges one-time pairing code for a bearer token.
func (s *Server) handlePair(c *gin.Context) {
	if s.pairing == nil || !s.pairing.RequireAuth() {
		c.JSON(http.StatusOK, gin.H{
			"paired":  true,
			"message": "auth disabled",
		})
		return
	}

	clientKey := c.ClientIP()
	if clientKey == "" {
		clientKey = "unknown"
	}
	if s.pairRateLimiter != nil && !s.pairRateLimiter.Allow(clientKey) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many pairing attempts"})
		return
	}

	code := strings.TrimSpace(c.GetHeader("X-Pairing-Code"))
	if code == "" {
		var body struct {
			Code string `json:"code"`
		}
		_ = c.BindJSON(&body)
		code = strings.TrimSpace(body.Code)
	}

	token, ok, retryAfter := s.pairing.TryPair(code)
	if ok {
		c.JSON(http.StatusOK, gin.H{
			"paired": true,
			"token":  token,
		})
		return
	}
	if retryAfter > 0 {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":       "too many failed attempts",
			"retry_after": retryAfter,
		})
		return
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "invalid pairing code"})
}

func (s *Server) handlePairingStatus(c *gin.Context) {
	code := ""
	if s.pairing != nil {
		code = s.pairing.PairingCode()
	}
	c.JSON(http.StatusOK, gin.H{
		"require_auth": s.pairing != nil && s.pairing.RequireAuth(),
		"paired":       s.pairing != nil && s.pairing.IsPaired(),
		"pairing_code": code,
	})
}

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.publicConfig())
}

// handlePatchConfig updates the configuration.
func (s *Server) handlePatchConfig(c *gin.Context) {
	var patch map[string]json.RawMessage
	if err := c.BindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply patches to config fields.
	if agentRaw, ok := patch["agent"]; ok {
		var agentPatch struct {
			Model     *string                          `json:"model"`
			Workspace *string                          `json:"workspace"`
			Sandbox   *config.SandboxConfig            `json:"sandbox"`
			Providers map[string]config.ProviderConfig `json:"providers"`
		}
		if err := json.Unmarshal(agentRaw, &agentPatch); err == nil {
			if agentPatch.Model != nil {
				s.cfg.Agent.Model = strings.TrimSpace(*agentPatch.Model)
			}
			if agentPatch.Workspace != nil {
				s.cfg.Agent.Workspace = strings.TrimSpace(*agentPatch.Workspace)
			}
			if agentPatch.Sandbox != nil {
				s.cfg.Agent.Sandbox = *agentPatch.Sandbox
			}
			if len(agentPatch.Providers) > 0 {
				if s.cfg.Agent.Providers == nil {
					s.cfg.Agent.Providers = make(map[string]config.ProviderConfig)
				}
				for key, provider := range agentPatch.Providers {
					existing := s.cfg.Agent.Providers[key]
					if strings.TrimSpace(provider.APIKey) != "" {
						existing.APIKey = strings.TrimSpace(provider.APIKey)
					}
					if provider.BaseURL != "" {
						existing.BaseURL = strings.TrimSpace(provider.BaseURL)
					}
					s.cfg.Agent.Providers[key] = existing
				}
			}
		}
	}

	if gatewayRaw, ok := patch["gateway"]; ok {
		var gatewayPatch struct {
			Port *int    `json:"port"`
			Bind *string `json:"bind"`
			Mode *string `json:"mode"`
			Auth *struct {
				Mode     *string `json:"mode"`
				Token    *string `json:"token"`
				Password *string `json:"password"`
			} `json:"auth"`
		}
		if err := json.Unmarshal(gatewayRaw, &gatewayPatch); err == nil {
			if gatewayPatch.Port != nil && *gatewayPatch.Port > 0 {
				s.cfg.Gateway.Port = *gatewayPatch.Port
			}
			if gatewayPatch.Bind != nil {
				s.cfg.Gateway.Bind = strings.TrimSpace(*gatewayPatch.Bind)
			}
			if gatewayPatch.Mode != nil {
				s.cfg.Gateway.Mode = strings.TrimSpace(*gatewayPatch.Mode)
			}
			if gatewayPatch.Auth != nil {
				if gatewayPatch.Auth.Mode != nil {
					s.cfg.Gateway.Auth.Mode = strings.TrimSpace(*gatewayPatch.Auth.Mode)
				}
				if gatewayPatch.Auth.Token != nil {
					s.cfg.Gateway.Auth.Token = strings.TrimSpace(*gatewayPatch.Auth.Token)
				}
				if gatewayPatch.Auth.Password != nil {
					s.cfg.Gateway.Auth.Password = *gatewayPatch.Auth.Password
				}
			}
		}
	}

	if channelsRaw, ok := patch["channels"]; ok {
		var channelsPatch struct {
			Telegram *struct {
				BotToken *string `json:"botToken"`
			} `json:"telegram"`
			Discord *struct {
				Token *string `json:"token"`
			} `json:"discord"`
			Slack *struct {
				BotToken *string `json:"botToken"`
				AppToken *string `json:"appToken"`
			} `json:"slack"`
		}
		if err := json.Unmarshal(channelsRaw, &channelsPatch); err == nil {
			if channelsPatch.Telegram != nil && channelsPatch.Telegram.BotToken != nil {
				s.cfg.Channels.Telegram.BotToken = strings.TrimSpace(*channelsPatch.Telegram.BotToken)
			}
			if channelsPatch.Discord != nil && channelsPatch.Discord.Token != nil {
				s.cfg.Channels.Discord.Token = strings.TrimSpace(*channelsPatch.Discord.Token)
			}
			if channelsPatch.Slack != nil {
				if channelsPatch.Slack.BotToken != nil {
					s.cfg.Channels.Slack.BotToken = strings.TrimSpace(*channelsPatch.Slack.BotToken)
				}
				if channelsPatch.Slack.AppToken != nil {
					s.cfg.Channels.Slack.AppToken = strings.TrimSpace(*channelsPatch.Slack.AppToken)
				}
			}
		}
	}

	if webRaw, ok := patch["web"]; ok {
		var webPatch struct {
			Auth *struct {
				Enabled           *bool   `json:"enabled"`
				Username          *string `json:"username"`
				Password          *string `json:"password"`
				SessionTTLMinutes *int    `json:"sessionTtlMinutes"`
			} `json:"auth"`
			Preferences *struct {
				ShowTerminalInSidebar *bool `json:"showTerminalInSidebar"`
				AutoStart             *bool `json:"autoStart"`
			} `json:"preferences"`
		}
		if err := json.Unmarshal(webRaw, &webPatch); err == nil {
			if webPatch.Auth != nil {
				if webPatch.Auth.Enabled != nil {
					s.cfg.Web.Auth.Enabled = *webPatch.Auth.Enabled
				}
				if webPatch.Auth.Username != nil {
					s.cfg.Web.Auth.Username = strings.TrimSpace(*webPatch.Auth.Username)
				}
				if webPatch.Auth.Password != nil {
					s.cfg.Web.Auth.Password = *webPatch.Auth.Password
				}
				if webPatch.Auth.SessionTTLMinutes != nil && *webPatch.Auth.SessionTTLMinutes > 0 {
					s.cfg.Web.Auth.SessionTTLMinutes = *webPatch.Auth.SessionTTLMinutes
				}
			}
			if webPatch.Preferences != nil {
				if webPatch.Preferences.ShowTerminalInSidebar != nil {
					s.cfg.Web.Preferences.ShowTerminalInSidebar = *webPatch.Preferences.ShowTerminalInSidebar
				}
				if webPatch.Preferences.AutoStart != nil {
					s.cfg.Web.Preferences.AutoStart = *webPatch.Preferences.AutoStart
				}
			}
		}
	}

	// Save to disk
	if err := config.Save(s.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated", "config": s.publicConfig()})
}

func (s *Server) handleMeta(c *gin.Context) {
	providersConfigured := map[string]bool{}
	for key, provider := range s.cfg.Agent.Providers {
		providersConfigured[key] = strings.TrimSpace(provider.APIKey) != ""
	}

	c.JSON(http.StatusOK, gin.H{
		"version": "v2026.2.13",
		"paths": gin.H{
			"configDir":  config.ConfigDir(),
			"configFile": config.ConfigPath(),
			"workspace":  s.cfg.Agent.Workspace,
		},
		"providers": gin.H{
			"configured": providersConfigured,
		},
		"channels": gin.H{
			"telegram": strings.TrimSpace(s.cfg.Channels.Telegram.BotToken) != "",
			"discord":  strings.TrimSpace(s.cfg.Channels.Discord.Token) != "",
			"slack":    strings.TrimSpace(s.cfg.Channels.Slack.BotToken) != "",
		},
		"web": gin.H{
			"authEnabled":           s.cfg.Web.Auth.Enabled,
			"showTerminalInSidebar": s.cfg.Web.Preferences.ShowTerminalInSidebar,
			"autoStart":             s.cfg.Web.Preferences.AutoStart,
		},
	})
}

func (s *Server) publicConfig() *config.Config {
	if s.cfg == nil {
		return &config.Config{}
	}
	cp := *s.cfg
	cp.Gateway.Auth.Token = ""
	cp.Gateway.Auth.Password = ""
	cp.Web.Auth.Password = ""
	if cp.Agent.Providers != nil {
		redacted := make(map[string]config.ProviderConfig, len(cp.Agent.Providers))
		for key, provider := range cp.Agent.Providers {
			provider.APIKey = ""
			redacted[key] = provider
		}
		cp.Agent.Providers = redacted
	}
	cp.Channels.Telegram.BotToken = ""
	cp.Channels.Discord.Token = ""
	cp.Channels.Slack.BotToken = ""
	cp.Channels.Slack.AppToken = ""
	cp.Channels.BlueBubbles.Password = ""
	return &cp
}

// handleListSessions returns all sessions.
func (s *Server) handleListSessions(c *gin.Context) {
	if s.sessions == nil {
		c.JSON(http.StatusOK, gin.H{"sessions": []any{}})
		return
	}

	sessions := s.sessions.List()
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// handleGetSession returns a specific session.
func (s *Server) handleGetSession(c *gin.Context) {
	key := c.Param("key")

	if s.sessions == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	sess, ok := s.sessions.Get(key)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	messages := sess.Messages()

	c.JSON(http.StatusOK, gin.H{
		"key":          sess.Key,
		"channel":      sess.Channel,
		"model":        sess.Model,
		"agentId":      sess.AgentID,
		"messageCount": sess.MessageCount,
		"messages":     messages,
		"createdAt":    sess.CreatedAt,
		"lastActivity": sess.LastActivityAt,
	})
}

// handleCreateSession creates a new session.
func (s *Server) handleCreateSession(c *gin.Context) {
	var req struct {
		Key     string `json:"key"`
		Channel string `json:"channel"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	if s.sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session manager not available"})
		return
	}

	channel := req.Channel
	if channel == "" {
		channel = "web"
	}

	sess := s.sessions.GetOrCreate(req.Key, channel)

	c.JSON(http.StatusCreated, gin.H{
		"key":       sess.Key,
		"channel":   sess.Channel,
		"createdAt": sess.CreatedAt,
	})
}

// handleDeleteSession deletes a session.
func (s *Server) handleDeleteSession(c *gin.Context) {
	key := c.Param("key")

	if s.sessions == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if _, ok := s.sessions.Get(key); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	s.sessions.Delete(key)
	c.JSON(http.StatusOK, gin.H{"deleted": key})
}

// handlePatchSession updates a session.
func (s *Server) handlePatchSession(c *gin.Context) {
	key := c.Param("key")

	if s.sessions == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	sess, ok := s.sessions.Get(key)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	var patch struct {
		Model         *string `json:"model"`
		ThinkingLevel *string `json:"thinkingLevel"`
		VerboseLevel  *string `json:"verboseLevel"`
		AgentID       *string `json:"agentId"`
	}

	if err := c.BindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if patch.Model != nil {
		sess.Model = *patch.Model
	}
	if patch.ThinkingLevel != nil {
		sess.ThinkingLevel = *patch.ThinkingLevel
	}
	if patch.VerboseLevel != nil {
		sess.VerboseLevel = *patch.VerboseLevel
	}
	if patch.AgentID != nil {
		sess.AgentID = *patch.AgentID
	}

	c.JSON(http.StatusOK, gin.H{
		"key":           sess.Key,
		"model":         sess.Model,
		"thinkingLevel": sess.ThinkingLevel,
		"verboseLevel":  sess.VerboseLevel,
		"agentId":       sess.AgentID,
	})
}

// handleChat handles chat requests.
func (s *Server) handleChat(c *gin.Context) {
	if key := strings.TrimSpace(c.GetHeader("X-Idempotency-Key")); key != "" {
		if !s.recordIdempotencyKey(key) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "duplicate",
				"message": "request already processed",
			})
			return
		}
	}

	var req struct {
		Message   string `json:"message"`
		Session   string `json:"session"`
		PeerID    string `json:"peerId"`
		PeerKind  string `json:"peerKind"`
		GroupID   string `json:"groupId"`
		AccountID string `json:"accountId"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	sessionKey := req.Session
	if sessionKey == "" {
		// 使用 DM Scope 路由
		peerID := strings.TrimSpace(req.PeerID)
		if peerID == "" {
			peerID = c.ClientIP()
		}
		peer := session.PeerContext{
			Channel:      "web",
			PeerID:       peerID,
			PeerKind:     req.PeerKind,
			GroupID:      req.GroupID,
			AccountID:    req.AccountID,
			Conversation: c.ClientIP(),
		}
		sessionKey = session.ResolveSessionFromConfig(s.cfg, peer)
	}

	var history []agent.ChatMessage

	// Ensure session exists
	if s.sessions != nil {
		sess := s.sessions.GetOrCreate(sessionKey, "web")
		sess.AddMessage(protocol.ChatMessage{
			Role:    "user",
			Content: req.Message,
			Channel: "web",
		})
		history = sessionToAgentHistory(sess.Messages(), 16)
	}

	// Call agent
	if s.agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not available"})
		return
	}

	start := time.Now()
	result, err := s.agent.Run(c.Request.Context(), &agent.RunRequest{
		SessionKey: sessionKey,
		Channel:    "web",
		Sender:     c.ClientIP(),
		MessageID:  strings.TrimSpace(c.GetHeader("X-Idempotency-Key")),
		Message:    req.Message,
		History:    history,
	})
	if err != nil {
		s.logger.Error("agent run failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent error: " + err.Error()})
		return
	}
	latencyMS := time.Since(start).Milliseconds()

	// Store assistant response in session
	if s.sessions != nil {
		if sess, ok := s.sessions.Get(sessionKey); ok {
			sess.AddMessage(protocol.ChatMessage{
				Role:    "assistant",
				Content: result.Reply,
				Channel: "web",
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"response":  result.Reply,
		"usage":     result.TokensUsed,
		"latencyMs": latencyMS,
		"model":     s.cfg.Agent.Model,
	})
}

func sessionToAgentHistory(msgs []protocol.ChatMessage, limit int) []agent.ChatMessage {
	if len(msgs) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 12
	}
	start := 0
	if len(msgs) > limit {
		start = len(msgs) - limit
	}
	out := make([]agent.ChatMessage, 0, len(msgs)-start)
	for _, msg := range msgs[start:] {
		role := strings.TrimSpace(strings.ToLower(msg.Role))
		switch role {
		case "user", "assistant", "system":
		default:
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		content = trimForModelContext(content, 3000)
		out = append(out, agent.ChatMessage{
			Role:    role,
			Content: content,
		})
	}
	return out
}

func trimForModelContext(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func (s *Server) recordIdempotencyKey(key string) bool {
	s.idempotencyMu.Lock()
	defer s.idempotencyMu.Unlock()
	now := time.Now()
	for k, t := range s.idempotency {
		if now.Sub(t) > 5*time.Minute {
			delete(s.idempotency, k)
		}
	}
	if _, exists := s.idempotency[key]; exists {
		return false
	}
	s.idempotency[key] = now
	return true
}

// handleListChannels returns all available channels.
func (s *Server) handleListChannels(c *gin.Context) {
	channels := []gin.H{
		{"name": "telegram", "type": "bot", "configured": s.cfg.Channels.Telegram.BotToken != ""},
		{"name": "whatsapp", "type": "web", "configured": false},
		{"name": "discord", "type": "bot", "configured": s.cfg.Channels.Discord.Token != ""},
		{"name": "slack", "type": "bot", "configured": s.cfg.Channels.Slack.BotToken != ""},
	}
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

// handleChannelsStatus returns the status of all channels.
func (s *Server) handleChannelsStatus(c *gin.Context) {
	channels := []gin.H{
		{
			"name":   "telegram",
			"status": boolToStatus(s.cfg.Channels.Telegram.BotToken != ""),
			"icon":   "fab fa-telegram",
		},
		{
			"name":   "whatsapp",
			"status": "disconnected",
			"icon":   "fab fa-whatsapp",
		},
		{
			"name":   "discord",
			"status": boolToStatus(s.cfg.Channels.Discord.Token != ""),
			"icon":   "fab fa-discord",
		},
		{
			"name":   "slack",
			"status": boolToStatus(s.cfg.Channels.Slack.BotToken != ""),
			"icon":   "fab fa-slack",
		},
	}
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

// handleListModels returns all available models.
func (s *Server) handleListModels(c *gin.Context) {
	models := model.GetAllModelsComplete()
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// handleListProviders returns all available providers.
func (s *Server) handleListProviders(c *gin.Context) {
	providers := []gin.H{
		{"name": "anthropic", "models": len(model.AnthropicModels)},
		{"name": "openai", "models": len(model.OpenAIModels)},
		{"name": "google", "models": len(model.GoogleModels)},
		{"name": "bedrock", "models": len(model.BedrockModels)},
		{"name": "azure", "models": len(model.AzureModels)},
		{"name": "ollama", "models": len(model.OllamaModels)},
		{"name": "groq", "models": len(model.GroqModels)},
		{"name": "together", "models": len(model.TogetherModels)},
		{"name": "fireworks", "models": len(model.FireworksModels)},
		{"name": "cohere", "models": len(model.CohereModels)},
		{"name": "mistral", "models": len(model.MistralModels)},
		{"name": "perplexity", "models": len(model.PerplexityModels)},
		{"name": "deepseek", "models": len(model.DeepSeekModels)},
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

// handleListSkills 返回所有已发现的 user-defined skills
func (s *Server) handleListSkills(c *gin.Context) {
	mgr := skills.NewManager(s.cfg.Agent.Workspace)
	allSkills := mgr.LoadAll()

	openSkillsCount := 0
	workspaceCount := 0
	for _, sk := range allSkills {
		if sk.Source == "open-skills" {
			openSkillsCount++
		} else {
			workspaceCount++
		}
	}

	summary := map[string]int{
		"total":            len(allSkills),
		"open_skills":      openSkillsCount,
		"workspace_skills": workspaceCount,
	}

	c.JSON(http.StatusOK, gin.H{
		"skills":  allSkills,
		"summary": summary,
	})
}

// handleLogs returns the buffered log entries.
func (s *Server) handleLogs(c *gin.Context) {
	if s.logBuffer == nil {
		c.JSON(http.StatusOK, gin.H{"logs": []any{}})
		return
	}

	entries := s.logBuffer.Entries()
	c.JSON(http.StatusOK, gin.H{"logs": entries})
}

// handleRuntimeStats returns runtime statistics.
func (s *Server) handleRuntimeStats(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(s.startedAt)

	c.JSON(http.StatusOK, gin.H{
		"memoryMB":   float64(mem.Alloc) / 1024 / 1024,
		"totalMB":    float64(mem.TotalAlloc) / 1024 / 1024,
		"sysMB":      float64(mem.Sys) / 1024 / 1024,
		"goroutines": runtime.NumGoroutine(),
		"uptime":     formatUptime(uptime),
		"uptimeSec":  int(uptime.Seconds()),
		"gcCycles":   mem.NumGC,
	})
}

// formatUptime formats a duration into a human-readable string.
func formatUptime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// boolToStatus returns "configured" or "disconnected" based on a boolean.
func boolToStatus(configured bool) string {
	if configured {
		return "configured"
	}
	return "disconnected"
}
