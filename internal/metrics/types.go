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
	Outline struct {
		ActiveKeys          int    `json:"active_keys"`
		TotalBytesTransferred uint64 `json:"total_bytes_transferred"`
	} `json:"outline"`
	WireGuard struct {
		ActivePeers         int    `json:"active_peers"`
		TotalBytesTransferred uint64 `json:"total_bytes_transferred"`
	} `json:"wireguard"`
	TrustTunnel struct {
		ActiveClients         int    `json:"active_clients"`
		TotalBytesTransferred uint64 `json:"total_bytes_transferred"`
	} `json:"trusttunnel"`
}

// LatencyMetrics represents latency measurements
type LatencyMetrics struct {
	Avg float64 `json:"avg"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// OutlineMetrics represents Outline-specific server metrics
type OutlineMetrics struct {
	ServerStatus          string     `json:"server_status"`
	ServerVersion         string     `json:"server_version"`
	ServerName            string     `json:"server_name"`
	ActiveKeysCount       int        `json:"active_keys_count"`
	TotalBytesTransferred uint64     `json:"total_bytes_transferred"`
	OutlineAPIReachable   bool       `json:"outline_api_reachable"`
	LastSuccessfulCheck   *time.Time `json:"last_successful_check,omitempty"`
	LastError             string     `json:"last_error,omitempty"`
	ConsecutiveFailures   int        `json:"consecutive_failures"`
}

// DetailedOutlineMetrics represents time-series Outline metrics
type DetailedOutlineMetrics struct {
	Status        string              `json:"status"`
	TimeSeries24h []TimeSeriesDataPoint `json:"time_series_24h,omitempty"`
	TopConsumers  []TopConsumer       `json:"top_consumers,omitempty"`
}

// TimeSeriesDataPoint represents a single time-series data point
type TimeSeriesDataPoint struct {
	Timestamp int64  `json:"timestamp"`
	KeyID     string `json:"key_id"`
	Bytes     uint64 `json:"bytes"`
}

// TopConsumer represents a top bandwidth consumer
type TopConsumer struct {
	KeyID string `json:"key_id"`
	Bytes uint64 `json:"bytes"`
}

// ServerMetrics represents the complete metrics payload sent to backend
type ServerMetrics struct {
	ServerID  string         `json:"server_id"`
	Timestamp time.Time      `json:"timestamp"`
	System    SystemMetrics  `json:"system"`
	VPN       VPNMetrics     `json:"vpn"`
	Outline   *OutlineMetrics `json:"outline,omitempty"`
	Detailed  *DetailedOutlineMetrics `json:"detailed,omitempty"`
	Latency   LatencyMetrics `json:"latency_ms"`
}
