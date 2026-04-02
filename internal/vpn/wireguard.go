package vpn

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// VPN key name validation constants
const (
	VPNKeyNameMinLength = 3
	VPNKeyNameMaxLength = 50
)

// WireGuardClient handles communication with WireGuard interface
type WireGuardClient struct {
	interfaceName    string
	configPath       string
	serverIP         string
	serverPort       int
	clientDNS        string
	client           *wgctrl.Client
	networkCIDR      string
	startIP          int
	endIP            int
	ipAllocationLock sync.Mutex
}

// WireGuardPeer represents a WireGuard peer
type WireGuardPeer struct {
	PublicKey string `json:"public_key"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Config    string `json:"config"`
}

// NewWireGuardClient creates a new WireGuard client using wgctrl
func NewWireGuardClient(interfaceName, configPath, serverIP string, serverPort int, clientDNS string) (*WireGuardClient, error) {
	return NewWireGuardClientWithRange(interfaceName, configPath, serverIP, serverPort, clientDNS, "10.0.0.0/24", 2, 254)
}

// NewWireGuardClientWithRange creates a new WireGuard client with custom IP range
func NewWireGuardClientWithRange(interfaceName, configPath, serverIP string, serverPort int, clientDNS, networkCIDR string, startIP, endIP int) (*WireGuardClient, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	// Validate IP range
	if startIP < 2 || endIP > 254 || startIP >= endIP {
		return nil, fmt.Errorf("invalid IP range: start=%d, end=%d (must be 2-254, start < end)", startIP, endIP)
	}

	return &WireGuardClient{
		interfaceName: interfaceName,
		configPath:    configPath,
		serverIP:      serverIP,
		serverPort:    serverPort,
		clientDNS:     clientDNS,
		client:        client,
		networkCIDR:   networkCIDR,
		startIP:       startIP,
		endIP:         endIP,
	}, nil
}

// Close closes the wgctrl client
func (c *WireGuardClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// ValidateKeyName validates a VPN key name according to strict rules:
// - Length: 3-50 characters
// - Allowed: alphanumeric (a-zA-Z0-9), spaces, hyphens (-), underscores (_)
// - Blocked: Emoji, unicode confusables, shell special chars
func (c *WireGuardClient) ValidateKeyName(name string) bool {
	// Check length
	if len(name) < VPNKeyNameMinLength || len(name) > VPNKeyNameMaxLength {
		return false
	}

	// Check each character is allowed
	for _, r := range name {
		// Allow alphanumeric
		if unicode.IsLetter(r) && r <= unicode.MaxASCII {
			continue
		}
		if unicode.IsDigit(r) {
			continue
		}
		// Allow spaces, hyphens, underscores
		if r == ' ' || r == '-' || r == '_' {
			continue
		}
		// Block everything else (emoji, unicode confusables, special chars)
		return false
	}

	return true
}

// CreatePeer creates a new WireGuard peer using wgctrl
func (c *WireGuardClient) CreatePeer(ctx context.Context, name string) (*WireGuardPeer, error) {
	// Validate key name first (defense in depth)
	if !c.ValidateKeyName(name) {
		return nil, fmt.Errorf("invalid VPN key name: must be %d-%d characters, alphanumeric, spaces, hyphens, or underscores only", VPNKeyNameMinLength, VPNKeyNameMaxLength)
	}

	// Generate private key using wgctrl
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey := privateKey.PublicKey()

	// Generate pre-shared key
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate preshared key: %w", err)
	}

	// Get next available IP
	ip, err := c.getNextAvailableIP()
	if err != nil {
		return nil, err
	}

	// Parse IP for AllowedIPs
	_, ipNet, err := net.ParseCIDR(ip + "/32")
	if err != nil {
		return nil, fmt.Errorf("failed to parse IP: %w", err)
	}

	// Configure peer using wgctrl
	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:         publicKey,
				PresharedKey:      &psk,
				AllowedIPs:        []net.IPNet{*ipNet},
				ReplaceAllowedIPs: false,
			},
		},
	}

	err = c.client.ConfigureDevice(c.interfaceName, config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure device: %w", err)
	}

	// Get server public key
	device, err := c.client.Device(c.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	// Generate client config
	configStr := c.generateClientConfig(privateKey.String(), ip, device.PublicKey.String(), psk.String())

	return &WireGuardPeer{
		PublicKey: publicKey.String(),
		Name:      name,
		IPAddress: ip,
		Config:    configStr,
	}, nil
}

// DeletePeer removes a WireGuard peer using wgctrl.
// This operation is idempotent - returns success even if the peer doesn't exist.
// Idempotent behavior:
//   - Returns nil if peer is successfully deleted (204 equivalent)
//   - Returns nil if peer is not found in config (already deleted)
//   - Returns nil if wgctrl reports "no such process" or "not found"
func (c *WireGuardClient) DeletePeer(ctx context.Context, name string) error {
	// Find peer public key from config
	pubKey, err := c.findPeerPublicKey(name)
	if err != nil {
		// Peer not found in config file - assume already deleted (idempotent)
		if strings.Contains(err.Error(), "peer not found") {
			return nil
		}
		return err
	}

	// Parse public key
	key, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// Remove peer using wgctrl
	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: key,
				Remove:    true,
			},
		},
	}

	err = c.client.ConfigureDevice(c.interfaceName, config)
	if err != nil {
		// If peer doesn't exist, wgctrl may return an error
		// Treat "not found" errors as success (idempotent behavior)
		if strings.Contains(err.Error(), "no such process") ||
		   strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	return nil
}

// GetPeerUsage returns the data transfer for a specific peer
func (c *WireGuardClient) GetPeerUsage(ctx context.Context, name string) (uint64, error) {
	device, err := c.client.Device(c.interfaceName)
	if err != nil {
		return 0, err
	}

	// Find peer by name in config file
	pubKey, err := c.findPeerPublicKey(name)
	if err != nil {
		return 0, err
	}

	// Find peer in device and get bytes
	for _, peer := range device.Peers {
		if peer.PublicKey.String() == pubKey {
			return uint64(peer.ReceiveBytes + peer.TransmitBytes), nil
		}
	}

	return 0, nil
}

// GetActivePeersCount returns the number of active peers
func (c *WireGuardClient) GetActivePeersCount(ctx context.Context) (int, error) {
	device, err := c.client.Device(c.interfaceName)
	if err != nil {
		return 0, err
	}

	return len(device.Peers), nil
}

// GetTotalBytesTransferred returns total bytes transferred across all peers
func (c *WireGuardClient) GetTotalBytesTransferred(ctx context.Context) (uint64, error) {
	device, err := c.client.Device(c.interfaceName)
	if err != nil {
		return 0, err
	}

	var total uint64
	for _, peer := range device.Peers {
		total += uint64(peer.ReceiveBytes + peer.TransmitBytes)
	}

	return total, nil
}

// Helper methods

// getNextAvailableIP finds the next available IP address with file locking to prevent race conditions.
// Uses exclusive file locking (flock) to ensure only one IP allocation happens at a time.
// Lock timeout: 5 seconds. Returns error if lock cannot be acquired or no IPs available.
func (c *WireGuardClient) getNextAvailableIP() (string, error) {
	// Use mutex for in-process synchronization
	c.ipAllocationLock.Lock()
	defer c.ipAllocationLock.Unlock()

	// Create lock file for cross-process synchronization
	lockPath := c.configPath + ".alloc.lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return "", fmt.Errorf("failed to open lock file: %w", err)
	}
	defer lockFile.Close()

	// Acquire exclusive lock with timeout (5 seconds)
	lockTimeout := 5 * time.Second
	done := make(chan error, 1)
	go func() {
		err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("failed to acquire IP allocation lock: %w", err)
		}
	case <-time.After(lockTimeout):
		return "", fmt.Errorf("timeout acquiring IP allocation lock after %v", lockTimeout)
	}

	// Ensure lock is released when function returns
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Now safe to read config and find available IP
	content, err := os.ReadFile(c.configPath)
	if err != nil {
		// If config doesn't exist, return first IP in range
		if os.IsNotExist(err) {
			return fmt.Sprintf("10.0.0.%d", c.startIP), nil
		}
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	// Find all used IPs in the config
	ipPattern := regexp.MustCompile(`AllowedIPs\s*=\s*([\d.]+)/32`)
	matches := ipPattern.FindAllStringSubmatch(string(content), -1)

	usedIPs := make(map[string]bool)
	for _, match := range matches {
		usedIPs[match[1]] = true
	}

	// Find first available IP in configured range
	for i := c.startIP; i <= c.endIP; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		if !usedIPs[ip] {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no available IPs in range %d-%d", c.startIP, c.endIP)
}

func (c *WireGuardClient) findPeerPublicKey(name string) (string, error) {
	content, err := os.ReadFile(c.configPath)
	if err != nil {
		return "", err
	}

	// Look for peer comment with name
	pattern := fmt.Sprintf(`### CLIENT %s.*?PublicKey\s*=\s*([^\n]+)`, regexp.QuoteMeta(name))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(string(content))

	if len(matches) < 2 {
		return "", fmt.Errorf("peer not found: %s", name)
	}

	return strings.TrimSpace(matches[1]), nil
}

func (c *WireGuardClient) generateClientConfig(privKey, ip, serverPub, psk string) string {
	endpoint := fmt.Sprintf("%s:%d", c.serverIP, c.serverPort)

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
DNS = %s
MTU = 1420

[Peer]
PublicKey = %s
PresharedKey = %s
Endpoint = %s
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 15
`, privKey, ip, c.clientDNS, serverPub, psk, endpoint)
}
