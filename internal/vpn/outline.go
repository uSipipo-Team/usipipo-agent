package vpn

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-resty/resty/v2"
)

// OutlineClient handles communication with Outline Manager API
type OutlineClient struct {
	apiURL string
	client *resty.Client
}

// OutlineKey represents an Outline access key
type OutlineKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccessURL string `json:"access_url"`
	Port      int    `json:"port"`
	Method    string `json:"method"`
}

// OutlineServerInfo represents server information from GET /server
type OutlineServerInfo struct {
	Name                  string `json:"name"`
	ServerID              string `json:"serverId"`
	MetricsEnabled        bool   `json:"metricsEnabled"`
	Version               string `json:"version"`
	PortForNewAccessKeys  int    `json:"portForNewAccessKeys"`
	HostnameForAccessKeys string `json:"hostnameForAccessKeys"`
}

// OutlineTransferMetrics represents bandwidth usage per key (last 30 days)
type OutlineTransferMetrics struct {
	BytesTransferredByUserID map[string]uint64 `json:"bytesTransferredByUserId"`
}

// OutlineDetailedMetrics represents time-series metrics from Prometheus
type OutlineDetailedMetrics struct {
	Status string      `json:"status"`
	Data   MetricsData `json:"data"`
}

type MetricsData struct {
	ResultType string         `json:"resultType"`
	Result     []MetricResult `json:"result"`
}

type MetricResult struct {
	Metric MetricInfo      `json:"metric"`
	Values [][]interface{} `json:"values"` // [timestamp, value]
}

type MetricInfo struct {
	AccessKey string `json:"access_key"`
	Name      string `json:"__name__"`
}

// NewOutlineClient creates a new Outline API client
func NewOutlineClient(apiURL string, insecureSkipVerify bool) *OutlineClient {
	client := resty.New()

	// Configure TLS with secure defaults
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12, // Enforce TLS 1.2 minimum
	}

	// Allow insecure skip verify for self-signed certificates (development/testing)
	if insecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	client.SetTLSClientConfig(tlsConfig)
	
	return &OutlineClient{
		apiURL: apiURL,
		client: client,
	}
}

// CreateKey creates a new Outline access key
func (c *OutlineClient) CreateKey(ctx context.Context, name string) (*OutlineKey, error) {
	// Step 1: Create key
	resp, err := c.client.R().
		SetContext(ctx).
		Post(c.apiURL + "/access-keys")

	if err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	if resp.StatusCode() != 201 {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
	}

	var result struct {
		ID        string `json:"id"`
		AccessURL string `json:"accessUrl"`
		Port      int    `json:"port"`
		Method    string `json:"method"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Step 2: Rename key
	_, err = c.client.R().
		SetContext(ctx).
		SetBody(map[string]string{"name": name}).
		Put(c.apiURL + "/access-keys/" + result.ID + "/name")

	if err != nil {
		// Non-fatal, log warning
		log.Printf("[WARNING] failed to rename key: %v\n", err)
	}

	return &OutlineKey{
		ID:        result.ID,
		Name:      name,
		AccessURL: result.AccessURL,
		Port:      result.Port,
		Method:    result.Method,
	}, nil
}

// DeleteKey deletes an Outline access key.
// This operation is idempotent - returns success even if the key doesn't exist (404).
func (c *OutlineClient) DeleteKey(ctx context.Context, keyID string) error {
	resp, err := c.client.R().
		SetContext(ctx).
		Delete(c.apiURL + "/access-keys/" + keyID)

	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	// 204 = deleted successfully, 404 = already deleted (idempotent behavior)
	if resp.StatusCode() == 204 || resp.StatusCode() == 404 {
		return nil
	}

	return fmt.Errorf("unexpected status: %d", resp.StatusCode())
}

// GetKeyUsage returns the data transfer for a specific key
func (c *OutlineClient) GetKeyUsage(ctx context.Context, keyID string) (uint64, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		Get(c.apiURL + "/metrics/transfer")

	if err != nil {
		return 0, fmt.Errorf("failed to get metrics: %w", err)
	}

	var result struct {
		BytesTransferredByUserId map[string]uint64 `json:"bytesTransferredByUserId"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.BytesTransferredByUserId[keyID], nil
}

// ListKeys returns all Outline access keys
func (c *OutlineClient) ListKeys(ctx context.Context) ([]*OutlineKey, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		Get(c.apiURL + "/access-keys")

	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	var result struct {
		AccessKeys []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			AccessURL string `json:"accessUrl"`
			Port      int    `json:"port"`
			Method    string `json:"method"`
		} `json:"accessKeys"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	keys := make([]*OutlineKey, len(result.AccessKeys))
	for i, key := range result.AccessKeys {
		keys[i] = &OutlineKey{
			ID:        key.ID,
			Name:      key.Name,
			AccessURL: key.AccessURL,
			Port:      key.Port,
			Method:    key.Method,
		}
	}

	return keys, nil
}

// GetActiveKeysCount returns the number of active keys
func (c *OutlineClient) GetActiveKeysCount(ctx context.Context) (int, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		Get(c.apiURL + "/access-keys")

	if err != nil {
		return 0, fmt.Errorf("failed to get keys: %w", err)
	}

	var result struct {
		AccessKeys []interface{} `json:"accessKeys"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return len(result.AccessKeys), nil
}

// GetTotalBytesTransferred returns total bytes transferred across all keys
func (c *OutlineClient) GetTotalBytesTransferred(ctx context.Context) (uint64, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		Get(c.apiURL + "/metrics/transfer")

	if err != nil {
		return 0, fmt.Errorf("failed to get metrics: %w", err)
	}

	var result struct {
		BytesTransferredByUserId map[string]uint64 `json:"bytesTransferredByUserId"`
	}

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	var total uint64
	for _, bytes := range result.BytesTransferredByUserId {
		total += bytes
	}

	return total, nil
}

// CheckStatus verifies Outline API connectivity and returns server info
func (c *OutlineClient) CheckStatus(ctx context.Context) (*OutlineServerInfo, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		Get(c.apiURL + "/server")

	if err != nil {
		return nil, fmt.Errorf("Outline API unreachable: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("Outline API returned unexpected status: %d", resp.StatusCode())
	}

	var info OutlineServerInfo
	if err := json.Unmarshal(resp.Body(), &info); err != nil {
		return nil, fmt.Errorf("failed to parse server info: %w", err)
	}

	return &info, nil
}

// GetTransferMetrics retrieves bandwidth usage per key (last 30 days)
func (c *OutlineClient) GetTransferMetrics(ctx context.Context) (*OutlineTransferMetrics, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		Get(c.apiURL + "/metrics/transfer")

	if err != nil {
		return nil, fmt.Errorf("failed to get transfer metrics: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
	}

	var metrics OutlineTransferMetrics
	if err := json.Unmarshal(resp.Body(), &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &metrics, nil
}

// GetDetailedMetrics retrieves time-series metrics for specified period
// Supported since values: 1h, 24h, 7d, 30d
func (c *OutlineClient) GetDetailedMetrics(ctx context.Context, since string) (*OutlineDetailedMetrics, error) {
	resp, err := c.client.R().
		SetContext(ctx).
		SetQueryParam("since", since).
		Get(c.apiURL + "/experimental/server/metrics")

	if err != nil {
		return nil, fmt.Errorf("failed to get detailed metrics: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode())
	}

	var metrics OutlineDetailedMetrics
	if err := json.Unmarshal(resp.Body(), &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &metrics, nil
}
