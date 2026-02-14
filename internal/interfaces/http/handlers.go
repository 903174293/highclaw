package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/highclaw/highclaw/internal/agent"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/domain/model"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
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

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.cfg)
}

// handlePatchConfig updates the configuration.
func (s *Server) handlePatchConfig(c *gin.Context) {
	var patch map[string]json.RawMessage
	if err := c.BindJSON(&patch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply patches to config fields
	if agentRaw, ok := patch["agent"]; ok {
		var agentPatch config.AgentConfig
		if err := json.Unmarshal(agentRaw, &agentPatch); err == nil {
			if agentPatch.Model != "" {
				s.cfg.Agent.Model = agentPatch.Model
			}
			if agentPatch.Workspace != "" {
				s.cfg.Agent.Workspace = agentPatch.Workspace
			}
		}
	}

	if gatewayRaw, ok := patch["gateway"]; ok {
		var gatewayPatch config.GatewayConfig
		if err := json.Unmarshal(gatewayRaw, &gatewayPatch); err == nil {
			if gatewayPatch.Port != 0 {
				s.cfg.Gateway.Port = gatewayPatch.Port
			}
			if gatewayPatch.Bind != "" {
				s.cfg.Gateway.Bind = gatewayPatch.Bind
			}
			if gatewayPatch.Mode != "" {
				s.cfg.Gateway.Mode = gatewayPatch.Mode
			}
		}
	}

	// Save to disk
	if err := config.Save(s.cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "config updated", "config": s.cfg})
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
	var req struct {
		Message string `json:"message"`
		Session string `json:"session"`
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
		sessionKey = "main"
	}

	// Ensure session exists
	if s.sessions != nil {
		sess := s.sessions.GetOrCreate(sessionKey, "web")
		sess.AddMessage(protocol.ChatMessage{
			Role:    "user",
			Content: req.Message,
			Channel: "web",
		})
	}

	// Call agent
	if s.agent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not available"})
		return
	}

	result, err := s.agent.Run(c.Request.Context(), &agent.RunRequest{
		SessionKey: sessionKey,
		Channel:    "web",
		Message:    req.Message,
	})
	if err != nil {
		s.logger.Error("agent run failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "agent error: " + err.Error()})
		return
	}

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
		"response": result.Reply,
		"usage":    result.TokensUsed,
	})
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
	models := model.GetAllModels()
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

// handleListSkills returns all discovered skills with their status.
func (s *Server) handleListSkills(c *gin.Context) {
	if s.skills == nil {
		c.JSON(http.StatusOK, gin.H{"skills": []any{}, "summary": map[string]int{}})
		return
	}

	skills, err := s.skills.DiscoverSkills(c.Request.Context())
	if err != nil {
		s.logger.Error("failed to discover skills", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to discover skills: " + err.Error()})
		return
	}

	summary, _ := s.skills.GetSkillsSummary(c.Request.Context())

	c.JSON(http.StatusOK, gin.H{
		"skills":  skills,
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
