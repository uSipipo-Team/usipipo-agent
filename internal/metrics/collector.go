package metrics

import (
	"context"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Collector collects system and VPN metrics
type Collector struct {
	serverID  string
	cache     *ServerMetrics
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewCollector creates a new metrics collector
func NewCollector(serverID string) *Collector {
	return &Collector{
		serverID: serverID,
		cacheTTL: 10 * time.Second,
	}
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
	metrics.VPN.Outline.ActiveKeys = 0
	metrics.VPN.Outline.TotalBytesTransferred = 0
	metrics.VPN.WireGuard.ActivePeers = 0
	metrics.VPN.WireGuard.TotalBytesTransferred = 0

	// Latency metrics (to be implemented)
	metrics.Latency.Avg = 0
	metrics.Latency.P95 = 0
	metrics.Latency.P99 = 0

	// Cache the metrics
	c.cache = metrics
	c.cacheTime = time.Now()

	return metrics, nil
}
