package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/highclaw/highclaw/internal/domain/model"
)

// handleHealth returns the health status.
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"version": "v2026.2.13",
	})
}

// handleStatus returns the overall system status.
func (s *Server) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"gateway": gin.H{
			"running": true,
			"port":    s.cfg.Gateway.Port,
			"bind":    s.cfg.Gateway.Bind,
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
	})
}

// handleGetConfig returns the current configuration.
func (s *Server) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, s.cfg)
}

// handlePatchConfig updates the configuration.
func (s *Server) handlePatchConfig(c *gin.Context) {
	// TODO: Implement config patching
	c.JSON(http.StatusOK, gin.H{"message": "config.patch not yet implemented"})
}

// handleListSessions returns all sessions.
func (s *Server) handleListSessions(c *gin.Context) {
	// TODO: Get sessions from session manager
	c.JSON(http.StatusOK, []gin.H{
		{"key": "main", "channel": "cli", "messageCount": 0},
	})
}

// handleGetSession returns a specific session.
func (s *Server) handleGetSession(c *gin.Context) {
	key := c.Param("key")
	// TODO: Get session from session manager
	c.JSON(http.StatusOK, gin.H{
		"key":          key,
		"channel":      "cli",
		"messageCount": 0,
		"messages":     []gin.H{},
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
	
	// TODO: Create session
	c.JSON(http.StatusCreated, gin.H{
		"key":     req.Key,
		"channel": req.Channel,
	})
}

// handleDeleteSession deletes a session.
func (s *Server) handleDeleteSession(c *gin.Context) {
	key := c.Param("key")
	// TODO: Delete session
	c.JSON(http.StatusOK, gin.H{"deleted": key})
}

// handlePatchSession updates a session.
func (s *Server) handlePatchSession(c *gin.Context) {
	key := c.Param("key")
	// TODO: Patch session
	c.JSON(http.StatusOK, gin.H{"updated": key})
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
	
	// TODO: Send to agent and get response
	c.JSON(http.StatusOK, gin.H{
		"response": "[Agent placeholder] Message received: " + req.Message,
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
	c.JSON(http.StatusOK, channels)
}

// handleChannelsStatus returns the status of all channels.
func (s *Server) handleChannelsStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"telegram": gin.H{"connected": false},
		"whatsapp": gin.H{"connected": false},
		"discord":  gin.H{"connected": false},
		"slack":    gin.H{"connected": false},
	})
}

// handleListModels returns all available models.
func (s *Server) handleListModels(c *gin.Context) {
	models := model.GetAllModels()
	c.JSON(http.StatusOK, models)
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
	c.JSON(http.StatusOK, providers)
}

