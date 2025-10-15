#!/bin/bash
# Update/rebuild script for MMORTS server

set -e

echo "🔄 Updating MMORTS Server..."

# Stop services
echo "🛑 Stopping services..."
docker-compose down

# Rebuild
echo "📦 Rebuilding Docker images..."
docker-compose build --no-cache

# Start services
echo "🚀 Starting services..."
docker-compose up -d

echo "⏳ Waiting for services to be healthy..."
sleep 5

# Check health
echo "🏥 Checking service health..."
docker-compose ps

echo "✅ Update complete!"
echo ""
echo "📝 View logs: docker-compose logs -f mmorts-server"
