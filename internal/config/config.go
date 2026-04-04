package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/uSipipo-Team/usipipo-agent/internal/utils/validation"
)

// Config holds the agent configuration
type Config struct {
	Port                  string
	APIKey                string
	BackendURL            string
	ServerID              string
	OutlineAPIURL         string
	OutlineVerifySSL      bool
	WireGuardInterface    string
	WireGuardServerIP     string
	WireGuardServerPort   int
	WireGuardNetworkCIDR  string
	WireGuardStartIP      int
	WireGuardEndIP        int
	AgentURL              string
	SupportsOutline       bool
	SupportsWireGuard     bool
	RateLimitEnabled      bool
	RateLimitRPS          float64
	RateLimitBurst        int
	HTTPClientTimeout     time.Duration
}

// Load loads configuration from environment variables
func Load() *Config {
	// Parse rate limit settings
	rps, _ := strconv.ParseFloat(getEnv("RATE_LIMIT_RPS", "10.0"), 64)
	burst, _ := strconv.Atoi(getEnv("RATE_LIMIT_BURST", "20"))
	enabled := getEnv("RATE_LIMIT_ENABLED", "true") == "true"

	// Parse HTTP client timeout
	timeout, err := time.ParseDuration(getEnv("HTTP_CLIENT_TIMEOUT", "30s"))
	if err != nil {
		log.Printf("⚠️  WARNING: Invalid HTTP_CLIENT_TIMEOUT '%s': %v. Using default 30s",
			getEnv("HTTP_CLIENT_TIMEOUT", "30s"), err)
		timeout = 30 * time.Second
	}
	if timeout <= 0 {
		log.Printf("⚠️  WARNING: HTTP_CLIENT_TIMEOUT must be positive. Using default 30s")
		timeout = 30 * time.Second
	}

	// Parse WireGuard IP range settings
	startIP, _ := strconv.Atoi(getEnv("WIREGUARD_START_IP", "2"))
	endIP, _ := strconv.Atoi(getEnv("WIREGUARD_END_IP", "254"))

	// Validate IP range, use defaults if invalid
	if startIP < 2 || endIP > 254 || startIP >= endIP {
		log.Printf("⚠️  WARNING: Invalid WIREGUARD IP range (start=%d, end=%d). Using defaults (2-254)", startIP, endIP)
		startIP = 2
		endIP = 254
	}

	// Parse WireGuard server port
	wgPort, err := strconv.Atoi(getEnv("WG_SERVER_PORT", "64465"))
	if err != nil || wgPort < 1 || wgPort > 65535 {
		log.Printf("⚠️  WARNING: Invalid WG_SERVER_PORT '%s'. Using default 64465", getEnv("WG_SERVER_PORT", "64465"))
		wgPort = 64465
	}

	cfg := &Config{
		Port:                  getEnv("AGENT_PORT", "8080"),
		APIKey:                getEnv("AGENT_API_KEY", ""),
		BackendURL:            getEnv("BACKEND_URL", ""),
		ServerID:              getEnv("SERVER_ID", ""),
		OutlineAPIURL:         getEnv("OUTLINE_API_URL", "http://localhost:8081"),
		OutlineVerifySSL:      getEnv("OUTLINE_VERIFY_SSL", "true") == "true", // SECURE DEFAULT
		WireGuardInterface:    getEnv("WG_INTERFACE", "wg0"),
		WireGuardServerIP:     getEnv("WG_SERVER_IP", "165.140.241.96"),
		WireGuardServerPort:   wgPort,
		WireGuardNetworkCIDR:  getEnv("WIREGUARD_NETWORK_CIDR", "10.88.88.0/24"),
		WireGuardStartIP:      startIP,
		WireGuardEndIP:        endIP,
		AgentURL:              getEnv("AGENT_URL", "http://localhost:8080"),
		SupportsOutline:       getEnv("SUPPORTS_OUTLINE", "true") == "true",
		SupportsWireGuard:     getEnv("SUPPORTS_WIREGUARD", "true") == "true",
		RateLimitEnabled:      enabled,
		RateLimitRPS:          rps,
		RateLimitBurst:        burst,
		HTTPClientTimeout:     timeout,
	}
	
	// Log warning if TLS verification is disabled
	if !cfg.OutlineVerifySSL {
		log.Println("⚠️  WARNING: TLS verification is DISABLED (OUTLINE_VERIFY_SSL=false)")
		log.Println("   This is INSECURE for production use and should only be used for development")
		log.Println("   with self-signed certificates. Set OUTLINE_VERIFY_SSL=true for secure operation.")
	}

	return cfg
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
