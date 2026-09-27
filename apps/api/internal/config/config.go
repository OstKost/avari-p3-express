package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the application.
type Config struct {
	PublicManagerOrigin string   `env:"PUBLIC_MANAGER_ORIGIN"`
	ManagerKeyPath      string   `env:"MANAGER_KEY_PATH" envDefault:"./data/manager.key"`
	Host                string   `env:"HOST" envDefault:"127.0.0.1"`
	Port                string   `env:"PORT" envDefault:"4820"`
	BaseURL             string   `env:"BASE_URL" envDefault:"http://localhost:4820"`
	DBPath              string   `env:"DB_PATH" envDefault:"./data/p3.db"`
	Environment         string   `env:"ENVIRONMENT" envDefault:"development"`
	AllowedOrigins      []string `env:"ALLOWED_ORIGINS" envDefault:"http://localhost:4810,http://localhost:4800,http://127.0.0.1:4810"`
}

// Load loads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config from environment: %w", err)
	}

	var err error
	cfg.PublicManagerOrigin, err = NormalizePublicManagerOrigin(cfg.PublicManagerOrigin)
	if err != nil {
		return nil, err
	}

	// Normalize base URL
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	return cfg, nil
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

// NormalizePublicManagerOrigin validates the single opt-in HTTPS browser origin.
func NormalizePublicManagerOrigin(origin string) (string, error) {
	if origin == "" {
		return "", nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" || strings.ContainsAny(origin, "#\\") {
		return "", fmt.Errorf("PUBLIC_MANAGER_ORIGIN must be one HTTPS origin without credentials, query, fragment or path")
	}
	return "https://" + u.Host, nil
}
