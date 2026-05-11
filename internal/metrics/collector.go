package metrics

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

// Collector collects system and VPN metrics
type Collector struct {
	serverID    string
	cache       *ServerMetrics
	cacheTime   time.Time
	cacheTTL    time.Duration
	wgCollector *vpn.WireGuardMetricsCollector
}

// NewCollector creates a new metrics collector
func NewCollector(serverID string) *Collector {
	return &Collector{
		serverID: serverID,
		cacheTTL: 10 * time.Second,
	}
}

// SetWireGuardCollector sets the WireGuard metrics collector
func (c *Collector) SetWireGuardCollector(wgCollector *vpn.WireGuardMetricsCollector) {
	c.wgCollector = wgCollector
}

// GetMetrics returns current metrics (cached for cacheTTL duration)
func (c *Collector) GetMetrics(ctx context.Context) (*ServerMetrics, error) {
	// Return cached metrics if still valid
	if c.cache != nil && time.Since(c.cacheTime) < c.cacheTTL {
		return c.cache, nil
	}

	// Collect fresh metrics
	metrics := &ServerMetrics{
		ServerID:  c.serverID,
		Timestamp: time.Now(),
	}

	// CPU metrics
	cpuPercent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, err
	}
	if len(cpuPercent) > 0 {
		metrics.System.CPUPercent = cpuPercent[0]
	}

	// Memory metrics
	vmStats, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	metrics.System.MemoryPercent = vmStats.UsedPercent

	// Disk metrics
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}
	metrics.System.DiskPercent = diskUsage.UsedPercent

	// Network metrics
	ioCounters, err := net.IOCountersWithContext(ctx, false)
	if err != nil {
		return nil, err
	}
	if len(ioCounters) > 0 {
		metrics.System.NetworkRXBytes = ioCounters[0].BytesRecv
		metrics.System.NetworkTXBytes = ioCounters[0].BytesSent
	}

	// VPN metrics (to be implemented in subsequent tasks)
	// For now, return zeros
	metrics.VPN.WireGuard.ActivePeers = 0
	metrics.VPN.WireGuard.TotalBytesTransferred = 0

	// Collect WireGuard metrics if collector is available
	if c.wgCollector != nil {
		wgMetrics, err := c.GetWireGuardMetrics(c.wgCollector)
		if err != nil {
			// Log error but continue - WireGuard metrics are optional
			// WireGuard metrics will be populated in the handler level
		} else {
			// Update VPN WireGuard metrics from collector
			if peerCount, ok := wgMetrics["peer_count"].(int); ok {
				metrics.VPN.WireGuard.ActivePeers = peerCount
			}
			if totalBytes, ok := wgMetrics["total_bytes"].(uint64); ok {
				metrics.VPN.WireGuard.TotalBytesTransferred = totalBytes
			}
		}
	}

	// Latency metrics (to be implemented)
	metrics.Latency.Avg = 0
	metrics.Latency.P95 = 0
	metrics.Latency.P99 = 0

	// Cache the metrics
	c.cache = metrics
	c.cacheTime = time.Now()

	return metrics, nil
}

// GetWireGuardMetrics collects WireGuard peer metrics
func (c *Collector) GetWireGuardMetrics(wgCollector *vpn.WireGuardMetricsCollector) (map[string]interface{}, error) {
	metrics, err := wgCollector.GetPeerMetrics()
	if err != nil {
		return nil, err
	}

	// Calculate totals
	var totalRx, totalTx uint64
	var connectedCount int
	var lastHandshake *time.Time

	for _, peer := range metrics.Peers {
		totalRx += peer.BytesReceived
		totalTx += peer.BytesSent
		if peer.IsConnected {
			connectedCount++
		}
		if lastHandshake == nil || peer.LastHandshake.After(*lastHandshake) {
			lastHandshake = &peer.LastHandshake
		}
	}

	result := map[string]interface{}{
		"peer_count":      metrics.PeerCount,
		"connected_peers": connectedCount,
		"total_bytes_rx":  totalRx,
		"total_bytes_tx":  totalTx,
		"total_bytes":     totalRx + totalTx,
		"last_handshake":  nil,
	}

	if lastHandshake != nil {
		result["last_handshake"] = lastHandshake.Format(time.RFC3339)
	}

	return result, nil
}
