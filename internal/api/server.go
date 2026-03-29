package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	httpServer *http.Server
	router     *gin.Engine
}

// NewServer creates a new HTTP server with Gin
func NewServer(apiKey, outlineAPIURL string) *Server {
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

	// Public routes
	router.GET("/health", HealthHandler)

	// Protected routes
	protected := router.Group("/")
	protected.Use(APIKeyMiddleware(apiKey))
	{
		protected.GET("/status", StatusHandler)
		protected.GET("/metrics", MetricsHandler)
		
		// Outline VPN management routes
		protected.POST("/outline/keys", CreateOutlineKeyHandler)
		protected.DELETE("/outline/keys/:id", DeleteOutlineKeyHandler)
		
		// WireGuard VPN management routes
		protected.POST("/wireguard/peers", CreateWireGuardPeerHandler)
		protected.DELETE("/wireguard/peers/:name", DeleteWireGuardPeerHandler)
		protected.GET("/wireguard/peers/:name/usage", GetWireGuardPeerUsageHandler)
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
