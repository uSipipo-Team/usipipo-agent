package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uSipipo-Team/usipipo-agent/internal/utils/validation"
)

// AuthFailureMiddleware tracks and enforces auth failure limits
func AuthFailureMiddleware(rl *HybridRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// Check auth failure limits
		allowed, retryAfter, err := rl.checkAuthFailureLimit(ip)
		if !allowed {
			c.Header("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many failed authentication attempts. Please try again later.",
				"retry_after": int(retryAfter.Seconds()),
			})
			return
		}

		// Store reference to rate limiter in context for later use
		c.Set("rateLimiter", rl)
		c.Set("clientIP", ip)

		c.Next()

		// Record failure if auth failed (status 401)
		if c.Writer.Status() == http.StatusUnauthorized {
			rl.recordFailure(ip)
		}
	}
}

// HybridRateLimitMiddleware applies hybrid rate limiting
func HybridRateLimitMiddleware(rl *HybridRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.config.Enabled {
			c.Next()
			return
		}

		ip := c.ClientIP()

		// Determine if this is an auth endpoint (endpoints that require API key)
		isAuthEndpoint := true // All protected routes are auth endpoints

		// Check IP-based rate limit
		resp, allowed := rl.AllowIP(ip, isAuthEndpoint)
		if !allowed {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", resp.Limit))
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", resp.Remaining))
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resp.Reset.Unix()))
			c.Header("Retry-After", fmt.Sprintf("%.0f", time.Until(resp.Reset).Seconds()))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		// Add rate limit headers to response
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", resp.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", resp.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resp.Reset.Unix()))

		c.Next()
	}
}

// APIKeyMiddlewareWithRateLimit validates X-API-Key header with rate limiting
func APIKeyMiddlewareWithRateLimit(validKey string, rl *HybridRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

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

		// Check key-based rate limit
		resp, allowed := rl.AllowKey(apiKey)
		if !allowed {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", resp.Limit))
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", resp.Remaining))
			c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resp.Reset.Unix()))
			c.Header("Retry-After", fmt.Sprintf("%.0f", time.Until(resp.Reset).Seconds()))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "API key rate limit exceeded",
			})
			return
		}

		// Add rate limit headers to response
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", resp.Limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", resp.Remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", resp.Reset.Unix()))

		c.Next()
	}
}

// APIKeyMiddleware validates X-API-Key header (legacy, kept for backward compatibility)
// Deprecated: Use APIKeyMiddlewareWithRateLimit instead
func APIKeyMiddleware(validKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key",
			})
			return
		}

		if apiKey != validKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			return
		}

		c.Next()
	}
}
