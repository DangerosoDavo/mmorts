# Deployment Guide - shared_services Network

## Overview

This deployment uses the existing `shared_services` Docker network with shared Redis and MySQL instances.

## Network Configuration

### Existing Services on shared_services Network
- **Redis**: `loginserver_redis:6379`
- **MySQL**: `loginserver_mysql:3306`

### MMORTS Server
- **Container**: `mmorts-server`
- **Port**: `8080`
- **Network**: `shared_services` (external)

## Prerequisites

1. **Docker & Docker Compose** installed
2. **shared_services network** must exist:
   ```bash
   docker network ls | grep shared_services
   ```
   If not present, create it:
   ```bash
   docker network create shared_services
   ```

3. **Existing services** running on shared_services:
   - `loginserver_redis` (Redis)
   - `loginserver_mysql` (MariaDB/MySQL)

## Configuration

The server is configured to use existing shared services:

### configs/server.yaml
```yaml
redis:
  address: "loginserver_redis:6379"  # Existing Redis
  password: ""
  db: 0

database:
  host: "loginserver_mysql"  # Existing MySQL
  port: 3306
  user: "mmorts"
  password: "mmorts"
  database: "mmorts"
```

## Deployment

### Quick Deploy

```bash
# Build and start the server
./deploy.sh

# Or manually
docker compose build
docker compose up -d
```

### Verify Deployment

```bash
# Check container status
docker compose ps

# Check logs
docker compose logs -f mmorts-server

# Test health endpoint
curl http://localhost:8080/health
# Should return: {"status":"ok"}

# Verify network connectivity
docker exec mmorts-server ping -c 3 loginserver_redis
docker exec mmorts-server ping -c 3 loginserver_mysql
```

## MySQL Database Setup

The server expects a database named `mmorts` on the shared MySQL instance.

### Create Database (if needed)

```bash
# Connect to MySQL
docker exec -it loginserver_mysql mysql -u root -p

# Create database and user
CREATE DATABASE IF NOT EXISTS mmorts;
CREATE USER IF NOT EXISTS 'mmorts'@'%' IDENTIFIED BY 'mmorts';
GRANT ALL PRIVILEGES ON mmorts.* TO 'mmorts'@'%';
FLUSH PRIVILEGES;
EXIT;
```

## Redis Configuration

The server uses the shared Redis instance for JWT blacklist checking.

### Test Redis Connection

```bash
# Connect to Redis
docker exec -it loginserver_redis redis-cli

# Test connection
PING
# Should return: PONG

# Check JWT blacklist keys
KEYS jwt:blacklist:*

EXIT
```

## Updating

```bash
# Update and rebuild
./update.sh

# Or manually
docker compose down
docker compose build --no-cache
docker compose up -d
```

## Troubleshooting

### Server can't connect to Redis

```bash
# Check if Redis is running
docker ps | grep loginserver_redis

# Check if server is on shared_services network
docker network inspect shared_services

# Test connectivity from server
docker exec mmorts-server ping loginserver_redis
```

### Server can't connect to MySQL

```bash
# Check if MySQL is running
docker ps | grep loginserver_mysql

# Test connectivity
docker exec mmorts-server ping loginserver_mysql

# Check MySQL logs
docker logs loginserver_mysql
```

### Network issues

```bash
# Verify shared_services network exists and is external
docker network inspect shared_services

# Ensure server is connected
docker inspect mmorts-server | grep -A 10 Networks

# Reconnect if needed
docker network connect shared_services mmorts-server
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│         shared_services Network                 │
│                                                 │
│  ┌──────────────────┐    ┌──────────────────┐  │
│  │ loginserver_redis│    │loginserver_mysql │  │
│  │   (Existing)     │    │   (Existing)     │  │
│  │   Port: 6379     │    │   Port: 3306     │  │
│  └─────────┬────────┘    └────────┬─────────┘  │
│            │                      │             │
│            │                      │             │
│  ┌─────────┴──────────────────────┴─────────┐  │
│  │         mmorts-server                    │  │
│  │         Port: 8080                       │  │
│  │         - WebSocket: /ws                 │  │
│  │         - Health: /health                │  │
│  └──────────────────────────────────────────┘  │
│                                                 │
└─────────────────────────────────────────────────┘
                        │
                        │ Port 8080
                        ▼
                   Internet/LAN
```

## Standalone Mode (Optional)

If you need dedicated Redis and MySQL instances for testing, uncomment the services in `docker-compose.yml`:

```yaml
# Uncomment redis and mariadb services
# Change network to: external: false
# Uncomment volumes section
```

Then update `configs/server.yaml`:
```yaml
redis:
  address: "redis:6379"  # Use local instance

database:
  host: "mariadb"  # Use local instance
```

## Production Considerations

1. **SSL/TLS**: Use reverse proxy (nginx) for HTTPS
2. **Firewall**: Only expose port 8080, keep Redis/MySQL internal
3. **Monitoring**: Set up health check monitoring
4. **Backup**: Regular backups of MySQL database
5. **Secrets**: Use Docker secrets or environment variables for passwords
6. **Resource Limits**: Set memory/CPU limits in docker-compose.yml

## Example Production docker-compose.yml

```yaml
services:
  mmorts-server:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: mmorts-server
    ports:
      - "8080:8080"
    environment:
      - CONFIG_PATH=/root/configs/server.yaml
    volumes:
      - ./configs:/root/configs:ro
    networks:
      - shared_services
    restart: unless-stopped
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M
```

## Support

- See [README.md](README.md) for general documentation
- See [CLIENT_API.md](docs/CLIENT_API.md) for API documentation
- Check logs: `docker compose logs -f mmorts-server`

---

**Last Updated**: 2025-10-15
**Network**: shared_services (external)
**Services Used**: loginserver_redis, loginserver_mysql
