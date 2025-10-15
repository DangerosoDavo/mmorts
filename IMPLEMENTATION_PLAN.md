# MMORTS Backend - Phase 1 Implementation Plan

## Overview

This document outlines the step-by-step implementation plan for Phase 1 of the MMORTS backend. The goal is to establish a minimal viable product (MVP) with basic connectivity, player tracking, session management, and a simple global chat system for testing.

## Phase 1 Goals

✅ **Basic Infrastructure**: Project structure, dependencies, configuration
✅ **Connection Handling**: WebSocket server with JWT authentication
✅ **Player Management**: Track connected players, handle join/leave
✅ **Session Structure**: Basic game session with player list and status
✅ **Map Foundation**: Initial GameMap with blank hex chunks
✅ **Global Chat**: Simple chat system for testing client-server communication
✅ **Development Setup**: Docker configuration for easy local development

## Implementation Steps

### Step 1: Project Structure Setup

**Goal**: Create the directory structure and initialize Go modules

**Actions**:
1. Create directory structure following [ARCHITECTURE.md](ARCHITECTURE.md) specification
2. Initialize `go.mod` with module name `github.com/gravitas-games/mmorts`
3. Create placeholder files for main packages

**Files to Create**:
```
mmorts/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── server/
│   │   ├── server.go
│   │   ├── session.go
│   │   └── auth.go
│   ├── gamemap/
│   │   ├── map.go
│   │   └── chunk.go
│   ├── network/
│   │   ├── handlers.go
│   │   └── protocol.go
│   └── config/
│       └── config.go
├── pkg/
│   └── models/
│       ├── player.go
│       └── session.go
├── configs/
│   └── server.yaml
├── go.mod
└── go.sum
```

**Validation**: Run `go mod tidy` successfully

---

### Step 2: Dependencies & External Packages

**Goal**: Import all required external packages and verify they compile

**Actions**:
1. Add dependencies to `go.mod`:
   - `github.com/gorilla/websocket` (WebSocket support)
   - `github.com/golang-jwt/jwt/v5` (JWT validation)
   - `github.com/go-redis/redis/v8` (Redis for JWT blacklist)
   - `gopkg.in/yaml.v3` (YAML config parsing)
2. Import external packages:
   - `github.com/gravitas-games/udp_network`
   - `github.com/gravitas-games/ecscore`
   - `github.com/gravitas-games/commandcore`
   - `github.com/gravitas-games/cache`
   - `github.com/gravitas-games/hexcore`
   - `github.com/gravitas-games/inventory`
   - `github.com/gravitas-games/production`

**Files to Modify**:
- `go.mod` (add replace directives for local packages)
- `cmd/server/main.go` (basic imports to verify)

**Validation**: Run `go build ./cmd/server` successfully

---

### Step 3: Configuration System

**Goal**: Implement YAML-based configuration for server settings

**Actions**:
1. Define `Config` struct in `internal/config/config.go`
2. Add configuration loading from YAML file
3. Create default `configs/server.yaml` with initial settings

**Configuration Fields**:
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  tick_rate: 20  # Hz

jwt:
  issuer: "login-server"
  public_key_url: "https://login.gravitas-games.com/api/public-key"
  public_key_refresh_hours: 24

redis:
  address: "localhost:6379"
  password: ""
  db: 0
  blacklist_prefix: "jwt:blacklist:"

session:
  max_players: 100
  initial_map_radius: 5  # Number of hex chunks from origin

chat:
  max_message_length: 500
  rate_limit: 10  # messages per minute
```

**Files to Create**:
- `internal/config/config.go`
- `configs/server.yaml`

**Validation**: Load config and print values in `main.go`

---

### Step 4: Basic Server & WebSocket

**Goal**: Create HTTP server with WebSocket upgrade handler

**Actions**:
1. Implement `Server` struct in `internal/server/server.go`
2. Add WebSocket upgrade handler
3. Implement connection lifecycle (accept, upgrade, close)
4. Add graceful shutdown handling

**Key Types**:
```go
type Server struct {
    config      *config.Config
    upgrader    websocket.Upgrader
    sessions    map[string]*Session
    mu          sync.RWMutex

    // Will add later: Redis, DB, etc.
}

