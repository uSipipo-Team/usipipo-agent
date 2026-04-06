package vpn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrustTunnelMetricsCollector_GetMetrics_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
# HELP trusttunnel_bytes_total Total bytes transferred
# TYPE trusttunnel_bytes_total counter
trusttunnel_bytes_total{client="user1"} 1048576
trusttunnel_bytes_total{client="user2"} 2097152
# HELP trusttunnel_active_clients Number of active clients
# TYPE trusttunnel_active_clients gauge
trusttunnel_active_clients 2
`))
	}))
	defer server.Close()

	collector := NewTrustTunnelMetricsCollector(server.URL)
	metrics, err := collector.GetMetrics()

	assert.NoError(t, err)
	assert.Equal(t, 2, metrics.ActiveClients)
	assert.Equal(t, uint64(3145728), metrics.TotalBytesTransferred)
	assert.Equal(t, 2, len(metrics.ClientBytes))
	assert.Equal(t, uint64(1048576), metrics.ClientBytes["user1"])
}

func TestTrustTunnelMetricsCollector_GetMetrics_EndpointUnavailable(t *testing.T) {
	collector := NewTrustTunnelMetricsCollector("http://invalid-host:99999")
	metrics, err := collector.GetMetrics()

	assert.NoError(t, err)
	assert.Equal(t, 0, metrics.ActiveClients)
	assert.Equal(t, uint64(0), metrics.TotalBytesTransferred)
}
