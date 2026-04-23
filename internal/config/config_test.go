package config

import (
	"os"
	"testing"
)

func TestLoad_TrustTunnel_Defaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("TRUSTTUNNEL_BINARY")
	os.Unsetenv("TRUSTTUNNEL_CONFIG_DIR")
	os.Unsetenv("TRUSTTUNNEL_DOMAIN")
	os.Unsetenv("TRUSTTUNNEL_PORT")

	cfg := Load()

	if cfg.TrustTunnelBinary != "/opt/trusttunnel/trusttunnel_endpoint" {
		t.Errorf("TrustTunnelBinary = %q, want %q", cfg.TrustTunnelBinary, "/opt/trusttunnel/trusttunnel_endpoint")
	}
	if cfg.TrustTunnelConfigDir != "/opt/trusttunnel" {
		t.Errorf("TrustTunnelConfigDir = %q, want %q", cfg.TrustTunnelConfigDir, "/opt/trusttunnel")
	}
	if cfg.TrustTunnelDomain != "usipipotunnel.duckdns.org" {
		t.Errorf("TrustTunnelDomain = %q, want %q", cfg.TrustTunnelDomain, "usipipotunnel.duckdns.org")
	}
	if cfg.TrustTunnelPort != 8443 {
		t.Errorf("TrustTunnelPort = %d, want %d", cfg.TrustTunnelPort, 8443)
	}
}

func TestLoad_TrustTunnel_CustomValues(t *testing.T) {
	os.Setenv("TRUSTTUNNEL_BINARY", "/custom/binary")
	os.Setenv("TRUSTTUNNEL_CONFIG_DIR", "/custom/config")
	os.Setenv("TRUSTTUNNEL_DOMAIN", "custom.example.com")
	os.Setenv("TRUSTTUNNEL_PORT", "9999")
	defer func() {
		os.Unsetenv("TRUSTTUNNEL_BINARY")
		os.Unsetenv("TRUSTTUNNEL_CONFIG_DIR")
		os.Unsetenv("TRUSTTUNNEL_DOMAIN")
		os.Unsetenv("TRUSTTUNNEL_PORT")
	}()

	cfg := Load()

	if cfg.TrustTunnelBinary != "/custom/binary" {
		t.Errorf("TrustTunnelBinary = %q, want %q", cfg.TrustTunnelBinary, "/custom/binary")
	}
	if cfg.TrustTunnelConfigDir != "/custom/config" {
		t.Errorf("TrustTunnelConfigDir = %q, want %q", cfg.TrustTunnelConfigDir, "/custom/config")
	}
	if cfg.TrustTunnelDomain != "custom.example.com" {
		t.Errorf("TrustTunnelDomain = %q, want %q", cfg.TrustTunnelDomain, "custom.example.com")
	}
	if cfg.TrustTunnelPort != 9999 {
		t.Errorf("TrustTunnelPort = %d, want %d", cfg.TrustTunnelPort, 9999)
	}
}

func TestLoad_TrustTunnel_InvalidPort(t *testing.T) {
	os.Setenv("TRUSTTUNNEL_PORT", "invalid")
	defer os.Unsetenv("TRUSTTUNNEL_PORT")

	cfg := Load()

	// Should fall back to default 8443
	if cfg.TrustTunnelPort != 8443 {
		t.Errorf("TrustTunnelPort = %d, want 8443 (invalid port should use default)", cfg.TrustTunnelPort)
	}
}

func TestLoad_WGValidateKeys_Default(t *testing.T) {
	os.Unsetenv("WG_VALIDATE_KEYS")
	cfg := Load()

	if !cfg.WGValidateKeys {
		t.Errorf("WGValidateKeys = %v, want true (default)", cfg.WGValidateKeys)
	}
}

func TestLoad_WGValidateKeys_CustomFalse(t *testing.T) {
	os.Setenv("WG_VALIDATE_KEYS", "false")
	defer os.Unsetenv("WG_VALIDATE_KEYS")

	cfg := Load()

	if cfg.WGValidateKeys {
		t.Errorf("WGValidateKeys = %v, want false", cfg.WGValidateKeys)
	}
}

func TestLoad_WGValidateKeys_CustomTrue(t *testing.T) {
	os.Setenv("WG_VALIDATE_KEYS", "true")
	defer os.Unsetenv("WG_VALIDATE_KEYS")

	cfg := Load()

	if !cfg.WGValidateKeys {
		t.Errorf("WGValidateKeys = %v, want true", cfg.WGValidateKeys)
	}
}

func TestLoad_ConfigStrictPerms_Default(t *testing.T) {
	os.Unsetenv("CONFIG_STRICT_PERMS")
	cfg := Load()

	if cfg.ConfigStrictPerms {
		t.Errorf("ConfigStrictPerms = %v, want false (default)", cfg.ConfigStrictPerms)
	}
}

func TestLoad_ConfigStrictPerms_CustomTrue(t *testing.T) {
	os.Setenv("CONFIG_STRICT_PERMS", "true")
	defer os.Unsetenv("CONFIG_STRICT_PERMS")

	cfg := Load()

	if !cfg.ConfigStrictPerms {
		t.Errorf("ConfigStrictPerms = %v, want true", cfg.ConfigStrictPerms)
	}
}

func TestLoad_ConfigStrictPerms_CustomFalse(t *testing.T) {
	os.Setenv("CONFIG_STRICT_PERMS", "false")
	defer os.Unsetenv("CONFIG_STRICT_PERMS")

	cfg := Load()

	if cfg.ConfigStrictPerms {
		t.Errorf("ConfigStrictPerms = %v, want false", cfg.ConfigStrictPerms)
	}
}
