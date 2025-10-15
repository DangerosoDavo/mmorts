package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitas-games/mmorts/internal/network"
	"github.com/gravitas-games/mmorts/pkg/models"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192
)

// Connection represents a WebSocket connection to a client
type Connection struct {
	// WebSocket connection
	ws *websocket.Conn

	// Server reference
	server *Server

	// Player information (set after authentication)
	player *models.Player

	// Buffered channel for outbound messages
	send chan []byte

	// Is connection authenticated
	authenticated bool
}

// NewConnection creates a new connection
func NewConnection(ws *websocket.Conn, server *Server) *Connection {
	return &Connection{
		ws:            ws,
		server:        server,
		send:          make(chan []byte, 256),
		authenticated: false,
	}
}

// Handle manages the connection lifecycle
func (c *Connection) Handle() {
	// Set up connection parameters
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start read and write pumps
	go c.writePump()
	c.readPump() // Blocking
}

// readPump pumps messages from the WebSocket connection to the server
func (c *Connection) readPump() {
	defer func() {
		c.Close()
	}()

	for {
		// Read message
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Parse message
		var clientMsg network.ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Failed to parse client message: %v", err)
			c.SendError("invalid_message", "Failed to parse message")
			continue
		}

		// Handle message based on type
		c.handleMessage(&clientMsg)
	}
}

// writePump pumps messages from the send channel to the WebSocket connection
func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write message
			if err := c.ws.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			// Send ping
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.server.ctx.Done():
			// Server shutting down
			return
		}
	}
}

// handleMessage routes messages to appropriate handlers
func (c *Connection) handleMessage(msg *network.ClientMessage) {
	log.Printf("Received message type: %s", msg.Type)

	switch msg.Type {
	case network.MsgTypeJoin:
		c.handleJoin(msg.Payload)

	case network.MsgTypeLeave:
		c.handleLeave()

	case network.MsgTypeChat:
		c.handleChat(msg.Payload)

	case network.MsgTypePing:
		c.handlePing()

	default:
		log.Printf("Unknown message type: %s", msg.Type)
		c.SendError("unknown_message_type", "Unknown message type")
	}
}

// handleJoin handles player join requests
func (c *Connection) handleJoin(payload json.RawMessage) {
	log.Printf("Player join request from %s", c.player.Username)

	// Verify player is authenticated (should always be true now)
	if !c.authenticated || c.player == nil {
		c.SendError("not_authenticated", "Connection not authenticated")
		return
	}

	// Update player connection state
	c.player.Connected = true
	c.player.ConnectedAt = time.Now()
	c.player.SessionID = c.server.session.ID

	// Add player to session
	if err := c.server.session.AddPlayer(c.player, c); err != nil {
		log.Printf("Failed to add player to session: %v", err)
		c.SendError("join_failed", "Failed to join session")
		return
	}

	// Send welcome message
	welcome := network.ServerMessage{
		Type: network.MsgTypeWelcome,
		Payload: network.WelcomePayload{
			PlayerID:  c.player.ID,
			Username:  c.player.Username,
			SessionID: c.server.session.ID,
			SessionStatus: network.SessionStatus{
				State:       c.server.session.status.State,
				PlayerCount: c.server.session.status.PlayerCount,
				MaxPlayers:  c.server.session.status.MaxPlayers,
				ServerTick:  c.server.session.status.ServerTick,
				Uptime:      c.server.session.status.Uptime,
			},
		},
	}

	c.SendMessage(&welcome)

	// Broadcast player joined to all other players
	c.server.session.BroadcastExcept(c, &network.ServerMessage{
		Type: network.MsgTypePlayerJoined,
		Payload: network.PlayerJoinedPayload{
			PlayerID: c.player.ID,
			Username: c.player.Username,
			Email:    c.player.Email,
		},
	})

	log.Printf("Player %s joined session %s", c.player.Username, c.server.session.ID)
}

// handleLeave handles player leave requests
func (c *Connection) handleLeave() {
	if c.player != nil {
		c.server.session.RemovePlayer(c.player.ID)

		// Broadcast player left
		c.server.session.BroadcastMessage(&network.ServerMessage{
			Type: network.MsgTypePlayerLeft,
			Payload: network.PlayerLeftPayload{
				PlayerID: c.player.ID,
				Username: c.player.Username,
			},
		})
	}
}

// handleChat handles chat messages
func (c *Connection) handleChat(payload json.RawMessage) {
	if !c.authenticated || c.player == nil {
		c.SendError("not_authenticated", "Must be authenticated to chat")
		return
	}

	// Parse chat payload
	var chatMsg network.ChatPayload
	if err := json.Unmarshal(payload, &chatMsg); err != nil {
		log.Printf("Failed to parse chat payload: %v", err)
		c.SendError("invalid_chat", "Invalid chat message")
		return
	}

	// TODO: Add rate limiting
	// TODO: Add message length validation
	// TODO: Add profanity filter

	// Broadcast chat message to all players
	broadcast := &network.ServerMessage{
		Type: network.MsgTypeChatBroadcast,
		Payload: network.ChatBroadcastPayload{
			PlayerID:  c.player.ID,
			Username:  c.player.Username,
			Message:   chatMsg.Message,
			Timestamp: time.Now().Unix(),
		},
	}

	c.server.session.BroadcastMessage(broadcast)
	log.Printf("Chat from %s: %s", c.player.Username, chatMsg.Message)
}

// handlePing handles ping requests
func (c *Connection) handlePing() {
	c.SendMessage(&network.ServerMessage{
		Type:    network.MsgTypePong,
		Payload: map[string]interface{}{"timestamp": time.Now().Unix()},
	})
}

// SendMessage sends a message to the client
func (c *Connection) SendMessage(msg *network.ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		log.Printf("Send buffer full, dropping message")
	}
}

// SendError sends an error message to the client
func (c *Connection) SendError(code, message string) {
	c.SendMessage(&network.ServerMessage{
		Type: network.MsgTypeError,
		Payload: network.ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}

// Close closes the connection
func (c *Connection) Close() {
	// Remove player from session if authenticated
	if c.authenticated && c.player != nil {
		c.handleLeave()
	}

	// Close send channel
	close(c.send)

	// Close WebSocket connection
	c.ws.Close()
}
