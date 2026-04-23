package vpn

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

// TrustTunnelClient handles TrustTunnel client management
type TrustTunnelClient struct {
	binaryPath  string
	configDir   string
	domain      string
	port        int
	publicPort  int
	credsPath   string
	rulesPath   string
	fileLock    sync.Mutex
}

// CredentialsFile represents the credentials.toml structure
type CredentialsFile struct {
	Client []ClientCredential `toml:"client"`
}

// ClientCredential represents a single [[client]] entry
type ClientCredential struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// RulesFile represents the rules.toml structure
type RulesFile struct {
	Rule []AccessRule `toml:"rule"`
}

// AccessRule represents a single [[rule]] entry
type AccessRule struct {
	CIDR               string `toml:"cidr,omitempty"`
	ClientRandomPrefix string `toml:"client_random_prefix,omitempty"`
	Action             string `toml:"action"`
}

// NewTrustTunnelClient creates a new TrustTunnel client
func NewTrustTunnelClient(binaryPath, configDir, domain string, port, publicPort int) *TrustTunnelClient {
	return &TrustTunnelClient{
		binaryPath:  binaryPath,
		configDir:   configDir,
		domain:      domain,
		port:        port,
		publicPort:  publicPort,
		credsPath:   fmt.Sprintf("%s/credentials.toml", configDir),
		rulesPath:   fmt.Sprintf("%s/rules.toml", configDir),
	}
}

// ValidateUsername checks if username is valid (3-50 chars, alphanumeric, hyphens, underscores)
func (c *TrustTunnelClient) ValidateUsername(name string) bool {
	if len(name) < 3 || len(name) > 50 {
		return false
	}

	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// CreateClient adds a new client to credentials.toml
func (c *TrustTunnelClient) CreateClient(username, password string) error {
	if !c.ValidateUsername(username) {
		return fmt.Errorf("invalid username: must be 3-50 characters, alphanumeric, hyphens, or underscores only")
	}

	if len(password) < 8 {
		return fmt.Errorf("password too short: must be at least 8 characters")
	}

	c.fileLock.Lock()
	defer c.fileLock.Unlock()

	clients, err := c.readClients()
	if err != nil {
		return err
	}

	for _, client := range clients {
		if client.Username == username {
			return fmt.Errorf("client already exists: %s", username)
		}
	}

	clients = append(clients, ClientCredential{
		Username: username,
		Password: password,
	})

	return c.writeClients(clients)
}

// DeleteClient removes a client from credentials.toml (idempotent)
func (c *TrustTunnelClient) DeleteClient(username string) error {
	c.fileLock.Lock()
	defer c.fileLock.Unlock()

	clients, err := c.readClients()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]ClientCredential, 0, len(clients))
	for _, client := range clients {
		if client.Username == username {
			found = true
			continue
		}
		filtered = append(filtered, client)
	}

	if !found {
		return nil
	}

	return c.writeClients(filtered)
}

// ListClients returns all client usernames
func (c *TrustTunnelClient) ListClients() ([]string, error) {
	c.fileLock.Lock()
	defer c.fileLock.Unlock()

	clients, err := c.readClients()
	if err != nil {
		return nil, err
	}

	usernames := make([]string, len(clients))
	for i, client := range clients {
		usernames[i] = client.Username
	}

	return usernames, nil
}

