package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/gravitas-games/mmorts/internal/config"
)

// Server represents the game server
type Server struct {
	config      *config.Config
	session     *Session
	mu          sync.RWMutex
	upgrader    websocket.Upgrader
	httpSrv     *http.Server
	jwtValidator *JWTValidator
	redis       *redis.Client

	// Connection tracking
	connections map[*Connection]bool
	connMu      sync.RWMutex

	// Shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	log.Println("Initializing server...")

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Address,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}
	log.Println("Connected to Redis")

	srv := &Server{
		config:      cfg,
		connections: make(map[*Connection]bool),
		ctx:         ctx,
		cancel:      cancel,
		redis:       redisClient,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Add proper origin checking in production
				return true
			},
		},
	}

	// Initialize JWT validator
	jwtValidator, err := NewJWTValidator(cfg, redisClient)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize JWT validator: %w", err)
	}
	srv.jwtValidator = jwtValidator

	// Initialize session
	session, err := NewSession("main", cfg)
	if err != nil {
		cancel()
		return nil, err
	}
	srv.session = session

	log.Println("Server initialized successfully")
	return srv, nil
}

// Start begins listening for connections
func (s *Server) Start(addr string) error {
	log.Printf("Starting WebSocket server on %s", addr)

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	// Create HTTP server
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server
	log.Printf("WebSocket endpoint: ws://%s/ws", addr)
	log.Printf("Health endpoint: http://%s/health", addr)

	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	log.Println("Shutting down server...")

	// Cancel context to signal shutdown
	s.cancel()

	// Shutdown HTTP server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.httpSrv != nil {
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}

	// Close all WebSocket connections
	s.connMu.Lock()
	for conn := range s.connections {
		conn.Close()
	}
	s.connMu.Unlock()

	// Close Redis connection
	if s.redis != nil {
		if err := s.redis.Close(); err != nil {
			log.Printf("Redis close error: %v", err)
		}
	}

	// TODO: Stop session gracefully

	log.Println("Server shutdown complete")
	return nil
}

// handleWebSocket handles WebSocket connection requests
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("New WebSocket connection request from %s", r.RemoteAddr)

	// Extract JWT token from header
	tokenString := extractTokenFromHeader(r)
	if tokenString == "" {
		log.Printf("Missing JWT token from %s", r.RemoteAddr)
		http.Error(w, "Missing authentication token", http.StatusUnauthorized)
		return
	}

	// Validate JWT token
	player, err := s.jwtValidator.ValidateToken(tokenString)
	if err != nil {
		log.Printf("Invalid JWT token from %s: %v", r.RemoteAddr, err)
		http.Error(w, fmt.Sprintf("Invalid token: %v", err), http.StatusUnauthorized)
		return
	}

	log.Printf("Authenticated user: %s (%s) from %s", player.Username, player.ID, r.RemoteAddr)

	// Upgrade HTTP connection to WebSocket
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// Create connection with authenticated player
	conn := NewConnection(ws, s)
	conn.player = player
	conn.authenticated = true

	// Register connection
	s.connMu.Lock()
	s.connections[conn] = true
	s.connMu.Unlock()

	log.Printf("WebSocket connection established: %s (%s)", player.Username, r.RemoteAddr)

	// Handle connection (blocking)
	conn.Handle()

	// Unregister connection when done
	s.connMu.Lock()
	delete(s.connections, conn)
	s.connMu.Unlock()

	log.Printf("WebSocket connection closed: %s (%s)", player.Username, r.RemoteAddr)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
