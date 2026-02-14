// Package gateway implements the HighClaw gateway server.
// The gateway is the central control plane that manages WebSocket connections,
// HTTP API, sessions, channels, and agent communication.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/gateway/protocol"
	"github.com/highclaw/highclaw/internal/gateway/session"
)

// Server is the main gateway server that handles WS and HTTP connections.
type Server struct {
	cfg      *config.Config
	logger   *slog.Logger
	listener net.Listener

	httpServer *http.Server
	upgrader   websocket.Upgrader

	sessions *session.Manager
	clients  map[string]*Client
	mu       sync.RWMutex

	// Shutdown coordination.
	ctx    context.Context
	cancel context.CancelFunc
}

// Client represents a connected WebSocket client.
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Role   string // "agent", "app", "cli", "node"
	Info   protocol.ClientInfo
	SendCh chan []byte
	done   chan struct{}
}

// NewServer creates a new gateway server instance.
func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		cfg:    cfg,
		logger: logger.With("component", "gateway"),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true // TODO: implement proper origin check
			},
		},
		sessions: session.NewManager(),
		clients:  make(map[string]*Client),
		ctx:      ctx,
		cancel:   cancel,
	}

	return s, nil
}

// Start begins listening for incoming connections.
func (s *Server) Start() error {
	host := "127.0.0.1"
	if s.cfg.Gateway.Bind == "all" {
		host = "0.0.0.0"
	}

	addr := fmt.Sprintf("%s:%d", host, s.cfg.Gateway.Port)

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	mux := http.NewServeMux()

	// WebSocket endpoint.
	mux.HandleFunc("/", s.handleWebSocket)

	// HTTP API endpoints.
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/status", s.handleStatus)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()

	return nil
}

// Address returns the address the server is listening on.
func (s *Server) Address() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	s.cancel()

	// Close all client connections.
	s.mu.Lock()
	for id, client := range s.clients {
		s.logger.Debug("closing client", "id", id)
		close(client.done)
		client.Conn.Close()
	}
	s.mu.Unlock()

	// Shutdown HTTP server.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// handleWebSocket upgrades HTTP connections to WebSocket and manages RPC.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket upgrade request.
	if !websocket.IsWebSocketUpgrade(r) {
		// Serve Control UI for non-WS requests.
		s.handleControlUI(w, r)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
	client := &Client{
		ID:     clientID,
		Conn:   conn,
		SendCh: make(chan []byte, 256),
		done:   make(chan struct{}),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.logger.Info("client connected", "id", clientID, "remote", conn.RemoteAddr())

	// Start read and write pumps.
	go s.writePump(client)
	go s.readPump(client)
}

// readPump reads messages from a WebSocket client.
func (s *Server) readPump(client *Client) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, client.ID)
		s.mu.Unlock()
		client.Conn.Close()
		s.logger.Info("client disconnected", "id", client.ID)
	}()

	client.Conn.SetReadLimit(1 << 20) // 1 MB max message.
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Warn("websocket read error", "id", client.ID, "error", err)
			}
			return
		}

		s.handleMessage(client, message)
	}
}

// writePump sends messages to a WebSocket client.
func (s *Server) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.SendCh:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				s.logger.Warn("websocket write error", "id", client.ID, "error", err)
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-client.done:
			return

		case <-s.ctx.Done():
			return
		}
	}
}

// handleMessage handles an incoming RPC message from a client.
func (s *Server) handleMessage(client *Client, data []byte) {
	var msg protocol.RPCRequest
	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Warn("invalid RPC message", "id", client.ID, "error", err)
		s.sendError(client, "", protocol.ErrInvalidRequest, "invalid JSON")
		return
	}

	s.logger.Debug("RPC request", "id", client.ID, "method", msg.Method, "reqId", msg.ID)

	// Route to method handler.
	result, err := s.dispatchMethod(client, &msg)
	if err != nil {
		s.sendError(client, msg.ID, protocol.ErrMethodNotFound, err.Error())
		return
	}

	// Send response.
	resp := protocol.RPCResponse{
		ID:     msg.ID,
		Result: result,
	}

	respData, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error("marshal response failed", "error", err)
		return
	}

	select {
	case client.SendCh <- respData:
	default:
		s.logger.Warn("client send buffer full, dropping message", "id", client.ID)
	}
}

// sendError sends an RPC error response.
func (s *Server) sendError(client *Client, reqID string, code int, message string) {
	resp := protocol.RPCResponse{
		ID: reqID,
		Error: &protocol.RPCError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(resp)
	select {
	case client.SendCh <- data:
	default:
	}
}

// dispatchMethod routes an RPC request to the appropriate handler.
func (s *Server) dispatchMethod(client *Client, req *protocol.RPCRequest) (any, error) {
	switch req.Method {
	case "connect":
		return s.methodConnect(client, req)
	case "health":
		return s.methodHealth(client, req)
	case "sessions.list":
		return s.methodSessionsList(client, req)
	case "sessions.get":
		return s.methodSessionsGet(client, req)
	case "sessions.create":
		return s.methodSessionsCreate(client, req)
	case "sessions.delete":
		return s.methodSessionsDelete(client, req)
	case "sessions.reset":
		return s.methodSessionsReset(client, req)
	case "sessions.patch":
		return s.methodSessionsPatch(client, req)
	case "chat.send":
		return s.methodChatSend(client, req)
	case "config.get":
		return s.methodConfigGet(client, req)
	case "config.patch":
		return s.methodConfigPatch(client, req)
	case "channels.status":
		return s.methodChannelsStatus(client, req)
	case "agents.list":
		return s.methodAgentsList(client, req)
	case "models.list":
		return s.methodModelsList(client, req)
	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}

// --- RPC Method Implementations ---

func (s *Server) methodConnect(client *Client, req *protocol.RPCRequest) (any, error) {
	var params protocol.ConnectParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid connect params: %w", err)
	}

	client.Role = params.Role
	client.Info = params.ClientInfo

	s.logger.Info("client identified",
		"id", client.ID,
		"role", params.Role,
		"name", params.ClientInfo.Name,
	)

	return map[string]any{
		"ok":      true,
		"version": "go-dev",
	}, nil
}

func (s *Server) methodHealth(client *Client, req *protocol.RPCRequest) (any, error) {
	return map[string]any{
		"status":   "ok",
		"version":  "go-dev",
		"uptime":   time.Since(time.Now()).Seconds(), // TODO: track real uptime
		"clients":  len(s.clients),
		"sessions": s.sessions.Count(),
	}, nil
}

func (s *Server) methodSessionsList(client *Client, req *protocol.RPCRequest) (any, error) {
	return s.sessions.List(), nil
}

func (s *Server) methodSessionsGet(client *Client, req *protocol.RPCRequest) (any, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}
	sess, ok := s.sessions.Get(params.Key)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", params.Key)
	}
	return sess, nil
}

