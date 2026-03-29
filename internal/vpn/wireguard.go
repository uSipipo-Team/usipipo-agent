package vpn

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// WireGuardClient handles communication with WireGuard interface
type WireGuardClient struct {
	interfaceName string
	configPath    string
	serverIP      string
	serverPort    int
	clientDNS     string
}

// WireGuardPeer represents a WireGuard peer
type WireGuardPeer struct {
	PublicKey string `json:"public_key"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Config    string `json:"config"`
}

// NewWireGuardClient creates a new WireGuard client
func NewWireGuardClient(interfaceName, configPath, serverIP string, serverPort int, clientDNS string) *WireGuardClient {
	return &WireGuardClient{
		interfaceName: interfaceName,
		configPath:    configPath,
		serverIP:      serverIP,
		serverPort:    serverPort,
		clientDNS:     clientDNS,
	}
}

// CreatePeer creates a new WireGuard peer
func (c *WireGuardClient) CreatePeer(ctx context.Context, name string) (*WireGuardPeer, error) {
	// Generate private key
	privKey, err := c.runCommand("wg", "genkey")
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate public key
	pubKey, err := c.runCommandWithInput("wg", "pubkey", privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	// Generate pre-shared key
	psk, err := c.runCommand("wg", "genpsk")
	if err != nil {
		return nil, fmt.Errorf("failed to generate preshared key: %w", err)
	}

	// Get next available IP
	ip, err := c.getNextAvailableIP()
	if err != nil {
		return nil, err
	}

	// Add peer to interface
	_, err = c.runCommand("wg", "set", c.interfaceName, "peer", pubKey, "allowed-ips", ip+"/32", "preshared-key", psk)
	if err != nil {
		return nil, fmt.Errorf("failed to add peer: %w", err)
	}

	// Get server public key
	serverPubKey, err := c.runCommand("wg", "show", c.interfaceName, "public-key")
	if err != nil {
		return nil, fmt.Errorf("failed to get server public key: %w", err)
	}

	// Generate client config
	config := c.generateClientConfig(privKey, ip, serverPubKey, psk)

	return &WireGuardPeer{
		PublicKey: pubKey,
		Name:      name,
		IPAddress: ip,
		Config:    config,
	}, nil
}

// DeletePeer deletes a WireGuard peer by name
func (c *WireGuardClient) DeletePeer(ctx context.Context, name string) error {
	// Find peer public key from config
	pubKey, err := c.findPeerPublicKey(name)
	if err != nil {
		return err
	}

	// Remove peer
	_, err = c.runCommand("wg", "set", c.interfaceName, "peer", pubKey, "remove")
	if err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	return nil
}

// GetPeerUsage returns the data transfer for a specific peer
func (c *WireGuardClient) GetPeerUsage(ctx context.Context, name string) (uint64, error) {
	pubKey, err := c.findPeerPublicKey(name)
	if err != nil {
		return 0, err
	}

	output, err := c.runCommand("wg", "show", c.interfaceName, "dump")
	if err != nil {
		return 0, err
	}

	// Parse output to find transfer for this peer
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] { // Skip header
		parts := strings.Split(line, "\t")
		if len(parts) >= 7 && parts[0] == pubKey {
			// parts[5] = rx, parts[6] = tx
			var rx, tx uint64
			fmt.Sscanf(parts[5], "%d", &rx)
			fmt.Sscanf(parts[6], "%d", &tx)
			return rx + tx, nil
		}
	}

	return 0, nil
}

// GetActivePeersCount returns the number of active peers
func (c *WireGuardClient) GetActivePeersCount(ctx context.Context) (int, error) {
	output, err := c.runCommand("wg", "show", c.interfaceName, "dump")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(output, "\n")
	// Count non-header lines
	count := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	return count, nil
}

// GetTotalBytesTransferred returns total bytes transferred across all peers
func (c *WireGuardClient) GetTotalBytesTransferred(ctx context.Context) (uint64, error) {
	output, err := c.runCommand("wg", "show", c.interfaceName, "dump")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(output, "\n")
	var total uint64
	for _, line := range lines[1:] {
		parts := strings.Split(line, "\t")
		if len(parts) >= 7 {
			var rx, tx uint64
			fmt.Sscanf(parts[5], "%d", &rx)
			fmt.Sscanf(parts[6], "%d", &tx)
			total += rx + tx
		}
	}

	return total, nil
}

// Helper methods

func (c *WireGuardClient) runCommand(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	// Prepend sudo for wg commands to run with elevated privileges
	if name == "wg" {
		sudoArgs := append([]string{"wg"}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func (c *WireGuardClient) runCommandWithInput(name, input string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	// Prepend sudo for wg commands and use pipe for stdin
	if name == "wg" {
		// Use bash -c to properly handle stdin with sudo
		bashCmd := fmt.Sprintf("wg %s", strings.Join(args, " "))
		cmd = exec.CommandContext(ctx, "bash", "-c", bashCmd)
		cmd.Stdin = strings.NewReader(input)
	} else {
		cmd = exec.CommandContext(ctx, name, args...)
		cmd.Stdin = strings.NewReader(input)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %w, stderr: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

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
