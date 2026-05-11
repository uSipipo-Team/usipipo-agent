package reporter

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
	"github.com/uSipipo-Team/usipipo-agent/internal/registrar"
)

// Reporter pushes metrics to the backend
type Reporter struct {
	backendURL string
	serverID   string
	apiKey     string
	client     *resty.Client
	collector  *metrics.Collector
	interval   time.Duration
	stopChan   chan struct{}
}

// NewReporter creates a new metrics reporter with secure TLS configuration
func NewReporter(backendURL, serverID, apiKey string, collector *metrics.Collector, timeout time.Duration) *Reporter {
	client := resty.New()

	// Configure TLS with secure defaults
	client.SetTLSClientConfig(&tls.Config{
		MinVersion: tls.VersionTLS12, // Enforce TLS 1.2 minimum
	})
	client.SetTimeout(timeout)
	// Note: resty v2.11.0 doesn't have SetConnectTimeout, using SetTimeout instead

	// Retry logic for transient failures
	client.SetRetryCount(3)
	client.SetRetryWaitTime(2 * time.Second)
	client.SetRetryMaxWaitTime(10 * time.Second)

	return &Reporter{
		backendURL: backendURL,
		serverID:   serverID,
		apiKey:     apiKey,
		client:     client,
		collector:  collector,
		interval:   1 * time.Minute,
		stopChan:   make(chan struct{}),
	}
}

// Start starts the metrics reporter loop
func (r *Reporter) Start() {
	log.Printf("Starting metrics reporter (interval: %v)", r.interval)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Send initial metrics immediately
	go r.sendMetrics()

	for {
		select {
		case <-ticker.C:
			go r.sendMetrics()
		case <-r.stopChan:
			log.Println("Stopping metrics reporter")
			return
		}
	}
}

// Stop stops the metrics reporter
func (r *Reporter) Stop() {
	close(r.stopChan)
}

// sendMetrics collects and sends metrics to backend
func (r *Reporter) sendMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure we have a valid server_id
	if r.serverID == "" || !registrar.IsValidUUID(r.serverID) {
		log.Println("Server ID not set or invalid, attempting registration...")

		reg := registrar.NewRegistrarFromValues(r.backendURL, r.apiKey, r.serverID)
		serverID, err := reg.RegisterOrGetServerID()
		if err != nil {
			log.Printf("Failed to register: %v", err)
			return
		}

		r.serverID = serverID
		log.Printf("Registered with server_id: %s", serverID)
	}

	// Collect metrics
	m, err := r.collector.GetMetrics(ctx)
	if err != nil {
		log.Printf("Failed to collect metrics: %v", err)
		return
	}

	// Send metrics
	endpoint := r.backendURL + "/api/v1/metrics/agents/" + r.serverID

	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("X-API-Key", r.apiKey).
		SetBody(m).
		Post(endpoint)

	if err != nil {
		log.Printf("Failed to send metrics: %v", err)
		return
	}

	if resp.StatusCode() != 200 {
		log.Printf("Unexpected status from backend: %d", resp.StatusCode())
		return
	}

	log.Printf("Metrics sent successfully to backend")
}
