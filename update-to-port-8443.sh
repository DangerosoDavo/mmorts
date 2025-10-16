#!/bin/bash
# Update MMORTS WebSocket to use Cloudflare-compatible port 8443

echo "=== Updating to Port 8443 (Cloudflare Compatible) ==="
echo ""

if [ "$EUID" -ne 0 ]; then
    echo "Please run with sudo: sudo ./update-to-port-8443.sh"
    exit 1
fi

# 1. Update nginx config
echo "1. Updating nginx configuration..."
cp /etc/nginx/sites-available/mmorts-game.conf /etc/nginx/sites-available/mmorts-game.conf.backup.$(date +%Y%m%d-%H%M%S)
sed -i 's/:8143/:8443/g' /etc/nginx/sites-available/mmorts-game.conf
sed -i 's/listen 8143/listen 8443/g' /etc/nginx/sites-available/mmorts-game.conf
echo "   ✓ Config updated"

# 2. Test nginx config
echo ""
echo "2. Testing nginx configuration..."
if nginx -t 2>&1 | tail -1 | grep -q "successful"; then
    echo "   ✓ Nginx config is valid"
else
    echo "   ✗ Nginx config has errors"
    nginx -t
    exit 1
fi

# 3. Reload nginx
echo ""
echo "3. Reloading nginx..."
systemctl reload nginx
echo "   ✓ Nginx reloaded"

# 4. Update firewall
echo ""
echo "4. Updating firewall rules..."
if command -v ufw &> /dev/null; then
    ufw allow 8443/tcp
    ufw delete allow 8143/tcp 2>/dev/null
    ufw reload
    echo "   ✓ Firewall updated (port 8443 allowed, 8143 removed)"
else
    echo "   UFW not found, skipping firewall update"
fi

# 5. Test the endpoint
echo ""
echo "5. Testing new endpoint..."
HTTP_CODE=$(curl -k -s -o /dev/null -w "%{http_code}" https://localhost:8443/health 2>/dev/null)
if [ "$HTTP_CODE" = "200" ]; then
    echo "   ✓ Health endpoint responds: HTTP $HTTP_CODE"
else
    echo "   ⚠ Health endpoint returned: HTTP $HTTP_CODE"
    echo "   Check nginx error log: sudo tail -f /var/log/nginx/error.log"
fi

echo ""
echo "=== Update Complete ==="
echo ""
echo "Port 8443 is now active (Cloudflare compatible)"
echo ""
echo "Update your web client to connect to:"
echo "  const ws = new WebSocket('wss://mmorts.gravitas-games.com:8443/ws', ['access_token', token]);"
echo ""
echo "Test the connection:"
echo "  curl -k https://mmorts.gravitas-games.com:8443/health"
