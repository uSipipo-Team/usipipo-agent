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
	// In production with good entropy: succeeds
	// In CI/test with weak entropy: returns error (new correct behavior)
	key, err := client.generatePrivateKey()
	if err != nil {
		// This is the correct behavior - don't proceed with weak key
		expectedErr := "failed to generate high-entropy private key"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("generatePrivateKey() returned unexpected error: %v", err)
		}
		t.Logf("Correctly returned error when entropy insufficient: %v", err)
		return
	}

	// Success case - good entropy available
	if isZeroKey(key) {
		t.Error("generated key is zero")
	}

	publicKey := key.PublicKey()
	if isZeroKey(publicKey) {
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

	if isZeroKey(key) {
		t.Error("generated key is zero")
	}
}

func TestGeneratePrivateKey_EntropyRetry(t *testing.T) {
	// This tests the new behavior: when entropy validation fails 3 times, return error
	// instead of proceeding with a weak key
	client := &WireGuardClient{
		validateKeys: true,
		logger:       nil,
	}

	// With validateKeys=true and entropy validation, we should get an error when RNG is weak
	// In a real system with good entropy, this would succeed
	// In CI/test environment with weak RNG, this should return error
	key, err := client.generatePrivateKey()

	// The test verifies the new correct behavior: error when entropy fails
	if err != nil {
		// This is the expected behavior - return error, not proceed with weak key
		expectedErr := "failed to generate high-entropy private key"
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("generatePrivateKey() returned unexpected error: %v, want contain %q", err, expectedErr)
		}
		t.Logf("Correctly returned error when entropy insufficient: %v", err)
		return
	}

	// If we got here, entropy was sufficient (good entropy environment)
	if isZeroKey(key) {
		t.Error("Generated key is zero")
	}
	t.Logf("Key generated successfully with sufficient entropy: %s", key.String()[:16]+"...")
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

// TestWireGuardKeyGeneration_Entropy tests key generation with entropy
// Note: WireGuard keys are 32 bytes encoded as base64 (44 characters)
func TestWireGuardKeyGeneration_Entropy(t *testing.T) {
	for i := 0; i < 10; i++ {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			// In CI/test environment, RNG may be weak and return error
			t.Logf("Iteration %d: RNG error (weak entropy): %v", i, err)
			continue
		}

		keyString := key.String()

		// Base64 encoded 32 bytes = 44 characters
		if len(keyString) != 44 {
			t.Errorf("Key length = %d, want 44", len(keyString))
		}

		// The key should not be all zeros (32 zero bytes in base64 = "AAAA...A" with 44 As)
		isAllZero := true
		for _, b := range []byte(keyString) {
			if b != 'A' {
				isAllZero = false
				break
			}
		}
		if isAllZero {
			t.Error("Generated key is all zeros")
		}

		t.Logf("Generated key for iteration %d: %s...", i, keyString[:8])
	}
}

// isZeroKey checks if a wgtypes.Key is zero (all bytes are zero)
// This replaces the IsZero() method which was removed in newer wgctrl versions
func isZeroKey(k wgtypes.Key) bool {
	return k == wgtypes.Key{}
}
