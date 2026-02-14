// Package protocol defines the WebSocket RPC protocol used by the gateway.
// This matches the TypeScript version's protocol for compatibility with
// existing macOS/iOS/Android clients.
package protocol

import "encoding/json"

// RPCRequest represents an incoming JSON-RPC style request.
type RPCRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// RPCResponse represents an outgoing JSON-RPC response.
type RPCResponse struct {
	ID     string    `json:"id"`
	Result any       `json:"result,omitempty"`
	Error  *RPCError `json:"error,omitempty"`
}

// RPCError represents an RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RPCEvent represents a server-sent event to clients.
type RPCEvent struct {
	Event   string `json:"event"`
	Payload any    `json:"payload,omitempty"`
}

// Standard error codes.
const (
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

// ConnectParams is sent by clients when establishing a connection.
type ConnectParams struct {
	Role       string     `json:"role"` // "agent", "app", "cli", "node"
	Token      string     `json:"token,omitempty"`
	ClientInfo ClientInfo `json:"clientInfo"`
}

// ClientInfo describes the connecting client.
type ClientInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform"` // "macos", "ios", "android", "linux", "windows"
	NodeID   string `json:"nodeId,omitempty"`
}

// SessionInfo describes a session returned by sessions.list.
type SessionInfo struct {
	Key            string `json:"key"`
	Channel        string `json:"channel"`
	AgentID        string `json:"agentId,omitempty"`
	Model          string `json:"model,omitempty"`
	ThinkingLevel  string `json:"thinkingLevel,omitempty"`
	MessageCount   int    `json:"messageCount"`
	LastActivityAt int64  `json:"lastActivityAt"`
}

// ChatMessage represents a chat message in a session.
type ChatMessage struct {
	Role      string `json:"role"` // "user", "assistant", "system"
	Content   string `json:"content"`
	Channel   string `json:"channel,omitempty"`
	Sender    string `json:"sender,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// --- Gateway Events ---

const (
	EventChatMessage      = "chat.message"
	EventChatStream       = "chat.stream"
	EventChatToolUse      = "chat.toolUse"
	EventChatToolResult   = "chat.toolResult"
	EventSessionCreated   = "session.created"
	EventSessionUpdated   = "session.updated"
	EventPresenceChanged  = "presence.changed"
	EventChannelStatus    = "channel.status"
	EventNodeConnected    = "node.connected"
	EventNodeDisconnected = "node.disconnected"
)
