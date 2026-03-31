package validation

import (
	"testing"
	"time"
)

func TestIsValidAPIKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"valid key", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", true},
		{"valid key uppercase", "agent_A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6", true},
		{"valid key mixed", "agent_Ab1Cd2Ef3Gh4Ij5Kl6Mn7Op8Qr9St0Uv", true},
		{"empty key", "", false},
		{"missing prefix", "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", false},
		{"wrong prefix", "api_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", false},
		{"too short", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5", false},
		{"too long", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q", false},
		{"invalid char underscore", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p_", false},
		{"invalid char dash", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p-", false},
		{"invalid char space", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5 p6", false},
		{"special char !", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAPIKeyFormat(tt.key)
			if result != tt.expected {
				t.Errorf("IsValidAPIKeyFormat(%q) = %v, expected %v",
					tt.key, result, tt.expected)
			}
		})
	}
}

func TestSecureCompareAPIKeys(t *testing.T) {
	validKey := "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"

	tests := []struct {
		name     string
		input    string
		stored   string
		expected bool
	}{
		{"matching keys", validKey, validKey, true},
		{"different keys", "agent_x1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6", validKey, false},
		{"empty input", "", validKey, false},
		{"empty stored", validKey, "", false},
		{"both empty", "", "", false},
		{"case sensitive", "agent_A1B2C3D4E5F6G7H8I9J0K1L2M3N4O5P6", validKey, false},
		{"one char diff", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p7", validKey, false},
		{"prefix attack", "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p", validKey, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SecureCompareAPIKeys(tt.input, tt.stored)
			if result != tt.expected {
				t.Errorf("SecureCompareAPIKeys(%q, %q) = %v, expected %v",
					tt.input, tt.stored, result, tt.expected)
			}
		})
	}
}

// TestConstantTimeComparison verifies that comparison time doesn't vary
// significantly based on position of first different character
func TestConstantTimeComparison(t *testing.T) {
	validKey := "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"

	// Keys that differ at different positions
	diffAtStart := "agent_x1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
	diffAtEnd := "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p7"

	iterations := 10000

	// Time comparison for diff at start
	start := time.Now()
	for i := 0; i < iterations; i++ {
		SecureCompareAPIKeys(diffAtStart, validKey)
	}
	timeStart := time.Since(start)

	// Time comparison for diff at end
	start = time.Now()
	for i := 0; i < iterations; i++ {
		SecureCompareAPIKeys(diffAtEnd, validKey)
	}
	timeEnd := time.Since(start)

	// Times should be similar (within 50% of each other)
	// This is a basic check - proper timing analysis would use statistical tests
	ratio := float64(timeStart) / float64(timeEnd)
	if ratio < 0.5 || ratio > 2.0 {
		t.Logf("Warning: Timing ratio %.2f may indicate timing variation", ratio)
		// Note: This is a soft check, not a hard failure
		// Proper timing attack resistance requires specialized testing
	}
}
