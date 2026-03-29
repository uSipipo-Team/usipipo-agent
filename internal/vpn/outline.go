package vpn

import (
	"context"
	"fmt"

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

// NewOutlineClient creates a new Outline API client
func NewOutlineClient(apiURL string) *OutlineClient {
	return &OutlineClient{
		apiURL: apiURL,
		client: resty.New(),
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

	if err := resp.JSONInto(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Step 2: Rename key
	_, err = c.client.R().
		SetContext(ctx).
		SetBody(map[string]string{"name": name}).
		Put(fmt.Sprintf("%s/access-keys/%s/name", c.apiURL, result.ID))

	if err != nil {
		// Non-fatal, log warning
		fmt.Printf("Warning: failed to rename key: %v\n", err)
	}

	return &OutlineKey{
		ID:        result.ID,
		Name:      name,
		AccessURL: result.AccessURL,
		Port:      result.Port,
		Method:    result.Method,
	}, nil
}

// DeleteKey deletes an Outline access key
func (c *OutlineClient) DeleteKey(ctx context.Context, keyID string) error {
	resp, err := c.client.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("%s/access-keys/%s", c.apiURL, keyID))

	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	// 404 means already deleted
	if resp.StatusCode() == 404 || resp.StatusCode() == 204 {
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

	if err := resp.JSONInto(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.BytesTransferredByUserId[keyID], nil
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

	if err := resp.JSONInto(&result); err != nil {
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

	if err := resp.JSONInto(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	var total uint64
	for _, bytes := range result.BytesTransferredByUserId {
		total += bytes
	}

	return total, nil
}
