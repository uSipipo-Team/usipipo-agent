package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/uSipipo-Team/usipipo-agent/internal/metrics"
	"github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)

var (
	savedMetricsCollector *metrics.Collector
	savedWireguardClient  *vpn.WireGuardClient
)

func saveGlobals() {
	savedMetricsCollector = metricsCollector
	savedWireguardClient = wireguardClient
}

func restoreGlobals() {
	metricsCollector = savedMetricsCollector
	wireguardClient = savedWireguardClient
}

func setupHandlerTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/health", HealthHandler)
	r.GET("/status", StatusHandler)
	r.GET("/metrics", MetricsHandler)

	r.POST("/wireguard/peers", CreateWireGuardPeerHandler)
	r.DELETE("/wireguard/peers/:name", DeleteWireGuardPeerHandler)
	r.GET("/wireguard/peers/:name/usage", GetWireGuardPeerUsageHandler)
	r.POST("/wireguard/peers/:name/regenerate", RegenerateWireGuardPeerHandler)

	return r
}

func TestHealthHandler_WireGuardOnline(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = &vpn.WireGuardClient{}

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got '%v'", body["status"])
	}
	if body["agent_status"] != "online" {
		t.Errorf("expected agent_status 'online', got '%v'", body["agent_status"])
	}
	if body["wireguard"] != "online" {
		t.Errorf("expected wireguard 'online', got '%v'", body["wireguard"])
	}
}

func TestHealthHandler_WireGuardOffline(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = nil

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["wireguard"] != "offline" {
		t.Errorf("expected wireguard 'offline', got '%v'", body["wireguard"])
	}
}

func TestStatusHandler(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["status"] != "online" {
		t.Errorf("expected status 'online', got '%v'", body["status"])
	}
	if body["version"] != "0.1.0" {
		t.Errorf("expected version '0.1.0', got '%v'", body["version"])
	}
}

func TestMetricsHandler_NilCollector(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	metricsCollector = nil

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["error"] != "Metrics collector not initialized" {
		t.Errorf("expected error message, got '%v'", body["error"])
	}
}

func TestMetricsHandler_ValidCollector(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	metricsCollector = metrics.NewCollector("test-server")

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/metrics", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["server_id"] != "test-server" {
		t.Errorf("expected server_id 'test-server', got '%v'", body["server_id"])
	}
}

func TestCreateWireGuardPeerHandler_NilClient(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = nil

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/wireguard/peers", strings.NewReader(`{"name":"test-peer"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["error"] != "WireGuard client not initialized" {
		t.Errorf("expected error message, got '%v'", body["error"])
	}
}

func TestCreateWireGuardPeerHandler_BadRequest(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = &vpn.WireGuardClient{}

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/wireguard/peers", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateWireGuardPeerHandler_InvalidJSON(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = &vpn.WireGuardClient{}

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/wireguard/peers", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeleteWireGuardPeerHandler_NilClient(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = nil

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/wireguard/peers/test-peer", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["error"] != "WireGuard client not initialized" {
		t.Errorf("expected error message, got '%v'", body["error"])
	}
}

func TestDeleteWireGuardPeerHandler_InvalidName(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = &vpn.WireGuardClient{}

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/wireguard/peers/name%20with%20spaces", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetWireGuardPeerUsageHandler_NilClient(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = nil

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/wireguard/peers/test-peer/usage", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["error"] != "WireGuard client not initialized" {
		t.Errorf("expected error message, got '%v'", body["error"])
	}
}

func TestGetWireGuardPeerUsageHandler_InvalidName(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = &vpn.WireGuardClient{}

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/wireguard/peers//usage", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRegenerateWireGuardPeerHandler_NilClient(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = nil

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/wireguard/peers/test-peer/regenerate", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body["error"] != "WireGuard client not initialized" {
		t.Errorf("expected error message, got '%v'", body["error"])
	}
}

func TestRegenerateWireGuardPeerHandler_InvalidName(t *testing.T) {
	saveGlobals()
	defer restoreGlobals()

	wireguardClient = &vpn.WireGuardClient{}

	router := setupHandlerTestRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/wireguard/peers//regenerate", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
