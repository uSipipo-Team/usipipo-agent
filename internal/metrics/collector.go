package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

// Collector collects system and VPN metrics
type Collector struct {
	serverID              string
	cache                 *ServerMetrics
	cacheTime             time.Time
	cacheTTL              time.Duration
	outlineCache          *OutlineMetrics
	outlineCacheTime      time.Time
	outlineTTL            time.Duration // 5 minutes
	detailedCache         *DetailedOutlineMetrics
	detailedCacheTime     time.Time
	detailedTTL           time.Duration // 1 hour
}

// NewCollector creates a new metrics collector
func NewCollector(serverID string) *Collector {
	return &Collector{
		serverID:  serverID,
		cacheTTL:  10 * time.Second,
		outlineTTL: 5 * time.Minute,
		detailedTTL: 1 * time.Hour,
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

// GetOutlineMetrics collects Outline-specific metrics with caching
func (c *Collector) GetOutlineMetrics(ctx context.Context, outlineClient *vpn.OutlineClient) (*OutlineMetrics, error) {
	// Return cached metrics if still valid
	if c.outlineCache != nil && time.Since(c.outlineCacheTime) < c.outlineTTL {
		return c.outlineCache, nil
	}

	metrics := &OutlineMetrics{
		ConsecutiveFailures: 0,
	}

	// Check server status
	info, err := outlineClient.CheckStatus(ctx)
	if err != nil {
		metrics.OutlineAPIReachable = false
		metrics.LastError = err.Error()
		metrics.ServerStatus = "error"
		if c.outlineCache != nil {
			metrics.ConsecutiveFailures = c.outlineCache.ConsecutiveFailures + 1
		}
		// Cache error state for shorter period (1 min)
		c.outlineCache = metrics
		c.outlineCacheTime = time.Now()
		c.outlineTTL = 1 * time.Minute
		return metrics, nil
	}

	// Success - reset error state
	metrics.OutlineAPIReachable = true
	metrics.ServerStatus = "online"
	metrics.ServerVersion = info.Version
	metrics.ServerName = info.Name
	now := time.Now()
	metrics.LastSuccessfulCheck = &now
	metrics.ConsecutiveFailures = 0

	// Get active keys count
	keyCount, err := outlineClient.GetActiveKeysCount(ctx)
	if err != nil {
		metrics.ActiveKeysCount = 0
	} else {
		metrics.ActiveKeysCount = keyCount
	}

	// Get transfer metrics
	transfer, err := outlineClient.GetTransferMetrics(ctx)
	if err != nil {
		metrics.TotalBytesTransferred = 0
	} else {
		// Sum all bytes
		var total uint64
		for _, bytes := range transfer.BytesTransferredByUserID {
			total += bytes
		}
		metrics.TotalBytesTransferred = total
	}

	// Cache the metrics (5 min TTL)
	c.outlineCache = metrics
	c.outlineCacheTime = time.Now()
	c.outlineTTL = 5 * time.Minute

	return metrics, nil
}

// GetDetailedOutlineMetrics collects time-series metrics with hourly caching
func (c *Collector) GetDetailedOutlineMetrics(ctx context.Context, outlineClient *vpn.OutlineClient) (*DetailedOutlineMetrics, error) {
	// Return cached metrics if still valid (1 hour TTL)
	if c.detailedCache != nil && time.Since(c.detailedCacheTime) < c.detailedTTL {
		return c.detailedCache, nil
	}

	// Get detailed metrics from Outline API
	detailed, err := outlineClient.GetDetailedMetrics(ctx, "24h")
	if err != nil {
		return nil, fmt.Errorf("failed to get detailed metrics: %w", err)
	}

	// Transform into our internal format
	result := &DetailedOutlineMetrics{
		Status: detailed.Status,
	}

	// Parse time-series data
	for _, metricResult := range detailed.Data.Result {
		for _, valuePair := range metricResult.Values {
			if len(valuePair) == 2 {
				timestamp, _ := valuePair[0].(float64)
				bytesStr, _ := valuePair[1].(string)
				bytes, _ := strconv.ParseUint(bytesStr, 10, 64)

				result.TimeSeries24h = append(result.TimeSeries24h, TimeSeriesDataPoint{
					Timestamp: int64(timestamp),
					KeyID:     metricResult.Metric.AccessKey,
					Bytes:     bytes,
				})
			}
		}
	}

	// Calculate top consumers
	consumerBytes := make(map[string]uint64)
	for _, point := range result.TimeSeries24h {
		consumerBytes[point.KeyID] += point.Bytes
	}

	// Sort and get top 10
	type consumer struct {
		KeyID string
		Bytes uint64
	}
	var consumers []consumer
	for keyID, bytes := range consumerBytes {
		consumers = append(consumers, consumer{keyID, bytes})
	}
	// Simple sort by bytes (descending)
	for i := 0; i < len(consumers); i++ {
		for j := i + 1; j < len(consumers); j++ {
			if consumers[j].Bytes > consumers[i].Bytes {
				consumers[i], consumers[j] = consumers[j], consumers[i]
			}
		}
	}

	topCount := 10
	if len(consumers) < topCount {
		topCount = len(consumers)
	}
	for i := 0; i < topCount; i++ {
		result.TopConsumers = append(result.TopConsumers, TopConsumer{
			KeyID: consumers[i].KeyID,
			Bytes: consumers[i].Bytes,
		})
	}

	// Cache the metrics (1 hour TTL)
	c.detailedCache = result
	c.detailedCacheTime = time.Now()

	return result, nil
}

// ShouldCollectDetailed checks if it's time to collect detailed metrics
func (c *Collector) ShouldCollectDetailed() bool {
	return time.Since(c.detailedCacheTime) >= c.detailedTTL
}

// MarkDetailedCollected updates the last collection time
func (c *Collector) MarkDetailedCollected() {
	c.detailedCacheTime = time.Now()
}
