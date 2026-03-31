package config

import (
	"log"
	"os"
	"strconv"
	"time"
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
	HTTPClientTimeout  time.Duration
}

// Load loads configuration from environment variables
func Load() *Config {
	// Parse rate limit settings
	rps, _ := strconv.ParseFloat(getEnv("RATE_LIMIT_RPS", "10.0"), 64)
	burst, _ := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "20"))
	enabled := getEnv("RATE_LIMIT_ENABLED", "true") == "true"
	
	// Parse HTTP client timeout
	timeout, _ := time.ParseDuration(getEnv("HTTP_CLIENT_TIMEOUT", "30s"))
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	cfg := &Config{
		Port:               getEnv("AGENT_PORT", "8080"),
		APIKey:             getEnv("AGENT_API_KEY", ""),
		BackendURL:         getEnv("BACKEND_URL", ""),
		ServerID:           getEnv("SERVER_ID", ""),
		OutlineAPIURL:      getEnv("OUTLINE_API_URL", "http://localhost:8081"),
		OutlineVerifySSL:   getEnv("OUTLINE_VERIFY_SSL", "true") == "true", // SECURE DEFAULT
		WireGuardInterface: getEnv("WG_INTERFACE", "wg0"),
		AgentURL:           getEnv("AGENT_URL", "http://localhost:8080"),
		SupportsOutline:    getEnv("SUPPORTS_OUTLINE", "true") == "true",
		SupportsWireGuard:  getEnv("SUPPORTS_WIREGUARD", "true") == "true",
		RateLimitEnabled:   enabled,
		RateLimitRPS:       rps,
		RateLimitBurst:     burst,
		HTTPClientTimeout:  timeout,
	}
	
	// Log warning if TLS verification is disabled
	if !cfg.OutlineVerifySSL {
		log.Println("⚠️  WARNING: TLS verification is DISABLED (OUTLINE_VERIFY_SSL=false)")
		log.Println("   This is INSECURE for production use and should only be used for development")
		log.Println("   with self-signed certificates. Set OUTLINE_VERIFY_SSL=true for secure operation.")
	}

	return cfg
}

// getEnv gets environment variable or returns default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
