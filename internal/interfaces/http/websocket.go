package http

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/highclaw/highclaw/internal/agent"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
)

// WSClient represents a WebSocket client.
type WSClient struct {
	conn     *websocket.Conn
	send     chan []byte
	server   *Server
	id       string
	role     string // "agent", "app", "cli", "tui"
	mu       sync.Mutex
	lastPing time.Time
}

// WSMessage represents a WebSocket message (JSON-RPC style).
type WSMessage struct {
	Type   string          `json:"type"`   // "request", "response", "event"
	ID     string          `json:"id"`     // Request ID
	Method string          `json:"method"` // RPC method
	Params json.RawMessage `json:"params"` // Parameters
	Result json.RawMessage `json:"result"` // Result
	Error  *WSError        `json:"error"`  // Error
}

// WSError represents a WebSocket error.
type WSError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	clients   = make(map[string]*WSClient)
	clientsMu sync.RWMutex
)

// handleWebSocket handles WebSocket connections.
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &WSClient{
		conn:     conn,
		send:     make(chan []byte, 256),
		server:   s,
		id:       generateClientID(),
		lastPing: time.Now(),
	}

	// Register client
	clientsMu.Lock()
	clients[client.id] = client
	clientsMu.Unlock()

	s.logger.Info("websocket client connected", "id", client.id)

	// Start goroutines
	go client.readPump()
	go client.writePump()
}

// readPump reads messages from the WebSocket.
func (c *WSClient) readPump() {
	defer func() {
		c.cleanup()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.lastPing = time.Now()
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.server.logger.Error("websocket read error", "error", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// writePump writes messages to the WebSocket.
func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming WebSocket messages.
func (c *WSClient) handleMessage(data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError("", 400, "invalid JSON")
		return
	}

	// Handle RPC methods
	switch msg.Method {
	case "connect":
		c.handleConnect(msg)
	case "ping":
		c.handlePing(msg)
	case "chat.send":
		c.handleChatSend(msg)
	case "sessions.list":
		c.handleSessionsList(msg)
	default:
		c.sendError(msg.ID, 404, "unknown method: "+msg.Method)
	}
}

// handleConnect handles client connection.
func (c *WSClient) handleConnect(msg WSMessage) {
	var params struct {
		Role string `json:"role"`
	}
	json.Unmarshal(msg.Params, &params)

	c.role = params.Role
	c.sendResponse(msg.ID, map[string]any{
		"clientId": c.id,
		"role":     c.role,
		"status":   "connected",
	})
}

// handlePing handles ping requests.
func (c *WSClient) handlePing(msg WSMessage) {
	c.sendResponse(msg.ID, map[string]any{"pong": true})
}

// handleChatSend handles chat messages.
func (c *WSClient) handleChatSend(msg WSMessage) {
	var params struct {
		Message string `json:"message"`
		Session string `json:"session"`
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.sendError(msg.ID, 400, "invalid params: "+err.Error())
		return
	}

	if params.Message == "" {
		c.sendError(msg.ID, 400, "message is required")
		return
	}

	sessionKey := params.Session
	if sessionKey == "" {
		sessionKey = "main"
	}
	channel := params.Channel
	if channel == "" {
		channel = "websocket"
	}

	// Store user message in session
	if c.server.sessions != nil {
		sess := c.server.sessions.GetOrCreate(sessionKey, channel)
		sess.AddMessage(protocol.ChatMessage{
			Role:    "user",
			Content: params.Message,
			Channel: channel,
		})
	}

	// Call agent
	if c.server.agent == nil {
		c.sendError(msg.ID, 503, "agent not available")
		return
	}

	// Send typing event
	c.sendEvent("chat.typing", map[string]any{"session": sessionKey, "typing": true})

	result, err := c.server.agent.Run(context.Background(), &agent.RunRequest{
		SessionKey: sessionKey,
		Channel:    channel,
		Message:    params.Message,
	})
	if err != nil {
		c.server.logger.Error("agent run failed via websocket", "error", err)
		c.sendError(msg.ID, 500, "agent error: "+err.Error())
		return
	}

	// Store assistant response in session
	if c.server.sessions != nil {
		if sess, ok := c.server.sessions.Get(sessionKey); ok {
			sess.AddMessage(protocol.ChatMessage{
				Role:    "assistant",
				Content: result.Reply,
				Channel: channel,
			})
		}
	}

	// Send typing done event
	c.sendEvent("chat.typing", map[string]any{"session": sessionKey, "typing": false})

	c.sendResponse(msg.ID, map[string]any{
		"response": result.Reply,
		"usage":    result.TokensUsed,
		"session":  sessionKey,
	})
}

// handleSessionsList handles session list requests.
func (c *WSClient) handleSessionsList(msg WSMessage) {
	if c.server.sessions == nil {
		c.sendResponse(msg.ID, map[string]any{"sessions": []any{}})
		return
	}

	sessions := c.server.sessions.List()
	c.sendResponse(msg.ID, map[string]any{"sessions": sessions})
}

// sendResponse sends a response message.
func (c *WSClient) sendResponse(id string, result any) {
	data, _ := json.Marshal(result)
	msg := WSMessage{
		Type:   "response",
		ID:     id,
		Result: data,
	}
	c.sendMessage(msg)
}

// sendEvent sends an event message to the client.
func (c *WSClient) sendEvent(method string, data any) {
	result, _ := json.Marshal(data)
	msg := WSMessage{
		Type:   "event",
		Method: method,
		Result: result,
	}
	c.sendMessage(msg)
}

// sendError sends an error message.
func (c *WSClient) sendError(id string, code int, message string) {
	msg := WSMessage{
		Type: "response",
		ID:   id,
		Error: &WSError{
			Code:    code,
			Message: message,
		},
	}
	c.sendMessage(msg)
}

// sendMessage sends a message to the client.
func (c *WSClient) sendMessage(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case c.send <- data:
	default:
		c.cleanup()
	}
}

// cleanup cleans up the client.
func (c *WSClient) cleanup() {
	clientsMu.Lock()
	delete(clients, c.id)
	clientsMu.Unlock()

	close(c.send)
	c.server.logger.Info("websocket client disconnected", "id", c.id)
}

// generateClientID generates a unique client ID.
func generateClientID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}
