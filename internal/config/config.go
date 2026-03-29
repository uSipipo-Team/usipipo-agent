package config

import (
	"os"
)

// Config holds the agent configuration
type Config struct {
	Port               string
	APIKey             string
	BackendURL         string
	ServerID           string
	OutlineAPIURL      string
	WireGuardInterface string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		Port:               getEnv("AGENT_PORT", "8080"),
		APIKey:             getEnv("AGENT_API_KEY", ""),
		BackendURL:         getEnv("BACKEND_URL", ""),
		ServerID:           getEnv("SERVER_ID", ""),
		OutlineAPIURL:      getEnv("OUTLINE_API_URL", "http://localhost:8081"),
		WireGuardInterface: getEnv("WG_INTERFACE", "wg0"),
	}
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
