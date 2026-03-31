package api

import (
	"log"
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
			log.Printf("WARN: Missing API key from IP %s", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key",
			})
			return
		}

		// Validate API key format (reject malformed keys early)
		if !validation.IsValidAPIKeyFormat(apiKey) {
			log.Printf("WARN: Invalid API key format from IP %s", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key format",
			})
			return
		}

		// Validate API key format of stored key (defensive check)
		if !validation.IsValidAPIKeyFormat(validKey) {
			// Backward compatibility: old keys without agent_ prefix
			// Still use constant-time compare to prevent timing attacks
			if !validation.SecureCompareAPIKeys(apiKey, validKey) {
				log.Printf("WARN: Failed API key authentication from IP %s", c.ClientIP())
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
				})
				return
			}
		} else {
			// Use constant-time comparison for valid format keys
			if !validation.SecureCompareAPIKeys(apiKey, validKey) {
				log.Printf("WARN: Failed API key authentication from IP %s", c.ClientIP())
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid API key",
				})
				return
			}
		}

		c.Next()
	}
}
