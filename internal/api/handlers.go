package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

var metricsCollector *metrics.Collector
var outlineClient *vpn.OutlineClient
var wireguardClient *vpn.WireGuardClient

// SetMetricsCollector sets the metrics collector instance
func SetMetricsCollector(c *metrics.Collector) {
	metricsCollector = c
}

// SetOutlineClient sets the Outline client instance
func SetOutlineClient(client *vpn.OutlineClient) {
	outlineClient = client
}

// SetWireGuardClient sets the WireGuard client instance
func SetWireGuardClient(client *vpn.WireGuardClient) {
	wireguardClient = client
}

// HealthHandler returns server health status
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

// StatusHandler returns detailed server status
func StatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "online",
		"version": "0.1.0",
	})
}

// MetricsHandler returns detailed system metrics
func MetricsHandler(c *gin.Context) {
	if metricsCollector == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Metrics collector not initialized",
		})
		return
	}

	m, err := metricsCollector.GetMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, m)
}

// CreateKeyRequest represents the request to create a key
type CreateKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateKeyResponse represents the response for created key
type CreateKeyResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AccessURL string `json:"access_url"`
}

// CreateOutlineKeyHandler creates a new Outline key
func CreateOutlineKeyHandler(c *gin.Context) {
	if outlineClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Outline client not initialized",
		})
		return
	}

	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, err := outlineClient.CreateKey(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, CreateKeyResponse{
		ID:        key.ID,
		Name:      key.Name,
		AccessURL: key.AccessURL,
	})
}

// DeleteOutlineKeyHandler deletes an Outline key
func DeleteOutlineKeyHandler(c *gin.Context) {
	if outlineClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Outline client not initialized",
		})
		return
	}

	keyID := c.Param("id")

	err := outlineClient.DeleteKey(c.Request.Context(), keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// RegenerateOutlineKeyHandler regenerates an Outline key
func RegenerateOutlineKeyHandler(c *gin.Context) {
	if outlineClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Outline client not initialized",
		})
		return
	}

	keyID := c.Param("id")

	// Get existing keys to find the name
	keys, err := outlineClient.ListKeys(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Find the key to get its name
	var keyName string
	for _, key := range keys {
		if key.ID == keyID {
			keyName = key.Name
			break
		}
	}

	if keyName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	}

	// Delete old key
	err = outlineClient.DeleteKey(c.Request.Context(), keyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Create new key with same name
	newKey, err := outlineClient.CreateKey(c.Request.Context(), keyName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, CreateKeyResponse{
		ID:        newKey.ID,
		Name:      newKey.Name,
		AccessURL: newKey.AccessURL,
	})
}

// CreateWireGuardPeerHandler creates a new WireGuard peer
func CreateWireGuardPeerHandler(c *gin.Context) {
	if wireguardClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "WireGuard client not initialized",
		})
		return
	}

	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	peer, err := wireguardClient.CreatePeer(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"public_key":  peer.PublicKey,
		"name":        peer.Name,
		"ip_address":  peer.IPAddress,
		"config":      peer.Config,
	})
}

// DeleteWireGuardPeerHandler deletes a WireGuard peer
// Idempotent: returns 204 even if peer doesn't exist
func DeleteWireGuardPeerHandler(c *gin.Context) {
	if wireguardClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "WireGuard client not initialized",
		})
		return
	}

	name := c.Param("name")

	err := wireguardClient.DeletePeer(c.Request.Context(), name)
	if err != nil {
		// Check if error is "peer not found" - treat as success (idempotent)
		if strings.Contains(err.Error(), "peer not found") ||
		   strings.Contains(err.Error(), "no such process") ||
		   strings.Contains(err.Error(), "not found") {
			// Peer already deleted or doesn't exist - return success
			c.Status(http.StatusNoContent)
			return
		}
		
		// Real error - return 500
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetWireGuardPeerUsageHandler gets usage for a WireGuard peer
func GetWireGuardPeerUsageHandler(c *gin.Context) {
	if wireguardClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "WireGuard client not initialized",
		})
		return
	}

	name := c.Param("name")

	bytesUsed, err := wireguardClient.GetPeerUsage(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":         name,
		"bytes_used":   bytesUsed,
	})
}

// RegenerateWireGuardPeerHandler regenerates a WireGuard peer configuration
func RegenerateWireGuardPeerHandler(c *gin.Context) {
	if wireguardClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "WireGuard client not initialized",
		})
		return
	}

	name := c.Param("name")

	// Delete existing peer
	err := wireguardClient.DeletePeer(c.Request.Context(), name)
	if err != nil {
		// Ignore "not found" errors - we're going to recreate anyway
		if !strings.Contains(err.Error(), "not found") &&
		   !strings.Contains(err.Error(), "no such process") {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// Create new peer with same name
	peer, err := wireguardClient.CreatePeer(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"public_key":  peer.PublicKey,
		"name":        peer.Name,
		"ip_address":  peer.IPAddress,
		"config":      peer.Config,
	})
}
