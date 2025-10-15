# MMORTS Backend Architecture

## Project Overview

A hex-based MMORTS (Massively Multiplayer Online Real-Time Strategy) game backend built with Go, utilizing a modular architecture with external packages for core functionality. The game features a hex-chunk based map system with entities stored in spatial grids for efficient range-based queries. Web clients are served by go-spa-server, with JWT-based authentication handled by GoLoginServer (login.gravitas-games.com).

## Technology Stack

### Core Dependencies (External Packages)

1. **udp_network** - Network Transport Layer
   - Location: `external/udp_network`
   - Purpose: Unified server with dual transport support (UDP + WebSocket)
   - Features: Reliable UDP, end-to-end encryption, JSON messaging, persistent connections
   - Usage: Primary server implementation for client-server communication

2. **ecscore** - Entity Component System
   - Location: `external/ecscore`
   - Purpose: ECS scheduler for game entity management
   - Features: Deterministic tick loop, component storage strategies, async execution support
   - Usage: Core game object management system for all entities (units, buildings, resources, etc.)

3. **commandcore** - Command Processing Engine
   - Location: `external/commandcore`
   - Purpose: User and server command processing
   - Features: Per-user scheduling, rate limiting, sync/async execution, observability
   - Usage: Processing all player commands and server-side game logic commands

4. **cache** - Redis Cache Manager
   - Location: `external/cache`
   - Purpose: Entity caching with pub/sub synchronization
   - Features: Struct serialization, Redis backend, in-memory synchronization, messaging support
   - Usage: Caching player stats, blacklisted tokens, online players set, pub/sub messaging for social layer

5. **hexcore** - Hex Grid System
   - Location: `external/hexcore`
   - Purpose: Hex-based map and pathfinding
   - Components: chunk, hex, path modules
   - Features: HexChunk with radius & cells, Pocket (7 chunks), edge signatures for stitching
   - Usage: Map representation, coordinate systems, chunk-based world generation

6. **inventory** - Inventory Management
   - Location: `external/inventory`
   - Purpose: Item and resource management system
   - Usage: Player inventories, resource storage, item transfers

7. **production** - Production System
   - Location: `external/production`
   - Purpose: Building and unit production queues
   - Features: Recipe-based crafting, immediate consumption, efficiency modifiers, repeating jobs
   - Usage: Construction, training, research systems

### Supporting Infrastructure

8. **go-spa-server** - Web Client Server
   - Location: `../go-spa-server` (separate project)
   - Purpose: Serves web client static files
   - Features: Static file serving, HTML/CSS/JS delivery, client-side routing
   - Usage: Web client delivery via HTTP/HTTPS

9. **GoLoginServer** - Authentication Server
   - Location: Hosted at `login.gravitas-games.com` (separate project)
   - Purpose: User authentication and JWT token issuance
   - Features: User accounts, JWT generation, permission system, OAuth support
   - Usage: Client authenticates and receives JWT token, which is then sent to game server

## Architecture Components

### 1. Network Layer & Authentication

```
┌─────────────────────────────────────────────────────────┐
│                  Web Client (Browser)                   │
│              Served by go-spa-server                    │
│                  (Static Files Only)                    │
└───────────┬─────────────────────────────────────────────┘
            │
            │ 1. User Login Request
            ▼
┌─────────────────────────────────────────────────────────┐
│              GoLoginServer                              │
│         (login.gravitas-games.com)                      │
│  ┌───────────────────────────────────────────────────┐  │
│  │  - Validates credentials                          │  │
│  │  - Issues JWT token (ES256 algorithm)             │  │
│  │  - Token Claims:                                  │  │
│  │    * user_id (int64)                              │  │
│  │    * email, username                              │  │
│  │    * permissions (int64 bitwise flags)            │  │
│  │    * user_type, auth_method                       │  │
│  │    * activated (timestamp or ban status)          │  │
│  │  - Public key at /api/public-key                  │  │
│  └───────────────────────────────────────────────────┘  │
└───────────┬─────────────────────────────────────────────┘
            │ 2. JWT Token
            ▼
┌─────────────────────────────────────────────────────────┐
│                  Web Client (Browser)                   │
│              Stores JWT in localStorage                 │
└───────────┬─────────────────────────────────────────────┘
            │ 3. Connect with JWT in header
            ▼
┌─────────────────────────────────────────────────────────┐
│          Game Server (UDP Network / WebSocket)          │
│  ┌──────────┐      ┌─────────────┐                      │
│  │   UDP    │      │  WebSocket  │                      │
│  │ Transport│      │  Transport  │                      │
│  └──────────┘      └─────────────┘                      │
│         └──────────┬────────┘                           │
│           Unified API                                   │
│                                                          │
│  Connection Handler:                                    │
│  1. Receive JWT token from WebSocket connection header  │
│  2. Validate token signature with ECDSA P-256 public key│
│  3. Verify issuer is "login-server"                     │
│  4. Check token expiration                              │
│  5. Verify user is activated (activated > 0)            │
│  6. Verify user is not banned (activated != -1)         │
│  7. Check Redis blacklist for revoked tokens            │
│  8. Extract user claims (ID, username, email, perms)    │
│  9. Establish game session connection                   │
└──────────────────────────────────────────────────────────┘
```

**JWT Token Flow:**
1. Client (browser) makes login request to go-spa-server
2. go-spa-server proxies request to GoLoginServer (login.gravitas-games.com)
3. GoLoginServer validates credentials and issues JWT token (ES256)
4. Client stores JWT in localStorage
5. Client connects to game server via WebSocket with JWT in connection header
6. Game server fetches public key from GoLoginServer `/api/public-key` (cached)
7. Game server validates JWT signature, issuer, expiration, activation status
8. Game server checks Redis blacklist for revoked tokens
9. Banned/revoked/expired tokens are rejected
10. Valid tokens allow player to join game session

**JWT Token Structure:**
```json
{
  "user_id": 123,
  "email": "john@example.com",
  "username": "johndoe",
  "user_type": "user",
  "auth_method": "password",
  "permissions": 55,
  "activated": 1697123456789000000,
  "iss": "login-server",
  "iat": 1697123456,
  "exp": 1697127056
}
```

**Player Model (derived from JWT):**
```go
type Player struct {
    UserID      string  // JWT claim: user_id (converted from int64)
    Email       string  // JWT claim: email
    Username    string  // JWT claim: username
    UserType    string  // JWT claim: user_type (deprecated, use permissions)
    Permissions int64   // JWT claim: permissions (bitwise flags)
    Activated   int64   // JWT claim: activated (0=not activated, -1=banned, >0=timestamp)
    AuthMethod  string  // JWT claim: auth_method ('password' or 'oauth')

    // Note: Empire ID is not in JWT - assigned by game server
    EmpireID    string  // Assigned on first join or loaded from game database
}
```

**Token Validation Steps:**
1. Parse JWT and extract claims
2. Verify signature with ECDSA P-256 public key
3. Verify issuer is "login-server"
4. Check expiration (exp claim)
5. Check user activation status (activated > 0, activated != -1)
6. Check Redis blacklist (key format TBD with GoLoginServer team)
7. Accept or reject connection

### 2. Game Map Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    GameMap                              │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Hex Chunk Grid (from hexcore)                    │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐           │  │
│  │  │ Chunk   │  │ Chunk   │  │ Chunk   │           │  │
│  │  │ (R=9)   │  │ (R=9)   │  │ (R=9)   │           │  │
│  │  │ 169 hex │  │ 169 hex │  │ 169 hex │           │  │
│  │  └─────────┘  └─────────┘  └─────────┘           │  │
│  │                                                    │  │
│  │  Each HexChunk:                                    │  │
│  │  - Coord: hex.Axial (center position)             │  │
│  │  - Radius: 9 (configurable)                       │  │
│  │  - Cells: map[hex.Axial]HexState                  │  │
│  │  - EdgeSig: map[EdgeDirection]EdgeMask            │  │
│  └───────────────────────────────────────────────────┘  │
│                                                          │
│  HexState: Space (1) or Dead (0)                        │
│  Initial Map: All chunks generated with Space hexes     │
└──────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│              Spatial Grid (Entity Overlay)              │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Entities stored separately from map              │  │
│  │                                                    │  │
│  │  Grid Structure:                                   │  │
│  │  - Divides world into spatial cells               │  │
│  │  - Each cell contains entity references           │  │
│  │  - Optimized for range queries                    │  │
│  │                                                    │  │
│  │  Entity Types:                                     │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐          │  │
│  │  │  Units   │ │Buildings │ │Resources │          │  │
│  │  └──────────┘ └──────────┘ └──────────┘          │  │
│  │                                                    │  │
│  │  Benefits:                                         │  │
│  │  - Fast proximity searches                        │  │
│  │  - Efficient collision detection                  │  │
│  │  - Range-based queries (attack range, vision)     │  │
│  │  - Scales well with many entities                 │  │
│  └───────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**GameMap Structure:**
```go
type GameMap struct {
    Chunks map[hex.Axial]*HexChunk  // Hex chunk grid
    SpatialGrid *SpatialGrid         // Entity spatial index
}

type HexChunk struct {
    Coord   hex.Axial                    // Center coordinate
    Radius  int                          // Chunk radius (default 9)
    Cells   map[hex.Axial]HexState       // Hex cell states
    EdgeSig map[EdgeDirection]EdgeMask   // Edge signatures for stitching
}

type SpatialGrid struct {
    CellSize  int                           // Grid cell size in hex units
    Cells     map[GridCoord]*SpatialCell    // Grid cells
    Entities  map[EntityID]*EntityRef       // Quick entity lookup
}

type SpatialCell struct {
    Units     []EntityID  // Unit entities in this cell
    Buildings []EntityID  // Building entities in this cell
    Resources []EntityID  // Resource entities in this cell
}
```

