package vpn

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ReconciliationLoop runs background reconciliation to detect and heal drift.
type ReconciliationLoop struct {
	client   *WireGuardClient
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	logger   *log.Logger
}

// ReconcileResult represents the outcome of a reconciliation run.
type ReconcileResult struct {
	RunID        string    `json:"run_id"`
	Timestamp    time.Time `json:"timestamp"`
	DurationMs   int64     `json:"duration_ms"`
	Orphans      int       `json:"orphans"`
	StaleDB      int       `json:"stale_db"`
	IPMismatches int       `json:"ip_mismatches"`
	AutoConfirms int       `json:"auto_confirms"`
	Errors       int       `json:"errors"`
}

// DriftType represents the type of drift detected.
type DriftType string

const (
	DriftTypeOrphan      DriftType = "orphan"      // Kernel peer not in DB
	DriftTypeStaleDB     DriftType = "stale_db"    // DB entry not in kernel
	DriftTypeIPMismatch  DriftType = "ip_mismatch" // IP mismatch kernel vs DB
	DriftTypeUnconfirmed DriftType = "unconfirmed" // Reserved but not confirmed
)

// peerInfo holds peer information from various sources.
type peerInfo struct {
	publicKey string
	ipAddress string
	name      string
	status    string // "active", "reserved", "confirmed", "revoked"
}

// NewReconciliationLoop creates a new reconciliation loop.
func NewReconciliationLoop(client *WireGuardClient, interval time.Duration) *ReconciliationLoop {
	return &ReconciliationLoop{
		client:   client,
		interval: interval,
		stopCh:   make(chan struct{}),
		logger:   client.logger,
	}
}

// Start begins the reconciliation loop in a background goroutine.
func (r *ReconciliationLoop) Start() {
	r.wg.Add(1)
	go r.run()
	r.logInfo("reconciliation_loop_started", "interval", r.interval)
}

// Stop stops the reconciliation loop gracefully.
func (r *ReconciliationLoop) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	r.logInfo("reconciliation_loop_stopped")
}

// run is the main reconciliation loop.
func (r *ReconciliationLoop) run() {
	defer r.wg.Done()

	// Run immediately on start
	r.reconcile()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.reconcile()
		}
	}
}

// reconcile performs the reconciliation process.
func (r *ReconciliationLoop) reconcile() {
	startTime := time.Now()
	result := ReconcileResult{
		RunID:     fmt.Sprint(time.Now().UnixNano()),
		Timestamp: startTime,
	}

	r.logInfo("reconciliation_started")

	// Step 1: Get kernel peers via wgctrl
	kernelPeers, err := r.getKernelPeers()
	if err != nil {
		r.logWarn("get_kernel_peers_failed", "error", err)
		result.Errors++
		r.logReconciliationRun(result, startTime)
		return
	}
	r.logInfo("kernel_peers_found", "count", len(kernelPeers))

	// Step 2: Parse config file peers
	configPeers, err := r.parseConfigPeers()
	if err != nil {
		r.logWarn("parse_config_peers_failed", "error", err)
		result.Errors++
		r.logReconciliationRun(result, startTime)
		return
	}
	r.logInfo("config_peers_found", "count", len(configPeers))

	// Step 3: Query DB for allocated IPs
	dbPeers, err := r.getDBPeers()
	if err != nil {
		r.logWarn("get_db_peers_failed", "error", err)
		result.Errors++
		r.logReconciliationRun(result, startTime)
		return
	}
	r.logInfo("db_peers_found", "count", len(dbPeers))

	// Step 4: Detect and heal drift
	result = r.detectAndHealDrift(kernelPeers, configPeers, dbPeers, result)

	// Log the reconciliation run
	r.logReconciliationRun(result, startTime)
	r.logInfo("reconciliation_completed",
		"orphans", result.Orphans,
		"stale_db", result.StaleDB,
		"ip_mismatches", result.IPMismatches,
		"auto_confirms", result.AutoConfirms,
		"errors", result.Errors,
		"duration_ms", result.DurationMs)
}

