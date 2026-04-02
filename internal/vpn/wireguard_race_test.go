package vpn

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestConcurrentIPAllocation tests that concurrent calls to getNextAvailableIP
// do not assign the same IP to multiple peers (race condition fix)
func TestConcurrentIPAllocation(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wg0.conf")
	
	// Create initial config with a few peers
	initialConfig := `interface: wg0
  private key: <private-key>
  public key: <public-key>
  listen port: 51820

peer: <peer1-public-key>
  # CLIENT peer-1
  allowed ips: 10.0.0.2/32

peer: <peer2-public-key>
  # CLIENT peer-2
  allowed ips: 10.0.0.3/32

peer: <peer3-public-key>
  # CLIENT peer-3
  allowed ips: 10.0.0.4/32
`
	err := os.WriteFile(configPath, []byte(initialConfig), 0600)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Create WireGuard client
	client := &WireGuardClient{
		configPath:  configPath,
		networkCIDR: "10.0.0.0/24",
		startIP:     2,
		endIP:       254,
	}

	// Track allocated IPs
	allocatedIPs := make(map[string]bool)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Number of concurrent goroutines
	numGoroutines := 50
	ipChan := make(chan string, numGoroutines)
	errChan := make(chan error, numGoroutines)

	// Launch concurrent IP allocation requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ip, err := client.getNextAvailableIP()
			if err != nil {
				errChan <- err
				return
			}
			ipChan <- ip
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(ipChan)
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("IP allocation failed: %v", err)
	}

	// Collect all allocated IPs
	allocatedIPsList := make([]string, 0)
	for ip := range ipChan {
		if allocatedIPs[ip] {
			t.Errorf("DUPLICATE IP ASSIGNED: %s", ip)
		}
		allocatedIPs[ip] = true
		allocatedIPsList = append(allocatedIPsList, ip)
	}

	// Verify we got unique IPs for all requests
	if len(allocatedIPsList) != numGoroutines {
		t.Errorf("Expected %d unique IPs, got %d", numGoroutines, len(allocatedIPsList))
	}

	// Verify all IPs are in valid range (10.0.0.5 to 10.0.0.254, since .2-.4 are used)
	for ip := range allocatedIPs {
		if ip == "10.0.0.2" || ip == "10.0.0.3" || ip == "10.0.0.4" {
			t.Errorf("IP %s was already allocated in initial config", ip)
		}
	}

	t.Logf("Successfully allocated %d unique IPs concurrently", len(allocatedIPsList))
}

// TestIPAllocationWithLockTimeout tests lock timeout behavior
func TestIPAllocationWithLockTimeout(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wg0.conf")
	
	initialConfig := `interface: wg0
  private key: <private-key>
`
	err := os.WriteFile(configPath, []byte(initialConfig), 0600)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	client := &WireGuardClient{
		configPath:  configPath,
		networkCIDR: "10.0.0.0/24",
		startIP:     2,
		endIP:       254,
	}

	// Manually acquire lock to test timeout
	lockPath := configPath + ".alloc.lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("failed to open lock file: %v", err)
	}
	defer lockFile.Close()

	// Acquire exclusive lock (will block the function's attempt)
	err = lockFile.Lock()
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Try to allocate IP (should timeout after 5 seconds)
	done := make(chan error, 1)
	go func() {
		_, err := client.getNextAvailableIP()
		done <- err
	}()

	// Wait for timeout or error
	select {
	case err := <-done:
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
		if err.Error() != "timeout acquiring IP allocation lock after 5s" {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	}

	// Release lock
	lockFile.Unlock()
}