**Spatial Grid Benefits:**
- O(1) lookup for entities in a region
- Efficient range queries (e.g., "find all units within 5 hexes")
- Separate storage from map tiles
- Extensible for different entity types

### 3. Game Session Management

```
┌─────────────────────────────────────────────────────────┐
│                    Game Session                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Session ID: string                               │  │
│  │  GameMap: *GameMap (hex chunks + spatial grid)    │  │
│  │  Players: map[PlayerID]*Player                    │  │
│  │  ECS World: *ecs.World                            │  │
│  │  Tick Counter: int64                              │  │
│  │  Production Managers: map[BuildingID]*Manager     │  │
│  └───────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**Initial Scope:**
- Players join a game session with validated JWT
- Session tracks connected players
- GameMap with blank open hex chunks for web client rendering
- Basic session lifecycle (create, join, leave)
- Online players set stored in Redis

**Future Expansion:**
- Multiple concurrent game sessions
- Match-making system
- Session replay/recording
- Spectator support

### 4. Entity Management (ECS)

```
┌──────────────────────────────────────────────────────────┐
│                    ECS Scheduler                         │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Entities (ID + Components)                        │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐         │  │
│  │  │ Players  │  │ Buildings│  │  Units   │         │  │
│  │  └──────────┘  └──────────┘  └──────────┘         │  │
│  │                                                    │  │
│  │  Components:                                       │  │
│  │  - Position (hex.Axial)                           │  │
│  │  - Owner (PlayerID)                               │  │
│  │  - Stats (health, attack, defense)                │  │
│  │  - Inventory (*inventory.Inventory)               │  │
│  │  - Production (*production.Manager)               │  │
│  │                                                    │  │
│  │  Systems (Game Logic)                             │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │  │
│  │  │Movement  │  │ Combat   │  │Production│        │  │
│  │  └──────────┘  └──────────┘  └──────────┘        │  │
│  │  ┌──────────┐  ┌──────────┐                      │  │
│  │  │SpatialIdx│  │  Sync    │                      │  │
│  │  └──────────┘  └──────────┘                      │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  Note: Entities stored separately from map tiles        │
│        Referenced in SpatialGrid for efficient queries  │
└──────────────────────────────────────────────────────────┘
```

**Entity-Map Separation:**
- Map tiles are NOT entities (stored in GameMap.Chunks)
- Game entities (units, buildings) ARE entities (stored in ECS)
- SpatialGrid bridges the gap for efficient queries
- Systems can query both map and entities efficiently

### 5. Building & Production System

```
┌──────────────────────────────────────────────────────────┐
│                Building Component                        │
│  ┌────────────────────────────────────────────────────┐  │
│  │  EntityID: entity reference                        │  │
│  │  BuildingType: string (e.g., "barracks")           │  │
│  │  Position: hex.Axial                               │  │
│  │  Owner: PlayerID                                   │  │
│  │  Inventory: *inventory.Inventory                   │  │
│  │  ProductionMgr: *production.Manager                │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  Production Integration (from production package docs): │
│  ┌────────────────────────────────────────────────────┐  │
│  │  1. Create RecipeRegistry (shared)                 │  │
│  │     - Unit recipes (train soldier, train worker)   │  │
│  │     - Building recipes (build house, build farm)   │  │
│  │                                                    │  │
│  │  2. Each building gets a Manager                   │  │
│  │     mgr := production.NewManager(                  │  │
│  │         buildingID,                                │  │
│  │         recipeRegistry,                            │  │
│  │         inventoryProvider,                         │  │
│  │         eventBus,                                  │  │
│  │         modifierSources                            │  │
│  │     )                                              │  │
│  │                                                    │  │
│  │  3. ProductionTickSystem updates all managers      │  │
│  │     world.Query(func(building *BuildingComponent) {│  │
│  │         building.ProductionMgr.Update(now)         │  │
│  │     })                                             │  │
│  │                                                    │  │
│  │  4. Players start production via commands          │  │
│  │     jobID, err := mgr.StartProduction(             │  │
│  │         "train_soldier",                           │  │
│  │         playerID,                                  │  │
│  │         buildingInventoryID                        │  │
│  │     )                                              │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**Production System Features:**
- Immediate resource consumption (prevents exploits)
- Recipe-based crafting (inputs → outputs with duration)
- Efficiency modifiers (building level, player skills)
- Repeating jobs (e.g., continuous unit training)
- Event bus for production completion notifications

### 6. Command Processing

```
┌───────────────────────────────────────────────────────────┐
│                 Command Processor                         │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Player Commands (Client→Server)                    │  │
│  │  - JoinSession (with JWT validation)                │  │
│  │  - LeaveSession                                     │  │
│  │  - (Future: Move, Attack, Build, Train, etc.)      │  │
│  │                                                     │  │
│  │  Server Commands (Internal)                         │  │
│  │  - SpawnEntity                                      │  │
│  │  - UpdateGameState                                  │  │
│  │  - BroadcastUpdate                                  │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                           │
│  Rate Limiting & Validation (from commandcore)           │
│  - Per-user command queues                               │
│  - Cooldown enforcement                                  │
│  - Command verification                                  │
└───────────────────────────────────────────────────────────┘
```

### 7. Data Persistence Layer

```
┌──────────────────────────────────────────────────────────┐
│                 Datastore Interface                      │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Abstract Interface                                │  │
│  └───────────┬────────────────────────────────────────┘  │
│              │                                           │
│  ┌───────────▼────────────────────────────────────────┐  │
│  │  MySQL/MariaDB Adapter (Primary)                   │  │
│  │  - Player accounts & profiles                      │  │
│  │  - Session data & history                          │  │
│  │  - Game state snapshots (periodic)                 │  │
│  │  - Building & entity persistent data               │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Redis Cache (via cache package)                   │  │
│  │  - Blacklisted JWT tokens (set)                    │  │
│  │  - Online players set (set)                        │  │
│  │  - Player stats & summaries                        │  │
│  │  - Pub/sub for social layer messaging              │  │
│  │  - NOT used for active game state (too volatile)   │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**Datastore Interface Design:**
```go
type Datastore interface {
    // Player operations
    CreatePlayer(ctx context.Context, player *Player) error
    GetPlayer(ctx context.Context, playerID string) (*Player, error)
    UpdatePlayer(ctx context.Context, player *Player) error

    // Session operations
    CreateSession(ctx context.Context, session *GameSession) error
    GetSession(ctx context.Context, sessionID string) (*GameSession, error)
    UpdateSession(ctx context.Context, session *GameSession) error
    DeleteSession(ctx context.Context, sessionID string) error

    // Building operations
    SaveBuilding(ctx context.Context, building *Building) error
    LoadBuildings(ctx context.Context, sessionID string) ([]*Building, error)

    // Snapshot operations
    SaveSnapshot(ctx context.Context, snapshot *GameSnapshot) error
    LoadSnapshot(ctx context.Context, snapshotID string) (*GameSnapshot, error)
}
```

**Redis Usage:**
```go
type RedisCache interface {
    // Token blacklist
    BlacklistToken(ctx context.Context, token string, expiry time.Duration) error
    IsTokenBlacklisted(ctx context.Context, token string) (bool, error)

    // Online players
    AddOnlinePlayer(ctx context.Context, playerID string) error
    RemoveOnlinePlayer(ctx context.Context, playerID string) error
    GetOnlinePlayers(ctx context.Context) ([]string, error)

    // Player stats
    SetPlayerStats(ctx context.Context, playerID string, stats *PlayerStats) error
    GetPlayerStats(ctx context.Context, playerID string) (*PlayerStats, error)

    // Pub/sub messaging (for social layer)
    PublishMessage(ctx context.Context, channel string, message []byte) error
    Subscribe(ctx context.Context, channel string, handler func([]byte)) error
}
```

### 8. Social Layer (External Package)

```
┌──────────────────────────────────────────────────────────┐
│                 Social Layer Package                     │
│              (Located in external/social)                │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Uses Redis Pub/Sub (via cache package)            │  │
│  │                                                    │  │
│  │  Features:                                         │  │
│  │  - Online/offline status tracking                 │  │
│  │  - Friend lists                                   │  │
│  │  - Chat (transient) & Mail (persistent)          │  │
│  │  - Group system (alliances, teams, clans)        │  │
│  │  - Global announcements                           │  │
│  │  - Player presence (in-game, lobby, offline)      │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  Generic Design:                                        │
│  - Uses "groups" instead of game-specific "alliances"   │
│  - Game server maps alliances → social groups          │
│  - Portable across multiple game projects              │
│                                                          │
│  API Specification:                                     │
│  See external/social/SOCIAL_LAYER_API.md               │
└──────────────────────────────────────────────────────────┘
```

**Game Server Integration:**

The game server uses the social layer's generic "group" functionality to implement alliances:

```go
// When player creates an alliance
func (s *Server) CreateAlliance(playerID, allianceName, tag string) error {
    // Create group in social layer
    groupID, err := s.socialLayer.CreateGroup(ctx, allianceName, playerID, social.GroupOptions{
        Tag:         tag,
        Description: "Alliance: " + allianceName,
        IsPublic:    true,
    })

    // Store alliance metadata in game database
    alliance := &Alliance{
        ID:        groupID,
        Name:      allianceName,
        Tag:       tag,
        FounderID: playerID,
    }
    s.datastore.CreateAlliance(ctx, alliance)

    return nil
}

