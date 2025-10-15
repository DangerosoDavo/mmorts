package models

import "time"

// Player represents a player in the game
type Player struct {
	// From JWT claims
	ID          string `json:"id"`          // Converted from int64 user_id
	Username    string `json:"username"`    // JWT claim
	Email       string `json:"email"`       // JWT claim
	UserType    string `json:"user_type"`   // JWT claim (deprecated, use permissions)
	Permissions int64  `json:"permissions"` // JWT claim: bitwise permission flags
	Activated   int64  `json:"activated"`   // JWT claim: activation timestamp or ban status
	AuthMethod  string `json:"auth_method"` // JWT claim: "password" or "oauth"

	// Connection state
	Connected   bool      `json:"connected"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`

	// Session state
	SessionID string `json:"session_id"`

	// Game-specific (not from JWT)
	// Empire ID will be assigned by game server or loaded from database
	EmpireID string `json:"empire_id,omitempty"`
}

// IsActive checks if the player account is activated and not banned
func (p *Player) IsActive() bool {
	// activated > 0 means activated
	// activated == 0 means not activated
	// activated == -1 means banned
	return p.Activated > 0
}

// IsBanned checks if the player is banned
func (p *Player) IsBanned() bool {
	return p.Activated == -1
}

// IsConnected checks if the player is currently connected
func (p *Player) IsConnected() bool {
	return p.Connected
}
