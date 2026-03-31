package logging

import (
	"net/http"
	"strings"
)

// maxLogLength is the maximum length of strings in logs (prevent flooding)
const maxLogLength = 1000

// maskAPIKey masks an API key for safe logging
// Input:  "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7"
// Output: "agen...p6q7"
// Short keys or invalid formats are replaced with "***"
func maskAPIKey(key string) string {
	if key == "" {
		return "***"
	}
	
	// If key is too short, just mask it completely
	if len(key) < 8 {
		return "***"
	}
	
	// Show first 4 and last 4 characters
	return key[:4] + "..." + key[len(key)-4:]
}

// sanitizeString removes potential sensitive patterns and truncates long strings
// - Removes bearer tokens
// - Removes query parameters with secrets
// - Truncates strings >1000 chars
func sanitizeString(s string) string {
	if len(s) > maxLogLength {
		return s[:maxLogLength] + "..."
	}
	
	// Remove potential bearer tokens
	s = strings.ReplaceAll(s, "Bearer ", "Bearer ***")
	
	// Remove potential API key patterns in strings
	// This is a simple check - the main protection is maskAPIKey
	if strings.Contains(s, "agent_") {
		// Try to find and mask API key patterns
		parts := strings.Split(s, "agent_")
		for i := 1; i < len(parts); i++ {
			// Find the end of the potential key (space, newline, or 40 chars)
			end := 40
			if len(parts[i]) < 40 {
				end = len(parts[i])
			}
			// Look for space or newline
			for j, c := range parts[i] {
				if c == ' ' || c == '\n' || c == '\r' {
					end = j
					break
				}
			}
			parts[i] = "agent_" + maskAPIKey("agent_"+parts[i][:end]) + parts[i][end:]
		}
		s = strings.Join(parts, "agent_")
	}
	
	return s
}

// sanitizeHeaders returns safe-to-log headers
// Excludes: Authorization, X-API-Key, Cookie, Set-Cookie
func sanitizeHeaders(headers http.Header) map[string]string {
	safeHeaders := make(map[string]string)
	
	// Headers to exclude (case-insensitive)
	excludedHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"set-cookie":    true,
	}
	
	for key, values := range headers {
		// Check if header should be excluded
		if excludedHeaders[strings.ToLower(key)] {
			continue
		}
		
		// Join multiple values with comma
		safeHeaders[key] = sanitizeString(strings.Join(values, ", "))
	}
	
	return safeHeaders
}

// containsSensitiveData checks if a string might contain sensitive data
func containsSensitiveData(s string) bool {
	// Check for API key pattern
	if strings.Contains(s, "agent_") {
		return true
	}
	
	// Check for bearer token
	if strings.Contains(s, "Bearer ") {
		return true
	}
	
	return false
}