// When player joins alliance
func (s *Server) JoinAlliance(playerID, allianceID string) error {
    // Join group in social layer
    err := s.socialLayer.AcceptGroupInvitation(ctx, playerID, allianceID)

    // Trigger vision sharing
    s.sharedVisionMgr.OnPlayerJoinGroup(playerID, allianceID)

    return nil
}
```

### 9. Vision & Client Synchronization System

The vision and synchronization system manages what players can see and keeps clients updated with game state changes.

```
┌─────────────────────────────────────────────────────────────┐
│                      Game Server                            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              ECS Tick Loop (20 Hz)                    │  │
│  │  ┌─────────────────────────────────────────────────┐  │  │
│  │  │  1. Process Commands                            │  │  │
│  │  │  2. Run Game Logic Systems                      │  │  │
│  │  │  3. Update Spatial Grid                         │  │  │
│  │  │  4. Calculate Vision (Vision System)            │  │  │
│  │  │  5. Determine Dirty Entities                    │  │  │
│  │  │  6. Generate Client Updates (Sync System)       │  │  │
│  │  │  7. Send Updates via Network                    │  │  │
│  │  └─────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

**Terminology:**
- **Vision**: What an empire can "see" on the map
- **Fog of War (FoW)**: Areas previously explored but not currently visible
- **Shroud**: Areas never explored (completely dark)
- **Vision Range**: Distance a unit/building can see
- **Shared Vision**: Vision sharing between empires/groups (alliances)
- **Observer**: Any entity that provides vision (unit, building, or special vision entity)

#### Vision System Components

**Vision Component:**
```go
type VisionComponent struct {
    Range        int      // Vision range in hex tiles
    Enabled      bool     // Can be disabled (e.g., stunned unit)
    VisionType   VisionType
}

type VisionType string

const (
    VisionTypeNormal  VisionType = "normal"   // Standard vision
    VisionTypeShared  VisionType = "shared"   // Provides vision to allies
    VisionTypeStealth VisionType = "stealth"  // Can see stealthed units
)
```

**Vision Manager (Per Empire):**
```go
type VisionManager struct {
    EmpireID string

    // Vision map: which hexes are visible
    VisibleHexes map[hex.Axial]VisibilityState

    // Visible entities cache
    VisibleEntities map[ecs.EntityID]*EntitySnapshot

    // Previous tick's visible entities (for delta calculation)
    PreviouslyVisible map[ecs.EntityID]*EntitySnapshot

    // Observers (entities providing vision for this empire)
    Observers map[ecs.EntityID]*ObserverInfo

    // Shared vision relationships
    SharedVisionFrom []string  // Empires sharing vision to us
    SharedVisionTo   []string  // Empires we share vision to
}

type VisibilityState int

const (
    VisibilityShroud  VisibilityState = 0  // Never seen
    VisibilityFoW     VisibilityState = 1  // Previously seen (fog of war)
    VisibilityVisible VisibilityState = 2  // Currently visible
)
```

#### Vision Calculation

The VisionSystem runs every tick:

1. **Clear Previous Visibility**: Mark currently visible hexes as FoW
2. **Calculate Vision Per Empire**: For each observer (unit/building), calculate visible hexes
3. **Query Spatial Grid**: Find entities in visible hexes
4. **Apply Shared Vision**: Copy vision from empires sharing with us
5. **Create Entity Snapshots**: Snapshot visible entities for sync system

**Spatial Grid Integration:**
```go
// Use spatial grid for fast entity lookups in vision range
func (s *VisionSystem) calculateVisionForEmpire(empireID string, vm *VisionManager) {
    vm.Observers = s.gatherObservers(empireID, world)

    for _, observer := range vm.Observers {
        visibleHexes := s.getHexesInRange(observer.Position, observer.Range)

        for _, hexPos := range visibleHexes {
            vm.VisibleHexes[hexPos] = VisibilityVisible

            // Query spatial grid for entities at this hex
            entities := s.spatialGrid.GetEntitiesAtHex(hexPos)

            for _, entityID := range entities {
                snapshot := s.createEntitySnapshot(entityID, world)
                vm.VisibleEntities[entityID] = snapshot
            }
        }
    }
}
```

#### Shared Vision Management

**Group-Based Vision Sharing:**

The game uses the social layer's "group" concept for vision sharing. In the MMORTS context, alliances are implemented as groups in the social layer.

```go
type SharedVisionManager struct {
    sharingRelationships map[string][]string  // Empire → Empires they share with
}

// Automatic group vision sharing (alliances use this)
func (m *SharedVisionManager) OnPlayerJoinGroup(empireID, groupID string) {
    members := m.getGroupMembers(groupID)

    // Auto-share vision with group members (configurable)
    if m.config.AutoShareVisionWithGroup {
        for _, member := range members {
            if member != empireID {
                m.ShareVision(empireID, member)
            }
        }
    }
}

// Manual vision sharing with specific empire
func (m *SharedVisionManager) ShareVision(fromEmpire, toEmpire string) {
    if !contains(m.sharingRelationships[fromEmpire], toEmpire) {
        m.sharingRelationships[fromEmpire] = append(
            m.sharingRelationships[fromEmpire],
            toEmpire,
        )
    }
}
```

**Vision Sharing Features:**
- Group members (alliances) automatically share vision (configurable)
- Manual vision sharing with specific empires outside group
- One-way or mutual vision sharing
- Stop sharing vision at any time

**Integration with Social Layer:**
- Social layer provides generic "group" functionality
- Game server maps "alliances" → social layer "groups"
- Vision sharing triggered on group join/leave events from social layer

#### Client Synchronization System

**Client Sync Manager (Per Client):**
```go
type ClientSyncManager struct {
    PlayerID string
    EmpireID string

    // Last known state sent to client
    LastSyncState *ClientState

    // Timestamp of last sync
    LastSyncTime time.Time

    // Update queue (prioritized)
    UpdateQueue *PriorityQueue

    // Bandwidth throttling
    BytesPerSecond int
    BytesSentThisTick int
}

type ClientState struct {
    Tick            int64
    VisibleEntities map[ecs.EntityID]*EntitySnapshot
    VisionMap       map[hex.Axial]VisibilityState
    Resources       map[string]int  // Empire resources
}
```

#### Delta Update Generation

The ClientSyncSystem generates delta updates by comparing current vision state with last sent state:

