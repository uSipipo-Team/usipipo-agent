package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// ValidateKeyEntropy validates that a WireGuard private key has sufficient entropy
// WireGuard private keys are 32 bytes (64 hex characters)
// This is a basic validation - accepts any valid WireGuard key format
func ValidateKeyEntropy(hexKey string) bool {
	if len(hexKey) == 0 {
		return false
	}

	// Decode hex key to bytes
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil || len(keyBytes) == 0 {
		return false
	}

	// Must be exactly 32 bytes for WireGuard private key
	if len(keyBytes) != 32 {
		return false
	}

	// Basic check: reject if all bytes are the same (weak pattern)
	firstByte := keyBytes[0]
	for _, b := range keyBytes[1:] {
		if b != firstByte {
			return true
		}
	}

	return false
}

// GenerateValidatedPrivateKey generates a WireGuard private key with entropy validation
// Retries up to maxAttempts if entropy validation fails
// Returns the key as hex string
func GenerateValidatedPrivateKey(maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return "", fmt.Errorf("failed to generate private key: %w", err)
		}

		keyHex := privateKey.String()

		// Validate entropy
		if ValidateKeyEntropy(keyHex) {
			return keyHex, nil
		}

		lastErr = fmt.Errorf("key failed entropy validation (attempt %d/%d)", attempt+1, maxAttempts)
	}

	return "", fmt.Errorf("failed to generate high-entropy key after %d attempts: %w", maxAttempts, lastErr)
}

// CheckRNGSecurity checks if the system's crypto/rand RNG is available and working
// Returns error if RNG is not available or not secure
func CheckRNGSecurity() error {
	// Test: generate 32 bytes and verify we can read them
	testBytes := make([]byte, 32)
	n, err := rand.Read(testBytes)
	if err != nil {
		return fmt.Errorf("crypto/rand RNG unavailable: %w", err)
	}
	if n != 32 {
		return fmt.Errorf("incomplete read from RNG: got %d bytes, want 32", n)
	}

	// Basic sanity check: bytes should not all be zero
	allZero := true
	for _, b := range testBytes {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Errorf("RNG returned all zeros - likely broken")
	}

	return nil
}
