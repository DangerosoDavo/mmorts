package network

import "encoding/json"

// Message types - Client → Server
const (
	MsgTypeJoin  = "join"
	MsgTypeLeave = "leave"
	MsgTypeChat  = "chat"
	MsgTypePing  = "ping"
)

// Message types - Server → Client
const (
	MsgTypeWelcome       = "welcome"
	MsgTypePlayerJoined  = "player_joined"
	MsgTypePlayerLeft    = "player_left"
	MsgTypeChatBroadcast = "chat"
	MsgTypeSessionStatus = "session_status"
	MsgTypeError         = "error"
	MsgTypePong          = "pong"
)

// ClientMessage represents any message from client to server
type ClientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ServerMessage represents any message from server to client
type ServerMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// --- Client Message Payloads ---

// JoinPayload is sent by client to join the session
type JoinPayload struct {
	// Currently empty - join happens automatically after auth
	// Future: could include empire selection, spawn preferences, etc.
}

// ChatPayload is sent by client to send a chat message
type ChatPayload struct {
	Message string `json:"message"`
}

// --- Server Message Payloads ---

// WelcomePayload is sent to client after successful connection
type WelcomePayload struct {
	PlayerID      string        `json:"player_id"`
	Username      string        `json:"username"`
	SessionID     string        `json:"session_id"`
	SessionStatus SessionStatus `json:"session_status"`
}

// PlayerJoinedPayload notifies clients when a player joins
type PlayerJoinedPayload struct {
	PlayerID string `json:"player_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// PlayerLeftPayload notifies clients when a player leaves
type PlayerLeftPayload struct {
	PlayerID string `json:"player_id"`
	Username string `json:"username"`
}

// ChatBroadcastPayload broadcasts a chat message to all clients
type ChatBroadcastPayload struct {
	PlayerID  string `json:"player_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"` // Unix timestamp
}

// SessionStatus represents the current session state
type SessionStatus struct {
	State       string `json:"state"`
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	ServerTick  int64  `json:"server_tick"`
	Uptime      int64  `json:"uptime"`
}

// ErrorPayload contains error information
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