// TestIPRangeExhaustion tests behavior when IP range is exhausted
func TestIPRangeExhaustion(t *testing.T) {
	// Create temporary config file with all IPs used
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wg0.conf")
	
	// Create config with all IPs in small range used
	config := `interface: wg0
`
	for i := 2; i <= 10; i++ {
		config += `peer: <peer-key>
  # CLIENT peer-` + string(rune('0'+i-1)) + `
  allowed ips: 10.0.0.` + string(rune('0'+i-1)) + `/32
`
	}
	
	err := os.WriteFile(configPath, []byte(config), 0600)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	client := &WireGuardClient{
		configPath:  configPath,
		networkCIDR: "10.0.0.0/24",
		startIP:     2,
		endIP:       10, // Small range for testing
	}

	// Try to allocate IP (should fail - no available IPs)
	ip, err := client.getNextAvailableIP()
	if err == nil {
		t.Errorf("Expected error for exhausted IP range, got IP: %s", ip)
	}
	
	if ip != "" {
		t.Errorf("Expected empty IP on exhaustion, got: %s", ip)
	}
}

// TestCustomIPRange tests IP allocation with custom range
func TestCustomIPRange(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wg0.conf")
	
	initialConfig := `interface: wg0
  private key: <private-key>
`
	err := os.WriteFile(configPath, []byte(initialConfig), 0600)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Test with custom range (100-110)
	client := &WireGuardClient{
		configPath:  configPath,
		networkCIDR: "10.0.0.0/24",
		startIP:     100,
		endIP:       110,
	}

	ip, err := client.getNextAvailableIP()
	if err != nil {
		t.Fatalf("failed to allocate IP: %v", err)
	}

	expectedIP := "10.0.0.100"
	if ip != expectedIP {
		t.Errorf("Expected IP %s, got %s", expectedIP, ip)
	}
}

// TestInvalidIPRangeValidation tests validation of IP range parameters
func TestInvalidIPRangeValidation(t *testing.T) {
	tests := []struct {
		name    string
		startIP int
		endIP   int
		wantErr bool
	}{
		{"start < 2", 1, 254, true},
		{"end > 254", 2, 255, true},
		{"start >= end", 100, 50, true},
		{"start == end", 100, 100, true},
		{"valid range", 2, 254, false},
		{"valid custom range", 10, 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "wg0.conf")
			
			_, err := NewWireGuardClientWithRange(
				"wg0",
				configPath,
				"localhost",
				51820,
				"1.1.1.1",
				"10.0.0.0/24",
				tt.startIP,
				tt.endIP,
			)

			if tt.wantErr && err == nil {
				t.Error("Expected error for invalid IP range, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestBackwardCompatibility tests that existing peers are not affected
func TestBackwardCompatibility(t *testing.T) {
	// Create temporary config file with existing peers
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wg0.conf")
	
	existingConfig := `interface: wg0
  private key: <private-key>
  public key: <public-key>
  listen port: 51820

peer: <peer1-public-key>
  # CLIENT existing-peer-1
  allowed ips: 10.0.0.2/32

peer: <peer2-public-key>
  # CLIENT existing-peer-2
  allowed ips: 10.0.0.5/32

peer: <peer3-public-key>
  # CLIENT existing-peer-3
  allowed ips: 10.0.0.10/32
`
	err := os.WriteFile(configPath, []byte(existingConfig), 0600)
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	client := &WireGuardClient{
		configPath:  configPath,
		networkCIDR: "10.0.0.0/24",
		startIP:     2,
		endIP:       254,
	}

	// Allocate new IP - should skip existing ones
	ip, err := client.getNextAvailableIP()
	if err != nil {
		t.Fatalf("failed to allocate IP: %v", err)
	}

	// Should get first available IP (10.0.0.3, since .2 is used)
	expectedIP := "10.0.0.3"
	if ip != expectedIP {
		t.Errorf("Expected first available IP %s, got %s", expectedIP, ip)
	}

	// Allocate another - should get next available
	ip2, err := client.getNextAvailableIP()
	if err != nil {
		t.Fatalf("failed to allocate second IP: %v", err)
	}

	expectedIP2 := "10.0.0.4"
	if ip2 != expectedIP2 {
		t.Errorf("Expected second available IP %s, got %s", expectedIP2, ip2)
	}
}
