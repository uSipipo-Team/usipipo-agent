package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

func TestCollector_GetOutlineMetrics_Success(t *testing.T) {
	// Mock Outline API endpoints
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name":           "Test Server",
				"serverId":       "test-uuid",
				"version":        "1.10.0",
				"metricsEnabled": true,
			})
		case "/access-keys":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"accessKeys": []interface{}{
					map[string]interface{}{"id": "1"},
					map[string]interface{}{"id": "2"},
				},
			})
		case "/metrics/transfer":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"bytesTransferredByUserId": map[string]interface{}{
					"1": float64(5242880000),
					"2": float64(10485760000),
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	collector := NewCollector("test-server-id")
	outlineClient := vpn.NewOutlineClient(server.URL, false)

	metrics, err := collector.GetOutlineMetrics(context.Background(), outlineClient)

	assert.NoError(t, err)
	assert.Equal(t, "online", metrics.ServerStatus)
	assert.Equal(t, "Test Server", metrics.ServerName)
	assert.Equal(t, "1.10.0", metrics.ServerVersion)
	assert.Equal(t, 2, metrics.ActiveKeysCount)
	assert.Equal(t, uint64(15728640000), metrics.TotalBytesTransferred)
	assert.True(t, metrics.OutlineAPIReachable)
	assert.Equal(t, 0, metrics.ConsecutiveFailures)
}

func TestCollector_GetOutlineMetrics_Caching(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/server":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name":    "Test",
				"version": "1.0.0",
			})
		case "/access-keys":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"accessKeys": []interface{}{},
			})
		case "/metrics/transfer":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"bytesTransferredByUserId": map[string]interface{}{},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	collector := NewCollector("test-server-id")
	outlineClient := vpn.NewOutlineClient(server.URL, false)

	// First call - should hit API 3 times (server, access-keys, transfer)
	_, err := collector.GetOutlineMetrics(context.Background(), outlineClient)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)

	// Second call (should use cache, no additional API calls)
	_, err = collector.GetOutlineMetrics(context.Background(), outlineClient)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount) // Still 3, not 6
}

func TestCollector_GetOutlineMetrics_ErrorState(t *testing.T) {
	collector := NewCollector("test-server-id")
	outlineClient := vpn.NewOutlineClient("http://invalid-host:99999", false)

	metrics, err := collector.GetOutlineMetrics(context.Background(), outlineClient)

	assert.NoError(t, err) // Should not error, just return error state
	assert.Equal(t, "error", metrics.ServerStatus)
	assert.False(t, metrics.OutlineAPIReachable)
	assert.Contains(t, metrics.LastError, "unreachable")
	assert.Equal(t, 1, metrics.ConsecutiveFailures)
}

func TestCollector_GetOutlineMetrics_ConsecutiveFailures(t *testing.T) {
	// First call: fresh error state (no prior cache)
	collector := NewCollector("test-server-id")
	outlineClient := vpn.NewOutlineClient("http://invalid-host:99999", false)

	metrics1, err := collector.GetOutlineMetrics(context.Background(), outlineClient)
	assert.NoError(t, err)
	assert.Equal(t, 1, metrics1.ConsecutiveFailures)

	// Wait for cache to expire (1 min TTL for error state)
	collector.outlineCacheTime = time.Now().Add(-2 * time.Minute)

	// Second call: should increment consecutive failures
	metrics2, err := collector.GetOutlineMetrics(context.Background(), outlineClient)
	assert.NoError(t, err)
	assert.Equal(t, 2, metrics2.ConsecutiveFailures)
}

func TestCollector_GetOutlineMetrics_ErrorRecovery(t *testing.T) {
	// Start with failing client
	collector := NewCollector("test-server-id")
	invalidClient := vpn.NewOutlineClient("http://invalid-host:99999", false)

	// Get error state
	metrics, err := collector.GetOutlineMetrics(context.Background(), invalidClient)
	assert.NoError(t, err)
	assert.Equal(t, "error", metrics.ServerStatus)
	assert.Equal(t, 1, metrics.ConsecutiveFailures)

	// Expire the cache
	collector.outlineCacheTime = time.Now().Add(-2 * time.Minute)

	// Now use a valid mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name":    "Recovered Server",
				"version": "1.5.0",
			})
		case "/access-keys":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"accessKeys": []interface{}{
					map[string]interface{}{"id": "1"},
				},
			})
		case "/metrics/transfer":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"bytesTransferredByUserId": map[string]interface{}{
					"1": float64(1048576),
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	validClient := vpn.NewOutlineClient(server.URL, false)

	// Should recover and reset failure counter
	metrics, err = collector.GetOutlineMetrics(context.Background(), validClient)
	assert.NoError(t, err)
	assert.Equal(t, "online", metrics.ServerStatus)
	assert.Equal(t, "Recovered Server", metrics.ServerName)
	assert.True(t, metrics.OutlineAPIReachable)
	assert.Equal(t, 0, metrics.ConsecutiveFailures)
}

func TestCollector_ShouldCollectDetailed(t *testing.T) {
	collector := NewCollector("test-server-id")

	// Initially should collect (never collected)
	assert.True(t, collector.ShouldCollectDetailed())

	// Mark as collected
	collector.MarkDetailedCollected()
	assert.False(t, collector.ShouldCollectDetailed())

	// Simulate time passing (1 hour + 1 second)
	collector.detailedCacheTime = time.Now().Add(-1 * time.Hour - 1 * time.Second)
	assert.True(t, collector.ShouldCollectDetailed())
}

func TestCollector_GetOutlineMetrics_PartialFailure(t *testing.T) {
	// Server responds, but transfer endpoint fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"name":    "Partial Server",
				"version": "1.2.0",
			})
		case "/access-keys":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"accessKeys": []interface{}{
					map[string]interface{}{"id": "1"},
					map[string]interface{}{"id": "2"},
					map[string]interface{}{"id": "3"},
				},
			})
		case "/metrics/transfer":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	collector := NewCollector("test-server-id")
	outlineClient := vpn.NewOutlineClient(server.URL, false)

	metrics, err := collector.GetOutlineMetrics(context.Background(), outlineClient)

	assert.NoError(t, err)
	assert.Equal(t, "online", metrics.ServerStatus)
	assert.True(t, metrics.OutlineAPIReachable)
	assert.Equal(t, 3, metrics.ActiveKeysCount)
	assert.Equal(t, uint64(0), metrics.TotalBytesTransferred) // Failed endpoint returns 0
}
