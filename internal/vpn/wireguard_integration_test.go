//go:build integration
// +build integration

package vpn

import (
	"os"
	"strings"
	"testing"
)

// TestWireGuardGenKey tests wg genkey command with sudo
func TestWireGuardGenKey(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("WIREGUARD_TEST_INTERFACE") == "" {
		t.Skip("WIREGUARD_TEST_INTERFACE not set, skipping integration test")
	}

	client := NewWireGuardClient(
		os.Getenv("WIREGUARD_TEST_INTERFACE"),
		"/etc/wireguard",
		"localhost",
		51820,
		"1.1.1.1",
	)

	// Test genkey
	key, err := client.runCommand("wg", "genkey")
	if err != nil {
		t.Fatalf("wg genkey failed: %v", err)
	}

	if key == "" {
		t.Fatal("wg genkey returned empty key")
	}

	// Verify key format (base64, 44 characters)
	if len(key) != 44 {
		t.Fatalf("wg genkey returned key with invalid length: %d, expected 44", len(key))
	}

	t.Logf("Generated key: %s", key)
}

// TestWireGuardPubkey tests wg pubkey command with sudo
func TestWireGuardPubkey(t *testing.T) {
	if os.Getenv("WIREGUARD_TEST_INTERFACE") == "" {
		t.Skip("WIREGUARD_TEST_INTERFACE not set, skipping integration test")
	}

	// Known private key for testing (from WireGuard documentation)
	privateKey := "wG69hNKhK1GZ7LzFZvKzNzFZvKzNzFZvKzNzFZvKzNk="

	client := NewWireGuardClient("wg0", "/etc/wireguard", "localhost", 51820, "1.1.1.1")

	pubkey, err := client.runCommandWithInput("wg", privateKey, "pubkey")
	if err != nil {
		t.Fatalf("wg pubkey failed: %v", err)
	}

	if pubkey == "" {
		t.Fatal("wg pubkey returned empty key")
	}

	// Verify key format (base64, 44 characters)
	if len(pubkey) != 44 {
		t.Fatalf("wg pubkey returned key with invalid length: %d, expected 44", len(pubkey))
	}

	t.Logf("Public key: %s", pubkey)
}

// TestWireGuardShow tests wg show command with sudo
func TestWireGuardShow(t *testing.T) {
	if os.Getenv("WIREGUARD_TEST_INTERFACE") == "" {
		t.Skip("WIREGUARD_TEST_INTERFACE not set, skipping integration test")
	}

	iface := os.Getenv("WIREGUARD_TEST_INTERFACE")
	client := NewWireGuardClient(iface, "/etc/wireguard", "localhost", 51820, "1.1.1.1")

	// Test wg show
	output, err := client.runCommand("wg", "show", iface)
	if err != nil {
		t.Fatalf("wg show failed: %v", err)
	}

	if output == "" {
		t.Fatal("wg show returned empty output")
	}

	// Verify output contains interface name
	if !strings.Contains(output, "interface") {
		t.Fatal("wg show output doesn't contain 'interface'")
	}

	t.Logf("wg show output:\n%s", output)
}

// TestWireGuardShowDump tests wg show dump command with sudo
func TestWireGuardShowDump(t *testing.T) {
	if os.Getenv("WIREGUARD_TEST_INTERFACE") == "" {
		t.Skip("WIREGUARD_TEST_INTERFACE not set, skipping integration test")
	}

	iface := os.Getenv("WIREGUARD_TEST_INTERFACE")
	client := NewWireGuardClient(iface, "/etc/wireguard", "localhost", 51820, "1.1.1.1")

	// Test wg show dump
	output, err := client.runCommand("wg", "show", iface, "dump")
	if err != nil {
		t.Fatalf("wg show dump failed: %v", err)
	}

	if output == "" {
		t.Fatal("wg show dump returned empty output")
	}

	t.Logf("wg show dump output:\n%s", output)
}
