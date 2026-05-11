package vpn

import (
	"os"
	"strings"
	"testing"
)

func TestValidateKeyName(t *testing.T) {
	client := &WireGuardClient{}

	tests := []struct {
		name  string
		valid bool
	}{
		{"valid-name", true},
		{"Valid Name", true},
		{"valid_name", true},
		{"Name123", true},
		{"a", false},
		{"ab", false},
		{"", false},
	}

	for _, tt := range tests {
		result := client.ValidateKeyName(tt.name)
		if result != tt.valid {
			t.Errorf("ValidateKeyName(%q) = %v, want %v", tt.name, result, tt.valid)
		}
	}
}

func TestValidateKeyNameLongValid(t *testing.T) {
	client := &WireGuardClient{}
	long := strings.Repeat("a", 50)
	if !client.ValidateKeyName(long) {
		t.Errorf("expected valid for 50-char name")
	}
}

func TestValidateKeyNameTooLong(t *testing.T) {
	client := &WireGuardClient{}
	long := strings.Repeat("a", 51)
	if client.ValidateKeyName(long) {
		t.Errorf("expected invalid for 51-char name")
	}
}

func TestValidateKeyNameSpecialChars(t *testing.T) {
	client := &WireGuardClient{}
	invalidNames := []string{
		"name!",
		"name@",
		"name#",
		"name$",
		"name%",
		"name^",
		"name&",
		"name*",
		"name+",
		"name=",
		"name;",
		"name:",
		"name<",
		"name>",
		"name/",
		"name\\",
		"name|",
		"name~",
		"name`",
		"emoji_😀",
		"unicode_ñ",
		"name.",
		"name,",
	}

	for _, n := range invalidNames {
		if client.ValidateKeyName(n) {
			t.Errorf("expected invalid for name %q", n)
		}
	}
}

func TestReadNetworkConfig(t *testing.T) {
	content := `[Interface]
PrivateKey = someprivatekey
Address = 10.88.88.1/24
MTU = 1420
ListenPort = 51820

[Peer]
PublicKey = somepublickey
AllowedIPs = 10.88.88.2/32
`

	tmpFile, err := os.CreateTemp(t.TempDir(), "wg*.conf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	networkCIDR, startIP, endIP, err := readNetworkConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if networkCIDR != "10.88.88.1/24" {
		t.Errorf("expected networkCIDR '10.88.88.1/24', got '%s'", networkCIDR)
	}
	if startIP != 2 {
		t.Errorf("expected startIP 2, got %d", startIP)
	}
	if endIP != 254 {
		t.Errorf("expected endIP 254, got %d", endIP)
	}
}

func TestReadNetworkConfigNotFound(t *testing.T) {
	_, _, _, err := readNetworkConfig("/nonexistent/path/wg.conf")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadNetworkConfigNoAddress(t *testing.T) {
	content := `[Interface]
PrivateKey = somekey
MTU = 1420
`
	tmpFile, err := os.CreateTemp(t.TempDir(), "wg*.conf")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	_, _, _, err = readNetworkConfig(tmpFile.Name())
	if err == nil {
		t.Error("expected error for config without Address")
	}
}

func TestBuildIP(t *testing.T) {
	tests := []struct {
		cidr      string
		lastOctet int
		expected  string
	}{
		{"10.88.88.0/24", 2, "10.88.88.2"},
		{"10.88.88.0/24", 254, "10.88.88.254"},
		{"192.168.1.0/24", 10, "192.168.1.10"},
		{"10.0.0.0/16", 5, "10.0.0.5"},
	}

	for _, tt := range tests {
		client := &WireGuardClient{networkCIDR: tt.cidr}
		result := client.buildIP(tt.lastOctet)
		if result != tt.expected {
			t.Errorf("buildIP(%q, %d) = %q, want %q", tt.cidr, tt.lastOctet, result, tt.expected)
		}
	}
}

func TestBuildIPInvalidCIDR(t *testing.T) {
	client := &WireGuardClient{networkCIDR: "invalid"}
	result := client.buildIP(42)
	if result != "10.88.88.42" {
		t.Errorf("expected fallback '10.88.88.42', got '%s'", result)
	}
}

func TestGenerateClientConfig(t *testing.T) {
	client := &WireGuardClient{
		serverIP:   "165.140.241.96",
		serverPort: 64465,
		clientDNS:  "1.1.1.1",
	}

	config := client.generateClientConfig(
		"privateKey123",
		"10.88.88.2",
		"serverPublicKey456",
		"presharedKey789",
	)

	expectedFields := []string{
		"PrivateKey = privateKey123",
		"Address = 10.88.88.2/32",
		"DNS = 1.1.1.1",
		"PublicKey = serverPublicKey456",
		"PresharedKey = presharedKey789",
		"Endpoint = 165.140.241.96:64465",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 15",
	}

	for _, field := range expectedFields {
		if !strings.Contains(config, field) {
			t.Errorf("expected config to contain %q", field)
		}
	}

	if !strings.HasPrefix(config, "[Interface]") {
		t.Error("expected config to start with [Interface]")
	}
	if !strings.Contains(config, "[Peer]") {
		t.Error("expected config to contain [Peer] section")
	}
}

func TestGeneratePrivateKey(t *testing.T) {
	client := &WireGuardClient{validateKeys: false}
	key, err := client.generatePrivateKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.String() == "" {
		t.Error("expected non-empty key string")
	}
}
