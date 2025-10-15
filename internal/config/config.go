package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all server configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	JWT      JWTConfig      `yaml:"jwt"`
	Redis    RedisConfig    `yaml:"redis"`
	Session  SessionConfig  `yaml:"session"`
	Chat     ChatConfig     `yaml:"chat"`
	Database DatabaseConfig `yaml:"database"`
}

// ServerConfig holds server-specific settings
type ServerConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	TickRate int    `yaml:"tick_rate"` // Hz
}

// JWTConfig holds JWT authentication settings
type JWTConfig struct {
	Issuer              string `yaml:"issuer"`
	PublicKeyURL        string `yaml:"public_key_url"`
	PublicKeyRefreshHrs int    `yaml:"public_key_refresh_hours"`
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Address         string `yaml:"address"`
	Password        string `yaml:"password"`
	DB              int    `yaml:"db"`
	BlacklistPrefix string `yaml:"blacklist_prefix"`
}

// SessionConfig holds game session settings
type SessionConfig struct {
	MaxPlayers       int `yaml:"max_players"`
	InitialMapRadius int `yaml:"initial_map_radius"` // Number of hex chunks from origin
}

// ChatConfig holds chat system settings
type ChatConfig struct {
	MaxMessageLength int `yaml:"max_message_length"`
	RateLimit        int `yaml:"rate_limit"` // messages per minute
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if not provided
	if cfg.Server.TickRate == 0 {
		cfg.Server.TickRate = 20
	}
	if cfg.JWT.PublicKeyRefreshHrs == 0 {
		cfg.JWT.PublicKeyRefreshHrs = 24
	}
	if cfg.Chat.MaxMessageLength == 0 {
		cfg.Chat.MaxMessageLength = 500
	}
	if cfg.Chat.RateLimit == 0 {
		cfg.Chat.RateLimit = 10
	}
	if cfg.Session.MaxPlayers == 0 {
		cfg.Session.MaxPlayers = 100
	}
	if cfg.Session.InitialMapRadius == 0 {
		cfg.Session.InitialMapRadius = 5
	}

	return &cfg, nil
}
