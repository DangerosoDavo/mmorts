# WebSocket Connection Troubleshooting

This guide helps diagnose WebSocket connection issues with your MMORTS game server.

## Quick Checklist

Run through these checks in order:

### 1. ✓ Verify Game Server is Running

```bash
# Check if container is running
docker ps | grep mmorts-server

# Check server logs
docker logs mmorts-server --tail=50

# Expected output should show:
# "Server listening on 0.0.0.0:8080"
# "WebSocket endpoint: ws://0.0.0.0:8080/ws"
```

### 2. ✓ Test Direct Connection to Game Server

```bash
# From the host, test if port 8080 is accessible
curl http://localhost:8080/health

# Expected output: {"status":"ok"}
```

If this fails, check:
- Container is in `shared_services` network: `docker network inspect shared_services`
- Port binding is correct in docker-compose.yml: `127.0.0.1:8080:8080`

### 3. ✓ Verify Nginx Configuration

```bash
# Check if nginx config is enabled
ls -la /etc/nginx/sites-enabled/ | grep mmorts-game

# Test nginx configuration
sudo nginx -t

# Expected output: "syntax is ok" and "test is successful"
```

**CRITICAL**: Ensure the WebSocket map directive is in `/etc/nginx/nginx.conf`:

```bash
# Check if map exists in main nginx config
sudo grep -A 3 "connection_upgrade" /etc/nginx/nginx.conf
```

If not found, add this inside the `http {}` block in `/etc/nginx/nginx.conf`:

```nginx
http {
    # ... existing config ...

    # Map for WebSocket upgrade
    map $http_upgrade $connection_upgrade {
        default upgrade;
        '' close;
    }

    # ... rest of config ...
}
```

Then reload nginx:
```bash
sudo systemctl reload nginx
```

### 4. ✓ Test Nginx Can Reach Game Server

```bash
# Check nginx can connect to localhost:8080
curl -H "Host: your-domain.com" http://localhost:8100/health

# This should redirect to HTTPS, but nginx should still process it
```

### 5. ✓ Verify SSL Certificates

```bash
# Check certificate exists
sudo ls -la /etc/letsencrypt/live/your-domain.com/

# Test SSL certificate
openssl s_client -connect your-domain.com:8143 -servername your-domain.com

# Press Ctrl+C after connection info appears
```

### 6. ✓ Test HTTPS Health Endpoint

```bash
# Test health check through nginx
curl https://your-domain.com:8143/health

# Expected output: {"status":"ok"}
```

### 7. ✓ Check Firewall Rules

```bash
# Verify ports are open
sudo ufw status | grep 8143
sudo ufw status | grep 8100

# Should show:
# 8100/tcp                   ALLOW       Anywhere
# 8143/tcp                   ALLOW       Anywhere
```

If not:
```bash
sudo ufw allow 8100/tcp
sudo ufw allow 8143/tcp
sudo ufw reload
```

### 8. ✓ Test WebSocket Connection

Using browser console:

```javascript
// Replace with your actual domain
const token = 'your-jwt-token-here';
const ws = new WebSocket('wss://your-domain.com:8143/ws', ['access_token', token]);

ws.onopen = () => console.log('Connected!');
ws.onerror = (err) => console.error('Connection error:', err);
ws.onclose = (event) => console.log('Closed:', event.code, event.reason);
```

## Common Issues and Solutions

### Issue: "Connection refused"

**Cause**: Game server isn't running or nginx can't reach it

**Solutions**:
1. Check game server is running: `docker ps | grep mmorts-server`
2. Check logs: `docker logs mmorts-server`
3. Verify port binding: `netstat -tlnp | grep 8080`
4. Restart container: `docker compose restart mmorts-server`

### Issue: "502 Bad Gateway"

**Cause**: Nginx can't connect to the backend

**Solutions**:
1. Check upstream in nginx config points to `localhost:8080`
2. Verify game server is healthy: `curl http://localhost:8080/health`
3. Check nginx error log: `sudo tail -f /var/log/nginx/error.log`
4. If using Docker Desktop, may need to use `host.docker.internal` instead of `localhost`

### Issue: "SSL certificate error"

**Cause**: Certificate path is wrong or expired

**Solutions**:
1. Verify certificate paths in `/etc/nginx/sites-available/mmorts-game`:
   ```nginx
   ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
   ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
   ```
2. Check certificate exists: `sudo ls -la /etc/letsencrypt/live/your-domain.com/`
3. Renew certificate: `sudo certbot renew`
4. Reload nginx: `sudo systemctl reload nginx`

### Issue: "WebSocket connection closed immediately"

**Cause**: Missing or incorrect WebSocket upgrade headers

