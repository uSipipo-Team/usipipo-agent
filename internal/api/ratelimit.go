package api

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// FailureTracker tracks authentication failures for an IP
type FailureTracker struct {
	Count        int           // Number of failed attempts
	FirstFail    time.Time     // When first failure occurred (for window calculation)
	LastFail     time.Time     // When last failure occurred
	LockedUntil  time.Time     // If locked, when lockout expires
	BackoffLevel int           // Current backoff level (0-5)
}

// HybridRateLimiter combines IP-based and key-based rate limiting
type HybridRateLimiter struct {
	// IP-based limiters
	ipLimiters map[string]*rate.Limiter

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
		ipLimiters:     make(map[string]*rate.Limiter),
		keyLimiters:    make(map[string]*rate.Limiter),
		authFailures:   make(map[string]*FailureTracker),
		config:         config,
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
	// Backoff levels: 0=1s, 1=2s, 2=4s, 3=8s, 4=16s, 5+=30s
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
func (rl *HybridRateLimiter) checkAuthFailureLimit(ip string) (bool, time.Duration, error) {
	rl.mu.RLock()
	tracker, exists := rl.authFailures[ip]
	rl.mu.RUnlock()

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

	// Check if lockout expired
	if tracker.isLockedOut() {
		// Still locked, don't increment
		return
	}

	// Check if we need to reset the window (5 minutes)
	if now.Sub(tracker.FirstFail) > 5*time.Minute {
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

	limiter, exists := rl.ipLimiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rps), burst)
		rl.ipLimiters[ip] = limiter
	}

	// Calculate response info
	now := time.Now()
	remaining := limiter.Burst() - limiter.Tokens()
	if remaining < 0 {
		remaining = 0
	}
	resetTime := now.Add(time.Duration(float64(time.Second) * float64(burst) / rps))

	resp := &RateLimitResponse{
		Remaining: remaining,
		Reset:     resetTime,
		Limit:     burst,
	}

	return resp, limiter.Allow()
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

	remaining := limiter.Burst() - limiter.Tokens()
	if remaining < 0 {
		remaining = 0
	}
	resetTime := time.Now().Add(
		time.Duration(float64(time.Second)*float64(rl.config.KeyBurst)/rl.config.KeyRPS),
	)

	resp := &RateLimitResponse{
		Remaining: remaining,
		Reset:     resetTime,
		Limit:     rl.config.KeyBurst,
	}

	return resp, limiter.Allow()
}

// cleanup removes old entries to prevent memory leaks
func (rl *HybridRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	ttl := rl.config.EntryTTL

	// Cleanup IP limiters (aggressive - IPs change frequently)
	for ip, limiter := range rl.ipLimiters {
		// Check if limiter is fully replenished (not being used)
		if limiter.Tokens() >= float64(limiter.Burst()) {
			// Limiter is full, check how long since last use
			// For simplicity, we'll use a heuristic: if it's been full for TTL, remove it
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

// startCleanup starts the periodic cleanup goroutine
func (rl *HybridRateLimiter) startCleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	go func() {
		for range ticker.C {
			rl.cleanup()
		}
	}()
}
