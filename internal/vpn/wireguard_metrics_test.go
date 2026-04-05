package vpn

import (
	"testing"
)

func TestNewWireGuardMetricsCollector(t *testing.T) {
	collector := NewWireGuardMetricsCollector("wg0")
	if collector == nil {
		t.Fatal("Expected collector to be created, got nil")
	}
	if collector.interfaceName != "wg0" {
		t.Errorf("Expected interfaceName to be 'wg0', got '%s'", collector.interfaceName)
	}
}

func TestWireGuardMetricsCollector_GetPeerMetrics_Integration(t *testing.T) {
	// Skip if wg0 interface doesn't exist (requires real WireGuard interface)
	// This is an integration test that will only pass on a system with wg0 configured
	collector := NewWireGuardMetricsCollector("wg0")
	
	metrics, err := collector.GetPeerMetrics()
	if err != nil {
		// Expected on systems without WireGuard interface
		t.Skipf("Skipping integration test (no WireGuard interface): %v", err)
	}

	if metrics == nil {
		t.Fatal("Expected metrics to be returned, got nil")
	}

	if metrics.PeerCount < 0 {
		t.Errorf("Expected non-negative peer count, got %d", metrics.PeerCount)
	}

	// If there are peers, validate their structure
	for _, peer := range metrics.Peers {
		if peer.PublicKey == "" {
			t.Error("Expected peer public key to be non-empty")
		}
		if peer.AllowedIPs == nil {
			t.Error("Expected AllowedIPs slice to be initialized")
		}
	}
}

func TestWireGuardMetricsCollector_GetPeerByPublicKey_NotFound(t *testing.T) {
	// Test error case when peer doesn't exist
	collector := NewWireGuardMetricsCollector("wg0")
	
	_, err := collector.GetPeerByPublicKey("nonexistent-key")
	if err == nil {
		t.Error("Expected error for non-existent peer, got nil")
	}

	expectedMsg := "peer nonexistent-key not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}
