package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector("server-123")
	if c == nil {
		t.Fatal("expected non-nil collector")
	}
	if c.serverID != "server-123" {
		t.Errorf("expected serverID 'server-123', got '%s'", c.serverID)
	}
	if c.cacheTTL != 10*time.Second {
		t.Errorf("expected cacheTTL 10s, got %v", c.cacheTTL)
	}
}

func TestGetMetricsCache(t *testing.T) {
	ctx := context.Background()
	c := NewCollector("cache-test")

	first, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("first GetMetrics call failed: %v", err)
	}

	second, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("second GetMetrics call failed: %v", err)
	}

	if first != second {
		t.Error("expected second call to return cached pointer")
	}

	if first.ServerID != "cache-test" {
		t.Errorf("expected ServerID 'cache-test', got '%s'", first.ServerID)
	}
}

func TestGetMetricsCacheExpiry(t *testing.T) {
	ctx := context.Background()
	c := NewCollector("cache-expiry")

	c.cacheTTL = 1 * time.Nanosecond

	first, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("first GetMetrics call failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	second, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("second GetMetrics call failed: %v", err)
	}

	if first == second {
		t.Error("expected different pointers after cache expiry")
	}
}

func TestGetWireGuardMetrics_NoInterface(t *testing.T) {
	c := NewCollector("wg-test")
	wgCollector := vpn.NewWireGuardMetricsCollector("wg-nonexistent")

	_, err := c.GetWireGuardMetrics(wgCollector)
	if err == nil {
		t.Error("expected error from WireGuardMetricsCollector with no interface")
	}
}
