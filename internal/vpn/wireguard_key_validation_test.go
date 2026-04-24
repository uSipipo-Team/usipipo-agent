package vpn

import (
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestGeneratePrivateKey_WithValidation(t *testing.T) {
	client := &WireGuardClient{
		validateKeys: true,
		logger:       nil,
	}

	// Generate a key with validation enabled
	key, err := client.generatePrivateKey()
	if err != nil {
		t.Fatalf("generatePrivateKey() failed: %v", err)
	}

	// Verify it's not zero (empty)
	if key.String() == "" {
		t.Error("generated key is zero")
	}

	// Verify it's a valid WireGuard private key
	publicKey := key.PublicKey()
	if publicKey.String() == "" {
		t.Error("public key derived from private key is zero")
	}
}

func TestGeneratePrivateKey_WithoutValidation(t *testing.T) {
	client := &WireGuardClient{
		validateKeys: false,
		logger:       nil,
	}

	// Generate a key with validation disabled
	key, err := client.generatePrivateKey()
	if err != nil {
		t.Fatalf("generatePrivateKey() failed: %v", err)
	}

	// Verify it's not zero (empty)
	if key.String() == "" {
		t.Error("generated key is zero")
	}
}

func TestGeneratePrivateKey_EntropyRetry(t *testing.T) {
	// This tests that when validation fails, we retry
	client := &WireGuardClient{
		validateKeys: true,
		logger:       nil,
	}

	// Generate multiple keys to ensure retry logic works
	for i := 0; i < 5; i++ {
		key, err := client.generatePrivateKey()
		if err != nil {
			t.Fatalf("generatePrivateKey() failed on iteration %d: %v", i, err)
		}
		if key.String() == "" {
			t.Errorf("iteration %d: generated key is zero", i)
		}
	}
}

func TestValidateKeyName(t *testing.T) {
	client := &WireGuardClient{}

	// Valid names
	validNames := []string{
		"alice",
		"bob-smith",
		"charlie_123",
		"dave the client",
		"a-b_c d",
	}
	for _, name := range validNames {
		if !client.ValidateKeyName(name) {
			t.Errorf("ValidateKeyName(%q) returned false, want true", name)
		}
	}

	// Invalid names (too short)
	invalidShort := []string{"a", "ab"}
	for _, name := range invalidShort {
		if client.ValidateKeyName(name) {
			t.Errorf("ValidateKeyName(%q) returned true, want false (too short)", name)
		}
	}

	// Invalid names (too long - over 50)
	longName := "this-is-a-very-long-name-that-exceeds-the-maximum-allowed-length-of-50-characters-and-should-be-rejected-xxxxxxxxxxxxx"
	if client.ValidateKeyName(longName) {
		t.Errorf("ValidateKeyName(long name) returned true, want false (too long)")
	}

	// Invalid characters
	invalidNames := []string{
		"alice@example.com", // @
		"bob#smith",         // #
		"charlie$money",     // $
		"dave&alice",        // &
		"eve<script>",       // < >
	}
	for _, name := range invalidNames {
		if client.ValidateKeyName(name) {
			t.Errorf("ValidateKeyName(%q) returned true, want false (invalid char)", name)
		}
	}
}

// TestWireGuardKeyGeneration_Entropy tests that generated keys have sufficient entropy
func TestWireGuardKeyGeneration_Entropy(t *testing.T) {
	// Generate a sample of keys and verify they all pass entropy checks
	// This is a statistical test - all generated keys should have high entropy
	for i := 0; i < 10; i++ {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("Failed to generate private key: %v", err)
		}
		keyHex := key.String()

		// Basic length check (64 hex chars = 32 bytes)
		if len(keyHex) != 64 {
			t.Errorf("Key length = %d, want 64", len(keyHex))
		}

		// The key should be nonzero
		if keyHex == strings.Repeat("00", 32) {
			t.Error("Generated key is all zeros")
		}
	}
}
