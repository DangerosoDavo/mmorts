package server

import (
	"log"
	"sync"
	"time"

	"github.com/gravitas-games/mmorts/internal/config"
	"github.com/gravitas-games/mmorts/internal/gamemap"
	"github.com/gravitas-games/mmorts/internal/network"
	"github.com/gravitas-games/mmorts/pkg/models"
)

// Session represents a game session
type Session struct {
	ID        string
	CreatedAt time.Time

	// Player management
	players     map[string]*models.Player // playerID -> Player
	connections map[string]*Connection    // playerID -> Connection
	mu          sync.RWMutex

	// Game state (minimal for Phase 1)
	gameMap *gamemap.GameMap
	status  SessionStatus

	// Broadcasting
	broadcast chan []byte

	// Configuration
	config *config.Config
}

// SessionStatus represents the current state of the session
type SessionStatus struct {
	State       string `json:"state"`        // "waiting", "running", "paused"
	PlayerCount int    `json:"player_count"`
	MaxPlayers  int    `json:"max_players"`
	ServerTick  int64  `json:"server_tick"`
	Uptime      int64  `json:"uptime"` // seconds
}

// NewSession creates a new game session
func NewSession(id string, cfg *config.Config) (*Session, error) {
	log.Printf("Creating session: %s", id)

	// Initialize game map
	gameMap, err := gamemap.New(cfg.Session.InitialMapRadius)
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:          id,
		CreatedAt:   time.Now(),
		players:     make(map[string]*models.Player),
		connections: make(map[string]*Connection),
		gameMap:     gameMap,
		broadcast:   make(chan []byte, 256),
		config:      cfg,
		status: SessionStatus{
			State:      "waiting",
			MaxPlayers: cfg.Session.MaxPlayers,
		},
	}

	log.Printf("Session %s created with map radius %d", id, cfg.Session.InitialMapRadius)
	return session, nil
}

// AddPlayer adds a player to the session
func (s *Session) AddPlayer(player *models.Player, conn *Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.players[player.ID] = player
	s.connections[player.ID] = conn
	s.status.PlayerCount = len(s.players)

	log.Printf("Player %s (%s) joined session %s", player.Username, player.ID, s.ID)
	return nil
}

// RemovePlayer removes a player from the session
func (s *Session) RemovePlayer(playerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if player, exists := s.players[playerID]; exists {
		log.Printf("Player %s (%s) left session %s", player.Username, playerID, s.ID)
		delete(s.players, playerID)
		delete(s.connections, playerID)
		s.status.PlayerCount = len(s.players)
	}
}

// GetPlayer retrieves a player by ID
func (s *Session) GetPlayer(playerID string) (*models.Player, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	player, exists := s.players[playerID]
	return player, exists
}

// GetPlayers returns all players in the session
func (s *Session) GetPlayers() []*models.Player {
	s.mu.RLock()
	defer s.mu.RUnlock()

	players := make([]*models.Player, 0, len(s.players))
	for _, player := range s.players {
		players = append(players, player)
	}
	return players
}

// BroadcastMessage sends a message to all connected players
func (s *Session) BroadcastMessage(msg interface{}) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if serverMsg, ok := msg.(*network.ServerMessage); ok {
			conn.SendMessage(serverMsg)
		}
	}
}

// BroadcastExcept sends a message to all players except the specified connection
func (s *Session) BroadcastExcept(exclude *Connection, msg *network.ServerMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, conn := range s.connections {
		if conn != exclude {
			conn.SendMessage(msg)
		}
	}
}

// GetStatus returns the current session status
func (s *Session) GetStatus() SessionStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.status
	status.Uptime = int64(time.Since(s.CreatedAt).Seconds())
	return status
}
