package validation

import (
	"errors"
	"regexp"
	"strings"
)

// validNamePattern allows alphanumeric characters, hyphens, and underscores
var validNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidatePeerName validates a peer or key name for security and correctness.
// Rules:
//   - Must be 1-64 characters after trimming whitespace
//   - Allowed characters: letters (a-z, A-Z), digits (0-9), hyphen (-), underscore (_)
//   - No whitespace
//   - No path traversal sequences (.., /, \)
//   - No control characters
func ValidatePeerName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}

	trimmed := strings.TrimSpace(name)
	if trimmed != name {
		// Reject names with leading/trailing whitespace
		return errors.New("name cannot contain leading or trailing whitespace")
	}

	if len(trimmed) < 1 || len(trimmed) > 64 {
		return errors.New("name must be between 1 and 64 characters")
	}

	if !validNamePattern.MatchString(trimmed) {
		return errors.New("name contains invalid characters; only alphanumeric, hyphen, and underscore allowed")
	}

	// Prevent path traversal attempts
	if strings.Contains(trimmed, "..") {
		return errors.New("name contains path traversal sequence '..' which is not allowed")
	}
	if strings.ContainsAny(trimmed, `/\\`) {
		return errors.New("name contains path separator which is not allowed")
	}

	// Reject control characters (ASCII < 32)
	for _, r := range trimmed {
		if r < 32 {
			return errors.New("name contains invalid control character")
		}
	}

	return nil
}

// SanitizePeerName trims whitespace and returns the cleaned name.
// It does not enforce length or character rules; use ValidatePeerName for full validation.
func SanitizePeerName(name string) string {
	return strings.TrimSpace(name)
}
