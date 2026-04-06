package vpn

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

// TrustTunnelMetrics represents TrustTunnel metrics
type TrustTunnelMetrics struct {
	ActiveClients         int               `json:"active_clients"`
	TotalBytesTransferred uint64            `json:"total_bytes_transferred"`
	ClientBytes           map[string]uint64 `json:"client_bytes,omitempty"`
}

// TrustTunnelMetricsCollector collects metrics from TrustTunnel
type TrustTunnelMetricsCollector struct {
	metricsURL string
}

// NewTrustTunnelMetricsCollector creates a new TrustTunnel metrics collector
func NewTrustTunnelMetricsCollector(metricsURL string) *TrustTunnelMetricsCollector {
	return &TrustTunnelMetricsCollector{
		metricsURL: metricsURL,
	}
}

// GetMetrics collects TrustTunnel metrics from Prometheus endpoint
func (c *TrustTunnelMetricsCollector) GetMetrics() (*TrustTunnelMetrics, error) {
	resp, err := http.Get(c.metricsURL)
	if err != nil {
		return &TrustTunnelMetrics{
			ClientBytes: make(map[string]uint64),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &TrustTunnelMetrics{
			ClientBytes: make(map[string]uint64),
		}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TrustTunnelMetrics{
			ClientBytes: make(map[string]uint64),
		}, nil
	}

	return c.parsePrometheusMetrics(string(body))
}

func (c *TrustTunnelMetricsCollector) parsePrometheusMetrics(body string) (*TrustTunnelMetrics, error) {
	metrics := &TrustTunnelMetrics{
		ClientBytes: make(map[string]uint64),
	}

	bytesPattern := regexp.MustCompile(`trusttunnel_bytes_total\{client="([^"]+)"\}\s+(\d+)`)
	for _, match := range bytesPattern.FindAllStringSubmatch(body, -1) {
		client := match[1]
		bytes, _ := strconv.ParseUint(match[2], 10, 64)
		metrics.ClientBytes[client] = bytes
		metrics.TotalBytesTransferred += bytes
	}

	activePattern := regexp.MustCompile(`trusttunnel_active_clients\s+(\d+)`)
	match := activePattern.FindStringSubmatch(body)
	if len(match) > 1 {
		metrics.ActiveClients, _ = strconv.Atoi(match[1])
	}

	if metrics.ActiveClients == 0 && len(metrics.ClientBytes) > 0 {
		metrics.ActiveClients = len(metrics.ClientBytes)
	}

	return metrics, nil
}

// GetClientBytes returns bytes transferred for a specific client
func (c *TrustTunnelMetricsCollector) GetClientBytes(username string) (uint64, error) {
	metrics, err := c.GetMetrics()
	if err != nil {
		return 0, err
	}

	if bytes, ok := metrics.ClientBytes[username]; ok {
		return bytes, nil
	}

	return 0, fmt.Errorf("client not found: %s", username)
}
