package registrar

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/uSipipo-Team/usipipo-agent/internal/config"
	"github.com/uSipipo-Team/usipipo-agent/internal/utils/geoip"
)

// Registrar handles agent registration with backend
type Registrar struct {
	backendURL string
	apiKey     string
	serverID   string
	client     *resty.Client
	cfg        *config.Config
}

// RegistrationResponse represents backend response
type RegistrationResponse struct {
	ServerID string `json:"server_id"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

// RegistrationRequest represents registration payload
type RegistrationRequest struct {
	Hostname          string `json:"hostname"`
	IPAddress         string `json:"ip_address"`
	CountryCode       string `json:"country_code"`
	CountryName       string `json:"country_name"`
	Region            string `json:"region,omitempty"`
	City              string `json:"city,omitempty"`
	AgentVersion      string `json:"agent_version"`
	OSType            string `json:"os_type"`
	OSArch            string `json:"os_arch"`
	AgentURL          string `json:"agent_url"`
	SupportsWireGuard bool   `json:"supports_wireguard"`
	AgentAPIKey       string `json:"agent_api_key"`
}

// NewRegistrar creates a new registrar instance from config
func NewRegistrar(cfg *config.Config) *Registrar {
	return &Registrar{
		backendURL: cfg.BackendURL,
		apiKey:     cfg.APIKey,
		serverID:   cfg.ServerID,
		client:     resty.New(),
		cfg:        cfg,
	}
}

// NewRegistrarFromValues creates a new registrar instance from individual values
// This is a convenience function for callers that don't have a Config object
func NewRegistrarFromValues(backendURL, apiKey, serverID string) *Registrar {
	return &Registrar{
		backendURL: backendURL,
		apiKey:     apiKey,
		serverID:   serverID,
		client:     resty.New(),
	}
}

// RegisterOrGetServerID registers agent or returns existing server ID
func (r *Registrar) RegisterOrGetServerID() (string, error) {
	// If server ID already set and valid UUID, use it
	if r.serverID != "" && IsValidUUID(r.serverID) {
		return r.serverID, nil
	}

	// Collect metadata
	metadata, err := r.collectMetadata()
	if err != nil {
		return "", fmt.Errorf("failed to collect metadata: %w", err)
	}

	// Send registration request
	endpoint := r.backendURL + "/api/v1/servers/register-agent"

	resp, err := r.client.R().
		SetHeader("X-API-Key", r.apiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(metadata).
		Post(endpoint)

	if err != nil {
		return "", fmt.Errorf("registration request failed: %w", err)
	}

	if resp.StatusCode() == 409 {
		// Already registered, get existing server ID
		return r.getExistingServerID()
	}

	if resp.StatusCode() != 201 {
		return "", fmt.Errorf("registration failed with status: %d - %s", resp.StatusCode(), resp.String())
	}

	// Parse response
	var result RegistrationResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Save server ID to .env
	if err := saveServerIDToEnv(result.ServerID); err != nil {
		// Log but don't fail - agent can still function
		log.Printf("Warning: Could not save SERVER_ID to .env: %v", err)
	}

	return result.ServerID, nil
}

func (r *Registrar) collectMetadata() (*RegistrationRequest, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get geo location using config
	geo, err := geoip.GetLocation(r.cfg)
	if err != nil {
		// Use defaults if geo lookup fails
		geo = &geoip.GeoIPResponse{
			Query:       "unknown",
			CountryCode: "XX",
			CountryName: "Unknown",
			RegionName:  "Unknown",
			City:        "Unknown",
		}
	}

	return &RegistrationRequest{
		Hostname:          hostname,
		IPAddress:         geo.Query,
		CountryCode:       geo.CountryCode,
		CountryName:       geo.CountryName,
		Region:            geo.RegionName,
		City:              geo.City,
		AgentVersion:      getVersion(),
		OSType:            runtime.GOOS,
		OSArch:            runtime.GOARCH,
		AgentURL:          r.cfg.AgentURL,
		SupportsWireGuard: r.cfg.WireGuardInterface != "",
		AgentAPIKey:       r.apiKey,
	}, nil
}

func (r *Registrar) getExistingServerID() (string, error) {
	endpoint := r.backendURL + "/api/v1/servers/register-agent?api_key=" + r.apiKey

	resp, err := r.client.R().Get(endpoint)
	if err != nil {
		return "", err
	}

	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("failed to get existing registration: %d", resp.StatusCode())
	}

	var result RegistrationResponse
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", err
	}

	return result.ServerID, nil
}

// Helper functions
func IsValidUUID(uuid string) bool {
	// Simple UUID format check
	if len(uuid) != 36 {
		return false
	}
	for i, r := range uuid {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
			continue
		}
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func getVersion() string {
	// Set via ldflags during build: -X main.Version=0.2.0
	return Version
}

var Version = "0.12.0"

func saveServerIDToEnv(serverID string) error {
	envPath := "/opt/usipipo-agent/.env"

	// Read existing .env
	content, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}

	// Update SERVER_ID line
	lines := string(content)
	lines = replaceEnvVar(lines, "SERVER_ID", serverID)

	// Write back
	return os.WriteFile(envPath, []byte(lines), 0600)
}

func replaceEnvVar(content, key, value string) string {
	// Replace or add SERVER_ID line
	lines := strings.Split(content, "\n")
	found := false

	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	return strings.Join(lines, "\n")
}
