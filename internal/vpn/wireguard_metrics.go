package vpn

import (
	"fmt"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
)

// WireGuardPeerMetrics holds aggregated WireGuard peer statistics
type WireGuardPeerMetrics struct {
	PeerCount int          `json:"peer_count"`
	Peers     []PeerDetail `json:"peers"`
}

// PeerDetail holds metrics for a single WireGuard peer
type PeerDetail struct {
	PublicKey     string    `json:"public_key"`
	BytesReceived uint64    `json:"bytes_received"`
	BytesSent     uint64    `json:"bytes_sent"`
	LastHandshake time.Time `json:"last_handshake"`
	IsConnected   bool      `json:"is_connected"`
	AllowedIPs    []string  `json:"allowed_ips"`
}

// WireGuardMetricsCollector collects metrics from WireGuard interface
type WireGuardMetricsCollector struct {
	interfaceName string
}

// NewWireGuardMetricsCollector creates a new WireGuard metrics collector
func NewWireGuardMetricsCollector(interfaceName string) *WireGuardMetricsCollector {
	return &WireGuardMetricsCollector{
		interfaceName: interfaceName,
	}
}

// GetPeerMetrics collects metrics for all peers on the WireGuard interface
func (c *WireGuardMetricsCollector) GetPeerMetrics() (*WireGuardPeerMetrics, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}
	defer client.Close()

	device, err := client.Device(c.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get device %s: %w", c.interfaceName, err)
	}

	metrics := &WireGuardPeerMetrics{
		PeerCount: len(device.Peers),
		Peers:     make([]PeerDetail, 0, len(device.Peers)),
	}

	now := time.Now()
	for _, peer := range device.Peers {
		detail := PeerDetail{
			PublicKey:     peer.PublicKey.String(),
			BytesReceived: uint64(peer.ReceiveBytes),
			BytesSent:     uint64(peer.TransmitBytes),
			LastHandshake: peer.LastHandshakeTime,
			AllowedIPs:    make([]string, len(peer.AllowedIPs)),
		}

		// Connected if handshake within last 5 minutes
		if now.Sub(peer.LastHandshakeTime) < 5*time.Minute {
			detail.IsConnected = true
		}

		for i, ip := range peer.AllowedIPs {
			detail.AllowedIPs[i] = ip.String()
		}

		metrics.Peers = append(metrics.Peers, detail)
	}

	return metrics, nil
}

// GetPeerByPublicKey finds a specific peer by public key
func (c *WireGuardMetricsCollector) GetPeerByPublicKey(publicKey string) (*PeerDetail, error) {
	metrics, err := c.GetPeerMetrics()
	if err != nil {
		return nil, err
	}

	for _, peer := range metrics.Peers {
		if peer.PublicKey == publicKey {
			return &peer, nil
		}
	}

	return nil, fmt.Errorf("peer %s not found", publicKey)
}
