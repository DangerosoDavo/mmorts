#!/bin/bash
# Deploy script for MMORTS server

set -e

echo "🚀 Deploying MMORTS Server..."

# Build and start services
echo "📦 Building Docker images..."
docker-compose build

echo "🔄 Starting services..."
docker-compose up -d

echo "⏳ Waiting for services to be healthy..."
sleep 5

# Check health
echo "🏥 Checking service health..."
docker-compose ps

echo "✅ Deployment complete!"
echo ""
echo "📊 Service Status:"
echo "  - Game Server: http://localhost:8080"
echo "  - Health Check: http://localhost:8080/health"
echo "  - WebSocket: ws://localhost:8080/ws"
echo "  - Redis: localhost:6379"
echo "  - MariaDB: localhost:3306"
echo ""
echo "📝 View logs: docker-compose logs -f mmorts-server"
echo "🛑 Stop services: docker-compose down"
