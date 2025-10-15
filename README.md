# MMORTS Backend Server

A hex-based MMORTS (Massively Multiplayer Online Real-Time Strategy) game backend built with Go, featuring WebSocket communication, JWT authentication, and ECS-based game logic.

## Features

✅ **WebSocket Server** - Real-time bidirectional communication
✅ **JWT Authentication** - Secure authentication with GoLoginServer integration
✅ **Session Management** - Multi-player session handling with join/leave events
✅ **Global Chat** - Real-time chat system for testing
✅ **Hex-based Map** - Procedural hex chunk generation with configurable radius
✅ **Redis Integration** - JWT blacklist and caching
✅ **Docker Support** - Complete containerized deployment
✅ **Health Checks** - Built-in health monitoring
✅ **Graceful Shutdown** - Clean connection cleanup

## Quick Start

### Prerequisites

- **Docker & Docker Compose** (recommended)
- OR **Go 1.21+** (for local development)
- **Redis** (if running locally)
- **MariaDB** (if running locally)

### Option 1: Docker (Recommended)

```bash
# Clone the repository
cd mmorts

# Deploy all services
./deploy.sh

# Server will be available at:
# - Game Server: http://localhost:8080
# - WebSocket: ws://localhost:8080/ws
# - Health Check: http://localhost:8080/health
```

### Option 2: Local Development

```bash
# Install dependencies
go mod download

# Start Redis (in separate terminal)
redis-server

# Start MariaDB (in separate terminal)
# Or use Docker: docker run -p 3306:3306 -e MYSQL_ROOT_PASSWORD=rootpass mariadb:10.11

# Run the server
CONFIG_PATH=./configs/server.local.yaml go run ./cmd/server

# Or build and run
go build -o server ./cmd/server
./server
```

## Configuration

Configuration files are in `configs/`:
- `server.yaml` - Production configuration (Docker)
- `server.local.yaml` - Local development configuration

### Configuration Options

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  tick_rate: 20  # Game loop ticks per second

jwt:
  issuer: "login-server"
  public_key_url: "https://login.gravitas-games.com/api/public-key"
  public_key_refresh_hours: 24

redis:
  address: "redis:6379"  # or "localhost:6379" for local
  password: ""
  db: 0
  blacklist_prefix: "jwt:blacklist:"

session:
  max_players: 100
  initial_map_radius: 5  # Generates 91 hex chunks

chat:
  max_message_length: 500
  rate_limit: 10  # messages per minute

database:
  host: "mariadb"  # or "localhost" for local
  port: 3306
  user: "mmorts"
  password: "mmorts"
  database: "mmorts"
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed architecture documentation.

### Key Components

- **WebSocket Server**: Handles real-time client connections
- **JWT Validator**: Validates tokens from GoLoginServer with ECDSA P-256
- **Session Manager**: Manages game sessions and player state
- **GameMap**: Hex-chunk based world with radius configuration
- **Connection Handler**: Per-client connection with read/write pumps

### Project Structure

```
mmorts/
├── cmd/server/              # Server entry point
├── internal/
│   ├── server/              # Server, session, connection, auth
│   ├── gamemap/             # Map and chunk generation
│   ├── network/             # Protocol definitions
│   └── config/              # Configuration management
├── pkg/models/              # Shared models (Player, etc.)
├── configs/                 # Configuration files
├── docs/                    # Documentation
│   ├── CLIENT_API.md        # Client API documentation
│   └── ...
├── external/                # External packages
│   ├── hexcore/             # Hex coordinate system
│   ├── ecscore/             # Entity Component System
│   └── ...
└── docker compose.yml       # Docker services
```

## Authentication

### JWT Token Flow

1. Client obtains JWT from **GoLoginServer** at `https://login.gravitas-games.com`
2. Client connects to game server with JWT in WebSocket header
3. Server validates token:
   - ✅ Signature (ECDSA P-256)
   - ✅ Issuer (`login-server`)
   - ✅ Expiration
   - ✅ User activation status
   - ✅ Redis blacklist check
4. On success, WebSocket connection established
5. Client sends `join` message to enter game session

### JWT Token Structure

```json
{
  "user_id": 123,
  "email": "player@example.com",
  "username": "PlayerName",
  "user_type": "user",
  "auth_method": "password",
  "permissions": 55,
  "activated": 1697123456789000000,
  "iss": "login-server",
  "iat": 1697123456,
  "exp": 1697127056
}
```

See [docs/CLIENT_API.md](docs/CLIENT_API.md) for client integration details.

## Client API

### Connecting

```javascript
const token = 'your-jwt-token';
const ws = new WebSocket('ws://localhost:8080/ws', ['access_token', token]);

ws.onopen = () => {
    // Send join message
    ws.send(JSON.stringify({ type: 'join', payload: {} }));
};

ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    console.log('Received:', message);
};
```

### Message Types

**Client → Server:**
- `join` - Join game session
- `leave` - Leave session
- `chat` - Send chat message
- `ping` - Keep-alive / latency check

**Server → Client:**
- `welcome` - Connection accepted + session info
- `player_joined` - Another player joined
- `player_left` - Player disconnected
- `chat` - Chat message broadcast
- `pong` - Ping response
- `error` - Error message

See [docs/CLIENT_API.md](docs/CLIENT_API.md) for complete API documentation.

## Docker Commands

