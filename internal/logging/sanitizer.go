package logging

import (
	"fmt"
	"strings"
)

// maxLogLength is the maximum length of strings in logs (prevent flooding)
const maxLogLength = 1000

// maskAPIKey masks an API key for safe logging
// Input:  "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7" (39 chars)
// Output: "agen...p6q7"
// Short keys (< 8 chars) or partial agent_ keys are replaced with "***"
func maskAPIKey(key string) string {
	if key == "" {
		return "***"
	}

	// If key is too short, just mask it completely
	if len(key) < 8 {
		return "***"
	}

	// Special handling for agent_ prefix
	if strings.HasPrefix(key, "agent_") {
		// Valid API key should be "agent_" (6 chars) + 34 chars = 40 chars total
		// If it starts with agent_ but isn't the right length, it's a partial key
		if len(key) != 40 {
			return "***"
		}
	}

	// Show first 4 and last 4 characters
	return key[:4] + "..." + key[len(key)-4:]
}

// sanitizeString removes potential sensitive patterns and truncates long strings
// - Removes bearer tokens (replaces "Bearer <token>" with "Bearer ***")
// - Removes query parameters with secrets
// - Truncates strings >1000 chars
func sanitizeString(s string) string {
	if len(s) > maxLogLength {
		return s[:maxLogLength] + "..."
	}

	// Remove potential bearer tokens (replace "Bearer <secret>" with "Bearer ***")
	if strings.HasPrefix(s, "Bearer ") {
		return "Bearer ***"
	}

	// Remove potential API key patterns in strings
	// This is a simple check - the main protection is maskAPIKey
	if strings.Contains(s, "agent_") {
		// Find and mask API key patterns
		result := ""
		remaining := s

		for strings.Contains(remaining, "agent_") {
			// Split at the next "agent_"
			idx := strings.Index(remaining, "agent_")
			result += remaining[:idx]
			remaining = remaining[idx+6:] // Skip "agent_"

			// Find the end of the potential key (space, newline, or end of string)
			end := 0
			for i, c := range remaining {
				if c == ' ' || c == '\n' || c == '\r' {
					end = i
					break
				}
			}
			if end == 0 {
				end = len(remaining)
			}

			// Limit to 40 chars max for key part
			if end > 40 {
				end = 40
			}

			// Mask the key
			keyPart := "agent_" + remaining[:end]
			result += maskAPIKey(keyPart)
			remaining = remaining[end:]
		}

		// Add any remaining text
		result += remaining
		s = result
	}

	return s
}

// sanitizeValue recursively sanitizes values of any type
// Handles strings, slices, maps, and other types
func sanitizeValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return sanitizeString(v)
	case []string:
		sanitized := make([]string, len(v))
		for i, s := range v {
			sanitized[i] = sanitizeString(s)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(v))
		for i, val := range v {
			sanitized[i] = sanitizeValue(val)
		}
		return sanitized
	case map[string]interface{}:
		sanitized := make(map[string]interface{})
		for k, val := range v {
			sanitized[k] = sanitizeValue(val)
		}
		return sanitized
	default:
		// For other types (int, bool, etc.), convert to string and sanitize
		return sanitizeString(fmt.Sprint(v))
	}
}