```go
type DeltaUpdate struct {
    Tick               int64

    // Entity changes
    EntitiesAppeared    []*EntitySnapshot  // Newly visible
    EntitiesChanged     []*EntitySnapshot  // Changed properties
    EntitiesDisappeared []ecs.EntityID     // No longer visible

    // Vision changes
    VisionChanges       []VisionChange

    // Resource changes
    ResourceChanges     map[string]int
}

type VisionChange struct {
    Hex   hex.Axial
    State VisibilityState  // 0=shroud, 1=fow, 2=visible
}
```

**Update Generation Logic:**
1. **Entities Appeared**: In current vision but not in last state
2. **Entities Changed**: In both but properties differ
3. **Entities Disappeared**: In last state but not current vision
4. **Vision Changes**: Hex visibility state changed

#### Update Prioritization & Bandwidth Management

**Priority Levels:**
```go
type UpdatePriority int

const (
    PriorityCritical UpdatePriority = 0  // Own units, nearby enemies
    PriorityHigh     UpdatePriority = 1  // Nearby entities
    PriorityNormal   UpdatePriority = 2  // Visible but distant
    PriorityLow      UpdatePriority = 3  // Far away, low importance
)
```

**Priority Calculation:**
- **Own entities**: High priority
- **Distance-based**: Closer entities have higher priority
- **Entity type**: Combat units > workers > resources

**Bandwidth Throttling:**
- Each client has a bytes/second limit
- If update exceeds limit, send highest priority items first
- Queue remaining updates for next tick
- Prevents network congestion

#### JSON Message Format (to Web Client)

```json
{
  "type": "update",
  "tick": 12345,
  "appeared": [
    {
      "id": "unit_123",
      "type": "soldier",
      "pos": [10, 5],
      "owner": "empire_abc",
      "health": 100,
      "data": {}
    }
  ],
  "changed": [
    {
      "id": "unit_456",
      "pos": [11, 6],
      "health": 75
    }
  ],
  "disappeared": ["unit_789"],
  "vision": [
    {"hex": [10, 5], "state": 2},
    {"hex": [9, 4], "state": 1}
  ],
  "resources": {
    "gold": 1500,
    "wood": 800
  }
}
```

#### Change Tracking System: Delta Log + Snapshot

The sync system uses a **delta log with periodic snapshots** approach for tracking entity changes. This provides:
- Simple conceptual model (current state + replay changes)
- Automatic reconnect handling (replay from last known tick)
- Low allocation pressure (sync.Pool for memory reuse)
- Vision-filtered updates (anti-ESP protection)

**Core Architecture:**

```go
type SyncSystem struct {
    // Current authoritative state
    entities map[EntityID]*Entity

    // Delta log (ring buffer) - stores ALL changes
    deltaLog     []DeltaRecord
    deltaLogSize int           // e.g., 2000
    deltaLogHead int           // Write position
    oldestTick   int64         // Oldest delta in buffer
    newestTick   int64         // Current tick

    // Snapshot reconciliation
    lastSnapshotTick int64
    snapshotInterval int64     // Reconcile every N ticks (e.g., 100)

    // Vision system reference
    visionSystem *VisionSystem

    // Pooling for reduced allocations
    deltaPool    *sync.Pool
    snapshotPool *sync.Pool
}

type DeltaRecord struct {
    Tick          int64
    EntityID      EntityID
    ComponentType ComponentType
    Value         Component  // Deep copy from pool
}

type ClientSyncManager struct {
    PlayerID     string
    EmpireID     string     // Links to VisionManager
    LastSyncTick int64

    // Snapshot of what client currently has (only visible entities)
    entitySnapshots map[EntityID]*EntitySnapshot

    // Track known entities for vision changes
    knownEntities map[EntityID]bool
}
```

**Recording Changes (Vision-Agnostic):**

```go
// Record ALL changes to delta log (no vision filtering here)
func (s *SyncSystem) RecordChange(entityID EntityID, ctype ComponentType, value Component, tick int64) {
    // Update entity
    entity := s.entities[entityID]
    entity.Components[ctype] = value

    // Get delta from pool
    delta := s.deltaPool.Get().(*DeltaRecord)
    delta.Tick = tick
    delta.EntityID = entityID
    delta.ComponentType = ctype
    delta.Value = deepCopyComponent(value)  // Must copy

    // Add to ring buffer
    s.deltaLog[s.deltaLogHead] = *delta
    s.deltaLogHead = (s.deltaLogHead + 1) % s.deltaLogSize
    s.deltaPool.Put(delta)

    s.newestTick = tick
    if s.deltaLogHead == 0 {
        s.oldestTick = s.deltaLog[0].Tick  // Buffer wrapped
    }
}
```

**Client Sync with Vision Filtering:**

```go
func (s *SyncSystem) SyncClient(client *ClientSyncManager, currentTick int64) {
    // Get client's vision
    visionMgr := s.visionSystem.GetVisionManager(client.EmpireID)

    // 1. Handle entities entering vision (send full snapshot)
    for entityID := range visionMgr.VisibleEntities {
        if !client.knownEntities[entityID] {
            entity := s.entities[entityID]
            snapshot := s.createSnapshot(entity, currentTick)
            client.entitySnapshots[entityID] = snapshot
            client.knownEntities[entityID] = true
            s.sendEntityAppeared(client, snapshot)
        }
    }

    // 2. Handle entities leaving vision (send disappear)
    for entityID := range client.knownEntities {
        if _, visible := visionMgr.VisibleEntities[entityID]; !visible {
            s.removeClientEntity(client, entityID)
            s.sendEntityDisappeared(client, entityID)
        }
    }

    // 3. Send deltas for visible entities
    if client.LastSyncTick < s.oldestTick {
        s.resyncVisibleEntities(client, visionMgr, currentTick)
    } else {
        s.sendDeltasForVisibleEntities(client, visionMgr, currentTick)
    }

    client.LastSyncTick = currentTick
}

func (s *SyncSystem) sendDeltasForVisibleEntities(
    client *ClientSyncManager,
    visionMgr *VisionManager,
    currentTick int64,
) {
    deltas := []DeltaRecord{}

    // Iterate delta log
    for i := 0; i < s.deltaLogSize; i++ {
        delta := &s.deltaLog[i]

        // Skip if outside time window
        if delta.Tick <= client.LastSyncTick || delta.Tick > currentTick {
            continue
        }

        // CRITICAL: Only send if entity is visible to client
        if _, visible := visionMgr.VisibleEntities[delta.EntityID]; !visible {
            continue  // Vision filter - prevents ESP
        }

        if !client.knownEntities[delta.EntityID] {
            continue  // Safety check
        }

        deltas = append(deltas, *delta)
    }

    if len(deltas) > 0 {
        s.sendDeltas(client, deltas)
    }
}
```

**Snapshot Reconciliation (Every N Ticks):**

```go
func (s *SyncSystem) Tick(currentTick int64) {
    // ... run game systems, record changes ...

    // Periodic snapshot reconciliation
    if currentTick - s.lastSnapshotTick >= s.snapshotInterval {
        s.reconcileSnapshots(currentTick)
        s.lastSnapshotTick = currentTick
    }
}

func (s *SyncSystem) reconcileSnapshots(currentTick int64) {
    for _, client := range s.clients {
        if client.LastSyncTick < s.oldestTick {
            // Client too far behind - force full resync
            s.sendFullSnapshot(client, currentTick)
            continue
        }

        // Update client snapshots with recent deltas
        for entityID, snapshot := range client.entitySnapshots {
            s.applyDeltasToSnapshot(client, entityID, snapshot, currentTick)
        }
    }
}
```

**Anti-ESP Protection:**

The vision-filtered delta log provides strong anti-cheat protection:

1. **No Hidden Entity Data**: Clients never receive updates for entities outside their vision
2. **No Historical Leakage**: When entity enters vision, client receives current state only (not past positions)
3. **Automatic Cleanup**: Entities leaving vision send disappear message, no further updates sent
4. **Server Authoritative**: Delta log is server-side only, clients receive filtered view

**Example Scenario:**

```
Tick 0-49: Enemy scout moves from (100,100) to (50,50) - outside your vision
          - Changes recorded in delta log
          - Your client receives ZERO updates (vision filter blocks)

Tick 50:   Enemy scout enters your vision at (10,10)
          - handleEntitiesEnteredVision detects new entity
          - Sends FULL snapshot with CURRENT state (10,10)
          - You learn scout EXISTS but NOT its history
          - Previous positions (100,100) to (50,50) never transmitted

Tick 51-59: Scout moves to (11,11), (12,12), etc. - in your vision
          - Deltas sent normally (vision check passes)

Tick 60:   Scout moves to (50,50) - exits your vision
          - handleEntitiesLeftVision detects removal
          - Sends "entity disappeared" message
          - Delta recorded but vision filter blocks it
          - You never learn scout moved to (50,50)
```

