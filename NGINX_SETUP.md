# Nginx Configuration for Existing Installation

This guide covers integrating the MMORTS game server with your existing nginx installation and certbot SSL certificates.

## Prerequisites

- Existing nginx installation
- Certbot SSL certificates already generated for your domain
- MMORTS server running on `localhost:8080` (or accessible Docker container)

## Installation Steps

### 1. Copy the nginx configuration

```bash
# Copy the server block configuration
sudo cp nginx/mmorts-game.conf /etc/nginx/sites-available/mmorts-game

# Edit the configuration to match your domain
sudo nano /etc/nginx/sites-available/mmorts-game
```

### 2. Update the configuration

Replace the following placeholders in `/etc/nginx/sites-available/mmorts-game`:

```nginx
server_name your-domain.com;  # Replace with your actual domain (e.g., game.example.com)

# Update SSL certificate paths if needed
ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
ssl_trusted_certificate /etc/letsencrypt/live/your-domain.com/chain.pem;
```

### 3. Update upstream server if needed

**Default configuration (keep as-is for most cases):**

The default `server localhost:8080;` is correct when:
- System nginx proxies to Docker container with port binding `127.0.0.1:8080:8080`
- This is the standard setup described in this guide
- **You probably don't need to change this**

**Only change the upstream in these special cases:**

```nginx
upstream mmorts_gameserver {
    # DEFAULT: System nginx → Docker container with published port
    server localhost:8080;  # ✓ Use this (already configured)

    # ONLY IF: Both nginx AND game server run inside Docker on same network
    # server mmorts-server:8080;  # Uses Docker DNS to resolve container name

    # ONLY IF: Game server runs on a different machine
    # server 192.168.1.100:8080;  # Use actual IP address

    keepalive 32;
}
```

**How it works:**
- Your `docker-compose.yml` binds container port to host: `127.0.0.1:8080:8080`
- System nginx connects to `localhost:8080` on the host
- Docker forwards the connection to the container
- No changes needed!

### 4. Enable the site

```bash
# Create symlink to enable the site
sudo ln -s /etc/nginx/sites-available/mmorts-game /etc/nginx/sites-enabled/

# Test nginx configuration
sudo nginx -t

# If test passes, reload nginx
sudo systemctl reload nginx
```

### 5. Update firewall rules

```bash
# Allow game server ports
sudo ufw allow 8100/tcp  # HTTP (redirects to HTTPS)
sudo ufw allow 8143/tcp  # HTTPS/WSS
sudo ufw reload
```

## Docker Compose Adjustments

Since you're using an external nginx, you can simplify `docker-compose.yml`:

```yaml
version: '3.8'

services:
  mmorts-server:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: mmorts-server
    ports:
      - "127.0.0.1:8080:8080"  # Bind to localhost only, accessed via nginx
    environment:
      - CONFIG_PATH=/root/configs/server.yaml
    volumes:
      - ./configs:/root/configs:ro
    networks:
      - shared_services
    restart: unless-stopped

networks:
  shared_services:
    external: true
```

**Note**: Remove the nginx service from docker-compose.yml since you're using the system nginx.

## Client Configuration

Update your web client to connect via WSS:

```javascript
// Replace with your actual domain and port
const gameServerUrl = 'wss://your-domain.com:8143/ws';
const ws = new WebSocket(gameServerUrl, ['access_token', token]);
```

## Testing

### 1. Test SSL certificate

```bash
# Check certificate is valid
openssl s_client -connect your-domain.com:8143 -servername your-domain.com
```

### 2. Test WebSocket connection

```bash
# Test WSS endpoint (requires websocat or similar)
websocat wss://your-domain.com:8143/ws

# Or use browser console:
# ws = new WebSocket('wss://your-domain.com:8143/ws', ['access_token', 'your-token-here'])
```

### 3. Test health endpoint

```bash
curl https://your-domain.com:8143/health
```

## Troubleshooting

### Connection refused

```bash
# Check if game server is running
docker ps | grep mmorts-server

# Check if nginx can reach game server
sudo docker exec mmorts-server nc -zv localhost 8080
```

### SSL certificate errors

```bash
# Verify certificate paths exist
sudo ls -la /etc/letsencrypt/live/your-domain.com/

# Test nginx configuration
sudo nginx -t
```

### 502 Bad Gateway

```bash
# Check game server logs
docker logs mmorts-server

# Check nginx error logs
sudo tail -f /var/log/nginx/error.log
```

### SELinux issues (RHEL/CentOS)

```bash
# Allow nginx to connect to network
sudo setsebool -P httpd_can_network_connect 1
```

## Certbot Auto-Renewal

Your existing certbot setup will continue to work. After certificate renewal, reload nginx:

```bash
# This is usually automatic, but manual reload if needed:
sudo systemctl reload nginx
```

## Port Alternatives

If ports 8100/8143 conflict with existing services:

### Option 1: Use standard ports (requires root/CAP_NET_BIND_SERVICE)

```nginx
listen 80;      # HTTP
listen 443 ssl; # HTTPS/WSS
```

### Option 2: Use different ports

```nginx
listen 9100;      # HTTP
listen 9143 ssl;  # HTTPS/WSS
```

Update client and firewall accordingly.

## Monitoring

Add monitoring for the game server endpoint:

```bash
# Check if WSS endpoint is responding
watch -n 5 'curl -s -o /dev/null -w "%{http_code}" https://your-domain.com:8143/health'
```

## Production Recommendations

1. **Rate limiting**: Add rate limiting to prevent abuse
   ```nginx
   limit_req_zone $binary_remote_addr zone=game_limit:10m rate=10r/s;

   location /ws {
       limit_req zone=game_limit burst=20 nodelay;
       # ... rest of config
   }
   ```

2. **Access logs**: Enable access logging for monitoring
   ```nginx
   access_log /var/log/nginx/mmorts-game-access.log;
   error_log /var/log/nginx/mmorts-game-error.log;
   ```

3. **Log rotation**: Ensure nginx logs are rotated
   ```bash
   # Usually handled by logrotate automatically
   sudo cat /etc/logrotate.d/nginx
   ```

4. **Monitoring**: Set up monitoring for WebSocket connections
   ```bash
   # Check active connections
   sudo ss -tn | grep :8143 | wc -l
   ```
