# nginx Reverse Proxy for WSS Support

This directory contains the nginx configuration for providing WebSocket Secure (WSS) support to the MMORTS game server.

## Why nginx?

When your web client is served over HTTPS, browsers require WebSocket connections to also use WSS (secure). The nginx reverse proxy:
- Terminates SSL/TLS
- Proxies secure connections (WSS) to the game server (WS)
- Adds security headers
- Handles timeouts for long-lived WebSocket connections

## SSL Certificate Setup

You need SSL certificates in the `nginx/certs/` directory.

### Option 1: Using Existing Certificates (Recommended)

If you already have SSL certificates (e.g., from Let's Encrypt):

```bash
# Copy your existing certificates
cp /path/to/your/fullchain.pem nginx/certs/cert.pem
cp /path/to/your/privkey.pem nginx/certs/key.pem

# Set proper permissions
chmod 644 nginx/certs/cert.pem
chmod 600 nginx/certs/key.pem
```

### Option 2: Self-Signed Certificate (Development Only)

⚠️ **Warning**: Self-signed certificates will show browser warnings. Only use for development/testing.

```bash
# Create certs directory
mkdir -p nginx/certs

# Generate self-signed certificate (valid for 365 days)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/certs/key.pem \
  -out nginx/certs/cert.pem \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=yourdomain.com"

# Set permissions
chmod 644 nginx/certs/cert.pem
chmod 600 nginx/certs/key.pem
```

### Option 3: Let's Encrypt with Certbot

For production, use Let's Encrypt for free SSL certificates:

```bash
# Install certbot
sudo apt-get install certbot

# Generate certificate (replace with your domain)
sudo certbot certonly --standalone -d yourdomain.com

# Copy certificates
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem
sudo chown $USER:$USER nginx/certs/*.pem
chmod 644 nginx/certs/cert.pem
chmod 600 nginx/certs/key.pem
```

## Directory Structure

```
nginx/
├── nginx.conf          # nginx configuration
├── certs/              # SSL certificates (you create this)
│   ├── cert.pem        # SSL certificate (fullchain)
│   └── key.pem         # Private key
└── README.md           # This file
```

## Deployment

### 1. Set up certificates (see above)

### 2. Deploy with docker compose

```bash
# Build and start (includes nginx)
docker compose up -d

# Check nginx logs
docker compose logs -f nginx

# Check if nginx is running
docker compose ps
```

### 3. Test the connection

```bash
# Test HTTP (should redirect to HTTPS)
curl -I http://localhost:8100/health

# Test HTTPS health endpoint
curl -k https://localhost:8143/health

# Test from web client
# Use: wss://localhost:8143/ws
```

## Client Configuration

Update your web client to connect to the WSS endpoint:

```javascript
// Before (insecure WebSocket)
const ws = new WebSocket('ws://localhost:8080/ws', ['access_token', token]);

// After (secure WebSocket through nginx)
const ws = new WebSocket('wss://yourdomain.com:8143/ws', ['access_token', token]);

// Or if using default HTTPS port (443), configure nginx to listen on 443:
const ws = new WebSocket('wss://yourdomain.com/ws', ['access_token', token]);
```

## Port Configuration

Current setup:
- **8100**: HTTP (redirects to HTTPS)
- **8143**: HTTPS/WSS (secure WebSocket)

To use standard HTTPS port (443), change in docker-compose.yml:
```yaml
nginx:
  ports:
    - "80:80"     # HTTP
    - "443:443"   # HTTPS/WSS (standard port)
```

Then connect with: `wss://yourdomain.com/ws`

## nginx Configuration

The `nginx.conf` file includes:

### Security Features
- TLS 1.2 and 1.3 only
- Strong cipher suites
- Security headers (X-Frame-Options, HSTS, etc.)
- HTTP to HTTPS redirect

### WebSocket Support
- Proper Upgrade and Connection headers
- Long timeout for persistent connections (7 days)
- Real IP forwarding

### Endpoints
- `/ws` - WebSocket endpoint (proxied to game server)
- `/health` - Health check endpoint
- `/` - Returns 404 (nginx doesn't serve other content)

## Troubleshooting

### Browser shows "NET::ERR_CERT_AUTHORITY_INVALID"

This is expected with self-signed certificates. Options:
1. Accept the security warning (development only)
2. Use proper SSL certificates from Let's Encrypt (production)
3. Add the self-signed cert to your OS trust store (advanced)

### Connection refused

```bash
# Check nginx is running
docker compose ps nginx

# Check nginx logs
docker compose logs nginx

# Verify certificates exist
ls -la nginx/certs/
```

### WebSocket connection fails

```bash
# Test that nginx can reach game server
docker exec mmorts-nginx wget -qO- http://mmorts-server:8080/health

# Check game server logs
docker compose logs mmorts-server

# Verify network connectivity
docker network inspect shared_services
```

### Certificate errors in logs

```
nginx: [emerg] cannot load certificate "/etc/nginx/certs/cert.pem"
```

Make sure certificates exist and have correct permissions:
```bash
ls -la nginx/certs/
# Should show:
# -rw-r--r-- cert.pem
# -rw------- key.pem
```

## Production Recommendations

1. **Use Let's Encrypt** for SSL certificates
2. **Set up auto-renewal** for certificates
3. **Use standard HTTPS port** (443) instead of 8143
4. **Enable OCSP stapling** in nginx.conf
5. **Configure log rotation** for nginx logs
6. **Monitor certificate expiration**
7. **Use strong DH parameters**:
   ```bash
   openssl dhparam -out nginx/certs/dhparam.pem 2048
   ```
   Then add to nginx.conf:
   ```nginx
   ssl_dhparam /etc/nginx/certs/dhparam.pem;
   ```

## Certificate Renewal (Let's Encrypt)

```bash
# Renew certificates
sudo certbot renew

# Copy new certificates
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem

# Reload nginx (without downtime)
docker compose exec nginx nginx -s reload
```

## Testing SSL Configuration

Test your SSL setup with SSL Labs:
https://www.ssllabs.com/ssltest/

Or use testssl.sh:
```bash
docker run --rm -ti drwetter/testssl.sh yourdomain.com:8143
```

## References

- [nginx WebSocket proxying](https://nginx.org/en/docs/http/websocket.html)
- [Let's Encrypt documentation](https://letsencrypt.org/docs/)
- [Mozilla SSL Configuration Generator](https://ssl-config.mozilla.org/)
