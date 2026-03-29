package reporter

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
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

// NewReporter creates a new metrics reporter
func NewReporter(backendURL, serverID, apiKey string, collector *metrics.Collector) *Reporter {
	return &Reporter{
		backendURL: backendURL,
		serverID:   serverID,
		apiKey:     apiKey,
		client:     resty.New(),
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

	m, err := r.collector.GetMetrics(ctx)
	if err != nil {
		log.Printf("Failed to collect metrics: %v", err)
		return
	}

	endpoint := fmt.Sprintf("%s/api/v1/metrics/agents/%s", r.backendURL, r.serverID)

	resp, err := r.client.R().
		SetContext(ctx).
		SetHeader("X-API-Key", r.apiKey).
		SetBody(m).
		Post(endpoint)

	if err != nil {
		log.Printf("Failed to send metrics: %v", err)
		// Retry logic with exponential backoff could be added here
		return
	}

	if resp.StatusCode() != 200 {
		log.Printf("Unexpected status from backend: %d", resp.StatusCode())
		return
	}

	log.Printf("Metrics sent successfully to backend")
}
