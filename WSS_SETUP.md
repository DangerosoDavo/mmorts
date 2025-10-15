# WebSocket Secure (WSS) Setup Guide

## Problem

When your web client is served over HTTPS, browsers block insecure WebSocket connections (`ws://`):

```
SecurityError: Failed to construct 'WebSocket': An insecure WebSocket connection
may not be initiated from a page loaded over HTTPS.
```

## Solution

Use **nginx as a reverse proxy** to provide WSS (WebSocket Secure) support.

```
HTTPS Web Client → WSS (nginx) → WS (game server)
```

## Quick Start

### 1. Generate SSL Certificate

**Option A: Self-Signed (Development)**
```bash
cd nginx
./generate-self-signed-cert.sh
```

**Option B: Let's Encrypt (Production)**
```bash
# Install certbot
sudo apt-get install certbot

# Generate certificate
sudo certbot certonly --standalone -d yourdomain.com

# Copy certificates
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem
sudo chown $USER:$USER nginx/certs/*.pem
```

**Option C: Use Existing Certificates**
```bash
cp /path/to/your/fullchain.pem nginx/certs/cert.pem
cp /path/to/your/privkey.pem nginx/certs/key.pem
```

### 2. Deploy with nginx

```bash
# Start all services (includes nginx)
docker compose up -d

# Check nginx logs
docker compose logs -f nginx
```

### 3. Update Your Web Client

Change WebSocket connection from `ws://` to `wss://`:

```javascript
// Before (insecure - won't work from HTTPS page)
const ws = new WebSocket('ws://yourdomain.com:8100/ws', ['access_token', token]);

// After (secure - works from HTTPS page)
const ws = new WebSocket('wss://yourdomain.com:8143/ws', ['access_token', token]);
```

## Port Configuration

Default ports in docker-compose.yml:
- **8100**: HTTP (redirects to HTTPS)
- **8143**: HTTPS/WSS

### Using Standard HTTPS Port (443)

For production, use standard port 443:

1. Update `docker-compose.yml`:
```yaml
nginx:
  ports:
    - "80:80"     # HTTP
    - "443:443"   # HTTPS/WSS (standard port)
```

2. Connect from client:
```javascript
const ws = new WebSocket('wss://yourdomain.com/ws', ['access_token', token]);
```

## Architecture

```
┌─────────────────┐
│  HTTPS Client   │
│ (Your Web App)  │
└────────┬────────┘
         │ WSS (wss://yourdomain.com:8143/ws)
         ▼
┌─────────────────┐
│  nginx Proxy    │
│  (Port 8143)    │
│  - SSL/TLS      │
│  - Termination  │
└────────┬────────┘
         │ WS (ws://mmorts-server:8080/ws)
         ▼
┌─────────────────┐
│  Game Server    │
│  (Port 8080)    │
│  - Internal     │
└─────────────────┘
```

## Testing

### 1. Test HTTP Redirect
```bash
curl -I http://localhost:8100/health
# Should return: 301 Moved Permanently
```

### 2. Test HTTPS Health Check
```bash
curl -k https://localhost:8143/health
# Should return: {"status":"ok"}
```

### 3. Test WSS Connection (from browser console)
```javascript
const ws = new WebSocket('wss://localhost:8143/ws', ['access_token', 'your-jwt-token']);
ws.onopen = () => console.log('Connected!');
ws.onerror = (err) => console.error('Error:', err);
```

## Certificate Management

### Self-Signed Certificate Warnings

Self-signed certificates will show browser warnings:
- Chrome: "Your connection is not private"
- Firefox: "Warning: Potential Security Risk Ahead"

**For development**: Click "Advanced" → "Proceed to localhost"

**For production**: Use Let's Encrypt or proper CA certificates

### Let's Encrypt Auto-Renewal

```bash
# Test renewal
sudo certbot renew --dry-run

# Set up auto-renewal (add to crontab)
0 0 1 * * sudo certbot renew && sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem /path/to/nginx/certs/cert.pem && sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem /path/to/nginx/certs/key.pem && docker compose exec nginx nginx -s reload
```

## Troubleshooting

### "certificate not found" error

```bash
# Check certificates exist
ls -la nginx/certs/
# Should show cert.pem and key.pem

# If missing, generate certificates (see step 1 above)
```

### "connection refused" from browser

```bash
# Check nginx is running
docker compose ps nginx

# Check nginx logs for errors
docker compose logs nginx

# Verify nginx can reach game server
docker exec mmorts-nginx wget -qO- http://mmorts-server:8080/health
```

### Mixed content warnings

If your web client still has issues:
1. Ensure ALL resources load over HTTPS (scripts, styles, etc.)
2. Check browser console for mixed content warnings
3. Update all WebSocket connections to use `wss://`

### Certificate expired

```bash
# Check certificate expiration
openssl x509 -in nginx/certs/cert.pem -noout -enddate

# Renew Let's Encrypt certificate
sudo certbot renew

# Copy new certificates
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem nginx/certs/cert.pem
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem nginx/certs/key.pem

# Reload nginx
docker compose exec nginx nginx -s reload
```

## Security Best Practices

✅ **Use strong TLS versions** (TLS 1.2, 1.3 only) - ✅ Already configured
✅ **Use strong cipher suites** - ✅ Already configured
✅ **Enable HSTS** - ✅ Already configured
✅ **Add security headers** - ✅ Already configured
✅ **Keep certificates up to date** - Set up auto-renewal
✅ **Use proper CA certificates in production** - Use Let's Encrypt
✅ **Restrict cipher suites** - Already using HIGH:!aNULL:!MD5
✅ **Monitor certificate expiration** - Set up monitoring

## Production Checklist

- [ ] Use Let's Encrypt or CA-signed certificates (not self-signed)
- [ ] Set up certificate auto-renewal
- [ ] Use standard HTTPS port (443)
- [ ] Configure firewall (allow 80, 443 only)
- [ ] Set up monitoring for certificate expiration
- [ ] Enable OCSP stapling
- [ ] Generate strong DH parameters
- [ ] Set up nginx log rotation
- [ ] Configure rate limiting
- [ ] Test SSL configuration with SSL Labs

## Alternative: Using Cloudflare

If you use Cloudflare, you can leverage their SSL:

1. Enable "Full (strict)" SSL mode in Cloudflare
2. Generate Origin Certificate in Cloudflare dashboard
3. Use Origin Certificate in nginx
4. Cloudflare handles SSL termination at edge

## References

- [nginx WebSocket Proxying](https://nginx.org/en/docs/http/websocket.html)
- [Let's Encrypt](https://letsencrypt.org/)
- [Mozilla SSL Configuration Generator](https://ssl-config.mozilla.org/)
- [nginx README](nginx/README.md) - Detailed nginx configuration guide

---

**Quick Links**:
- [nginx Configuration](nginx/nginx.conf)
- [nginx README](nginx/README.md)
- [Main README](README.md)
- [Deployment Guide](DEPLOYMENT.md)
