package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/uSipipo-Team/usipipo-agent/internal/utils/validation"
)

// Config holds the agent configuration
type Config struct {
	Port               string
	APIKey             string
	BackendURL         string
	ServerID           string
	OutlineAPIURL      string
	OutlineVerifySSL   bool
	WireGuardInterface string
	AgentURL           string
	SupportsOutline    bool
	SupportsWireGuard  bool
	RateLimitEnabled   bool
	RateLimitRPS       float64
	RateLimitBurst     int
}

// Load loads configuration from environment variables
func Load() *Config {
	// Parse rate limit settings
	rps, _ := strconv.ParseFloat(getEnv("RATE_LIMIT_RPS", "10.0"), 64)
	burst, _ := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "20"))
	enabled := getEnv("RATE_LIMIT_ENABLED", "true") == "true"

	return &Config{
		Port:               getEnv("AGENT_PORT", "8080"),
		APIKey:             getEnv("AGENT_API_KEY", ""),
		BackendURL:         getEnv("BACKEND_URL", ""),
		ServerID:           getEnv("SERVER_ID", ""),
		OutlineAPIURL:      getEnv("OUTLINE_API_URL", "http://localhost:8081"),
		OutlineVerifySSL:   getEnv("OUTLINE_VERIFY_SSL", "false") == "true",
		WireGuardInterface: getEnv("WG_INTERFACE", "wg0"),
		AgentURL:           getEnv("AGENT_URL", "http://localhost:8080"),
		SupportsOutline:    getEnv("SUPPORTS_OUTLINE", "true") == "true",
		SupportsWireGuard:  getEnv("SUPPORTS_WIREGUARD", "true") == "true",
		RateLimitEnabled:   enabled,
		RateLimitRPS:       rps,
		RateLimitBurst:     burst,
	}
}

// ValidateAPIKey checks if the API key meets security requirements
// Returns error if key is invalid, nil if valid
func (c *Config) ValidateAPIKey() error {
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if !validation.IsValidAPIKeyFormat(c.APIKey) {
		return fmt.Errorf("API key does not match required format: %s",
			validation.APIKeyFormat())
	}

	return nil
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
