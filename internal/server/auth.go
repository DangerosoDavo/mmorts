package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gravitas-games/mmorts/internal/config"
	"github.com/gravitas-games/mmorts/pkg/models"
)

// JWTValidator handles JWT token validation
type JWTValidator struct {
	config    *config.Config
	publicKey *ecdsa.PublicKey
	keyMu     sync.RWMutex
	redis     *redis.Client
	ctx       context.Context
}

// Claims represents JWT token claims from GoLoginServer
type Claims struct {
	UserID      int64  `json:"user_id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	UserType    string `json:"user_type"`
	AuthMethod  string `json:"auth_method"`
	Permissions int64  `json:"permissions"`
	Activated   int64  `json:"activated"`
	jwt.RegisteredClaims
}

// NewJWTValidator creates a new JWT validator
func NewJWTValidator(cfg *config.Config, redisClient *redis.Client) (*JWTValidator, error) {
	validator := &JWTValidator{
		config: cfg,
		redis:  redisClient,
		ctx:    context.Background(),
	}

	// Fetch public key from GoLoginServer
	if err := validator.RefreshPublicKey(); err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}

	// Start background key refresh
	go validator.periodicKeyRefresh()

	log.Println("JWT validator initialized")
	return validator, nil
}

// RefreshPublicKey fetches the public key from GoLoginServer
func (v *JWTValidator) RefreshPublicKey() error {
	log.Printf("Fetching public key from %s", v.config.JWT.PublicKeyURL)

	resp, err := http.Get(v.config.JWT.PublicKeyURL)
	if err != nil {
		return fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("public key endpoint returned status %d", resp.StatusCode)
	}

	keyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Parse PEM-encoded public key
	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}

	// Parse ECDSA public key
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	ecdsaKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not ECDSA")
	}

	// Store public key
	v.keyMu.Lock()
	v.publicKey = ecdsaKey
	v.keyMu.Unlock()

	log.Println("Public key refreshed successfully")
	return nil
}

// periodicKeyRefresh refreshes the public key periodically
func (v *JWTValidator) periodicKeyRefresh() {
	refreshInterval := time.Duration(v.config.JWT.PublicKeyRefreshHrs) * time.Hour

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := v.RefreshPublicKey(); err != nil {
			log.Printf("Failed to refresh public key: %v", err)
		}
	}
}

// ValidateToken validates a JWT token and returns player information
func (v *JWTValidator) ValidateToken(tokenString string) (*models.Player, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		v.keyMu.RLock()
		defer v.keyMu.RUnlock()
		return v.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate issuer
	if claims.Issuer != v.config.JWT.Issuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", v.config.JWT.Issuer, claims.Issuer)
	}

	// Validate expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	// Validate activation status
	if claims.Activated == 0 {
		return nil, fmt.Errorf("user not activated")
	}

	if claims.Activated == -1 {
		return nil, fmt.Errorf("user is banned")
	}

	// Check Redis blacklist
	userIDStr := strconv.FormatInt(claims.UserID, 10)
	blacklistKey := fmt.Sprintf("%s%s", v.config.Redis.BlacklistPrefix, userIDStr)

	isBlacklisted, err := v.redis.Exists(v.ctx, blacklistKey).Result()
	if err != nil {
		log.Printf("Warning: Failed to check blacklist: %v", err)
		// Continue anyway - don't fail authentication if Redis is down
	} else if isBlacklisted > 0 {
		return nil, fmt.Errorf("token is blacklisted")
	}

	// Create player model from claims
	player := &models.Player{
		ID:          userIDStr,
		Username:    claims.Username,
		Email:       claims.Email,
		UserType:    claims.UserType,
		Permissions: claims.Permissions,
		Activated:   claims.Activated,
		AuthMethod:  claims.AuthMethod,
		Connected:   false,
		EmpireID:    userIDStr, // Auto-assign empire ID = user ID for Phase 1
	}

	return player, nil
}

// extractTokenFromHeader extracts JWT token from WebSocket connection header
func extractTokenFromHeader(r *http.Request) string {
	// Try Sec-WebSocket-Protocol header first (recommended)
	protocols := r.Header.Get("Sec-WebSocket-Protocol")
	if protocols != "" {
		// Format: "access_token, <token>"
		parts := parseProtocols(protocols)
		if len(parts) == 2 && parts[0] == "access_token" {
			return parts[1]
		}
	}

	// Try Authorization header
	auth := r.Header.Get("Authorization")
	if auth != "" && len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}

	// Try query parameter (less secure, but supported)
	token := r.URL.Query().Get("token")
	if token != "" {
		return token
	}

	return ""
}

// parseProtocols parses the Sec-WebSocket-Protocol header
func parseProtocols(protocols string) []string {
	var result []string
	for _, p := range splitAndTrim(protocols, ",") {
		result = append(result, p)
	}
	return result
}

// splitAndTrim splits a string and trims each part
func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitString(s, sep string) []string {
	// Simple string split implementation
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	// Trim leading and trailing spaces
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}
