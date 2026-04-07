package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uSipipo-Team/usipipo-agent/internal/logging"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

// logWarning logs a warning message to stderr
// This wrapper avoids errcheck linter issues with fmt.Printf
func logWarning(format string, args ...interface{}) {
	log.Printf("[WARNING] "+format, args...)
}

var metricsCollector *metrics.Collector
var outlineClient *vpn.OutlineClient
var wireguardClient *vpn.WireGuardClient
var securityLogger *logging.SecurityLogger

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

// SetSecurityLogger sets the security logger instance
func SetSecurityLogger(logger *logging.SecurityLogger) {
	securityLogger = logger
}

var trusttunnelClient *vpn.TrustTunnelClient
var trusttunnelMetricsCollector *vpn.TrustTunnelMetricsCollector

// SetTrustTunnelClient sets the TrustTunnel client instance
func SetTrustTunnelClient(client *vpn.TrustTunnelClient) {
	trusttunnelClient = client
}

// SetTrustTunnelMetricsCollector sets the TrustTunnel metrics collector
func SetTrustTunnelMetricsCollector(collector *vpn.TrustTunnelMetricsCollector) {
	trusttunnelMetricsCollector = collector
}

// HealthHandler returns server health status
func HealthHandler(c *gin.Context) {
	// Check if VPN clients are initialized
	outlineStatus := "offline"
	wireguardStatus := "offline"
	
	if outlineClient != nil {
		outlineStatus = "online"
	}
	if wireguardClient != nil {
		wireguardStatus = "online"
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":          "healthy",
		"agent_status":    "online",
		"outline":         outlineStatus,
		"wireguard":       wireguardStatus,
		"timestamp":       time.Now().Unix(),
	})
}

// StatusHandler returns detailed server status
func StatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "online",
		"version": "0.1.0",
	})
}

// MetricsHandler returns detailed system metrics including Outline
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

		// Collect Outline metrics if client is available
	if outlineClient != nil {
		outlineMetrics, err := metricsCollector.GetOutlineMetrics(c.Request.Context(), outlineClient)
		if err != nil {
			// Log error but continue - Outline metrics are optional
			logWarning("failed to collect Outline metrics: %v", err)
		} else {
			m.Outline = outlineMetrics
		}

		// Collect detailed metrics if interval > 1 hour
		if metricsCollector.ShouldCollectDetailed() {
			detailedMetrics, err := metricsCollector.GetDetailedOutlineMetrics(c.Request.Context(), outlineClient)
			if err != nil {
				logWarning("failed to collect detailed Outline metrics: %v", err)
			} else {
				m.Detailed = detailedMetrics
				metricsCollector.MarkDetailedCollected()
			}
		}
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

// CreateTrustTunnelClientRequest represents the request to create a TrustTunnel client
type CreateTrustTunnelClientRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateTrustTunnelClientHandler creates a new TrustTunnel client
func CreateTrustTunnelClientHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	var req CreateTrustTunnelClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := trusttunnelClient.CreateClient(req.Username, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"username": req.Username,
		"status":   "created",
	})
}

// DeleteTrustTunnelClientHandler deletes a TrustTunnel client
func DeleteTrustTunnelClientHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	username := c.Param("username")

	err := trusttunnelClient.DeleteClient(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListTrustTunnelClientsHandler lists all TrustTunnel clients
func ListTrustTunnelClientsHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	clients, err := trusttunnelClient.ListClients()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"clients": clients,
		"count":   len(clients),
	})
}

// ExportTrustTunnelClientHandler exports a TrustTunnel client configuration
func ExportTrustTunnelClientHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	username := c.Param("username")

	config, err := trusttunnelClient.ExportClientConfig(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"config":   config,
	})
}

// ExportTrustTunnelDeeplinkHandler exports a TrustTunnel client configuration as deep link
func ExportTrustTunnelDeeplinkHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	username := c.Param("username")

	deeplink, err := trusttunnelClient.ExportClientDeeplink(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"deeplink": deeplink,
	})
}

// GetTrustTunnelMetricsHandler returns TrustTunnel metrics
func GetTrustTunnelMetricsHandler(c *gin.Context) {
	if trusttunnelMetricsCollector == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel metrics collector not initialized",
		})
		return
	}

	metrics, err := trusttunnelMetricsCollector.GetMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// AddTrustTunnelRuleRequest represents the request to add a rule
type AddTrustTunnelRuleRequest struct {
	CIDR   string `json:"cidr,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	Action string `json:"action" binding:"required"`
}

// AddTrustTunnelRuleHandler adds an access rule
func AddTrustTunnelRuleHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	var req AddTrustTunnelRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := trusttunnelClient.AddRule(req.CIDR, req.Prefix, req.Action)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "rule added",
	})
}

// RemoveTrustTunnelRuleHandler removes an access rule
func RemoveTrustTunnelRuleHandler(c *gin.Context) {
	if trusttunnelClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TrustTunnel client not initialized",
		})
		return
	}

	var req AddTrustTunnelRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := trusttunnelClient.RemoveRule(req.CIDR, req.Prefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