// ExportClientConfig runs the CLI to export a client configuration
func (c *TrustTunnelClient) ExportClientConfig(username string) (string, error) {
	clients, err := c.ListClients()
	if err != nil {
		return "", err
	}

	found := false
	for _, name := range clients {
		if name == username {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("client not found: %s", username)
	}

	addr := fmt.Sprintf("%s:%d", c.domain, c.port)
	cmd := exec.Command(c.binaryPath,
		fmt.Sprintf("%s/vpn.toml", c.configDir),
		fmt.Sprintf("%s/hosts.toml", c.configDir),
		"-c", username,
		"-a", addr,
		"-f", "toml",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to export client config: %w, stderr: %s", err, string(output))
	}

	return string(output), nil
}

// ExportClientDeeplink generates a deep-link (tt://) URI for mobile client configuration
func (c *TrustTunnelClient) ExportClientDeeplink(username string) (string, error) {
	clients, err := c.ListClients()
	if err != nil {
		return "", err
	}

	found := false
	for _, name := range clients {
		if name == username {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("client not found: %s", username)
	}

	addr := fmt.Sprintf("%s:%d", c.domain, c.publicPort)
	cmd := exec.Command(c.binaryPath,
		fmt.Sprintf("%s/vpn.toml", c.configDir),
		fmt.Sprintf("%s/hosts.toml", c.configDir),
		"-c", username,
		"-a", addr,
		"-f", "deeplink",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to export deeplink: %w, stderr: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// AddRule adds an access rule to rules.toml
func (c *TrustTunnelClient) AddRule(cidr, prefix, action string) error {
	c.fileLock.Lock()
	defer c.fileLock.Unlock()

	rules, err := c.readRules()
	if err != nil {
		if os.IsNotExist(err) {
			rules = []AccessRule{}
		} else {
			return err
		}
	}

	rule := AccessRule{Action: action}
	if cidr != "" {
		rule.CIDR = cidr
	}
	if prefix != "" {
		rule.ClientRandomPrefix = prefix
	}

	rules = append(rules, rule)
	return c.writeRules(rules)
}

// RemoveRule removes an access rule from rules.toml
func (c *TrustTunnelClient) RemoveRule(cidr, prefix string) error {
	c.fileLock.Lock()
	defer c.fileLock.Unlock()

	rules, err := c.readRules()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	found := false
	filtered := make([]AccessRule, 0, len(rules))
	for _, rule := range rules {
		if rule.CIDR == cidr && rule.ClientRandomPrefix == prefix {
			found = true
			continue
		}
		filtered = append(filtered, rule)
	}

	if !found {
		return nil
	}

	return c.writeRules(filtered)
}

// Reload sends SIGHUP to TrustTunnel process to reload TLS hosts
func (c *TrustTunnelClient) Reload() error {
	cmd := exec.Command("pgrep", "trusttunnel_endpoint")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("TrustTunnel process not found: %w", err)
	}

	pid := strings.TrimSpace(string(output))
	if pid == "" {
		return fmt.Errorf("TrustTunnel process not running")
	}

	killCmd := exec.Command("kill", "-HUP", pid)
	if err := killCmd.Run(); err != nil {
		return fmt.Errorf("failed to send SIGHUP: %w", err)
	}

	return nil
}

// Helper methods

func (c *TrustTunnelClient) readClients() ([]ClientCredential, error) {
	content, err := os.ReadFile(c.credsPath)
	if err != nil {
		return nil, err
	}

	var creds CredentialsFile
	if err := toml.Unmarshal(content, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse credentials.toml: %w", err)
	}

	if creds.Client == nil {
		return []ClientCredential{}, nil
	}

	return creds.Client, nil
}

func (c *TrustTunnelClient) writeClients(clients []ClientCredential) error {
	creds := CredentialsFile{Client: clients}

	// Use a more targeted approach: build TOML manually with double quotes
	// This avoids the issues with go-toml/v2 using single quotes and
	// the naive string replacement causing parsing issues
	var sb strings.Builder
	for i, client := range clients {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("[[client]]\n")
		sb.WriteString(fmt.Sprintf("username = \"%s\"\n", client.Username))
		sb.WriteString(fmt.Sprintf("password = \"%s\"", client.Password))
	}
	sb.WriteString("\n")

	tmpPath := c.credsPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, c.credsPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

func (c *TrustTunnelClient) readRules() ([]AccessRule, error) {
	content, err := os.ReadFile(c.rulesPath)
	if err != nil {
		return nil, err
	}

	var rules RulesFile
	if err := toml.Unmarshal(content, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse rules.toml: %w", err)
	}

	if rules.Rule == nil {
		return []AccessRule{}, nil
	}

	return rules.Rule, nil
}

func (c *TrustTunnelClient) writeRules(rules []AccessRule) error {
	rulesFile := RulesFile{Rule: rules}

	content, err := toml.Marshal(rulesFile)
	if err != nil {
		return fmt.Errorf("failed to marshal rules.toml: %w", err)
	}

	tmpPath := c.rulesPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, c.rulesPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}
