package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleHealth returns the health status.
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "v2026.2.13",
		"uptime":  formatUptime(time.Since(s.startedAt)),
	})
}

// handleChannelsReload triggers channel hot-reload.
func (s *Server) handleChannelsReload(c *gin.Context) {
	if s.reloadChannels == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel reload not available"})
		return
	}

	result, err := s.reloadChannels(c.Request.Context())
	if err != nil {
		s.logger.Error("channel reload failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

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


// handleChannelStatus 返回各 channel 的运行时状态（仅限 localhost 访问）
func (s *Server) handleChannelStatus(c *gin.Context) {
	if s.getChannelStatus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channel status not available"})
		return
	}

	result := s.getChannelStatus()
	c.JSON(http.StatusOK, result)
}
