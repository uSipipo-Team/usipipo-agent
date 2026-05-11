package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiterConfig holds rate limiting configuration
type RateLimiterConfig struct {
	RequestsPerSecond float64
	BurstSize         int
	Enabled           bool
}

// Visitor holds rate limiter for a specific IP
type Visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages rate limiters for all visitors
type RateLimiter struct {
	visitors map[string]*Visitor
	config   RateLimiterConfig
	mu       sync.RWMutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*Visitor),
		config:   config,
	}
}

// Allow checks if a request from this IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	visitor, exists := rl.visitors[ip]
	if !exists {
		visitor = &Visitor{
			limiter: rate.NewLimiter(rate.Limit(rl.config.RequestsPerSecond), rl.config.BurstSize),
		}
		rl.visitors[ip] = visitor
	}

	visitor.lastSeen = time.Now()
	return visitor.limiter.Allow()
}

// Cleanup removes old visitors (called periodically)
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for ip, visitor := range rl.visitors {
		if time.Since(visitor.lastSeen) > 3*time.Minute {
			delete(rl.visitors, ip)
		}
	}
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.config.Enabled {
			c.Next()
			return
		}

		ip := c.ClientIP()
		if !rl.Allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// Server represents the HTTP server
type Server struct {
	httpServer *http.Server
	router     *gin.Engine
}

// NewServer creates a new HTTP server with Gin and hybrid rate limiting
func NewServer(apiKey string, rateConfig RateLimiterConfig) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"}, // Restricted by API Key
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Hybrid rate limiting configuration
	hybridConfig := &HybridRateLimiterConfig{
		Enabled:           rateConfig.Enabled,
		RequestsPerSecond: rateConfig.RequestsPerSecond,
		BurstSize:         rateConfig.BurstSize,
		AuthRPS:           3.0, // Stricter for auth endpoints
		AuthBurst:         5,
		KeyRPS:            100.0, // Per-API-key limit
		KeyBurst:          200,
		LockoutThreshold:  10, // 10 failed attempts
		LockoutDuration:   5 * time.Minute,
		BackoffBase:       1 * time.Second,
		BackoffMax:        30 * time.Second,
		CleanupInterval:   1 * time.Minute,
		EntryTTL:          3 * time.Minute,
	}

	// Create hybrid rate limiter
	hybridRateLimiter := NewHybridRateLimiter(hybridConfig)

	// Apply auth failure tracking middleware (before rate limiting)
	router.Use(AuthFailureMiddleware(hybridRateLimiter))

	// Apply hybrid rate limiting to all routes
	router.Use(HybridRateLimitMiddleware(hybridRateLimiter))

	// Public routes
	router.GET("/health", HealthHandler)

	// Protected routes
	protected := router.Group("/")
	protected.Use(APIKeyMiddlewareWithRateLimit(apiKey, hybridRateLimiter))
	{
		protected.GET("/status", StatusHandler)
		protected.GET("/metrics", MetricsHandler)

		// WireGuard VPN management routes
		protected.POST("/wireguard/peers", CreateWireGuardPeerHandler)
		protected.DELETE("/wireguard/peers/:name", DeleteWireGuardPeerHandler)
		protected.GET("/wireguard/peers/:name/usage", GetWireGuardPeerUsageHandler)
		protected.POST("/wireguard/peers/:name/regenerate", RegenerateWireGuardPeerHandler)
	}

	return &Server{
		router: router,
	}
}

// Start starts the HTTP server
func (s *Server) Start(port string) error {
	s.httpServer = &http.Server{
		Addr:         ":" + port,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("HTTP server starting on port %s", port)
	return s.httpServer.ListenAndServe()
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}
