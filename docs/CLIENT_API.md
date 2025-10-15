# MMORTS Client API Documentation

## Overview

This document describes the WebSocket API for connecting to the MMORTS game server. This API is designed for web client developers who will be implementing the game client in a separate repository.

**Protocol**: WebSocket (JSON messages)
**Endpoint**: `ws://<server-address>:8080/ws`
**Message Format**: JSON

---

## Table of Contents

1. [Connection Setup](#connection-setup)
2. [Authentication](#authentication)
3. [Message Protocol](#message-protocol)
4. [Client → Server Messages](#client--server-messages)
5. [Server → Client Messages](#server--client-messages)
6. [Connection Lifecycle](#connection-lifecycle)
7. [Error Handling](#error-handling)
8. [Keep-Alive](#keep-alive)
9. [Example Implementation](#example-implementation)

---

## Connection Setup

### WebSocket Connection

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    console.log('Connected to server');
};

ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    handleServerMessage(message);
};

ws.onclose = () => {
    console.log('Disconnected from server');
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};
```

### Authentication with JWT

**Important**: JWT authentication must be included in the WebSocket connection.

```javascript
// Option 1: Using Sec-WebSocket-Protocol header (recommended)
const token = 'your-jwt-token-here';
const ws = new WebSocket('ws://localhost:8080/ws', ['access_token', token]);

// Option 2: As query parameter (alternative)
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);
```

**JWT Token Source**:
- Obtain JWT from GoLoginServer at `https://login.gravitas-games.com`
- Token is valid for 1 hour
- Token contains: `user_id`, `email`, `username`, `permissions`, `activated`
- See [JWT Token Structure](#jwt-token-structure) below

---

## Message Protocol

All messages are JSON objects with a `type` and `payload` field.

### Message Structure

```json
{
  "type": "message_type",
  "payload": { /* type-specific data */ }
}
```

### Sending Messages

```javascript
function sendMessage(type, payload) {
    const message = {
        type: type,
        payload: payload
    };
    ws.send(JSON.stringify(message));
}
```

---

## Client → Server Messages

### 1. Join Session

Sent after connection is established to join the game session.

**Type**: `join`
**Payload**: `{}` (empty object)

```json
{
  "type": "join",
  "payload": {}
}
```

**Example**:
```javascript
sendMessage('join', {});
```

**Response**: Server sends `welcome` message

---

### 2. Leave Session

Sent when player wants to leave the session (optional - connection close also triggers leave).

**Type**: `leave`
**Payload**: `{}` (empty object)

```json
{
  "type": "leave",
  "payload": {}
}
```

**Example**:
```javascript
sendMessage('leave', {});
ws.close();
```

**Response**: Server broadcasts `player_left` to other players

---

### 3. Chat Message

Send a chat message to all players in the session.

**Type**: `chat`
**Payload**:
```typescript
{
  message: string  // Max 500 characters
}
```

**Example**:
```json
{
  "type": "chat",
  "payload": {
    "message": "Hello, world!"
  }
}
```

**Response**: Server broadcasts `chat` message to all players

**Rate Limit**: 10 messages per minute per player (enforced server-side)

---

### 4. Ping

Send a ping to test connection and measure latency.

**Type**: `ping`
**Payload**: `{}` (empty object)

```json
{
  "type": "ping",
  "payload": {}
}
```

**Example**:
```javascript
const startTime = Date.now();
sendMessage('ping', {});

// In message handler:
if (message.type === 'pong') {
    const latency = Date.now() - startTime;
    console.log(`Latency: ${latency}ms`);
}
```

**Response**: Server sends `pong` message

---

## Server → Client Messages

### 1. Welcome

Sent immediately after player joins the session. Contains player info and session status.

**Type**: `welcome`
**Payload**:
```typescript
{
  player_id: string,
  username: string,
  session_id: string,
  session_status: {
    state: string,          // "waiting", "running", "paused"
    player_count: number,
    max_players: number,
    server_tick: number,
    uptime: number          // seconds
  }
}
```

**Example**:
```json
{
  "type": "welcome",
  "payload": {
    "player_id": "123",
    "username": "Alice",
    "session_id": "main",
    "session_status": {
      "state": "waiting",
      "player_count": 1,
      "max_players": 100,
      "server_tick": 0,
      "uptime": 45
    }
  }
}
```

**Client Action**: Store player info, update UI with session status

---

### 2. Player Joined

Broadcast when another player joins the session.

**Type**: `player_joined`
**Payload**:
```typescript
{
  player_id: string,
  username: string,
  email: string
}
```

**Example**:
```json
{
  "type": "player_joined",
  "payload": {
    "player_id": "456",
    "username": "Bob",
    "email": "bob@example.com"
  }
}
```

**Client Action**: Add player to player list, show notification

---

### 3. Player Left

Broadcast when a player leaves the session.

**Type**: `player_left`
**Payload**:
```typescript
{
  player_id: string,
  username: string
}
```

**Example**:
```json
{
  "type": "player_left",
  "payload": {
    "player_id": "456",
    "username": "Bob"
  }
}
```

**Client Action**: Remove player from player list, show notification

---

### 4. Chat Broadcast

Chat message from another player (or yourself).

**Type**: `chat`
**Payload**:
```typescript
{
  player_id: string,
  username: string,
  message: string,
  timestamp: number  // Unix timestamp (seconds)
}
```

**Example**:
```json
{
  "type": "chat",
  "payload": {
    "player_id": "456",
    "username": "Bob",
    "message": "Hello everyone!",
    "timestamp": 1697123456
  }
}
```

**Client Action**: Display chat message in chat UI

---

### 5. Pong

Response to client ping.

**Type**: `pong`
**Payload**:
```typescript
{
  timestamp: number  // Unix timestamp (seconds)
}
```

**Example**:
```json
{
  "type": "pong",
  "payload": {
    "timestamp": 1697123456
  }
}
```

**Client Action**: Calculate latency, update connection status

---

### 6. Session Status Update

Periodic updates about session state (future feature).

**Type**: `session_status`
**Payload**:
```typescript
{
  state: string,
  player_count: number,
  max_players: number,
  server_tick: number,
  uptime: number
}
```

**Client Action**: Update UI with current session info

---

### 7. Error

Server error message.

**Type**: `error`
**Payload**:
```typescript
{
  code: string,
  message: string
}
```

**Example**:
```json
{
  "type": "error",
  "payload": {
    "code": "not_authenticated",
    "message": "Must be authenticated to chat"
  }
}
```

**Error Codes**:
- `invalid_message` - Failed to parse message
- `unknown_message_type` - Unknown message type
- `not_authenticated` - Action requires authentication
- `join_failed` - Failed to join session
- `invalid_chat` - Invalid chat message
- `rate_limited` - Too many requests
- `invalid_token` - JWT token validation failed
- `token_expired` - JWT token has expired
- `user_banned` - User account is banned

**Client Action**: Display error to user, log for debugging

---

## Connection Lifecycle

### 1. Initial Connection

```
Client                          Server
  |                               |
  |--- WebSocket Connect -------->|
  |    (with JWT in header)       |
  |                               |
  |<------ Connection OK ---------|
  |                               |
```

### 2. Join Session

```
Client                          Server
  |                               |
  |--- join message ------------->|
  |                               |
  |<------ welcome message -------|
  |<------ player_joined ---------|  (broadcast to others)
  |                               |
```

### 3. Active Session

```
Client                          Server
  |                               |
  |--- chat message ------------->|
  |<------ chat broadcast --------|  (to all players)
  |                               |
  |--- ping ------------------->  |
  |<------ pong -----------------|
  |                               |
```

### 4. Disconnect

```
Client                          Server
  |                               |
  |--- leave message ------------>|  (optional)
  |--- WebSocket Close ---------->|
  |                               |
  |<------ player_left ----------|  (broadcast to others)
  |                               |
```

---

## Error Handling

### Connection Errors

```javascript
ws.onerror = (error) => {
    console.error('WebSocket error:', error);
    // Attempt reconnection with exponential backoff
    setTimeout(() => reconnect(), 1000);
};

ws.onclose = (event) => {
    if (event.wasClean) {
        console.log('Connection closed cleanly');
    } else {
        console.error('Connection died');
        // Attempt reconnection
        setTimeout(() => reconnect(), 1000);
    }
};
```

### Message Errors

```javascript
function handleServerMessage(message) {
    if (message.type === 'error') {
        const { code, message: errorMsg } = message.payload;

        switch (code) {
            case 'invalid_token':
            case 'token_expired':
                // Redirect to login
                redirectToLogin();
                break;

            case 'user_banned':
                // Show banned message
                showBannedMessage();
                break;

            case 'rate_limited':
                // Show rate limit warning
                showRateLimitWarning();
                break;

            default:
                // Show generic error
                showError(errorMsg);
        }
    }
}
```

### Reconnection Strategy

```javascript
class ReconnectingWebSocket {
    constructor(url) {
        this.url = url;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.reconnectAttempts = 0;
        this.connect();
    }

    connect() {
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
            this.reconnectAttempts = 0;
            this.reconnectDelay = 1000;
            console.log('Connected');
        };

        this.ws.onclose = () => {
            this.reconnect();
        };
    }

    reconnect() {
        this.reconnectAttempts++;
        const delay = Math.min(
            this.reconnectDelay * Math.pow(2, this.reconnectAttempts),
            this.maxReconnectDelay
        );

        console.log(`Reconnecting in ${delay}ms...`);
        setTimeout(() => this.connect(), delay);
    }
}
```

---

## Keep-Alive

The server implements automatic ping/pong keep-alive.

**Server Ping Interval**: Every 54 seconds
**Timeout**: 60 seconds without pong response

```javascript
// Server automatically sends WebSocket ping frames
// Browser automatically responds with pong frames
// No client-side implementation needed

// Optional: Implement application-level ping for latency measurement
setInterval(() => {
    sendMessage('ping', {});
}, 30000); // Every 30 seconds
```

---

## JWT Token Structure

### Token Claims

The JWT token from GoLoginServer contains:

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

### Obtaining Token

```javascript
// 1. Redirect user to login page
window.location.href = 'https://login.gravitas-games.com/login';

// 2. After successful login, user is redirected back with token
// Example: https://yourgame.com/play?token=eyJhbGciOi...

// 3. Extract token from URL
const urlParams = new URLSearchParams(window.location.search);
const token = urlParams.get('token');

// 4. Store token
localStorage.setItem('jwt_token', token);

// 5. Connect to game server
const ws = new WebSocket('ws://localhost:8080/ws', ['access_token', token]);
```

### Token Validation

Server validates:
- ✅ Signature (ECDSA P-256)
- ✅ Issuer (`login-server`)
- ✅ Expiration (`exp` claim)
- ✅ User activated (`activated > 0`)
- ✅ Not banned (`activated != -1`)
- ✅ Not blacklisted (Redis check)

### Token Refresh

Tokens expire after 1 hour. Client should:
1. Check token expiration before connecting
2. Refresh token if expired (via GoLoginServer)
3. Handle `token_expired` errors gracefully
4. Redirect to login if refresh fails

---

## Example Implementation

### Complete Client Example

```javascript
class MMORTSClient {
    constructor(serverUrl, token) {
        this.serverUrl = serverUrl;
        this.token = token;
        this.ws = null;
        this.playerId = null;
        this.username = null;
        this.sessionId = null;
        this.handlers = {};
    }

    connect() {
        this.ws = new WebSocket(this.serverUrl, ['access_token', this.token]);

        this.ws.onopen = () => {
            console.log('Connected to server');
            this.join();
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
        };

        this.ws.onclose = () => {
            console.log('Disconnected from server');
            this.emit('disconnect');
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.emit('error', error);
        };
    }

    join() {
        this.send('join', {});
    }

    chat(message) {
        this.send('chat', { message });
    }

    ping() {
        this.send('ping', {});
    }

    disconnect() {
        this.send('leave', {});
        this.ws.close();
    }

    send(type, payload) {
        const message = { type, payload };
        this.ws.send(JSON.stringify(message));
    }

    handleMessage(message) {
        switch (message.type) {
            case 'welcome':
                this.playerId = message.payload.player_id;
                this.username = message.payload.username;
                this.sessionId = message.payload.session_id;
                this.emit('welcome', message.payload);
                break;

            case 'player_joined':
                this.emit('player_joined', message.payload);
                break;

            case 'player_left':
                this.emit('player_left', message.payload);
                break;

            case 'chat':
                this.emit('chat', message.payload);
                break;

            case 'pong':
                this.emit('pong', message.payload);
                break;

            case 'error':
                this.emit('error', message.payload);
                break;

            default:
                console.warn('Unknown message type:', message.type);
        }
    }

    on(event, handler) {
        if (!this.handlers[event]) {
            this.handlers[event] = [];
        }
        this.handlers[event].push(handler);
    }

    emit(event, data) {
        if (this.handlers[event]) {
            this.handlers[event].forEach(handler => handler(data));
        }
    }
}

// Usage
const token = localStorage.getItem('jwt_token');
const client = new MMORTSClient('ws://localhost:8080/ws', token);

client.on('welcome', (data) => {
    console.log('Joined session:', data.session_id);
    updateUI(data);
});

client.on('chat', (data) => {
    displayChatMessage(data.username, data.message);
});

client.on('player_joined', (data) => {
    showNotification(`${data.username} joined the game`);
});

client.on('error', (error) => {
    console.error('Server error:', error);
    if (error.code === 'token_expired') {
        redirectToLogin();
    }
});

client.connect();
```

---

## Testing

### Health Check

Test if server is running:

```bash
curl http://localhost:8080/health
# Response: {"status":"ok"}
```

### WebSocket Connection Test

```javascript
// Simple connection test
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => console.log('Server is online');
ws.onerror = () => console.log('Server is offline');
```

### Message Flow Test

```javascript
// Test complete flow
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
    console.log('1. Connected');
    ws.send(JSON.stringify({ type: 'join', payload: {} }));
};

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('2. Received:', msg.type);

    if (msg.type === 'welcome') {
        console.log('3. Joined as:', msg.payload.username);
        ws.send(JSON.stringify({
            type: 'chat',
            payload: { message: 'Hello!' }
        }));
    }
};
```

---

## Rate Limits

| Action | Limit | Window |
|--------|-------|--------|
| Chat messages | 10 messages | 1 minute |
| Connection attempts | 5 attempts | 1 minute |
| Message size | 8 KB | per message |

---

## Best Practices

### 1. Token Management
```javascript
// Check token before connecting
function isTokenValid(token) {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return payload.exp * 1000 > Date.now();
}

if (!isTokenValid(token)) {
    refreshToken().then(newToken => connect(newToken));
}
```

### 2. Message Queue
```javascript
// Queue messages if connection is lost
class MessageQueue {
    constructor() {
        this.queue = [];
    }

    enqueue(message) {
        this.queue.push(message);
    }

    flush(ws) {
        while (this.queue.length > 0) {
            ws.send(JSON.stringify(this.queue.shift()));
        }
    }
}
```

### 3. State Synchronization
```javascript
// Track connection state
const ConnectionState = {
    DISCONNECTED: 'disconnected',
    CONNECTING: 'connecting',
    CONNECTED: 'connected',
    JOINED: 'joined'
};

let state = ConnectionState.DISCONNECTED;
```

### 4. Error Logging
```javascript
// Log errors for debugging
function logError(context, error) {
    console.error(`[${context}]`, {
        timestamp: new Date().toISOString(),
        error: error,
        state: state
    });

    // Send to error tracking service
    sendToErrorTracking(context, error);
}
```

---

## Future Features (Roadmap)

The following features are planned for future phases:

- 🔄 **Entity Synchronization** - Real-time game entity updates
- 👁️ **Vision System** - Fog of war and shared vision
- 🎮 **Game Commands** - Unit movement, building, combat
- 📊 **Delta Updates** - Efficient state synchronization
- 🗺️ **Map Data** - Hex chunk and terrain information
- 🏰 **Building System** - Construction and production
- ⚔️ **Combat System** - Unit battles and damage
- 👥 **Social Features** - Groups, alliances, messaging

---

## Support

For issues or questions:
- **Repository**: https://github.com/gravitas-games/mmorts
- **Documentation**: See ARCHITECTURE.md and IMPLEMENTATION_PLAN.md
- **Server Logs**: Check server console output for debugging

---

**Document Version**: 1.0
**Last Updated**: 2025-10-15
**Server Version**: Phase 1 MVP