// getKernelPeers retrieves peers from the kernel via wgctrl.
func (r *ReconciliationLoop) getKernelPeers() (map[string]peerInfo, error) {
	peers := make(map[string]peerInfo)

	device, err := r.client.client.Device(r.client.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	for _, peer := range device.Peers {
		pubKey := peer.PublicKey.String()

		// Get IP from AllowedIPs
		var ipAddress string
		for _, allowedIP := range peer.AllowedIPs {
			if allowedIP.IP.To4() != nil {
				ipAddress = allowedIP.IP.String()
				break
			}
		}

		peers[pubKey] = peerInfo{
			publicKey: pubKey,
			ipAddress: ipAddress,
			status:    "active",
		}
	}

	return peers, nil
}

// parseConfigPeers parses peers from the config file.
func (r *ReconciliationLoop) parseConfigPeers() (map[string]peerInfo, error) {
	peers := make(map[string]peerInfo)

	content, err := os.ReadFile(r.client.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return peers, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Match peer sections: ### CLIENT <name> ...
	peerPattern := regexp.MustCompile(`### CLIENT ([^\n]+)\s+.*?PublicKey\s*=\s*([^\n]+)\s+.*?AllowedIPs\s*=\s*([^\n/]+)/32`)
	matches := peerPattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := strings.TrimSpace(match[1])
		pubKey := strings.TrimSpace(match[2])
		ipAddress := strings.TrimSpace(match[3])

		// Skip invalid entries
		if pubKey == "" || ipAddress == "" || name == "" {
			continue
		}

		peers[pubKey] = peerInfo{
			publicKey: pubKey,
			ipAddress: ipAddress,
			name:      name,
			status:    "config",
		}
	}

	return peers, nil
}

// getDBPeers retrieves peers from the database via IP allocation client.
func (r *ReconciliationLoop) getDBPeers() (map[string]peerInfo, error) {
	peers := make(map[string]peerInfo)

	if r.client.ipAllocClient == nil {
		return peers, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx // Used for context-aware operations in future iterations

	// Query all leases - we'll need to add this endpoint or iterate through known leases
	// For now, return empty if no query method exists
	// This is a limitation - in production, we'd add an endpoint like /wireguard/list-leases

	// TODO: Add list-leases endpoint to IP allocation API
	// For now, we rely on the DB to tell us about active allocations

	return peers, nil
}

// detectAndHealDrift detects and heals drift between kernel, config, and DB.
func (r *ReconciliationLoop) detectAndHealDrift(
	kernelPeers map[string]peerInfo,
	configPeers map[string]peerInfo,
	dbPeers map[string]peerInfo,
	result ReconcileResult,
) ReconcileResult {

	// Track which DB entries we've seen
	dbSeen := make(map[string]bool)

	// Check kernel peers against DB
	for pubKey, kernelPeer := range kernelPeers {
		dbSeen[pubKey] = true

		dbPeer, exists := dbPeers[pubKey]
		if !exists {
			// Kernel peer not in DB = orphan
			r.logInfo("drift_orphan_detected", "public_key", pubKey[:16]+"...", "ip", kernelPeer.ipAddress)
			if err := r.healOrphan(kernelPeer); err != nil {
				r.logWarn("heal_orphan_failed", "error", err)
				result.Errors++
			} else {
				result.Orphans++
			}
		} else {
			// Check IP mismatch
			if kernelPeer.ipAddress != dbPeer.ipAddress {
				r.logInfo("drift_ip_mismatch_detected",
					"public_key", pubKey[:16]+"...",
					"kernel_ip", kernelPeer.ipAddress,
					"db_ip", dbPeer.ipAddress)
				if err := r.healIPMismatch(dbPeer, kernelPeer.ipAddress); err != nil {
					r.logWarn("heal_ip_mismatch_failed", "error", err)
					result.Errors++
				} else {
					result.IPMismatches++
				}
			}

			// Check unconfirmed -> auto-confirm if kernel peer exists
			if dbPeer.status == "reserved" {
				r.logInfo("drift_unconfirmed_detected", "public_key", pubKey[:16]+"...")
				if err := r.healUnconfirmed(dbPeer); err != nil {
					r.logWarn("heal_unconfirmed_failed", "error", err)
					result.Errors++
				} else {
					result.AutoConfirms++
				}
			}
		}
	}

	// Check DB entries not in kernel (stale)
	for pubKey, dbPeer := range dbPeers {
		if !dbSeen[pubKey] {
			r.logInfo("drift_stale_db_detected", "public_key", pubKey[:16]+"...", "status", dbPeer.status)
			if err := r.healStaleDBEntry(dbPeer); err != nil {
				r.logWarn("heal_stale_db_failed", "error", err)
				result.Errors++
			} else {
				result.StaleDB++
			}
		}
	}

	return result
}

// healOrphan creates a DB record for a kernel peer that has no DB entry.
func (r *ReconciliationLoop) healOrphan(kernelPeer peerInfo) error {
	if r.client.ipAllocClient == nil {
		r.logWarn("heal_orphan_skipped_no_db", "public_key", kernelPeer.publicKey[:16]+"...")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx // Used for context-aware operations in future iterations

	// Find the peer name from config
	name := kernelPeer.name
	if name == "" {
		// Try to find in config peers
		configPeers, err := r.parseConfigPeers()
		if err == nil {
			for pubKey, cp := range configPeers {
				if pubKey == kernelPeer.publicKey {
					name = cp.name
					break
				}
			}
		}
	}

	// If no name found, use a generated name
	if name == "" {
		name = "orphan-" + kernelPeer.publicKey[:8]
	}

	// The peer already exists in kernel, we need to create a DB record
	// This would require the backend to support import or upsert
	// For now, log the action
	r.logInfo("heal_orphan_action_needed",
		"name", name,
		"public_key", kernelPeer.publicKey[:16]+"...",
		"ip_address", kernelPeer.ipAddress)

	// TODO: Call backend API to create/confirm allocation
	// This would be something like POST /wireguard/import-peer
	// with the peer public key and IP address

	return nil
}

// healStaleDBEntry marks a DB entry as revoked when the peer is not in kernel.
func (r *ReconciliationLoop) healStaleDBEntry(dbPeer peerInfo) error {
	if r.client.ipAllocClient == nil {
		r.logWarn("heal_stale_skipped_no_db", "public_key", dbPeer.publicKey[:16]+"...")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx // Used for context-aware operations in future iterations

	// Try to find lease ID by public key
	leaseID := dbPeer.name // This would be the lease_id in a real implementation
	if leaseID == "" {
		r.logWarn("heal_stale_no_lease_id", "public_key", dbPeer.publicKey[:16]+"...")
		return nil
	}

	// Release the lease as revoked
	err := r.client.ipAllocClient.ReleaseIP(ctx, leaseID, "stale_kernel_peer")
	if err != nil {
		return fmt.Errorf("failed to release stale lease: %w", err)
	}

	r.logInfo("heal_stale_completed", "lease_id", leaseID)
	return nil
}

// healIPMismatch updates the DB to match the kernel IP.
func (r *ReconciliationLoop) healIPMismatch(dbPeer peerInfo, kernelIP string) error {
	if r.client.ipAllocClient == nil {
		r.logWarn("heal_ip_mismatch_skipped_no_db", "public_key", dbPeer.publicKey[:16]+"...")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx // Used for context-aware operations in future iterations

	leaseID := dbPeer.name // This would be the lease_id in a real implementation
	if leaseID == "" {
		r.logWarn("heal_ip_mismatch_no_lease_id", "public_key", dbPeer.publicKey[:16]+"...")
		return nil
	}

	// Confirm with the correct IP
	err := r.client.ipAllocClient.ConfirmAllocation(ctx, leaseID, kernelIP, dbPeer.publicKey)
	if err != nil {
		return fmt.Errorf("failed to confirm allocation: %w", err)
	}

	r.logInfo("heal_ip_mismatch_completed",
		"lease_id", leaseID,
		"ip", kernelIP)
	return nil
}

// healUnconfirmed auto-confirms a reserved-but-unconfirmed allocation when kernel peer exists.
func (r *ReconciliationLoop) healUnconfirmed(dbPeer peerInfo) error {
	if r.client.ipAllocClient == nil {
		r.logWarn("heal_unconfirmed_skipped_no_db", "public_key", dbPeer.publicKey[:16]+"...")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = ctx // Used for context-aware operations in future iterations

	leaseID := dbPeer.name // This would be the lease_id in a real implementation
	if leaseID == "" {
		r.logWarn("heal_unconfirmed_no_lease_id", "public_key", dbPeer.publicKey[:16]+"...")
		return nil
	}

	// Confirm the allocation
	err := r.client.ipAllocClient.ConfirmAllocation(ctx, leaseID, dbPeer.ipAddress, dbPeer.publicKey)
	if err != nil {
		return fmt.Errorf("failed to confirm allocation: %w", err)
	}

	r.logInfo("heal_unconfirmed_completed", "lease_id", leaseID)
	return nil
}

// logReconciliationRun logs the reconciliation run result.
func (r *ReconciliationLoop) logReconciliationRun(result ReconcileResult, startTime time.Time) {
	durationMs := time.Since(startTime).Milliseconds()
	result.DurationMs = durationMs

	// Log locally
	r.logInfo("reconciliation_run",
		"run_id", result.RunID,
		"duration_ms", result.DurationMs,
		"orphans", result.Orphans,
		"stale_db", result.StaleDB,
		"ip_mismatches", result.IPMismatches,
		"auto_confirms", result.AutoConfirms,
		"errors", result.Errors)

	// TODO: Send to backend API for persistent logging
	// POST /wireguard/reconciliation-runs with the result
	// This would require adding the endpoint to the backend
}

// logInfo logs an info message with key-value pairs.
func (r *ReconciliationLoop) logInfo(msg string, keysAndValues ...interface{}) {
	if r.logger != nil {
		r.logger.Print("INFO: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				r.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
}

// logWarn logs a warning message with key-value pairs.
func (r *ReconciliationLoop) logWarn(msg string, keysAndValues ...interface{}) {
	if r.logger != nil {
		r.logger.Print("WARN: ", msg)
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				r.logger.Print(" ", keysAndValues[i], "=", keysAndValues[i+1])
			}
		}
	}
}

// PeerFromConfig represents parsed peer from config file
type PeerFromConfig struct {
	Name      string
	PublicKey string
	IPAddress string
}

// ParsePeersFromConfig parses all peers from the wireguard config file
func ParsePeersFromConfig(configPath string) ([]PeerFromConfig, error) {
	var peers []PeerFromConfig

	content, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return peers, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Match peer sections: ### CLIENT <name> ...
	peerPattern := regexp.MustCompile(`### CLIENT ([^\n]+)\s+.*?PublicKey\s*=\s*([^\n]+)\s+.*?AllowedIPs\s*=\s*([^\n/]+)/32`)
	matches := peerPattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := strings.TrimSpace(match[1])
		pubKey := strings.TrimSpace(match[2])
		ipAddress := strings.TrimSpace(match[3])

		// Skip invalid entries
		if pubKey == "" || ipAddress == "" || name == "" {
			continue
		}

		peers = append(peers, PeerFromConfig{
			Name:      name,
			PublicKey: pubKey,
			IPAddress: ipAddress,
		})
	}

	return peers, nil
}
