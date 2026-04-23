package vpn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestTrustTunnel(t *testing.T) (*TrustTunnelClient, func()) {
	tmpDir, err := os.MkdirTemp("", "trusttunnel-test")
	require.NoError(t, err)

	credsPath := filepath.Join(tmpDir, "credentials.toml")
	err = os.WriteFile(credsPath, []byte(""), 0600)
	require.NoError(t, err)

	rulesPath := filepath.Join(tmpDir, "rules.toml")
	err = os.WriteFile(rulesPath, []byte(""), 0644)
	require.NoError(t, err)

	client := &TrustTunnelClient{
		binaryPath:  "/usr/bin/trusttunnel_endpoint",
		configDir:   tmpDir,
		domain:      "test.duckdns.org",
		port:        8443,
		publicPort:  443,
		credsPath:   credsPath,
		rulesPath:   rulesPath,
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return client, cleanup
}

func TestTrustTunnelClient_CreateClient_Success(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	err := client.CreateClient("user1", "secure_password_1")
	assert.NoError(t, err)

	content, err := os.ReadFile(client.credsPath)
	assert.NoError(t, err)
	// go-toml/v2 uses double quotes after string replacement
	assert.Contains(t, string(content), `[[client]]`)
	assert.Contains(t, string(content), `username = "user1"`)
	assert.Contains(t, string(content), `password = "secure_password_1"`)
}

func TestTrustTunnelClient_CreateClient_Duplicate(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	err := client.CreateClient("user1", "password1")
	require.NoError(t, err)

	err = client.CreateClient("user1", "password2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestTrustTunnelClient_DeleteClient_Success(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	_ = client.CreateClient("user1", "password1")
	err := client.DeleteClient("user1")
	assert.NoError(t, err)

	content, _ := os.ReadFile(client.credsPath)
	assert.NotContains(t, string(content), "user1")
}

func TestTrustTunnelClient_DeleteClient_NotFound(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	err := client.DeleteClient("nonexistent")
	assert.NoError(t, err)
}

// Test removed - TrustTunnel CLI does not provide native user list command
// Users can only be created and used, not listed via CLI

func TestTrustTunnelClient_ValidateUsername(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	assert.True(t, client.ValidateUsername("user1"))
	assert.True(t, client.ValidateUsername("my-user"))
	assert.True(t, client.ValidateUsername("my_user"))
	assert.True(t, client.ValidateUsername("User123"))

	assert.False(t, client.ValidateUsername("user@1"))
	assert.False(t, client.ValidateUsername("user name"))
	assert.False(t, client.ValidateUsername("us"))
	assert.False(t, client.ValidateUsername(""))
}

func TestTrustTunnelClient_AddRule(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	err := client.AddRule("192.168.1.0/24", "", "deny")
	assert.NoError(t, err)

	content, _ := os.ReadFile(client.rulesPath)
	assert.Contains(t, string(content), `[[rule]]`)
	// go-toml/v2 uses single quotes by default
	assert.Contains(t, string(content), `cidr = '192.168.1.0/24'`)
	assert.Contains(t, string(content), `action = 'deny'`)
}

func TestTrustTunnelClient_RemoveRule(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	_ = client.AddRule("192.168.1.0/24", "", "deny")
	err := client.RemoveRule("192.168.1.0/24", "")
	assert.NoError(t, err)

	content, _ := os.ReadFile(client.rulesPath)
	assert.NotContains(t, string(content), "192.168.1.0/24")
}

func TestTrustTunnelClient_RemoveRule_NotFound(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	err := client.RemoveRule("nonexistent", "")
	assert.NoError(t, err)
}

func TestTrustTunnelClient_ExportClientDeeplink_NotFound(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	// Without the binary, this will fail — verify error handling for non-existent client
	_, err := client.ExportClientDeeplink("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client not found")
}

func TestTrustTunnelClient_ExportClientDeeplink_BinaryMissing(t *testing.T) {
	client, cleanup := setupTestTrustTunnel(t)
	defer cleanup()

	// Create a valid client first
	err := client.CreateClient("testuser", "password123")
	require.NoError(t, err)

	// Without the actual binary, export should fail with appropriate error
	_, err = client.ExportClientDeeplink("testuser")
	assert.Error(t, err)
	// Error should mention binary failure or command execution issue
	assert.NotEmpty(t, err.Error())
}
