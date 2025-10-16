#!/bin/bash
# Diagnostic script for WebSocket connection issues

echo "=== MMORTS WebSocket Diagnostics ==="
echo ""

echo "1. Checking if game server container is running..."
docker ps | grep mmorts-server
if [ $? -eq 0 ]; then
    echo "   ✓ Container is running"
else
    echo "   ✗ Container is NOT running"
    echo "   Run: docker compose up -d"
fi
echo ""

echo "2. Checking game server logs (last 20 lines)..."
docker logs mmorts-server --tail=20 2>&1 | tail -20
echo ""

echo "3. Checking if game server responds on localhost:8110..."
curl -s -o /dev/null -w "HTTP %{http_code}\n" http://localhost:8110/health
if [ $? -eq 0 ]; then
    echo "   ✓ Game server responds on localhost:8110"
    curl -s http://localhost:8110/health
else
    echo "   ✗ Cannot reach game server on localhost:8110"
    echo "   Check docker-compose.yml port binding"
fi
echo ""

echo "4. Checking if nginx is running..."
systemctl status nginx --no-pager | grep "Active:"
echo ""

echo "5. Checking nginx is listening on ports 8100 and 8143..."
ss -tlnp | grep nginx | grep -E ':(8100|8143)'
if [ $? -eq 0 ]; then
    echo "   ✓ nginx is listening on game server ports"
else
    echo "   ✗ nginx is NOT listening on ports 8100 or 8143"
    echo "   Check if mmorts-game is enabled in sites-enabled/"
fi
echo ""

echo "6. Checking if mmorts-game config is enabled..."
if [ -L "/etc/nginx/sites-enabled/mmorts-game" ]; then
    echo "   ✓ Site is enabled"
    ls -la /etc/nginx/sites-enabled/mmorts-game
else
    echo "   ✗ Site is NOT enabled"
    echo "   Run: sudo ln -s /etc/nginx/sites-available/mmorts-game /etc/nginx/sites-enabled/"
fi
echo ""

echo "7. Checking nginx configuration syntax..."
nginx -t 2>&1 | tail -2
echo ""

echo "8. Checking for connection_upgrade variable..."
if grep -q "connection_upgrade" /etc/nginx/nginx.conf; then
    echo "   ✓ Map directive found in nginx.conf"
else
    echo "   ✗ Map directive NOT found in nginx.conf"
    echo "   Add this to http {} block in /etc/nginx/nginx.conf:"
    echo "   map \$http_upgrade \$connection_upgrade {"
    echo "       default upgrade;"
    echo "       '' close;"
    echo "   }"
fi
echo ""

echo "9. Checking firewall status..."
if command -v ufw &> /dev/null; then
    ufw status | grep -E '(8100|8143|8120)'
else
    echo "   UFW not installed, skipping"
fi
echo ""

echo "10. Checking nginx error log (last 20 lines)..."
if [ -f "/var/log/nginx/error.log" ]; then
    tail -20 /var/log/nginx/error.log
else
    echo "   Error log not found"
fi
echo ""

echo "11. Testing connections..."
echo "   Testing localhost:8110 (game server direct):"
curl -s -o /dev/null -w "   HTTP %{http_code}\n" http://localhost:8110/health

echo "   Testing localhost:8143 (nginx HTTPS):"
curl -k -s -o /dev/null -w "   HTTP %{http_code}\n" https://localhost:8143/health

echo ""
echo "12. Network connections..."
echo "   Processes listening on game server ports:"
ss -tlnp | grep -E ':(8100|8110|8120|8143)'
echo ""

echo "=== Diagnostic Complete ==="
echo ""
echo "If you see issues above, check:"
echo "- Game server container is running: docker ps"
echo "- Site is enabled: ls -la /etc/nginx/sites-enabled/"
echo "- Firewall allows ports: sudo ufw status"
echo "- Nginx error log: sudo tail -f /var/log/nginx/error.log"
