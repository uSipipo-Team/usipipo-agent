package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/uSipipo-Team/usipipo-agent/internal/api"
	"github.com/uSipipo-Team/usipipo-agent/internal/config"
	"github.com/uSipipo-Team/usipipo-agent/internal/logging"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
	"github.com/uSipipo-Team/usipipo-agent/internal/reporter"
	"github.com/uSipipo-Team/usipipo-agent/internal/utils/security"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

// Version is set at build time via ldflags
var Version = "v0.12.0"

// Global references for cleanup
var reconcileLoop *vpn.ReconciliationLoop

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

	// Log WireGuard key validation setting
	if cfg.WGValidateKeys {
		log.Println("✓ WireGuard key entropy validation: ENABLED")
	} else {
		log.Println("⚠️  WARNING: WireGuard key entropy validation: DISABLED (WG_VALIDATE_KEYS=false)")
		log.Println("   This reduces cryptographic security. Only disable for testing/dev environments.")
	}

	// Check configuration file permissions
	if err := security.CheckEnvFilePermissions(".env", cfg.ConfigStrictPerms); err != nil {
		if cfg.ConfigStrictPerms {
			log.Fatalf("SECURITY: Insecure configuration detected: %v", err)
		}
		log.Printf("SECURITY WARNING: %v", err)
		log.Println("   Consider setting CONFIG_STRICT_PERMS=true and fixing file permissions with: chmod 600 .env")
	} else {
		log.Println("✓ Configuration file permissions verified")
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
	log.Printf("WireGuard Interface: %s (%s:%d)", cfg.WireGuardInterface, cfg.WireGuardServerIP, cfg.WireGuardServerPort)
	log.Printf("Rate Limiting: enabled=%v, rps=%.1f, burst=%d",
		cfg.RateLimitEnabled, cfg.RateLimitRPS, cfg.RateLimitBurst)

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(cfg.ServerID)
	api.SetMetricsCollector(metricsCollector)

	// Initialize WireGuard client using wgctrl
	wireguardClient, err := vpn.NewWireGuardClient(
		cfg.WireGuardInterface,
		"/etc/wireguard/wg0.conf",
		cfg.WireGuardServerIP,   // Public IP for client endpoint
		cfg.WireGuardServerPort, // Port from WG_SERVER_PORT
		"1.1.1.1",               // Cloudflare DNS
		cfg.WGValidateKeys,
	)
	if err != nil {
		log.Printf("Warning: WireGuard client initialization failed: %v", err)
		log.Printf("WireGuard features will be limited")
	} else {
		api.SetWireGuardClient(wireguardClient)
		log.Printf("WireGuard client initialized successfully")

		// Initialize IP allocation client if DB-first enabled
		if cfg.EnableDBIPAllocation && cfg.BackendAPIURL != "" && cfg.BackendAPIKey != "" {
			ipAllocClient := vpn.NewIPAllocationClient(
				cfg.BackendAPIURL,
				cfg.BackendAPIKey,
				cfg.ServerID,
				log.New(log.Writer(), "[IPAlloc] ", log.Flags()),
			)
			wireguardClient.SetIPAllocationClient(ipAllocClient, log.New(log.Writer(), "[WG] ", log.Flags()))
			wireguardClient.SetLockFilePath(cfg.WGLockPath)
			log.Printf("IP allocation client initialized (DB-first enabled, lock=%s)", cfg.WGLockPath)

			// Start reconciliation loop
			reconcileLoop = vpn.NewReconciliationLoop(wireguardClient, cfg.ReconcileInterval)
			reconcileLoop.Start()
			log.Printf("Reconciliation loop started (interval=%v)", cfg.ReconcileInterval)
		} else if cfg.EnableDBIPAllocation {
			log.Printf("Warning: ENABLE_DB_IP_ALLOCATION=true but BACKEND_API_URL or BACKEND_API_KEY not set")
		}
	}

	// Initialize WireGuard metrics collector
	wireguardMetricsCollector := vpn.NewWireGuardMetricsCollector(cfg.WireGuardInterface)
	metricsCollector.SetWireGuardCollector(wireguardMetricsCollector)

	// Create HTTP server with rate limiting
	rateConfig := api.RateLimiterConfig{
		RequestsPerSecond: cfg.RateLimitRPS,
		BurstSize:         cfg.RateLimitBurst,
		Enabled:           cfg.RateLimitEnabled,
	}
	server := api.NewServer(cfg.APIKey, rateConfig)

	// Initialize security logger
	logLevel := logging.ParseLogLevel(os.Getenv("LOG_LEVEL"))
	securityLogger := logging.NewSecurityLogger(cfg.ServerID, Version, logLevel)
	api.SetSecurityLogger(securityLogger)

	// Log startup event
	// TODO: Implement actual config hash calculation
	securityLogger.LogStartup("config-hash-placeholder")

	// Initialize and start metrics reporter
	metricsReporter := reporter.NewReporter(
		cfg.BackendURL,
		cfg.ServerID,
		cfg.APIKey,
		metricsCollector,
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

	// Stop reconciliation loop
	if reconcileLoop != nil {
		reconcileLoop.Stop()
		log.Println("Reconciliation loop stopped")
	}

	// Log shutdown event
	securityLogger.LogShutdown()

	// Stop reporter
	metricsReporter.Stop()

	// Stop HTTP server
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	log.Println("VPN Agent stopped")
}
