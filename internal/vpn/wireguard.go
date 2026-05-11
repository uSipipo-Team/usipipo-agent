package vpn

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/uSipipo-Team/usipipo-agent/internal/utils/crypto"
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
	ipAllocClient    *IPAllocationClient
	logger           *log.Logger
	ipAllocationLock sync.Mutex
	lockFilePath     string
	validateKeys     bool
}

// WireGuardPeer represents a WireGuard peer
type WireGuardPeer struct {
	PublicKey string `json:"public_key"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Config    string `json:"config"`
}

// NewWireGuardClient creates a new WireGuard client using wgctrl.
// Reads the server network CIDR from the wg0.conf Address field.
func NewWireGuardClient(interfaceName, configPath, serverIP string, serverPort int, clientDNS string, validateKeys bool) (*WireGuardClient, error) {
	// Read network CIDR from wg0.conf Address field
	networkCIDR, startIP, endIP, err := readNetworkConfig(configPath)
	if err != nil {
		// Fallback to defaults if config can't be read
		networkCIDR = "10.88.88.0/24"
		startIP = 2
		endIP = 254
	}
	return NewWireGuardClientWithRange(interfaceName, configPath, serverIP, serverPort, clientDNS, networkCIDR, startIP, endIP, validateKeys)
}

// readNetworkConfig parses the wg0.conf Address field to extract the network CIDR and IP range.
// Returns the network CIDR, start IP (first client IP after server), and end IP (last host).
func readNetworkConfig(configPath string) (string, int, int, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", 0, 0, err
	}

	// Match Address line, e.g. "Address = 10.88.88.1/24" or "Address = 10.88.88.1/24,fd42:42:42::1/64"
	addrPattern := regexp.MustCompile(`Address\s*=\s*([\d.]+)/(\d+)`)
	matches := addrPattern.FindStringSubmatch(string(content))
	if len(matches) < 3 {
		return "", 0, 0, fmt.Errorf("no Address found in config")
	}

	serverIP := matches[1]
	prefixLen, _ := strconv.Atoi(matches[2])

	// Parse the server IP directly (don't use ParseCIDR - it normalizes to network address)
	ip := net.ParseIP(serverIP).To4()
	if ip == nil {
		return "", 0, 0, fmt.Errorf("invalid IP address: %s", serverIP)
	}

	// Reconstruct network CIDR string
	networkCIDR := fmt.Sprintf("%d.%d.%d.%d/%d", ip[0], ip[1], ip[2], ip[3], prefixLen)

	// Start at server IP + 1 (skip server's own IP), end at .254
	startIP := int(ip[3]) + 1
	endIP := 254

	return networkCIDR, startIP, endIP, nil
}

// NewWireGuardClientWithRange creates a new WireGuard client with custom IP range
func NewWireGuardClientWithRange(interfaceName, configPath, serverIP string, serverPort int, clientDNS, networkCIDR string, startIP, endIP int, validateKeys bool) (*WireGuardClient, error) {
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
		validateKeys:  validateKeys,
	}, nil
}

// Close closes the wgctrl client
func (c *WireGuardClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// SetIPAllocationClient sets the IP allocation client for DB-based IP management
func (c *WireGuardClient) SetIPAllocationClient(ipAllocClient *IPAllocationClient, logger *log.Logger) {
	c.ipAllocClient = ipAllocClient
	if logger != nil {
		c.logger = logger
	}
}

// SetLockFilePath sets the lock file path for cross-process locking
func (c *WireGuardClient) SetLockFilePath(path string) {
	c.lockFilePath = path
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

// CreatePeer creates a new WireGuard peer using DB-based IP allocation.
// Falls back to legacy IP allocation if IPAllocationClient is not configured.
func (c *WireGuardClient) CreatePeer(ctx context.Context, name string) (*WireGuardPeer, error) {
	// Validate key name first (defense in depth)
	if !c.ValidateKeyName(name) {
		return nil, fmt.Errorf("invalid VPN key name: must be %d-%d characters, alphanumeric, spaces, hyphens, or underscores only", VPNKeyNameMinLength, VPNKeyNameMaxLength)
	}

	// DB-first flow if IPAllocationClient is configured
	if c.ipAllocClient != nil {
		return c.createPeerDBFirst(ctx, name)
	}

	// Fallback to legacy flow
	return c.CreatePeerLegacy(ctx, name)
}

// createPeerDBFirst creates a new WireGuard peer with DB-based IP allocation
func (c *WireGuardClient) createPeerDBFirst(ctx context.Context, name string) (*WireGuardPeer, error) {
	c.logInfo("create_peer_started", "name", name)

	// Phase 0: Acquire cross-process lock to prevent concurrent allocations
	var lockFile *os.File
	var lockReleased bool
	if c.lockFilePath != "" {
		lf, err := os.OpenFile(c.lockFilePath, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to open lock file: %w", err)
		}
		lockFile = lf
		lockTimeout := 10 * time.Second
		if err := acquireLockWithTimeout(lockFile, lockTimeout); err != nil {
			lockFile.Close()
			return nil, fmt.Errorf("failed to acquire allocation lock: %w", err)
		}
		// Defer release of cross-process lock
		defer func() {
			if !lockReleased && lockFile != nil {
				releaseLock(lockFile)
				lockFile.Close()
			}
		}()
	}

	// Phase 1: Reserve IP from DB
	reserveResp, err := c.ipAllocClient.ReserveIP(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve IP: %w", err)
	}
	c.logInfo("ip_reserved", "name", name, "ip", reserveResp.IPAddress, "lease_id", reserveResp.LeaseID)

	peerIP := net.ParseIP(reserveResp.IPAddress)
	if peerIP == nil {
		// Compensation: release reserved IP
		c.ipAllocClient.ReleaseIP(ctx, reserveResp.LeaseID, "invalid_ip")
		return nil, fmt.Errorf("invalid IP address from reservation: %s", reserveResp.IPAddress)
	}

	// Phase 2: Generate keys with entropy validation
	privateKey, err := c.generatePrivateKey()
	if err != nil {
		// Compensation: release reserved IP
		c.ipAllocClient.ReleaseIP(ctx, reserveResp.LeaseID, "keygen_failed")
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	publicKey := privateKey.PublicKey()

	psk, err := wgtypes.GenerateKey()
	if err != nil {
		// Compensation: release reserved IP
		c.ipAllocClient.ReleaseIP(ctx, reserveResp.LeaseID, "keygen_failed")
		return nil, fmt.Errorf("failed to generate preshared key: %w", err)
	}
	c.logInfo("keys_generated", "name", name, "public_key", publicKey.String()[:16]+"...")

	// Phase 3: Configure kernel
	peerConfig := wgtypes.PeerConfig{
		PublicKey:         publicKey,
		PresharedKey:      &psk,
		AllowedIPs:        []net.IPNet{{IP: peerIP, Mask: net.CIDRMask(32, 32)}},
		ReplaceAllowedIPs: false,
	}
	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerConfig},
	}

	if err := c.client.ConfigureDevice(c.interfaceName, config); err != nil {
		// Compensation: release reserved IP
		c.ipAllocClient.ReleaseIP(ctx, reserveResp.LeaseID, "kernel_failed")
		return nil, fmt.Errorf("failed to configure kernel device: %w", err)
	}
	c.logInfo("kernel_configured", "name", name, "interface", c.interfaceName)

	// Phase 4: Get server public key and generate client config
	device, err := c.client.Device(c.interfaceName)
	if err != nil {
		// Compensation: release reserved IP
		c.ipAllocClient.ReleaseIP(ctx, reserveResp.LeaseID, "config_failed")
		return nil, fmt.Errorf("failed to get device info: %w", err)
	}

	clientConfig := c.generateClientConfig(privateKey.String(), reserveResp.IPAddress, device.PublicKey.String(), psk.String())

	// Phase 5: Write config file (atomic)
	if err := c.writePeerConfigAtomic(name, clientConfig); err != nil {
		// Compensation: release reserved IP
		c.ipAllocClient.ReleaseIP(ctx, reserveResp.LeaseID, "config_failed")
		return nil, fmt.Errorf("failed to write peer config: %w", err)
	}
	c.logInfo("config_written", "name", name, "config_path", c.configPath)

	// Phase 6: Confirm allocation in DB
	if err := c.ipAllocClient.ConfirmAllocation(ctx, reserveResp.LeaseID, reserveResp.IPAddress, publicKey.String()); err != nil {
		// Non-fatal: reconciler can fix later
		c.logWarn("confirm_allocation_failed", "name", name, "lease_id", reserveResp.LeaseID, "error", err)
	} else {
		c.logInfo("allocation_confirmed", "name", name, "lease_id", reserveResp.LeaseID)
	}

	// Release cross-process lock
	if lockFile != nil {
		lockReleased = true
		releaseLock(lockFile)
		lockFile.Close()
	}

	return &WireGuardPeer{
		PublicKey: publicKey.String(),
		Name:      name,
		IPAddress: reserveResp.IPAddress,
		Config:    clientConfig,
	}, nil
}

// writePeerConfigAtomic writes peer configuration atomically using temp file and rename
func (c *WireGuardClient) writePeerConfigAtomic(name, config string) error {
	// Determine the directory containing the config file
	dir := filepath.Dir(c.configPath)
	if dir == "" {
		dir = "."
	}

	// Create temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, ".wg-"+name+".tmp")
	if err != nil {
		// Fallback: direct write
		return c.writePeerConfig(name, config)
	}
	tempPath := tmpFile.Name()
	defer os.Remove(tempPath)

	if _, err := tmpFile.WriteString(config); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp config: %w", err)
	}
	tmpFile.Close()

	// Create backup before replacing
	backupPath := c.configPath + ".backup"
	if _, err := os.Stat(c.configPath); err == nil {
		if err := copyFile(c.configPath, backupPath); err != nil {
			c.logWarn("backup_failed", "error", err)
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, c.configPath); err != nil {
		// Try fallback restore from backup
		if _, err := os.Stat(backupPath); err == nil {
			copyFile(backupPath, c.configPath)
		}
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	return nil
}

// writePeerConfig writes peer configuration to config file (legacy non-atomic)
func (c *WireGuardClient) writePeerConfig(name, config string) error {
	f, err := os.OpenFile(c.configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(config)
	return err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// logInfo logs an info message with key-value pairs
func (c *WireGuardClient) logInfo(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Print("INFO: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				c.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
}

// logWarn logs a warning message with key-value pairs
func (c *WireGuardClient) logWarn(msg string, keysAndValues ...interface{}) {
	if c.logger != nil {
		c.logger.Print("WARN: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				c.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
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

// CreatePeerLegacy creates a new WireGuard peer using legacy file-based IP allocation
// This is the original CreatePeer implementation kept for backward compatibility
func (c *WireGuardClient) CreatePeerLegacy(ctx context.Context, name string) (*WireGuardPeer, error) {
	// Generate private key with optional entropy validation
	privateKey, err := c.generatePrivateKey()
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
	defer func() { _ = lockFile.Close() }()

	// Acquire exclusive lock with timeout (5 seconds)
	// On Windows, this is a no-op (mutex provides synchronization)
	// On Unix, uses flock for cross-process locking
	lockTimeout := 5 * time.Second
	if err := acquireLockWithTimeout(lockFile, lockTimeout); err != nil {
		return "", fmt.Errorf("failed to acquire IP allocation lock: %w", err)
	}

	// Ensure lock is released when function returns
	defer releaseLock(lockFile)

	// Now safe to read config and find available IP
	content, err := os.ReadFile(c.configPath)
	if err != nil {
		// If config doesn't exist, return first IP in range
		if os.IsNotExist(err) {
			return c.buildIP(c.startIP), nil
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
		ip := c.buildIP(i)
		if !usedIPs[ip] {
			return ip, nil
		}
	}

	return "", fmt.Errorf("no available IPs in range %d-%d", c.startIP, c.endIP)
}

func (c *WireGuardClient) buildIP(lastOctet int) string {
	// Parse network CIDR to get base IP
	_, ipNet, err := net.ParseCIDR(c.networkCIDR)
	if err != nil {
		// Fallback
		return "10.88.88." + strconv.Itoa(lastOctet)
	}
	base := ipNet.IP.To4()
	if base == nil {
		return "10.88.88." + strconv.Itoa(lastOctet)
	}
	return fmt.Sprintf("%d.%d.%d.%d", base[0], base[1], base[2], lastOctet)
}

// generatePrivateKey generates a WireGuard private key with optional entropy validation
// If validateKeys is true, it will retry up to 3 times to ensure sufficient entropy
func (c *WireGuardClient) generatePrivateKey() (wgtypes.Key, error) {
	var privateKey wgtypes.Key
	var err error
	maxAttempts := 3

	if c.validateKeys {
		// Try to generate a validated key
		keyHex := ""
		for attempt := 0; attempt < maxAttempts; attempt++ {
			privateKey, err = wgtypes.GeneratePrivateKey()
			if err != nil {
				return wgtypes.Key{}, fmt.Errorf("failed to generate private key: %w", err)
			}

			keyHex = privateKey.String()
			if crypto.ValidateKeyEntropy(keyHex) {
				// Good entropy, return
				if c.logger != nil {
					c.logger.Printf("INFO: Private key entropy validated (attempt %d/%d)", attempt+1, maxAttempts)
				}
				return privateKey, nil
			}

			// Entropy validation failed
			if c.logger != nil {
				c.logger.Printf("WARN: Private key failed entropy validation (attempt %d/%d), retrying...", attempt+1, maxAttempts)
			}
		}

		// All attempts failed entropy validation - MUST fail, don't proceed with weak key
		if c.logger != nil {
			c.logger.Printf("ERROR: Private key entropy validation failed after %d attempts, CANNOT proceed", maxAttempts)
		}
		return wgtypes.Key{}, fmt.Errorf("failed to generate high-entropy private key after %d attempts", maxAttempts)
	}

	// Validation disabled - just generate once
	privateKey, err = wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("failed to generate private key: %w", err)
	}
	return privateKey, nil
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