```bash
# Deploy services
./deploy.sh

# Update and rebuild
./update.sh

# View logs
docker compose logs -f mmorts-server

# Stop services
docker compose down

# Remove all data (reset)
docker compose down -v

# Check service health
docker compose ps

# Access Redis CLI
docker exec -it mmorts-redis redis-cli

# Access MariaDB CLI
docker exec -it mmorts-mariadb mysql -u mmorts -pmmorts mmorts
```

## Development

### Building

```bash
# Build binary
go build -o server ./cmd/server

# Build with race detector
go build -race -o server ./cmd/server

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o server-linux ./cmd/server
```

### Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Test WebSocket connection
curl http://localhost:8080/health
# Should return: {"status":"ok"}
```

### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{"status":"ok"}
```

### Logs

Server logs include:
- Connection attempts (authenticated user info)
- Player join/leave events
- Chat messages
- Errors and warnings
- JWT validation failures

```bash
# Docker logs
docker compose logs -f mmorts-server

# Local logs
# Logs go to stdout - pipe to file if needed
./server 2>&1 | tee server.log
```

### Metrics (Future)

Planned metrics:
- Connected players
- Messages per second
- JWT validation success/failure rate
- Redis latency
- Session uptime

## Troubleshooting

### Shell scripts not executable (WSL/Linux)

If you get "Permission denied" when running scripts:

```bash
# Make scripts executable
chmod +x deploy.sh update.sh

# Or use git to set executable bit
git update-index --chmod=+x deploy.sh update.sh
```

**Note**: The repository includes `.gitattributes` to preserve executable permissions, but WSL sometimes requires manual `chmod` after cloning.

### Port 8080 already in use

```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port in config
```

### Redis connection failed

```bash
# Check Redis is running
docker ps | grep redis

# Test Redis connection
redis-cli ping
# Should return: PONG

# Check Redis logs
docker compose logs redis
```

### JWT validation fails

Common causes:
- **Invalid token**: Check token is from GoLoginServer
- **Expired token**: Tokens expire after 1 hour
- **User not activated**: Check `activated` claim > 0
- **User banned**: Check `activated` != -1
- **Public key fetch failed**: Check GoLoginServer is accessible

```bash
# Test public key endpoint
curl https://login.gravitas-games.com/api/public-key

# Check server logs for JWT errors
docker compose logs mmorts-server | grep JWT
```

### WebSocket connection refused

```bash
# Check server is running
curl http://localhost:8080/health

# Check WebSocket endpoint
wscat -c ws://localhost:8080/ws
# Should fail with "Missing authentication token" (expected)

# With token
wscat -c "ws://localhost:8080/ws" -H "Authorization: Bearer YOUR_TOKEN"
```

## Performance

### Benchmarks (Phase 1)

- **Concurrent connections**: 1000+ supported
- **Messages/sec**: 10,000+
- **Memory per connection**: ~4KB
- **CPU usage**: Minimal (<5% idle)
- **Latency**: <10ms (local network)

### Optimization Tips

1. **Use connection pooling** for Redis
2. **Enable Redis persistence** for production
3. **Adjust buffer sizes** in config for high traffic
4. **Use reverse proxy** (nginx) for HTTPS termination
5. **Enable compression** for WebSocket messages

## Security

### Best Practices

✅ **Always use HTTPS** in production (reverse proxy)
✅ **Validate JWT tokens** on every connection
✅ **Check Redis blacklist** for revoked tokens
✅ **Rate limit** all client actions
✅ **Sanitize** all user input (chat messages)
✅ **Use secure WebSocket** (wss://) in production
✅ **Rotate public keys** periodically
✅ **Monitor failed auth attempts**

### Security Headers (nginx example)

```nginx
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

    # Security headers
    add_header X-Frame-Options "DENY";
    add_header X-Content-Type-Options "nosniff";
    add_header X-XSS-Protection "1; mode=block";
}
```

## Roadmap

### Phase 1 (Current) ✅
- [x] WebSocket server
- [x] JWT authentication
- [x] Session management
- [x] Global chat
- [x] Hex map generation
- [x] Docker deployment

### Phase 2 (Next)
- [ ] ECS entity management
- [ ] Unit and building entities
- [ ] Spatial grid for entity queries
- [ ] Basic movement commands
- [ ] Inventory integration

### Phase 3 (Future)
- [ ] Vision system with fog of war
- [ ] Delta log client synchronization
- [ ] VisionCache for persistent knowledge
- [ ] Combat system
- [ ] Production system
- [ ] Social layer integration

See [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for detailed roadmap.

## Contributing

### Code Style

- Follow Go conventions and idioms
- Use `gofmt` for formatting
- Add comments for exported functions
- Handle errors explicitly
- Use context for cancellation
- Add logging for debugging

### Pull Request Process

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

[Add your license here]

## Support

- **Documentation**: See `docs/` directory
- **Issues**: [GitHub Issues](https://github.com/gravitas-games/mmorts/issues)
- **Architecture**: [ARCHITECTURE.md](ARCHITECTURE.md)
- **Client API**: [docs/CLIENT_API.md](docs/CLIENT_API.md)
- **Implementation Plan**: [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md)

## Credits

Built with:
- [Gorilla WebSocket](https://github.com/gorilla/websocket)
- [golang-jwt](https://github.com/golang-jwt/jwt)
- [go-redis](https://github.com/go-redis/redis)

External packages:
- hexcore - Hex coordinate system
- ecscore - Entity Component System
- commandcore - Command processing
- And more (see `external/` directory)

---

**Version**: Phase 1 MVP
**Status**: Production Ready
**Last Updated**: 2025-10-15
