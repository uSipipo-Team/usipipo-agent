package api

import (
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPLimiter wraps rate limiter with last access time for proper cleanup
type IPLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// FailureTracker tracks authentication failures for an IP
type FailureTracker struct {
	Count        int       // Number of failed attempts
	FirstFail    time.Time // When first failure occurred (for window calculation)
	LastFail     time.Time // When last failure occurred
	LockedUntil  time.Time // If locked, when lockout expires
	BackoffLevel int       // Current backoff level (0-5)
}

// HybridRateLimiter combines IP-based and key-based rate limiting
type HybridRateLimiter struct {
	// IP-based limiters with access tracking
	ipLimiters map[string]*IPLimiter

	// API key-based limiters
	keyLimiters map[string]*rate.Limiter

	// Auth failure tracking
	authFailures map[string]*FailureTracker

	// Configuration
	config *HybridRateLimiterConfig

	mu sync.RWMutex
}

// HybridRateLimiterConfig holds all rate limiting configuration
type HybridRateLimiterConfig struct {
	// General API limits
	Enabled           bool
	RequestsPerSecond float64
	BurstSize         int

	// Auth endpoint limits
	AuthRPS   float64
	AuthBurst int

	// Per-key limits
	KeyRPS   float64
	KeyBurst int

	// Auth failure protection
	LockoutThreshold int           // Failed attempts before lockout
	LockoutDuration  time.Duration // How long lockout lasts
	BackoffBase      time.Duration // Initial backoff delay
	BackoffMax       time.Duration // Maximum backoff delay
	FailureWindow    time.Duration // Window for counting failures (default: 5m)

	// Cleanup
	CleanupInterval time.Duration // How often to clean old entries
	EntryTTL        time.Duration // TTL for inactive entries
}

// RateLimitResponse contains rate limit information for headers
type RateLimitResponse struct {
	Remaining int       // Requests remaining in current window
	Reset     time.Time // When the rate limit resets
	Limit     int       // Total requests allowed in window
}

// NewHybridRateLimiter creates a new hybrid rate limiter
func NewHybridRateLimiter(config *HybridRateLimiterConfig) *HybridRateLimiter {
	rl := &HybridRateLimiter{
		ipLimiters:   make(map[string]*IPLimiter),
		keyLimiters:  make(map[string]*rate.Limiter),
		authFailures: make(map[string]*FailureTracker),
		config:       config,
	}

	// Set default failure window if not specified
	if rl.config.FailureWindow == 0 {
		rl.config.FailureWindow = 5 * time.Minute
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		rl.startCleanup()
	}

	return rl
}

// getBackoffDelay calculates delay based on failure count
// Sequence: 1s, 2s, 4s, 8s, 16s, 30s, 30s, ...
func (ft *FailureTracker) getBackoffDelay(base, max time.Duration) time.Duration {
	// Exponential backoff: 2^n seconds, capped at 30s
	// Level 0: 1s, Level 1: 2s, Level 2: 4s, Level 3: 8s, Level 4: 16s, Level 5+: 30s
	delays := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second, // Cap at 30s
	}

	if ft.BackoffLevel >= len(delays) {
		return max
	}

	delay := delays[ft.BackoffLevel]
	if delay > max {
		return max
	}
	return delay
}

// isLockedOut checks if the IP is currently locked out
func (ft *FailureTracker) isLockedOut() bool {
	if ft.LockedUntil.IsZero() {
		return false
	}
	return time.Now().Before(ft.LockedUntil)
}

// checkAuthFailureLimit checks if request should be blocked due to auth failures
// Holds read lock for the entire check to ensure consistency
func (rl *HybridRateLimiter) checkAuthFailureLimit(ip string) (bool, time.Duration, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	tracker, exists := rl.authFailures[ip]
	if !exists {
		return true, 0, nil // Allow
	}

	// Check if locked out
	if tracker.isLockedOut() {
		remaining := time.Until(tracker.LockedUntil)
		return false, remaining, fmt.Errorf("IP temporarily locked out")
	}

	// Check if in backoff period
	backoffDelay := tracker.getBackoffDelay(rl.config.BackoffBase, rl.config.BackoffMax)
	elapsed := time.Since(tracker.LastFail)
	if elapsed < backoffDelay {
		remaining := backoffDelay - elapsed
		return false, remaining, fmt.Errorf("in backoff period")
	}

	return true, 0, nil
}

