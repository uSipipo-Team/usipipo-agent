package metrics

import "time"

// SystemMetrics represents system-level metrics
type SystemMetrics struct {
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryPercent  float64   `json:"memory_percent"`
	DiskPercent    float64   `json:"disk_percent"`
	NetworkRXBytes uint64    `json:"network_rx_bytes"`
	NetworkTXBytes uint64    `json:"network_tx_bytes"`
	Timestamp      time.Time `json:"timestamp"`
}

// VPNMetrics represents VPN-specific metrics
type VPNMetrics struct {
	WireGuard struct {
		ActivePeers           int    `json:"active_peers"`
		TotalBytesTransferred uint64 `json:"total_bytes_transferred"`
	} `json:"wireguard"`
}

// LatencyMetrics represents latency measurements
type LatencyMetrics struct {
	Avg float64 `json:"avg"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// ServerMetrics represents the complete metrics payload sent to backend
type ServerMetrics struct {
	ServerID  string         `json:"server_id"`
	Timestamp time.Time      `json:"timestamp"`
	System    SystemMetrics  `json:"system"`
	VPN       VPNMetrics     `json:"vpn"`
	Latency   LatencyMetrics `json:"latency_ms"`
}
