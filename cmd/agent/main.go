package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/uSipipo-Team/usipipo-agent/internal/api"
	"github.com/uSipipo-Team/usipipo-agent/internal/config"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
	"github.com/uSipipo-Team/usipipo-agent/internal/reporter"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

func main() {
	cfg := config.Load()

	// Validate required configuration
	if cfg.APIKey == "" {
		log.Fatal("AGENT_API_KEY is required")
	}

	// Validate API key format at startup (fail fast)
	if err := cfg.ValidateAPIKey(); err != nil {
		log.Fatalf("Invalid API key configuration: %v", err)
	}

	if cfg.BackendURL == "" {
		log.Fatal("BACKEND_URL is required")
	}
	if cfg.ServerID == "" {
		log.Fatal("SERVER_ID is required")
	}

	log.Printf("Starting VPN Agent on port %s", cfg.Port)
	log.Printf("Server ID: %s", cfg.ServerID)
	log.Printf("Backend URL: %s", cfg.BackendURL)
	log.Printf("Outline API URL: %s", cfg.OutlineAPIURL)
	log.Printf("WireGuard Interface: %s", cfg.WireGuardInterface)
	log.Printf("Rate Limiting: enabled=%v, rps=%.1f, burst=%d", 
		cfg.RateLimitEnabled, cfg.RateLimitRPS, cfg.RateLimitBurst)

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(cfg.ServerID)
	api.SetMetricsCollector(metricsCollector)

	// Initialize Outline client
	outlineClient := vpn.NewOutlineClient(cfg.OutlineAPIURL, !cfg.OutlineVerifySSL)
	api.SetOutlineClient(outlineClient)

	// Initialize WireGuard client using wgctrl
	wireguardClient, err := vpn.NewWireGuardClient(
		cfg.WireGuardInterface,
		"/etc/wireguard/wg0.conf",
		cfg.ServerID, // Will be replaced with actual server IP
		51820,        // Default WireGuard port
		"1.1.1.1",    // Cloudflare DNS
	)
	if err != nil {
		log.Printf("Warning: WireGuard client initialization failed: %v", err)
		log.Printf("WireGuard features will be limited")
	} else {
		api.SetWireGuardClient(wireguardClient)
		log.Printf("WireGuard client initialized successfully")
	}

	// Create HTTP server with rate limiting
	rateConfig := api.RateLimiterConfig{
		RequestsPerSecond: cfg.RateLimitRPS,
		BurstSize:         cfg.RateLimitBurst,
		Enabled:           cfg.RateLimitEnabled,
	}
	server := api.NewServer(cfg.APIKey, cfg.OutlineAPIURL, rateConfig)

	// Initialize and start metrics reporter
	metricsReporter := reporter.NewReporter(
		cfg.BackendURL,
		cfg.ServerID,
		cfg.APIKey,
		metricsCollector,
		cfg.OutlineVerifySSL,
		cfg.HTTPClientTimeout,
	)
	go metricsReporter.Start()

	// Start HTTP server in goroutine
	go func() {
		if err := server.Start(cfg.Port); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Println("VPN Agent started successfully")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Stop reporter
	metricsReporter.Stop()

	// Stop HTTP server
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	log.Println("VPN Agent stopped")
}
