package config

import (
	"os"
	"strconv"
)

// Config holds the agent configuration
type Config struct {
	Port               string
	APIKey             string
	BackendURL         string
	ServerID           string
	OutlineAPIURL      string
	WireGuardInterface string
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
		WireGuardInterface: getEnv("WG_INTERFACE", "wg0"),
		RateLimitEnabled:   enabled,
		RateLimitRPS:       rps,
		RateLimitBurst:     burst,
	}
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
