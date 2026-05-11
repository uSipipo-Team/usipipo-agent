package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/uSipipo-Team/usipipo-agent/internal/utils/validation"
)

// Config holds the agent configuration
type Config struct {
	Port                 string
	APIKey               string
	BackendURL           string
	BackendAPIURL        string
	BackendAPIKey        string
	ServerID             string
	WireGuardInterface   string
	WireGuardServerIP    string
	WireGuardServerPort  int
	WireGuardNetworkCIDR string
	WireGuardStartIP     int
	WireGuardEndIP       int
	AgentURL             string
	SupportsWireGuard    bool
	RateLimitEnabled     bool
	RateLimitRPS         float64
	RateLimitBurst       int
	HTTPClientTimeout    time.Duration
	GeoIPEnabled         bool
	GeoIPTimeout         time.Duration
	GeoIPMaxRetries      int
	GeoIPRetryBackoff    time.Duration
	WGValidateKeys       bool
	ConfigStrictPerms    bool
	EnableDBIPAllocation bool
	WGLockPath           string
	ReconcileInterval    time.Duration
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
		Port:                 getEnv("AGENT_PORT", "8080"),
		APIKey:               getEnv("AGENT_API_KEY", ""),
		BackendURL:           getEnv("BACKEND_URL", ""),
		BackendAPIURL:        getEnv("BACKEND_API_URL", getEnv("BACKEND_URL", "")),
		BackendAPIKey:        getEnv("BACKEND_API_KEY", ""),
		ServerID:             getEnv("SERVER_ID", ""),
		WireGuardInterface:   getEnv("WG_INTERFACE", "wg0"),
		WireGuardServerIP:    getEnv("WG_SERVER_IP", "165.140.241.96"),
		WireGuardServerPort:  wgPort,
		WireGuardNetworkCIDR: getEnv("WIREGUARD_NETWORK_CIDR", "10.88.88.0/24"),
		WireGuardStartIP:     startIP,
		WireGuardEndIP:       endIP,
		AgentURL:             getEnv("AGENT_URL", "http://localhost:8080"),
		SupportsWireGuard:    getEnv("SUPPORTS_WIREGUARD", "true") == "true",
		RateLimitEnabled:     enabled,
		RateLimitRPS:         rps,
		RateLimitBurst:       burst,
		HTTPClientTimeout:    timeout,
	}
	cfg.EnableDBIPAllocation = getEnv("ENABLE_DB_IP_ALLOCATION", "false") == "true"
	cfg.WGLockPath = getEnv("WG_LOCK_PATH", "/var/run/usipipo-agent/ip_alloc.lock")

	// Parse reconcile interval
	reconcileInterval, err := time.ParseDuration(getEnv("WG_RECONCILE_INTERVAL", "10s"))
	if err != nil || reconcileInterval <= 0 {
		log.Printf("WARNING: Invalid WG_RECONCILE_INTERVAL '%s'. Using default 10s", getEnv("WG_RECONCILE_INTERVAL", "10s"))
		reconcileInterval = 10 * time.Second
	}
	cfg.ReconcileInterval = reconcileInterval

	// GeoIP settings
	geoIPEnabled := getEnv("GEOIP_ENABLED", "true") == "true"
	geoIPTimeout, err := time.ParseDuration(getEnv("GEOIP_TIMEOUT", "5s"))
	if err != nil || geoIPTimeout <= 0 {
		log.Printf("⚠️  WARNING: Invalid GEOIP_TIMEOUT '%s': %v. Using default 5s",
			getEnv("GEOIP_TIMEOUT", "5s"), err)
		geoIPTimeout = 5 * time.Second
	}
	geoIPMaxRetries, _ := strconv.Atoi(getEnv("GEOIP_MAX_RETRIES", "3"))
	if geoIPMaxRetries < 0 {
		geoIPMaxRetries = 3
	}
	geoIPRetryBackoff, err := time.ParseDuration(getEnv("GEOIP_RETRY_BACKOFF", "1s"))
	if err != nil || geoIPRetryBackoff <= 0 {
		geoIPRetryBackoff = 1 * time.Second
	}

	cfg.GeoIPEnabled = geoIPEnabled
	cfg.GeoIPTimeout = geoIPTimeout
	cfg.GeoIPMaxRetries = geoIPMaxRetries
	cfg.GeoIPRetryBackoff = geoIPRetryBackoff

	// WireGuard key validation
	cfg.WGValidateKeys = getEnv("WG_VALIDATE_KEYS", "true") == "true"

	// Config file permissions security
	cfg.ConfigStrictPerms = getEnv("CONFIG_STRICT_PERMS", "false") == "true"

	// Ensure lock directory exists
	if cfg.EnableDBIPAllocation && cfg.WGLockPath != "" {
		lockDir := filepath.Dir(cfg.WGLockPath)
		if err := os.MkdirAll(lockDir, 0750); err != nil {
			log.Printf("WARNING: Failed to create lock directory %s: %v", lockDir, err)
		}
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