type Connection struct {
    conn        *websocket.Conn
    playerID    string
    empireID    string
    send        chan []byte
    server      *Server
}
```

**Files to Create/Modify**:
- `internal/server/server.go`
- `cmd/server/main.go` (instantiate server)

**Validation**: Server starts and accepts WebSocket connections on `ws://localhost:8080/ws`

---

### Step 5: JWT Authentication

**Goal**: Validate JWT tokens from GoLoginServer and check Redis blacklist

**Actions**:
1. Implement JWT validation in `internal/server/auth.go`
2. Add Redis client connection
3. Implement blacklist checking
4. Extract player ID and empire ID from JWT claims

**JWT Claims Structure** (from GoLoginServer):
```go
type Claims struct {
    UserID      int64  `json:"user_id"`       // Unique user identifier
    Email       string `json:"email"`         // User's email address
    Username    string `json:"username"`      // User's username
    UserType    string `json:"user_type"`     // "user", "moderator", "admin", "superadmin" (deprecated)
    AuthMethod  string `json:"auth_method"`   // "password" or "oauth"
    Permissions int64  `json:"permissions"`   // Bitwise permission flags
    Activated   int64  `json:"activated"`     // Nanoseconds since epoch (0 = not activated, -1 = banned)
    jwt.RegisteredClaims                      // iss, iat, exp
}

// Note: UserID is int64, we'll convert to string for internal use
// Activated: 0 = not activated, -1 = banned, >0 = activation timestamp
```

**Authentication Flow**:
1. Client connects with JWT in header: `Sec-WebSocket-Protocol: access_token, <token>`
2. Server extracts token from header
3. Validate token signature with ECDSA P-256 public key (ES256 algorithm)
4. Validate issuer is "login-server"
5. Check token expiration
6. Check if user is activated (activated > 0) and not banned (activated != -1)
7. Check if token is blacklisted in Redis
8. Extract claims (user ID, username, email, permissions)
9. Accept or reject connection

**Public Key Retrieval**:
- Fetch from GoLoginServer at `/api/public-key` endpoint on startup
- Cache in memory for token validation
- Refresh periodically (daily) or on validation failures

**Files to Create/Modify**:
- `internal/server/auth.go`
- `internal/server/server.go` (integrate auth into connection handler)

**Validation**:
- Valid JWT → Connection accepted
- Invalid JWT → Connection rejected
- Blacklisted JWT → Connection rejected

---

### Step 6: Player Model

**Goal**: Define player data structure based on User.js model

**Actions**:
1. Create `Player` struct in `pkg/models/player.go`
2. Add basic fields needed for Phase 1

**Player Structure** (simplified for Phase 1):
```go
type Player struct {
    // From JWT
    ID          string `json:"id"`          // Converted from int64 user_id
    Username    string `json:"username"`
    Email       string `json:"email"`
    Permissions int64  `json:"permissions"` // Bitwise flags
    UserType    string `json:"user_type"`   // For convenience (deprecated in favor of permissions)

    // Connection state
    Connected   bool      `json:"connected"`
    ConnectedAt time.Time `json:"connected_at"`
    LastSeen    time.Time `json:"last_seen"`

    // Session state
    SessionID string `json:"session_id"`

    // Note: No empire_id in JWT - this is game-specific and will be assigned
    // by the game server or loaded from game database in future phases
}
```

**Files to Create**:
- `pkg/models/player.go`

**Validation**: Create and serialize Player struct to JSON

---

### Step 7: Session Management

**Goal**: Implement game session with player tracking

**Actions**:
1. Create `Session` struct in `internal/server/session.go`
2. Implement player join/leave handling
3. Track connected players
4. Add session status reporting

**Session Structure**:
```go
type Session struct {
    ID          string
    CreatedAt   time.Time

    // Player management
    players     map[string]*models.Player  // playerID -> Player
    connections map[string]*Connection     // playerID -> Connection
    mu          sync.RWMutex

    // Game state (minimal for Phase 1)
    gameMap     *gamemap.GameMap
    status      SessionStatus

    // Broadcasting
    broadcast   chan []byte

    // Will add later: ECS world, command queue, etc.
}

type SessionStatus struct {
    State        string `json:"state"`         // "waiting", "running", "paused"
    PlayerCount  int    `json:"player_count"`
    MaxPlayers   int    `json:"max_players"`
    ServerTick   int64  `json:"server_tick"`
    Uptime       int64  `json:"uptime"`
}
```