**Solutions**:
1. **MOST COMMON**: Missing `map` directive in nginx.conf
   ```bash
   # Check if map exists
   sudo grep "connection_upgrade" /etc/nginx/nginx.conf
   ```

   If not found, add to `http {}` block in `/etc/nginx/nginx.conf`:
   ```nginx
   map $http_upgrade $connection_upgrade {
       default upgrade;
       '' close;
   }
   ```

2. Verify nginx site config uses the variable:
   ```bash
   sudo grep "Connection.*connection_upgrade" /etc/nginx/sites-available/mmorts-game
   ```

   Should show:
   ```nginx
   proxy_set_header Connection $connection_upgrade;
   ```

3. Reload nginx:
   ```bash
   sudo nginx -t && sudo systemctl reload nginx
   ```

### Issue: "Invalid token" or "401 Unauthorized"

**Cause**: JWT token validation failed

**Solutions**:
1. Check JWT public key is accessible:
   ```bash
   curl https://login.gravitas-games.com/api/public-key
   ```
2. Check server logs for JWT errors: `docker logs mmorts-server`
3. Verify token hasn't expired
4. Check token is being sent correctly:
   ```javascript
   // Correct way - token in subprotocol
   new WebSocket('wss://domain.com:8143/ws', ['access_token', token]);
   ```

### Issue: "Failed to connect to Redis"

**Cause**: Redis connection issue

**Solutions**:
1. Check Redis is running on shared_services network:
   ```bash
   docker ps | grep loginserver_redis
   ```
2. Test connection from game server:
   ```bash
   docker exec mmorts-server nc -zv loginserver_redis 6379
   ```
3. Check Redis password in [configs/server.yaml](configs/server.yaml)

### Issue: "Failed to connect to database"

**Cause**: MySQL connection issue

**Solutions**:
1. Check MySQL is running:
   ```bash
   docker ps | grep loginserver_mysql
   ```
2. Verify database exists:
   ```bash
   docker exec loginserver_mysql mysql -u mmorts -pmmorts -e "SHOW DATABASES;"
   ```
3. Create database if needed:
   ```bash
   docker exec loginserver_mysql mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS mmorts;"
   docker exec loginserver_mysql mysql -u root -p -e "GRANT ALL ON mmorts.* TO 'mmorts'@'%';"
   ```

### Issue: Connection works locally but not remotely

**Cause**: Firewall or network configuration

**Solutions**:
1. Check external firewall allows ports 8100 and 8143
2. Verify nginx is listening on all interfaces: `0.0.0.0:8143`
3. Test from external IP: `curl https://your-external-ip:8143/health`
4. Check cloud security groups (AWS, GCP, Azure, etc.)

## Debug Mode

Enable detailed logging to diagnose issues:

### Enable Nginx Debug Logging

Edit `/etc/nginx/sites-available/mmorts-game`:

```nginx
server {
    listen 8143 ssl http2;

    # Add these lines for debugging
    access_log /var/log/nginx/mmorts-access.log;
    error_log /var/log/nginx/mmorts-error.log debug;

    # ... rest of config
}
```

Reload nginx and watch logs:
```bash
sudo systemctl reload nginx
sudo tail -f /var/log/nginx/mmorts-error.log
```

### View Game Server Logs in Real-Time

```bash
docker logs -f mmorts-server
```

### Check All Network Connections

```bash
# See what's listening
sudo ss -tlnp | grep -E '(8080|8100|8143)'

# See active WebSocket connections
sudo ss -tn | grep :8143
```

## Configuration Summary

Your expected configuration:

| Component | Setting | Value |
|-----------|---------|-------|
| **Game Server** | Listen Address | `0.0.0.0:8080` (inside container) |
| **Docker Port** | Binding | `127.0.0.1:8080:8080` |
| **Nginx Upstream** | Backend | `localhost:8080` |
| **Nginx HTTP** | Port | `8100` (redirects to HTTPS) |
| **Nginx HTTPS/WSS** | Port | `8143` |
| **Client Connection** | URL | `wss://your-domain.com:8143/ws` |
| **WebSocket Path** | Endpoint | `/ws` |
| **Health Check** | Endpoint | `/health` |

## Still Having Issues?

If you've tried everything above, gather this information:

```bash
# Run these commands and share the output:

# 1. Container status
docker ps -a | grep mmorts

# 2. Server logs (last 100 lines)
docker logs mmorts-server --tail=100

# 3. Nginx test
sudo nginx -t

# 4. Nginx error log (last 50 lines)
sudo tail -n 50 /var/log/nginx/error.log

# 5. Port status
sudo ss -tlnp | grep -E '(8080|8100|8143)'

# 6. Network connectivity
docker network inspect shared_services | grep -A 5 mmorts-server

# 7. Test direct connection
curl -v http://localhost:8080/health

# 8. Test through nginx
curl -k -v https://localhost:8143/health
```

Share these outputs for further debugging.
