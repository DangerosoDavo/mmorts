#!/bin/bash
# Quick setup script for nginx WSS configuration

set -e

echo "=== MMORTS Nginx WSS Setup ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run with sudo: sudo ./setup-nginx-wss.sh"
    exit 1
fi

# 1. Backup nginx.conf
echo "1. Backing up nginx.conf..."
cp /etc/nginx/nginx.conf /etc/nginx/nginx.conf.backup.$(date +%Y%m%d-%H%M%S)
echo "   ✓ Backup created"

# 2. Check if map directive already exists
if grep -q "connection_upgrade" /etc/nginx/nginx.conf; then
    echo "2. Map directive already exists in nginx.conf"
else
    echo "2. Adding map directive to nginx.conf..."

    # Add map directive after the opening http { block
    sed -i '/^http {/a\
\
    # WebSocket upgrade map\
    map $http_upgrade $connection_upgrade {\
        default upgrade;\
        '\'''\'' close;\
    }' /etc/nginx/nginx.conf

    echo "   ✓ Map directive added"
fi

# 3. Copy and configure site config
echo "3. Setting up site configuration..."

DOMAIN="mmorts.gravitas-games.com"
read -p "   Enter your domain [$DOMAIN]: " INPUT_DOMAIN
DOMAIN=${INPUT_DOMAIN:-$DOMAIN}

# Copy the config
cp nginx/mmorts-game.conf /etc/nginx/sites-available/mmorts-game

# Update domain in config
sed -i "s/your-domain.com/$DOMAIN/g" /etc/nginx/sites-available/mmorts-game

echo "   ✓ Configuration copied and domain set to: $DOMAIN"

# 4. Check SSL certificates
echo "4. Checking SSL certificates..."
if [ -d "/etc/letsencrypt/live/$DOMAIN" ]; then
    echo "   ✓ SSL certificates found for $DOMAIN"
else
    echo "   ⚠ SSL certificates NOT found for $DOMAIN"
    echo "   Please run: sudo certbot --nginx -d $DOMAIN"
    echo "   Or update the SSL paths in /etc/nginx/sites-available/mmorts-game"
fi

# 5. Enable site
echo "5. Enabling site..."
if [ -L "/etc/nginx/sites-enabled/mmorts-game" ]; then
    echo "   Site already enabled"
else
    ln -s /etc/nginx/sites-available/mmorts-game /etc/nginx/sites-enabled/mmorts-game
    echo "   ✓ Site enabled"
fi

# 6. Test configuration
echo "6. Testing nginx configuration..."
if nginx -t; then
    echo "   ✓ Configuration test passed"
else
    echo "   ✗ Configuration test failed"
    echo "   Please check the errors above"
    exit 1
fi

# 7. Ask to reload
read -p "7. Reload nginx now? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    systemctl reload nginx
    echo "   ✓ Nginx reloaded"
else
    echo "   Skipped. Run 'sudo systemctl reload nginx' when ready"
fi

# 8. Check firewall
echo ""
echo "8. Checking firewall..."
if command -v ufw &> /dev/null; then
    if ufw status | grep -q "8143.*ALLOW"; then
        echo "   ✓ Port 8143 is open"
    else
        echo "   ⚠ Port 8143 not open in firewall"
        read -p "   Open port 8143? (y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            ufw allow 8143/tcp
            echo "   ✓ Port 8143 opened"
        fi
    fi

    if ufw status | grep -q "8100.*ALLOW"; then
        echo "   ✓ Port 8100 is open"
    else
        echo "   ⚠ Port 8100 not open in firewall"
        read -p "   Open port 8100? (y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            ufw allow 8100/tcp
            echo "   ✓ Port 8100 opened"
        fi
    fi
else
    echo "   UFW not found, skipping firewall check"
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "1. Ensure your game server is running:"
echo "   cd /path/to/mmorts && docker compose up -d"
echo ""
echo "2. Test the connection:"
echo "   curl http://localhost:8110/health"
echo "   curl https://$DOMAIN:8143/health"
echo ""
echo "3. Connect from your web client:"
echo "   const ws = new WebSocket('wss://$DOMAIN:8143/ws', ['access_token', token]);"
echo ""
echo "Troubleshooting guide: WEBSOCKET_TROUBLESHOOTING.md"