**Session Methods**:
- `AddPlayer(player *Player, conn *Connection) error`
- `RemovePlayer(playerID string)`
- `BroadcastMessage(msg []byte)`
- `GetStatus() SessionStatus`

**Files to Create/Modify**:
- `internal/server/session.go`
- `pkg/models/session.go`

**Validation**: Players can join and leave session, broadcast works

---

### Step 8: Network Protocol

**Goal**: Define message protocol for client-server communication

**Actions**:
1. Define message types in `internal/network/protocol.go`
2. Implement message serialization/deserialization
3. Create message handlers in `internal/network/handlers.go`

**Message Types** (Phase 1):
```go
// Client → Server
type ClientMessage struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

// Message types:
// - "join"       : Join session (sent after connection established)
// - "leave"      : Leave session
// - "chat"       : Send chat message
// - "ping"       : Keep-alive

// Server → Client
type ServerMessage struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload"`
}

// Message types:
// - "welcome"         : Connection accepted + session info
// - "player_joined"   : Another player joined
// - "player_left"     : Another player left
// - "chat"            : Chat message broadcast
// - "session_status"  : Session status update
// - "error"           : Error message
// - "pong"            : Keep-alive response
```

**Example Messages**:
```json
// Server → Client: Welcome
{
  "type": "welcome",
  "payload": {
    "player_id": "player_abc123",
    "empire_id": "empire_xyz789",
    "session_id": "session_main",
    "session_status": {
      "state": "waiting",
      "player_count": 1,
      "max_players": 100
    }
  }
}

// Client → Server: Chat
{
  "type": "chat",
  "payload": {
    "message": "Hello, world!"
  }
}

// Server → Client: Chat broadcast
{
  "type": "chat",
  "payload": {
    "player_id": "player_abc123",
    "username": "Alice",
    "message": "Hello, world!",
    "timestamp": 1234567890
  }
}

// Server → Client: Player joined
{
  "type": "player_joined",
  "payload": {
    "player_id": "player_def456",
    "username": "Bob",
    "empire_id": "empire_aaa111"
  }
}
```

**Files to Create**:
- `internal/network/protocol.go`
- `internal/network/handlers.go`

**Validation**: Send/receive messages between client and server

---

### Step 9: GameMap Foundation

**Goal**: Create basic GameMap structure with blank hex chunks

**Actions**:
1. Implement `GameMap` in `internal/gamemap/map.go`
2. Implement `HexChunk` in `internal/gamemap/chunk.go`
3. Generate initial map with blank chunks around origin

**GameMap Structure** (Phase 1 - simplified):
```go
type GameMap struct {
    Chunks      map[hex.Axial]*HexChunk
    ChunkRadius int // Number of chunks from origin
    mu          sync.RWMutex
}

type HexChunk struct {
    ChunkPos  hex.Axial           // Chunk position in chunk grid
    Hexes     map[hex.Axial]*Hex  // Local hex positions within chunk
    Generated bool
}

