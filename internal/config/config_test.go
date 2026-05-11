package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_ENV_SET", "custom_value")
	defer os.Unsetenv("TEST_ENV_SET")

	result := getEnv("TEST_ENV_SET", "default")
	if result != "custom_value" {
		t.Errorf("expected 'custom_value', got '%s'", result)
	}

	result = getEnv("TEST_ENV_UNSET", "fallback")
	if result != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", result)
	}
}

func TestLoad(t *testing.T) {
	os.Setenv("AGENT_PORT", "9090")
	os.Setenv("BACKEND_URL", "https://backend.example.com")
	os.Setenv("WG_INTERFACE", "wg1")
	os.Setenv("WIREGUARD_NETWORK_CIDR", "10.99.99.0/24")
	os.Setenv("WG_SERVER_PORT", "51820")
	os.Setenv("AGENT_API_KEY", "agent_testkey12345678901234567890123456")
	os.Setenv("SERVER_ID", "test-server-1")
	os.Setenv("WG_SERVER_IP", "10.0.0.1")
	os.Setenv("SUPPORTS_WIREGUARD", "false")
	os.Setenv("RATE_LIMIT_ENABLED", "false")
	os.Setenv("RATE_LIMIT_RPS", "5.5")
	os.Setenv("RATE_LIMIT_BURST", "10")
	os.Setenv("ENABLE_DB_IP_ALLOCATION", "true")
	defer func() {
		os.Unsetenv("AGENT_PORT")
		os.Unsetenv("BACKEND_URL")
		os.Unsetenv("WG_INTERFACE")
		os.Unsetenv("WIREGUARD_NETWORK_CIDR")
		os.Unsetenv("WG_SERVER_PORT")
		os.Unsetenv("AGENT_API_KEY")
		os.Unsetenv("SERVER_ID")
		os.Unsetenv("WG_SERVER_IP")
		os.Unsetenv("SUPPORTS_WIREGUARD")
		os.Unsetenv("RATE_LIMIT_ENABLED")
		os.Unsetenv("RATE_LIMIT_RPS")
		os.Unsetenv("RATE_LIMIT_BURST")
		os.Unsetenv("ENABLE_DB_IP_ALLOCATION")
	}()

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected Port '9090', got '%s'", cfg.Port)
	}
	if cfg.BackendURL != "https://backend.example.com" {
		t.Errorf("expected BackendURL 'https://backend.example.com', got '%s'", cfg.BackendURL)
	}
	if cfg.WireGuardInterface != "wg1" {
		t.Errorf("expected WireGuardInterface 'wg1', got '%s'", cfg.WireGuardInterface)
	}
	if cfg.WireGuardNetworkCIDR != "10.99.99.0/24" {
		t.Errorf("expected WireGuardNetworkCIDR '10.99.99.0/24', got '%s'", cfg.WireGuardNetworkCIDR)
	}
	if cfg.WireGuardServerPort != 51820 {
		t.Errorf("expected WireGuardServerPort 51820, got %d", cfg.WireGuardServerPort)
	}
	if cfg.ServerID != "test-server-1" {
		t.Errorf("expected ServerID 'test-server-1', got '%s'", cfg.ServerID)
	}
	if cfg.WireGuardServerIP != "10.0.0.1" {
		t.Errorf("expected WireGuardServerIP '10.0.0.1', got '%s'", cfg.WireGuardServerIP)
	}
	if cfg.SupportsWireGuard != false {
		t.Errorf("expected SupportsWireGuard false")
	}
	if cfg.RateLimitEnabled != false {
		t.Errorf("expected RateLimitEnabled false")
	}
	if cfg.RateLimitRPS != 5.5 {
		t.Errorf("expected RateLimitRPS 5.5, got %f", cfg.RateLimitRPS)
	}
	if cfg.RateLimitBurst != 10 {
		t.Errorf("expected RateLimitBurst 10, got %d", cfg.RateLimitBurst)
	}
	if cfg.BackendAPIURL != "https://backend.example.com" {
		t.Errorf("expected BackendAPIURL to default to BackendURL, got '%s'", cfg.BackendAPIURL)
	}
	if cfg.EnableDBIPAllocation != true {
		t.Errorf("expected EnableDBIPAllocation true")
	}
}

func TestLoadInvalidPort(t *testing.T) {
	os.Setenv("WG_SERVER_PORT", "invalid")
	defer os.Unsetenv("WG_SERVER_PORT")

	cfg := Load()

	if cfg.WireGuardServerPort != 64465 {
		t.Errorf("expected default port 64465, got %d", cfg.WireGuardServerPort)
	}
}

func TestValidateAPIKey(t *testing.T) {
	cfg := &Config{APIKey: "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"}
	if err := cfg.ValidateAPIKey(); err != nil {
		t.Errorf("expected valid key, got error: %v", err)
	}

	cfg = &Config{APIKey: ""}
	if err := cfg.ValidateAPIKey(); err == nil {
		t.Error("expected error for empty API key")
	}

	cfg = &Config{APIKey: "invalid_key"}
	if err := cfg.ValidateAPIKey(); err == nil {
		t.Error("expected error for malformed API key")
	}

	cfg = &Config{APIKey: "agent_short"}
	if err := cfg.ValidateAPIKey(); err == nil {
		t.Error("expected error for too-short API key")
	}
}

func TestLoadIPRange(t *testing.T) {
	os.Setenv("WIREGUARD_START_IP", "10")
	os.Setenv("WIREGUARD_END_IP", "200")
	defer func() {
		os.Unsetenv("WIREGUARD_START_IP")
		os.Unsetenv("WIREGUARD_END_IP")
	}()

	cfg := Load()

	if cfg.WireGuardStartIP != 10 {
		t.Errorf("expected WireGuardStartIP 10, got %d", cfg.WireGuardStartIP)
	}
	if cfg.WireGuardEndIP != 200 {
		t.Errorf("expected WireGuardEndIP 200, got %d", cfg.WireGuardEndIP)
	}
}

func TestLoadIPRangeInvalid(t *testing.T) {
	os.Setenv("WIREGUARD_START_IP", "250")
	os.Setenv("WIREGUARD_END_IP", "10")
	defer func() {
		os.Unsetenv("WIREGUARD_START_IP")
		os.Unsetenv("WIREGUARD_END_IP")
	}()

	cfg := Load()

	if cfg.WireGuardStartIP != 2 {
		t.Errorf("expected default start IP 2, got %d", cfg.WireGuardStartIP)
	}
	if cfg.WireGuardEndIP != 254 {
		t.Errorf("expected default end IP 254, got %d", cfg.WireGuardEndIP)
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got '%s'", cfg.Port)
	}
	if cfg.WireGuardInterface != "wg0" {
		t.Errorf("expected default WireGuardInterface 'wg0', got '%s'", cfg.WireGuardInterface)
	}
	if cfg.WireGuardNetworkCIDR != "10.88.88.0/24" {
		t.Errorf("expected default CIDR '10.88.88.0/24', got '%s'", cfg.WireGuardNetworkCIDR)
	}
	if cfg.WireGuardServerPort != 64465 {
		t.Errorf("expected default port 64465, got %d", cfg.WireGuardServerPort)
	}
}
