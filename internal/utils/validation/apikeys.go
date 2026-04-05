package validation

import (
	"crypto/subtle"
	"regexp"
)

// API key format: agent_[32 alphanumeric characters]
// Example: agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
var apiKeyPattern = regexp.MustCompile(`^agent_[a-zA-Z0-9]{32}$`)

// APIKeyFormat returns the expected API key format for documentation
func APIKeyFormat() string {
	return "agent_[32 alphanumeric characters]"
}

// IsValidAPIKeyFormat validates that the API key matches the required format
// Returns true if valid, false otherwise
func IsValidAPIKeyFormat(key string) bool {
	if key == "" {
		return false
	}
	return apiKeyPattern.MatchString(key)
}

// SecureCompareAPIKeys compares two API keys using constant-time comparison
// to prevent timing attacks
// Returns true if keys match, false otherwise
func SecureCompareAPIKeys(input, stored string) bool {
	if input == "" || stored == "" {
		return false
	}

	// Length check before constant-time compare (safe, doesn't leak timing info)
	if len(input) != len(stored) {
		return false
	}

	// Constant-time byte comparison
	return subtle.ConstantTimeCompare([]byte(input), []byte(stored)) == 1
}