**Memory & Performance:**

- **Delta log size**: `2000 records * 32 bytes = 64 KB`
- **Per-client overhead**: Only snapshots of visible entities (from pool)
- **Allocation pressure**: Near zero (sync.Pool reuses memory)
- **CPU per sync**: O(deltas in window) filtered by vision - typically 100-200 deltas
- **Reconciliation**: Amortized over 100 ticks (every 5 seconds @ 20Hz)

**Configuration:**

```yaml
client_sync:
  delta_log_size: 2000        # Ring buffer size
  snapshot_interval: 100      # Reconcile every 100 ticks (5 sec)
  max_delta_age: 200          # Force resync if >200 ticks behind
  enable_pooling: true        # Use sync.Pool for allocations
```

#### Performance Optimizations

1. **Spatial Grid Queries**: O(1) lookup for entities in vision range
2. **Vision Caching**: Cache static observer vision (buildings don't move)
3. **Delta Log Ring Buffer**: Bounded memory, constant-time writes
4. **sync.Pool Usage**: Reuse snapshots and delta records (zero allocation pressure)
5. **Vision Filtering**: Only process deltas for visible entities
6. **Snapshot Reconciliation**: Periodic cleanup every N ticks (amortized cost)

#### Client-Side State Management

**Web Client Responsibilities:**
```javascript
class GameStateManager {
    constructor() {
        this.entities = new Map();
        this.visionMap = new Map();
    }

    onUpdateReceived(update) {
        // Apply appeared entities
        for (const entityData of update.appeared || []) {
            const entity = this.createEntity(entityData);
            this.entities.set(entity.id, entity);
        }

        // Apply changes
        for (const entityData of update.changed || []) {
            this.updateEntity(this.entities.get(entityData.id), entityData);
        }

        // Remove disappeared entities
        for (const entityID of update.disappeared || []) {
            this.entities.delete(entityID);
        }

        // Update vision map
        for (const visionData of update.vision || []) {
            const hexKey = `${visionData.hex[0]},${visionData.hex[1]}`;
            this.visionMap.set(hexKey, visionData.state);
        }
    }

    // Interpolation for smooth 60 FPS rendering
    update(deltaTime) {
        for (const entity of this.entities.values()) {
            if (entity.targetPosition) {
                entity.position = this.interpolate(
                    entity.position,
                    entity.targetPosition,
                    deltaTime
                );
            }
        }
    }
}
```

#### Configuration

```yaml
vision:
  default_unit_vision: 5
  default_building_vision: 8
  enable_fog_of_war: true
  update_interval: 1  # Every N ticks
  auto_share_group_vision: true  # Auto-share with group (alliance) members

client_sync:
  sync_rate: 20  # Hz (same as tick rate)
  bandwidth_limit: 50000  # bytes/sec per client
  enable_prioritization: true
  priority_distances:
    critical: 5
    high: 15
    normal: 40
  enable_compression: true
```

#### VisionCache System: Persistent Strategic Knowledge

The VisionCache system provides **persistent memory** of what players have explored and seen, surviving between sessions. This enables strategic gameplay where scouted enemy settlements remain known even after losing vision.

**Core Concept:**

- **Explored Hexes**: Additive fog of war - once explored, terrain is remembered
- **Cached Entities**: Selective snapshots of important entities (settlements, NPCs, special units)
- **Timescale-Based Retention**: Entities cached permanently or for specific durations
- **Stale Data Indication**: Clients render cached snapshots differently (faded/grayed out)

**Architecture:**

```go
type VisionCache struct {
    EmpireID string

    // Historical vision - all hexes ever explored (additive, persists forever)
    ExploredHexes map[hex.Axial]ExploredHexInfo

    // Cached entity snapshots - selective, important entities only
    CachedEntities map[EntityID]*CachedEntitySnapshot

    LastUpdated time.Time
    Version     int64
}

type ExploredHexInfo struct {
    FirstSeen   int64        // Tick when first explored
    LastSeen    int64        // Tick when last had vision
    TerrainType TerrainType  // Terrain type (immutable)
}

type CachedEntitySnapshot struct {
    EntityID    EntityID
    EntityType  EntityType
    Owner       string
    Position    hex.Axial

    // Selective components (not all - only essential)
    Components  map[ComponentType]Component

    FirstSeen   int64           // When first encountered
    LastSeen    int64           // When last in vision
    LastUpdated int64           // When snapshot last updated

    CacheMode   CacheMode       // Permanent or timed
    ExpiresAt   int64           // Tick when expires (0 = never)
    IsStale     bool            // Not currently visible
}

type CacheMode string

const (
    CachePermanent CacheMode = "permanent"  // Cache until re-enters vision
    CacheTimed     CacheMode = "timed"      // Cache for specific duration
)
```

**Cache Policy - What Gets Cached:**

```go
type EntityCachePolicy interface {
    ShouldCache(entity *Entity) bool
    GetCacheMode(entity *Entity) CacheMode
    GetCacheDuration(entity *Entity) int64  // Ticks (0 = permanent)
    GetCachedComponents(entity *Entity) []ComponentType
}

type DefaultCachePolicy struct{}

func (p *DefaultCachePolicy) ShouldCache(entity *Entity) bool {
    switch entity.Type {
    case EntityTypeBuilding:
        return true  // Cache all buildings

    case EntityTypeUnit:
        if unit, ok := entity.GetComponent(ComponentUnit).(*UnitComponent); ok {
            return unit.IsHero || unit.IsEndgameUnit || unit.IsNPC
        }
        return false

    case EntityTypeNPC:
        return true  // Always cache NPCs

    default:
        return false
    }
}

func (p *DefaultCachePolicy) GetCacheMode(entity *Entity) CacheMode {
    switch entity.Type {
    case EntityTypeBuilding:
        building := entity.GetComponent(ComponentBuilding).(*BuildingComponent)
        if building.BuildingType == "capital" || building.BuildingType == "settlement" {
            return CachePermanent  // Settlements cached until re-scouted
        }
        return CacheTimed  // Other buildings timed

    case EntityTypeNPC:
        return CachePermanent  // NPCs cached permanently

    case EntityTypeUnit:
        unit := entity.GetComponent(ComponentUnit).(*UnitComponent)
        if unit.IsEndgameUnit {
            return CachePermanent
        }
        return CacheTimed  // Heroes cached for duration
    }

    return CacheTimed
}

func (p *DefaultCachePolicy) GetCacheDuration(entity *Entity) int64 {
    // Only for CacheTimed mode
    switch entity.Type {
    case EntityTypeBuilding:
        return 12000  // ~10 minutes @ 20Hz

    case EntityTypeUnit:
        unit := entity.GetComponent(ComponentUnit).(*UnitComponent)
        if unit.IsHero {
            return 36000  // ~30 minutes
        }
        return 6000  // ~5 minutes
    }

    return 6000  // Default 5 minutes
}
```

**Cache Update Logic:**

```go
func (s *VisionSystem) updateVisionCache(
    cache *VisionCache,
    visionMgr *VisionManager,
    tick int64,
) {
    // 1. Update explored hexes (additive - never remove)
    for hexPos, visibility := range visionMgr.VisibleHexes {
        if visibility == VisibilityVisible {
            explored := cache.ExploredHexes[hexPos]
            if explored.FirstSeen == 0 {
                explored.FirstSeen = tick
            }
            explored.LastSeen = tick
            explored.TerrainType = s.getTerrainType(hexPos)
            cache.ExploredHexes[hexPos] = explored
        }
    }

    // 2. Handle entities entering vision
    for entityID, snapshot := range visionMgr.VisibleEntities {
        entity := s.getEntity(entityID)

        // Remove from cache if re-enters vision
        if cached := cache.CachedEntities[entityID]; cached != nil {
            delete(cache.CachedEntities, entityID)  // Purge on re-vision
        }
    }

    // 3. Handle entities leaving vision (add to cache)
    for entityID, prevSnapshot := range visionMgr.PreviouslyVisible {
        // Check if still visible
        if _, visible := visionMgr.VisibleEntities[entityID]; visible {
            continue  // Still visible, skip
        }

        entity := s.getEntity(entityID)
        if entity == nil {
            continue  // Entity destroyed
        }

        // Should we cache this entity?
        if !s.cachePolicy.ShouldCache(entity) {
            continue
        }

        // Create cache entry
        cacheMode := s.cachePolicy.GetCacheMode(entity)
        expiresAt := int64(0)

        if cacheMode == CacheTimed {
            duration := s.cachePolicy.GetCacheDuration(entity)
            expiresAt = tick + duration
        }

        cached := &CachedEntitySnapshot{
            EntityID:    entityID,
            EntityType:  entity.Type,
            Owner:       entity.Owner,
            Position:    prevSnapshot.Position,
            Components:  make(map[ComponentType]Component),
            FirstSeen:   prevSnapshot.FirstSeen,
            LastSeen:    tick,
            LastUpdated: tick,
            CacheMode:   cacheMode,
            ExpiresAt:   expiresAt,
            IsStale:     true,
        }

        // Copy essential components only
        cachedComponents := s.cachePolicy.GetCachedComponents(entity)
        for _, ctype := range cachedComponents {
            if comp, ok := entity.Components[ctype]; ok {
                cached.Components[ctype] = deepCopyComponent(comp)
            }
        }

        cache.CachedEntities[entityID] = cached
    }

    cache.LastUpdated = time.Now()
    cache.Version++
}
```

**Pruning Timed Entries:**

```go
func (s *VisionSystem) PruneExpiredCacheEntries(tick int64) {
    for empireID, cache := range s.visionCaches {
        pruned := false

        for entityID, cached := range cache.CachedEntities {
            // Check if timed cache expired
            if cached.CacheMode == CacheTimed && cached.ExpiresAt > 0 {
                if tick >= cached.ExpiresAt {
                    delete(cache.CachedEntities, entityID)
                    pruned = true
                }
            }

            // Check if entity no longer exists
            if s.getEntity(entityID) == nil {
                delete(cache.CachedEntities, entityID)
                pruned = true
            }
        }

        if pruned {
            s.datastore.SaveVisionCache(context.Background(), cache)
        }
    }
}

// Run periodically (e.g., every 1000 ticks = ~50 seconds)
func (s *VisionSystem) Tick(tick int64) {
    // ... normal vision calculation ...

    if tick % 1000 == 0 {
        s.PruneExpiredCacheEntries(tick)
    }
}
```

**Player Join - Send Cached Data:**

```go
func (s *SyncSystem) OnPlayerJoin(client *ClientSyncManager, currentTick int64) {
    // Load vision cache from database
    cache, err := s.visionCacheDatastore.LoadVisionCache(ctx, client.EmpireID)
    if err != nil {
        cache = s.newEmptyCache(client.EmpireID)
    }

    // 1. Send explored hexes (fog of war state)
    s.sendExploredHexes(client, cache)

    // 2. Send cached entities (ONLY on connection)
    visionMgr := s.visionSystem.GetVisionManager(client.EmpireID)

    for entityID, cached := range cache.CachedEntities {
        // Skip if currently visible (will be sent via normal vision)
        if _, visible := visionMgr.VisibleEntities[entityID]; visible {
            continue
        }

        // Send cached snapshot with IsStale flag
        s.sendCachedEntitySnapshot(client, cached)
        client.knownEntities[entityID] = true
    }

    // 3. Send current visible entities
    for entityID := range visionMgr.VisibleEntities {
        entity := s.entities[entityID]
        snapshot := s.createSnapshot(entity, currentTick)
        snapshot.IsStale = false  // Current data
        client.entitySnapshots[entityID] = snapshot
        client.knownEntities[entityID] = true
        s.sendEntitySnapshot(client, snapshot)
    }
}
```

**Cache Events - When to Send:**

```go
// Only send cached entities in these scenarios:

// 1. Player connects (initial load)
func (s *SyncSystem) OnPlayerJoin(client *ClientSyncManager, tick int64) {
    // Send all cached entities (shown above)
}

// 2. Entity added to cache (leaves vision)
func (s *SyncSystem) OnEntityLeaveVision(client *ClientSyncManager, entityID EntityID) {
    cache := s.visionCaches[client.EmpireID]

    // Check if entity was added to cache
    if cached := cache.CachedEntities[entityID]; cached != nil {
        // Send cache notification
        s.sendEntityCached(client, cached)
    } else {
        // Not cached - just disappear
        s.sendEntityDisappeared(client, entityID)
    }
}

// 3. Entity re-enters vision (purge from cache)
func (s *SyncSystem) OnEntityEnterVision(client *ClientSyncManager, entityID EntityID) {
    cache := s.visionCaches[client.EmpireID]

    // Remove from cache if exists
    if cached := cache.CachedEntities[entityID]; cached != nil {
        delete(cache.CachedEntities, entityID)
        // Don't notify client - just update with current data
    }

    // Send current entity data
    entity := s.entities[entityID]
    snapshot := s.createSnapshot(entity, tick)
    snapshot.IsStale = false
    s.sendEntitySnapshot(client, snapshot)
}

// NOT sent during normal sync - cached entities are static until vision changes
```

**Client Message Formats:**

```json
// Explored hexes (on join)
{
  "type": "explored_hexes",
  "hexes": [
    {"hex": [10, 5], "terrain": "plains"},
    {"hex": [11, 6], "terrain": "forest"}
  ]
}

// Cached entity snapshot (on join or when added to cache)
{
  "type": "entity_snapshot",
  "entity": {
    "id": "settlement_abc",
    "type": "building",
    "owner": "enemy_empire_123",
    "pos": [50, 50],
    "is_stale": true,  // Render differently (faded/grayed)
    "cache_mode": "permanent",
    "last_seen": 5000,
    "data": {
      "building_type": "settlement",
      "health": 5000
    }
  }
}

// Entity re-enters vision (update from stale to current)
{
  "type": "entity_snapshot",
  "entity": {
    "id": "settlement_abc",
    "is_stale": false,  // Now current - render normally
    "health": 4200,     // Updated value
    "last_seen": 10000
  }
}

// Entity leaves vision and is cached
{
  "type": "entity_cached",
  "entity_id": "hero_unit_xyz",
  "cache_mode": "timed",
  "expires_at": 16000
}
```

**Database Schema:**

```sql
-- Explored hexes (persistent fog of war)
CREATE TABLE vision_cache_explored_hexes (
    empire_id VARCHAR(255) NOT NULL,
    hex_q INT NOT NULL,
    hex_r INT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    terrain_type VARCHAR(50),
    PRIMARY KEY (empire_id, hex_q, hex_r),
    INDEX idx_empire (empire_id)
);

-- Cached entity snapshots
CREATE TABLE vision_cache_entities (
    empire_id VARCHAR(255) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    owner VARCHAR(255),
    pos_q INT NOT NULL,
    pos_r INT NOT NULL,
    first_seen BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    last_updated BIGINT NOT NULL,
    cache_mode ENUM('permanent', 'timed') NOT NULL,
    expires_at BIGINT DEFAULT 0,
    is_stale BOOLEAN DEFAULT TRUE,
    components_json JSON,
    PRIMARY KEY (empire_id, entity_id),
    INDEX idx_empire (empire_id),
    INDEX idx_expires (expires_at),
    INDEX idx_cache_mode (cache_mode)
);
```

**Configuration:**

```yaml
vision_cache:
  enable: true

  # What to cache
  cache_buildings: true
  cache_settlements_permanent: true
  cache_hero_units: true
  cache_endgame_units_permanent: true
  cache_npcs_permanent: true

  # Timed cache durations (in ticks @ 20Hz)
  building_cache_duration: 12000      # ~10 minutes
  hero_cache_duration: 36000          # ~30 minutes
  unit_cache_duration: 6000           # ~5 minutes

  # Maintenance
  prune_interval: 1000                # Prune every 1000 ticks (~50 sec)
  save_interval: 60                   # Save every 60 seconds
  save_on_disconnect: true

  # Limits
  max_cached_entities_per_empire: 1000
  max_explored_hexes_per_empire: 100000
```

**VisionCache Benefits:**

✅ **Persistent Strategic Memory**: Scouted settlements remembered between sessions
✅ **Fog of War History**: Explored terrain persists forever (additive)
✅ **Timescale-Based Retention**: Permanent (settlements, NPCs) or timed (heroes, buildings)
✅ **Stale Data Rendering**: Client visually distinguishes cached vs current data
✅ **Automatic Purging**: Re-entering vision removes from cache, updates with current data
✅ **Selective Caching**: Only important entities, not all units
✅ **Database Persistence**: Survives server restarts and player reconnects
✅ **Automatic Pruning**: Timed entries expire, destroyed entities cleaned up

#### System Benefits

✅ **Server-Authoritative**: Server is single source of truth
✅ **Vision-Filtered**: Clients only receive updates for entities they can see
✅ **Anti-ESP Protection**: No historical leakage, entities enter vision with current state only
✅ **Persistent Knowledge**: VisionCache remembers scouted locations across sessions
✅ **Bandwidth Efficient**: Delta updates with compression and prioritization
✅ **Reconnect Handling**: Automatic catch-up via delta log replay
✅ **Low Allocation**: sync.Pool reuses memory, near-zero GC pressure
✅ **Strategic Gameplay**: Fog of war, shared vision, and persistent scouting knowledge
✅ **Scalable**: Spatial grid enables fast vision queries
✅ **Smooth Client Experience**: 60 FPS interpolation on 20 Hz updates

## Project Structure

```
mmorts/
├── ARCHITECTURE.md                    # This file
├── SOCIAL_LAYER_API.md                # Social layer API specification
├── go.mod                             # Root module definition
├── go.sum                             # Dependency checksums
├── Dockerfile                         # Docker container definition
├── docker compose.yml                 # Docker compose for dev environment
├── deploy.sh                          # Deployment script
├── update.sh                          # Update/rebuild script
│
├── cmd/
│   └── server/
│       └── main.go                    # Server entry point
│
├── internal/
│   ├── server/
│   │   ├── server.go                  # Main server implementation
│   │   ├── session.go                 # Game session management
│   │   └── auth.go                    # JWT validation & blacklist checking
│   │
│   ├── gamemap/
│   │   ├── map.go                     # GameMap structure
│   │   ├── chunk.go                   # Hex chunk management
│   │   └── spatial_grid.go            # Spatial grid implementation
│   │
│   ├── datastore/
│   │   ├── interface.go               # Datastore interface definition
│   │   ├── mysql/
│   │   │   ├── adapter.go             # MySQL implementation
│   │   │   └── schema.sql             # Database schema
│   │   └── redis/
│   │       └── cache.go               # Redis cache operations
│   │
│   ├── ecs/
│   │   ├── components/
│   │   │   ├── position.go            # Position component
│   │   │   ├── owner.go               # Owner component
│   │   │   ├── building.go            # Building component
│   │   │   ├── unit.go                # Unit component
│   │   │   ├── stats.go               # Stats component
│   │   │   └── vision.go              # Vision component
│   │   ├── systems/
│   │   │   ├── movement.go            # Movement system
│   │   │   ├── combat.go              # Combat system
│   │   │   ├── production.go          # Production tick system
│   │   │   ├── spatial_index.go       # Spatial grid update system
│   │   │   ├── vision.go              # Vision calculation system
│   │   │   └── client_sync.go         # Client synchronization system
│   │   └── world.go                   # ECS world setup
│   │
│   ├── vision/
│   │   ├── manager.go                 # Vision manager (per empire)
│   │   ├── shared_vision.go           # Shared vision relationships
│   │   └── types.go                   # Vision-related types
│   │
│   ├── sync/
│   │   ├── delta_log.go               # Delta log ring buffer
│   │   ├── client_manager.go          # Client sync manager
│   │   ├── snapshot.go                # Snapshot management with pooling
│   │   └── reconcile.go               # Snapshot reconciliation
│   │
│   ├── commands/
│   │   ├── player_commands.go         # Player command handlers
│   │   └── server_commands.go         # Server command handlers
│   │
│   └── network/
│       ├── handlers.go                # Network message handlers
│       └── protocol.go                # Protocol definitions
│
├── pkg/
│   └── models/
│       ├── player.go                  # Player data model (from User.js)
│       ├── session.go                 # Session data model
│       ├── building.go                # Building model
│       └── spatial.go                 # Spatial grid types
│
├── external/                          # External dependencies (git submodules)
│   ├── udp_network/
│   ├── ecscore/
│   ├── commandcore/
│   ├── cache/
│   ├── hexcore/
│   ├── inventory/
│   ├── production/
│   └── social/
│       ├── SOCIAL_LAYER_API.md      # Social layer API specification
│       └── (implementation files)   # Group, chat, mail systems
│
└── configs/
    ├── config.yaml                    # Server configuration
    ├── database.yaml                  # Database configuration
    └── docker.yaml                    # Docker-specific config
```

## Initial Implementation Scope

### Phase 1: Basic Server Setup & Infrastructure
- [ ] Go module initialization with external packages
- [ ] Docker setup for dev environment (shared_services network)
- [ ] Configuration management (YAML files)
- [ ] Logging setup
- [ ] Basic server startup with UDP network

### Phase 2: Authentication & Connection
- [ ] JWT validation logic (signature, expiry)
- [ ] Redis blacklist check on connection
- [ ] Player model from User.js JWT claims
- [ ] Connection callbacks (join/leave)
- [ ] Online players set in Redis

### Phase 3: GameMap & Spatial Grid
- [ ] GameMap structure with hex chunks
- [ ] Initialize blank open map (all Space hexes)
- [ ] SpatialGrid implementation
- [ ] Spatial grid query methods (range lookups)
- [ ] Map serialization for web client rendering

### Phase 4: Basic ECS Integration
- [ ] ECS world initialization
- [ ] Player entity creation on join
- [ ] Position, Owner, Stats components
- [ ] Spatial index system (updates grid on entity move)
- [ ] Basic sync system (send state to clients)

### Phase 5: Building & Production System
- [ ] Building component with production manager
- [ ] Recipe registry setup (unit training, construction)
- [ ] Production tick system
- [ ] Inventory integration for buildings
- [ ] Production command handlers

### Phase 6: Datastore Implementation
- [ ] Datastore interface definition
- [ ] MySQL adapter implementation
- [ ] Redis cache operations
- [ ] Connection pooling
- [ ] Schema creation & migrations

### Phase 7: Docker Deployment
- [ ] Dockerfile creation
- [ ] docker compose.yml for local dev (shared_services network)
- [ ] deploy.sh script (build, tag, deploy)
- [ ] update.sh script (pull, rebuild, restart)
- [ ] Environment variable management

### Phase 8: Integration Testing
- [ ] End-to-end connection test with JWT
- [ ] Session join/leave test
- [ ] Map rendering data test
- [ ] Spatial grid query test
- [ ] Data persistence verification

## Configuration

### Server Configuration
```yaml
server:
  address: ":8080"
  transport: "websocket"  # WebSocket for web clients
  max_clients: 1000
  tick_rate: 20  # ticks per second (50ms per tick)
  encryption: true

jwt:
  secret: "${JWT_SECRET}"  # Shared secret with GoLoginServer
  issuer: "login.gravitas-games.com"  # GoLoginServer issuer
  max_age: 86400  # 24 hours

redis:
  address: "redis:6379"  # Docker service name
  db: 0
  password: "${REDIS_PASSWORD}"
  blacklist_key_prefix: "blacklist:"
  online_players_key: "online_players"

mysql:
  host: "mariadb"  # Docker service name
  port: 3306
  database: "mmorts"
  username: "${MYSQL_USER}"
  password: "${MYSQL_PASSWORD}"
  max_connections: 100

map:
  chunk_radius: 9  # 169 hexes per chunk
  initial_chunks: 7  # Center + 6 neighbors (Pocket)
  spatial_grid_cell_size: 10  # Grid cells are 10x10 hexes

production:
  recipe_path: "./configs/recipes.yaml"
  enable_modifiers: true
```

### Docker Configuration

**Dockerfile:**
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs
EXPOSE 8080
CMD ["./server"]
```

**docker compose.yml (dev environment):**
```yaml
version: '3.8'

services:
  mmorts-server:
    build: .
    container_name: mmorts-server
    ports:
      - "8080:8080"
    environment:
      - JWT_SECRET=${JWT_SECRET}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - MYSQL_USER=${MYSQL_USER}
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
    depends_on:
      - redis
      - mariadb
    networks:
      - shared_services  # Connect to shared network with go-spa-server

  redis:
    image: redis:7-alpine
    container_name: mmorts-redis
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    networks:
      - shared_services

  mariadb:
    image: mariadb:10.6
    container_name: mmorts-mariadb
    environment:
      - MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
      - MYSQL_DATABASE=mmorts
      - MYSQL_USER=${MYSQL_USER}
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
    volumes:
      - mariadb_data:/var/lib/mysql
      - ./internal/datastore/mysql/schema.sql:/docker-entrypoint-initdb.d/schema.sql
    networks:
      - shared_services

volumes:
  redis_data:
  mariadb_data:

networks:
  shared_services:
    external: true  # Connects to existing shared_services network
```

## Deployment Strategy

### Development Environment
1. Shared Docker network: `docker network create shared_services`
2. go-spa-server runs on shared_services network (serves static web client)
3. mmorts-server runs on shared_services network
4. Redis and MariaDB on shared_services network
5. All services can communicate via Docker DNS
6. GoLoginServer hosted externally at login.gravitas-games.com

### Deployment Scripts

**deploy.sh:**
```bash
#!/bin/bash
# Build and deploy MMORTS server

set -e

echo "Building Docker image..."
docker build -t mmorts-server:latest .

echo "Stopping existing container..."
docker compose down

echo "Starting services..."
docker compose up -d

echo "Checking health..."
sleep 5
docker compose ps

echo "Deployment complete!"
```

**update.sh:**
```bash
#!/bin/bash
# Update and rebuild MMORTS server

set -e

echo "Pulling latest changes..."
git pull

echo "Updating submodules..."
git submodule update --init --recursive

echo "Rebuilding and restarting..."
docker compose build
docker compose up -d

echo "Update complete!"
```

### Production Deployment (Future)
- Separate machines for game servers, database, cache
- Load balancer for multiple game server instances
- Database replication (master-slave)
- Redis Sentinel for high availability
- Kubernetes orchestration (optional)

## Data Flow

### Player Joins Session (with JWT)

```
Client          go-spa-server   GoLoginServer   Game Server      Redis      MySQL    ECS
  |                  |                |              |             |          |       |
  |--Login---------->|                |              |             |          |       |
  |                  |--Auth--------->|              |             |          |       |
  |                  |<--JWT----------|              |             |          |       |
  |<--JWT------------|                |              |             |          |       |
  |                                   |              |             |          |       |
  |--WebSocket Connect (JWT in header)------------->|             |          |       |
  |                                   |              |--Validate JWT          |       |
  |                                   |              |--Verify Issuer         |       |
  |                                   |              |--Check Blacklist------>|       |
  |                                   |              |<--Not Blacklisted------|       |
  |                                   |              |--GetPlayer-------------------->|
  |                                   |              |<--PlayerData-------------------|
  |                                   |              |--AddOnline-->|                 |
  |                                   |              |<--OK---------|                 |
  |                                   |              |--CreateEntity---------------------->|
  |                                   |              |<--EntityID------------------------- |
  |                                   |              |--GetMapData->                       |
  |                                   |              |<--MapChunks--|                      |
  |<--JoinConfirm-----------------------------------------|                                |
  |<--MapData---------------------------------------------|                                |
```

**Authentication Flow Details:**
1. Client loads from go-spa-server (static files)
2. Client posts login credentials to go-spa-server endpoint
3. go-spa-server proxies authentication request to GoLoginServer
4. GoLoginServer validates credentials and issues JWT
5. Client receives and stores JWT in localStorage
6. Client opens WebSocket connection to game server with JWT in connection header
7. Game server validates JWT signature and checks issuer matches GoLoginServer
8. Game server checks Redis for blacklisted/revoked tokens
9. Valid JWT allows player to join game session

### Building Production Flow

```
Player              Game Server         Production      Inventory      Redis
  |                     |                  Manager          |            |
  |--Train("soldier")--->|                    |             |            |
  |                     |--StartProd-------->|             |            |
  |                     |                    |--Consume--->|            |
  |                     |                    |<--OK--------|            |
  |                     |                    |             |            |
  |                     |<--JobID------------|             |            |
  |<--ProductionStarted-|                    |             |            |
  |                     |                    |             |            |
  |                [Tick Loop Updates]       |             |            |
  |                     |                    |             |            |
  |                     |--Tick------------->|             |            |
  |                     |                    |--Complete-->|            |
  |                     |                    |<--OK--------|            |
  |                     |<--Event------------|             |            |
  |<--ProductionDone----|                    |             |            |
  |                     |--PublishStats-------------------->            |
```

## Social Layer API Specification

The social layer is a portable, generic package located in `external/social/`. It uses Redis pub/sub (via cache package) for real-time messaging and MySQL for persistence.

### Generic Design

The social layer uses **"groups"** as a generic concept instead of game-specific terminology:
- **In MMORTS**: Groups = Alliances
- **In other games**: Groups could be clans, teams, guilds, etc.
- This makes the package portable across multiple game projects

### API Surface

See [external/social/SOCIAL_LAYER_API.md](external/social/SOCIAL_LAYER_API.md) for complete specification. Key features:

```go
type SocialLayer interface {
    // Player presence
    SetPlayerOnline(playerID string, metadata PresenceMetadata) error
    SetPlayerOffline(playerID string) error
    GetOnlinePlayers() ([]string, error)
    GetPlayerPresence(playerID string) (PresenceStatus, error)

    // Friends
    SendFriendRequest(from, to string) error
    AcceptFriendRequest(playerID, friendID string) error
    GetFriends(playerID string) ([]string, error)

    // Chat (transient, non-persistent)
    SendPrivateChat(from, to string, message ChatMessage) error
    SendGroupChat(groupID string, from string, message ChatMessage) error
    BroadcastAnnouncement(message ChatMessage) error

    // Mail (persistent, offline delivery)
    SendMail(from, to string, mail Mail) error
    SendGroupMail(groupID string, from string, mail Mail) error
    GetInbox(playerID string, pagination Pagination) ([]Mail, error)

    // Groups (generic: alliances, clans, teams, etc.)
    CreateGroup(name string, founderID string, options GroupOptions) (string, error)
    InvitePlayer(groupID string, inviterID, inviteeID string) error
    AcceptGroupInvitation(playerID, groupID string) error
    LeaveGroup(playerID string) error
    GetGroupMembers(groupID string) ([]GroupMember, error)

    // Subscribe to events
    Subscribe(playerID string, handler func(SocialEvent)) error
}
```

**Key Differences from Initial Design:**
- Uses "groups" instead of "alliances" for portability
- Separates chat (transient) from mail (persistent)
- MySQL datastore interface for persistence
- Connection management handled by caller (shared DB connection pool)

## Future Considerations

### Features to Add Later
- Unit movement with pathfinding (hexcore)
- Combat system with range calculations (spatial grid)
- Building construction with production system
- Resource gathering (inventory integration)
- Tech tree / research (production system)
- Fog of war (per-player visibility)
- Player chat (social layer)
- Alliance/guild system (social layer)
- Match-making
- Replay system
- Admin tools

### Scalability Considerations
- Horizontal scaling: Multiple game servers, session distribution
- Database: Read replicas for player data
- Redis: Clustering for cache and pub/sub
- Message queue: Kafka/RabbitMQ for async processing
- Microservices: Split social layer, matchmaking, analytics

### Security Considerations
- JWT validation with shared secret
- Token blacklist in Redis for bans
- Command rate limiting (commandcore)
- Input validation and sanitization
- Anti-cheat measures (server-authoritative)
- DDoS protection (rate limiting, connection limits)

## Development Guidelines

1. **Modularity**: Keep concerns separated (network, game logic, persistence)
2. **Testability**: Write unit tests for all business logic
3. **Configuration**: Use config files, avoid hardcoded values
4. **Logging**: Structured logging with appropriate levels
5. **Error Handling**: Graceful error handling with proper context
6. **Documentation**: Document public APIs and complex logic
7. **Spatial Efficiency**: Always use spatial grid for range queries
8. **Entity-Map Separation**: Map tiles != entities, entities reference map positions

## Answered Questions

1. **Session Model**: Start with single global session, expand to multiple later
2. **Map Size**: Chunks with radius 9 (169 hexes each), start with 1 Pocket (7 chunks)
3. **Player Capacity**: 1000 max clients initially (configurable)
4. **Persistence Strategy**: Periodic snapshots to MySQL, real-time state in memory
5. **Authentication**: JWT from GoLoginServer (login.gravitas-games.com), validated by game server
6. **Deployment**: Docker containers on shared_services network (dev), separate machines (prod)

## Dependencies and Versions

- Go: 1.21+
- Redis: 7.0+
- MySQL/MariaDB: 8.0+/10.6+
- Docker: 20.10+
- Docker Compose: 2.0+
- External packages: See go.mod in each external directory

## Notes

- All external packages are referenced via local replace directives in go.mod
- UDP Network has non-commercial license restrictions
- ECS scheduler provides deterministic tick execution for game logic
- Command processor handles rate limiting and validation automatically
- Cache package provides automatic Redis synchronization via pub/sub
- Spatial grid is essential for performance with many entities
- Production system examples in `external/production/examples/building_construction.go`
- Initial map is blank (all Space hexes) for web client rendering testing
- Web client will be served by go-spa-server on shared_services network
- JWT tokens issued by GoLoginServer (login.gravitas-games.com)
- JWT tokens contain full user model (user_id, username, email, permissions, activated status)
- JWT shared secret must match between GoLoginServer and game server
- Blacklisted tokens prevent banned players from connecting
- Client sends JWT in WebSocket connection header
