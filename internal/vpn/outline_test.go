package vpn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOutlineClient_CheckStatus_Success(t *testing.T) {
	// Mock Outline API response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/server", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":           "Test Server",
			"serverId":       "test-uuid-123",
			"metricsEnabled": true,
			"version":        "1.10.0",
		})
	}))
	defer server.Close()

	client := NewOutlineClient(server.URL, false)
	info, err := client.CheckStatus(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "Test Server", info.Name)
	assert.Equal(t, "test-uuid-123", info.ServerID)
	assert.Equal(t, "1.10.0", info.Version)
	assert.True(t, info.MetricsEnabled)
}

func TestOutlineClient_CheckStatus_Failure(t *testing.T) {
	// Mock server returning 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewOutlineClient(server.URL, false)
	_, err := client.CheckStatus(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status")
}

func TestOutlineClient_CheckStatus_NetworkError(t *testing.T) {
	// Invalid URL to trigger network error
	client := NewOutlineClient("http://invalid-host:99999", false)
	_, err := client.CheckStatus(context.Background())

	assert.Error(t, err)
}