func (s *Server) methodChatSend(client *Client, req *protocol.RPCRequest) (any, error) {
	var params struct {
		SessionKey string `json:"sessionKey"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}

	s.logger.Info("chat.send",
		"session", params.SessionKey,
		"message_len", len(params.Message),
	)

	// TODO: Route to agent runtime.
	return map[string]any{
		"ok":     true,
		"queued": true,
	}, nil
}

func (s *Server) methodConfigGet(client *Client, req *protocol.RPCRequest) (any, error) {
	return s.cfg, nil
}

func (s *Server) methodSessionsCreate(client *Client, req *protocol.RPCRequest) (any, error) {
	var params struct {
		Key     string `json:"key"`
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}

	sess := s.sessions.GetOrCreate(params.Key, params.Channel)
	return sess, nil
}

func (s *Server) methodSessionsDelete(client *Client, req *protocol.RPCRequest) (any, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}

	s.sessions.Delete(params.Key)
	return map[string]any{"ok": true}, nil
}

func (s *Server) methodSessionsReset(client *Client, req *protocol.RPCRequest) (any, error) {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}

	sess, ok := s.sessions.Get(params.Key)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", params.Key)
	}

	sess.Reset()
	return map[string]any{"ok": true}, nil
}

func (s *Server) methodSessionsPatch(client *Client, req *protocol.RPCRequest) (any, error) {
	var params struct {
		Key           string `json:"key"`
		Model         string `json:"model,omitempty"`
		ThinkingLevel string `json:"thinkingLevel,omitempty"`
		VerboseLevel  string `json:"verboseLevel,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, err
	}

	sess, ok := s.sessions.Get(params.Key)
	if !ok {
		return nil, fmt.Errorf("session not found: %s", params.Key)
	}

	// Update session fields
	if params.Model != "" {
		sess.Model = params.Model
	}
	if params.ThinkingLevel != "" {
		sess.ThinkingLevel = params.ThinkingLevel
	}
	if params.VerboseLevel != "" {
		sess.VerboseLevel = params.VerboseLevel
	}

	return sess, nil
}

func (s *Server) methodConfigPatch(client *Client, req *protocol.RPCRequest) (any, error) {
	// TODO: Implement config patching with validation and persistence
	return map[string]any{"ok": true, "message": "config.patch not yet implemented"}, nil
}

func (s *Server) methodChannelsStatus(client *Client, req *protocol.RPCRequest) (any, error) {
	// TODO: Query channel registry for status
	return map[string]any{
		"telegram": map[string]any{"connected": false},
		"whatsapp": map[string]any{"connected": false},
		"discord":  map[string]any{"connected": false},
	}, nil
}

func (s *Server) methodAgentsList(client *Client, req *protocol.RPCRequest) (any, error) {
	// TODO: Return list of configured agents
	return []map[string]any{
		{"id": "main", "name": "Main Agent", "model": s.cfg.Agent.Model},
	}, nil
}

func (s *Server) methodModelsList(client *Client, req *protocol.RPCRequest) (any, error) {
	// TODO: Query available models from providers
	return []map[string]any{
		{"id": "anthropic/claude-opus-4", "name": "Claude Opus 4", "provider": "anthropic"},
		{"id": "anthropic/claude-sonnet-4", "name": "Claude Sonnet 4", "provider": "anthropic"},
		{"id": "openai/gpt-4o", "name": "GPT-4o", "provider": "openai"},
	}, nil
}

// --- HTTP Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"version": "go-dev",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"version":  "go-dev",
		"clients":  clientCount,
		"sessions": s.sessions.Count(),
	})
}

func (s *Server) handleControlUI(w http.ResponseWriter, r *http.Request) {
	// TODO: Serve embedded Control UI static files.
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>HighClaw Gateway</title></head>
<body>
<h1>🦀 HighClaw Gateway</h1>
<p>Gateway is running. Control UI coming soon.</p>
</body>
</html>`)
}

// Broadcast sends a message to all connected clients.
func (s *Server) Broadcast(event string, payload any) {
	data, err := json.Marshal(protocol.RPCEvent{
		Event:   event,
		Payload: payload,
	})
	if err != nil {
		s.logger.Error("broadcast marshal failed", "error", err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		select {
		case client.SendCh <- data:
		default:
			s.logger.Warn("broadcast: client send buffer full", "id", client.ID)
		}
	}
}