type Hex struct {
    WorldPos hex.Axial   // World position
    Terrain  string      // "plains", "forest", etc. (all "plains" for Phase 1)
}
```

**Map Generation**:
- Generate chunks in a hex radius around origin (0,0)
- Each chunk contains blank "plains" hexes
- No entities yet (Phase 2)

**Files to Create**:
- `internal/gamemap/map.go`
- `internal/gamemap/chunk.go`

**Validation**: Create map with radius 5, verify chunks are generated

---

### Step 10: Global Chat System

**Goal**: Implement simple global chat for testing

**Actions**:
1. Add chat message handling in `internal/network/handlers.go`
2. Implement rate limiting (simple in-memory counter)
3. Add message validation (length, content)
4. Broadcast chat messages to all connected players

**Chat Features**:
- Global broadcast (all players in session see messages)
- Rate limiting: 10 messages per minute per player
- Message length limit: 500 characters
- Timestamps on all messages
- Basic profanity filter (optional)

**Chat Handler Logic**:
```go
func handleChatMessage(conn *Connection, msg ChatMessage) {
    // 1. Rate limit check
    // 2. Validate message length
    // 3. Sanitize message content
    // 4. Create broadcast message
    // 5. Send to all players in session
}
```

**Files to Modify**:
- `internal/network/handlers.go`
- `internal/server/session.go` (broadcast method)

**Validation**:
- Multiple clients can send/receive chat messages
- Rate limiting prevents spam
- Message length enforced

---

### Step 11: Connection Lifecycle

**Goal**: Implement complete connection handling with proper cleanup

**Actions**:
1. Add connection read/write loops
2. Implement graceful disconnect handling
3. Add ping/pong keep-alive
4. Handle abnormal disconnections (timeout, network errors)
5. Clean up player state on disconnect

**Connection Lifecycle**:
```
1. Client connects → WebSocket upgrade
2. Server validates JWT → Extract claims
3. Server sends "welcome" message → Player info + session status
4. Client sends "join" message → Add to session
5. Server broadcasts "player_joined" → Notify other players
6. Normal operation → Message exchange, ping/pong
7. Client disconnects or timeout → Remove from session
8. Server broadcasts "player_left" → Notify other players
9. Connection closed → Cleanup resources
```

**Keep-Alive**:
- Server sends ping every 30 seconds
- Client responds with pong
- Timeout after 90 seconds of no pong

**Files to Modify**:
- `internal/server/server.go`
- `internal/network/handlers.go`

**Validation**:
- Clean disconnect removes player from session
- Timeout removes inactive players
- No goroutine leaks

---

### Step 12: Docker Configuration

**Goal**: Create Docker setup for easy development

**Actions**:
1. Create `Dockerfile` for Go server
2. Create `docker-compose.yml` with all services
3. Add Redis and MariaDB containers
4. Configure shared_services network

**Docker Compose Services**:
```yaml
version: '3.8'

services:
  mmorts-server:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs
    environment:
      - CONFIG_PATH=/app/configs/server.yaml
    depends_on:
      - redis
      - mariadb
    networks:
      - shared_services

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    networks:
      - shared_services

  mariadb:
    image: mariadb:10.11
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: mmorts
      MYSQL_USER: mmorts
      MYSQL_PASSWORD: mmorts
    ports:
      - "3306:3306"
    volumes:
      - mariadb_data:/var/lib/mysql
    networks:
      - shared_services

networks:
  shared_services:
    external: true

volumes:
  mariadb_data:
```

**Dockerfile**:
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./server"]
```

**Files to Create**:
- `Dockerfile`
- `docker-compose.yml`
- `.dockerignore`

**Validation**: `docker-compose up` starts all services successfully

---

### Step 13: Testing & Documentation

**Goal**: Create testing tools and documentation for development

**Actions**:
1. Write README.md with setup instructions
2. Create simple test client (HTML + JavaScript)
3. Add example JWT generation script (for testing)
4. Document API endpoints and message protocol

**Test Client Features**:
- HTML page with WebSocket connection
- JWT input field
- Connection status display
- Player list display
- Chat interface
- Session status display

**README Sections**:
- Prerequisites
- Installation
- Configuration
- Running locally
- Docker deployment
- Testing with test client
- API documentation
- Troubleshooting

**Files to Create**:
- `README.md`
- `test/client.html` (test client)
- `test/generate_jwt.go` (JWT generation for testing)
- `docs/API.md` (API documentation)

**Validation**: Follow README to set up project from scratch

---

## Phase 1 Deliverables

✅ **Working WebSocket server** with JWT authentication
✅ **Player connection tracking** with join/leave notifications
✅ **Basic session management** with player list and status
✅ **GameMap structure** with blank hex chunks
✅ **Global chat system** with rate limiting
✅ **Docker setup** for development environment
✅ **Test client** for manual testing
✅ **Documentation** for setup and API usage

## What's NOT in Phase 1

❌ ECS entities and components
❌ Vision system and fog of war
❌ Movement and pathfinding
❌ Combat system
❌ Building and production
❌ Spatial grid for entity queries
❌ Delta log and client synchronization
❌ VisionCache system
❌ Social layer integration
❌ Database persistence (except config)
❌ Load balancing or scaling

