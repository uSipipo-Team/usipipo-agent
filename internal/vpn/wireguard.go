package vpn

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WireGuardClient handles communication with WireGuard interface
type WireGuardClient struct {
	interfaceName string
	configPath    string
	serverIP      string
	serverPort    int
	clientDNS     string
	client        *wgctrl.Client
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
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	return &WireGuardClient{
		interfaceName: interfaceName,
		configPath:    configPath,
		serverIP:      serverIP,
		serverPort:    serverPort,
		clientDNS:     clientDNS,
		client:        client,
	}, nil
}

// Close closes the wgctrl client
func (c *WireGuardClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// CreatePeer creates a new WireGuard peer using wgctrl
func (c *WireGuardClient) CreatePeer(ctx context.Context, name string) (*WireGuardPeer, error) {
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

// DeletePeer removes a WireGuard peer using wgctrl
// This operation is idempotent - returns success even if peer doesn't exist
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

func (c *WireGuardClient) getNextAvailableIP() (string, error) {
	// Read config file to find used IPs
	content, err := os.ReadFile(c.configPath)
	if err != nil {
		return "10.0.0.2", nil // Default fallback
	}

	// Find all used IPs
	ipPattern := regexp.MustCompile(`AllowedIPs\s*=\s*([\d.]+)/32`)
	matches := ipPattern.FindAllStringSubmatch(string(content), -1)

	usedIPs := make(map[string]bool)
	for _, match := range matches {
		usedIPs[match[1]] = true
	}

	// Find first available IP in 10.0.0.0/24 range
	for i := 2; i < 255; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		if !usedIPs[ip] {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no available IPs in range")
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
