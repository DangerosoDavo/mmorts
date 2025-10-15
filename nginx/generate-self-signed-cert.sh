#!/bin/bash
# Generate self-signed SSL certificate for development/testing
# WARNING: Self-signed certificates will show browser security warnings
# For production, use Let's Encrypt or proper CA-signed certificates

set -e

echo "🔐 Generating self-signed SSL certificate for development..."

# Create certs directory if it doesn't exist
mkdir -p certs

# Generate self-signed certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout certs/key.pem \
  -out certs/cert.pem \
  -subj "/C=US/ST=State/L=City/O=MMORTS/CN=localhost"

# Set proper permissions
chmod 644 certs/cert.pem
chmod 600 certs/key.pem

echo "✅ Certificate generated successfully!"
echo ""
echo "📁 Files created:"
echo "  - certs/cert.pem (public certificate)"
echo "  - certs/key.pem (private key)"
echo ""
echo "⚠️  WARNING: This is a self-signed certificate!"
echo "   - Browsers will show security warnings"
echo "   - Only use for development/testing"
echo "   - For production, use Let's Encrypt or proper CA certificates"
echo ""
echo "🚀 You can now run: docker compose up -d"