These will be added in subsequent phases.

## Success Criteria

Phase 1 is complete when:

1. ✅ Server starts and accepts WebSocket connections
2. ✅ JWT authentication works (valid tokens accepted, invalid rejected)
3. ✅ Multiple clients can connect simultaneously
4. ✅ Players see join/leave notifications for other players
5. ✅ Global chat works between all connected players
6. ✅ Rate limiting prevents chat spam
7. ✅ Session status is tracked and reported
8. ✅ Clean disconnects properly remove players
9. ✅ Docker setup runs all services
10. ✅ Test client can connect and chat

## Next Steps After Phase 1

**Phase 2**: Entity Management & Basic Gameplay
- Implement ECS with basic components (Position, Owner, Stats)
- Add units and buildings as entities
- Implement spatial grid for entity queries
- Add basic movement commands
- Integrate with inventory and production packages

**Phase 3**: Vision & Synchronization
- Implement vision system with fog of war
- Add client synchronization with delta log
- Implement anti-ESP protection
- Add shared vision for groups

**Phase 4**: Advanced Features
- VisionCache for persistent knowledge
- Combat system
- Social layer integration
- Performance optimization

## Timeline Estimate

**Phase 1 Duration**: 3-5 days of focused development

- Day 1: Steps 1-4 (Project setup, dependencies, config, basic server)
- Day 2: Steps 5-7 (Authentication, player model, session management)
- Day 3: Steps 8-10 (Protocol, handlers, GameMap, chat)
- Day 4: Steps 11-12 (Connection lifecycle, Docker)
- Day 5: Step 13 (Testing, documentation, polish)

## Development Notes

### Code Quality Standards
- Follow Go best practices and idioms
- Use meaningful variable and function names
- Add comments for complex logic
- Handle errors explicitly (no silent failures)
- Use context for cancellation and timeouts
- Add logging for debugging (use structured logging)

### Testing Strategy
- Unit tests for critical functions
- Integration tests for connection flow
- Manual testing with test client
- Load testing with multiple concurrent connections

### Security Considerations
- Validate all client input
- Use context timeouts to prevent goroutine leaks
- Rate limit all client actions
- Sanitize chat messages
- Use HTTPS in production (reverse proxy)
- Keep JWT public key secure
- Use Redis password in production

### Performance Considerations
- Use sync.Pool for frequently allocated objects
- Avoid blocking operations in message handlers
- Use buffered channels for send queues
- Monitor goroutine count
- Profile memory usage
- Set reasonable timeouts

## Questions to Resolve Before Starting

1. ✅ **JWT Public Key**: ~~Where do we get the public key from GoLoginServer?~~
   - **RESOLVED**: Fetch from `https://login.gravitas-games.com/api/public-key`
   - Algorithm: ES256 (ECDSA P-256)
   - Issuer: "login-server"

2. ✅ **JWT Claims**: ~~What claims are in the token?~~
   - **RESOLVED**: user_id (int64), email, username, user_type, auth_method, permissions, activated
   - No empire_id in JWT (game-specific, assigned by game server)

3. **Session Model**: Single session or multiple sessions?
   - Current plan: Single "main" session for Phase 1
   - Future: Multiple sessions (instances/shards)

4. **Database Schema**: Do we need MySQL in Phase 1?
   - Current plan: Not required for Phase 1 (no persistence)
   - Future: Add for empire data, VisionCache, player progress, etc.

5. **Redis Blacklist**: What's the key format?
   - Need to align with GoLoginServer implementation
   - Likely format: `jwt:blacklist:{token_jti}` or `jwt:blacklist:{user_id}`

6. **Empire Assignment**: How do players get empire IDs?
   - Option A: Auto-assign on first join (single empire per user)
   - Option B: Load from game database (future)
   - Option C: Player selects empire during onboarding (future)
   - Current plan: Auto-assign for Phase 1 (empire_id = user_id)

7. **Test JWT**: How to generate test tokens?
   - Option A: Create test endpoint in GoLoginServer
   - Option B: Use shared test key for local development
   - Option C: Mock JWT validation in development mode

---

**Document Version**: 1.0
**Last Updated**: 2025-10-15
**Status**: Ready for implementation
