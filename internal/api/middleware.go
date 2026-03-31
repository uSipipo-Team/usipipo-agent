package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uSipipo-Team/usipipo-agent/internal/utils/validation"
)

// APIKeyMiddleware validates X-API-Key header with secure comparison
func APIKeyMiddleware(validKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		// Check for missing API key
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key",
			})
			return
		}

		// Validate API key format (reject malformed keys early)
		if !validation.IsValidAPIKeyFormat(apiKey) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key format",
			})
			return
		}

		// Validate API key format of stored key (defensive check)
		if !validation.IsValidAPIKeyFormat(validKey) {
			// Log warning but allow request to proceed if format matches
			// This handles backward compatibility during migration
			if apiKey != validKey {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
				})
				return
			}
		} else {
			// Use constant-time comparison for valid format keys
			if !validation.SecureCompareAPIKeys(apiKey, validKey) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
				})
				return
			}
		}

		c.Next()
	}
}
