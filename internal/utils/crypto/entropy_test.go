package crypto

import (
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// TestValidateKeyEntropy_RealKey tests that a real WireGuard key passes entropy validation
func TestValidateKeyEntropy_RealKey(t *testing.T) {
	// Generate a real WireGuard private key
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	keyHex := privateKey.String()

	// Should pass entropy validation
	if !ValidateKeyEntropy(keyHex) {
		t.Error("Real WireGuard key failed entropy validation")
	}
}

// TestValidateKeyEntropy_WeakKeys tests that obviously weak keys fail
func TestValidateKeyEntropy_WeakKeys(t *testing.T) {
	tests := []struct {
		name     string
		hexKey   string
		wantPass bool
	}{
		{
			name:     "all zeros",
			hexKey:   "0000000000000000000000000000000000000000000000000000000000000000",
			wantPass: false,
		},
		{
			name:     "all same byte repeated",
			hexKey:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantPass: false,
		},
		{
			name:     "sequential pattern",
			hexKey:   "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
			wantPass: true, // Has 32 unique bytes, passes even though sequential
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateKeyEntropy(tt.hexKey)
			if got != tt.wantPass {
				if got {
					t.Errorf("ValidateKeyEntropy(%q) returned true, want false", tt.hexKey)
				} else {
					t.Errorf("ValidateKeyEntropy(%q) returned false, want true", tt.hexKey)
				}
			}
		})
	}
}

// TestCheckRNGSecurity verifies RNG is working
func TestCheckRNGSecurity(t *testing.T) {
	err := CheckRNGSecurity()
	if err != nil {
		t.Errorf("CheckRNGSecurity() returned error: %v", err)
	}
}

// TestGenerateValidatedPrivateKey tests the validated key generation
func TestGenerateValidatedPrivateKey(t *testing.T) {
	keyHex, err := GenerateValidatedPrivateKey(3)
	if err != nil {
		t.Fatalf("GenerateValidatedPrivateKey failed: %v", err)
	}

	// Should be 64 hex characters (32 bytes)
	if len(keyHex) != 64 {
		t.Errorf("Generated key length = %d, want 64", len(keyHex))
	}

	// Should pass entropy validation
	if !ValidateKeyEntropy(keyHex) {
		t.Error("Generated key failed entropy validation")
	}
}