// recordFailure records an authentication failure
func (rl *HybridRateLimiter) recordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	tracker, exists := rl.authFailures[ip]

	if !exists {
		tracker = &FailureTracker{
			Count:        0,
			FirstFail:    now,
			LastFail:     now,
			BackoffLevel: 0,
		}
		rl.authFailures[ip] = tracker
	}

	// Check if lockout expired and reset
	if !tracker.LockedUntil.IsZero() && tracker.LockedUntil.Before(now) {
		// Lockout expired, reset tracker
		tracker.Count = 0
		tracker.BackoffLevel = 0
		tracker.LockedUntil = time.Time{}
		tracker.FirstFail = now
	}

	// Check if still locked out (don't increment if so)
	if tracker.isLockedOut() {
		return
	}

	// Check if we need to reset the window
	if now.Sub(tracker.FirstFail) > rl.config.FailureWindow {
		tracker.Count = 0
		tracker.FirstFail = now
		tracker.BackoffLevel = 0
	}

	tracker.Count++
	tracker.LastFail = now
	tracker.BackoffLevel++

	// Check if lockout threshold reached
	if tracker.Count >= rl.config.LockoutThreshold {
		tracker.LockedUntil = now.Add(rl.config.LockoutDuration)
	}
}

// AllowIP checks if request is allowed by IP-based rate limit
func (rl *HybridRateLimiter) AllowIP(ip string, isAuthEndpoint bool) (*RateLimitResponse, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Select appropriate limiter based on endpoint type
	var rps float64
	var burst int
	if isAuthEndpoint {
		rps = rl.config.AuthRPS
		burst = rl.config.AuthBurst
	} else {
		rps = rl.config.RequestsPerSecond
		burst = rl.config.BurstSize
	}

	ipLimiter, exists := rl.ipLimiters[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(rps), burst)
		ipLimiter = &IPLimiter{
			limiter:    limiter,
			lastAccess: time.Now(),
		}
		rl.ipLimiters[ip] = ipLimiter
	} else {
		// Update last access time
		ipLimiter.lastAccess = time.Now()
	}

	limiter := ipLimiter.limiter

	// Consume token FIRST
	allowed := limiter.Allow()

	// Then calculate remaining tokens
	remaining := int(limiter.Tokens())
	if remaining < 0 {
		remaining = 0
	}
	now := time.Now()
	resetTime := now.Add(time.Duration(float64(time.Second) * float64(burst) / rps))

	resp := &RateLimitResponse{
		Remaining: remaining,
		Reset:     resetTime,
		Limit:     burst,
	}

	return resp, allowed
}

// AllowKey checks if request is allowed by API key-based rate limit
func (rl *HybridRateLimiter) AllowKey(apiKey string) (*RateLimitResponse, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.keyLimiters[apiKey]
	if !exists {
		limiter = rate.NewLimiter(
			rate.Limit(rl.config.KeyRPS),
			rl.config.KeyBurst,
		)
		rl.keyLimiters[apiKey] = limiter
	}

	// Consume token FIRST
	allowed := limiter.Allow()

	// Then calculate remaining tokens
	remaining := int(limiter.Tokens())
	if remaining < 0 {
		remaining = 0
	}
	resetTime := time.Now().Add(
		time.Duration(float64(time.Second) * float64(rl.config.KeyBurst) / rl.config.KeyRPS),
	)

	resp := &RateLimitResponse{
		Remaining: remaining,
		Reset:     resetTime,
		Limit:     rl.config.KeyBurst,
	}

	return resp, allowed
}

// cleanup removes old entries to prevent memory leaks
func (rl *HybridRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	ttl := rl.config.EntryTTL

	// Cleanup IP limiters with proper TTL tracking
	for ip, ipLimiter := range rl.ipLimiters {
		if now.Sub(ipLimiter.lastAccess) > ttl {
			delete(rl.ipLimiters, ip)
		}
	}

	// Cleanup auth failure trackers
	for ip, tracker := range rl.authFailures {
		// Remove if lockout expired and no recent failures
		if tracker.LockedUntil.IsZero() || tracker.LockedUntil.Before(now) {
			if now.Sub(tracker.LastFail) > ttl {
				delete(rl.authFailures, ip)
			}
		}
	}

	// Note: We don't cleanup key limiters aggressively as keys are fewer and more stable
}

// startCleanup starts the periodic cleanup goroutine with panic recovery
func (rl *HybridRateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// Log panic but don't crash the server
						log.Printf("Panic in rate limiter cleanup: %v", r)
					}
				}()
				rl.cleanup()
			}()
		}
	}()
}
