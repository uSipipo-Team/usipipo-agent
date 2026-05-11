// WireGuard IP Allocation Repair CLI Tool
//
// Commands:
//
//	wg-ip-allocation audit         - Audit mode: reports drift without fixing
//	wg-ip-allocation fix-drift    - Fix drift issues automatically
//	wg-ip-allocation force-release - Force release an IP
//	wg-ip-allocation import-peer  - Import manual peer to DB
//	wg-ip-allocation pool-status - Show pool utilization
//	wg-ip-allocation list        - List all allocations
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

type Config struct {
	BackendAPIURL string
	BackendAPIKey string
	ServerID      string
	DryRun        bool
	JSONOutput    bool
}

var cfg Config

var rootCmd = &cobra.Command{
	Use:   "wg-ip-allocation",
	Short: "WireGuard IP allocation repair operations",
	Long: `WireGuard IP Allocation Repair CLI Tool

Provides commands for auditing and fixing IP allocation drift issues
in the WireGuard backend database.

Environment Variables:
  BACKEND_API_URL  - Backend API URL (required)
  BACKEND_API_KEY - Backend API Key (required)

Examples:
  # Audit drift on a server
  wg-ip-allocation audit --server-id 550e8400-e29b-41d4-a716-446655440000

  # Fix drift issues
  wg-ip-allocation fix-drift --server-id 550e8400-e29b-41d4-a716-446655440000

  # Force release an IP address
  wg-ip-allocation force-release --server-id 550e8400-e29b-41d4-a716-446655440000 --ip 10.88.88.50

  # Import peer to database
  wg-ip-allocation import-peer --server-id 550e8400-e29b-41d4-a716-446655440000 --pubkey ABCDEF...= --ip 10.88.88.50`,
	SilenceUsage: true,
}

func init() {
	viper.SetEnvPrefix("BACKEND")
	viper.BindEnv("API_URL")
	viper.BindEnv("API_KEY")

	rootCmd.PersistentFlags().StringVar(&cfg.BackendAPIURL, "api-url", "", "Backend API URL (env: BACKEND_API_URL)")
	rootCmd.PersistentFlags().StringVar(&cfg.BackendAPIKey, "api-key", "", "Backend API Key (env: BACKEND_API_KEY)")
	rootCmd.PersistentFlags().BoolVar(&cfg.DryRun, "dry-run", false, "Show what would be done without making changes")
	rootCmd.PersistentFlags().BoolVar(&cfg.JSONOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cfg.BackendAPIURL = viper.GetString("API_URL")
		cfg.BackendAPIKey = viper.GetString("API_KEY")

		if cfg.BackendAPIURL == "" {
			cfg.BackendAPIURL = os.Getenv("BACKEND_API_URL")
		}
		if cfg.BackendAPIKey == "" {
			cfg.BackendAPIKey = os.Getenv("BACKEND_API_KEY")
		}

		if cfg.BackendAPIURL == "" {
			return fmt.Errorf("BACKEND_API_URL is required (set BACKEND_API_URL env var)")
		}
		if cfg.BackendAPIKey == "" {
			return fmt.Errorf("BACKEND_API_KEY is required (set BACKEND_API_KEY env var)")
		}
		return nil
	}

	rootCmd.AddCommand(auditCmd())
	rootCmd.AddCommand(fixDriftCmd())
	rootCmd.AddCommand(forceReleaseCmd())
	rootCmd.AddCommand(importPeerCmd())
	rootCmd.AddCommand(poolStatusCmd())
	rootCmd.AddCommand(listAllocationsCmd())
	rootCmd.AddCommand(versionCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func auditCmd() *cobra.Command {
	var serverID string

	cmd := &cobra.Command{
		Use:   "audit --server-id <uuid>",
		Short: "Audit mode: reports drift without fixing",
		Long: `Audit mode: Reports drift between the database and WireGuard interface
without making any changes.

This command compares IP allocations in the database against actual
WireGuard interface configuration to identify inconsistencies.

Examples:
  # Audit specific server
  wg-ip-allocation audit --server-id 550e8400-e29b-41d4-a716-446655440000

  # JSON output
  wg-ip-allocation audit --server-id 550e8400-e29b-41d4-a716-446655440000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ServerID = serverID
			return runAudit()
		},
	}

	f := cmd.Flags()
	f.StringVar(&serverID, "server-id", "", "Server ID (required)")

	if err := cmd.MarkFlagRequired("server-id"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}

	return cmd
}

func fixDriftCmd() *cobra.Command {
	var serverID string

	cmd := &cobra.Command{
		Use:   "fix-drift --server-id <uuid>",
		Short: "Fix drift issues automatically",
		Long: `Fix drift issues by synchronizing the database with the
actual WireGuard interface configuration.

This command will:
- Remove stale leases from database for IPs not in interface
- Add missing leases for IPs in interface not in database

Use --dry-run to preview changes without applying them.

Examples:
  # Fix drift on server
  wg-ip-allocation fix-drift --server-id 550e8400-e29b-41d4-a716-446655440000

  # Preview changes
  wg-ip-allocation fix-drift --server-id 550e8400-e29b-41d4-a716-446655440000 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ServerID = serverID
			return runFixDrift()
		},
	}

	f := cmd.Flags()
	f.StringVar(&serverID, "server-id", "", "Server ID (required)")

	if err := cmd.MarkFlagRequired("server-id"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}

	return cmd
}

func forceReleaseCmd() *cobra.Command {
	var serverID string
	var ip string
	var leaseID string

	cmd := &cobra.Command{
		Use:   "force-release",
		Short: "Force release an IP address",
		Long: `Force release an IP address from the database.

Can identify IP by address or lease ID.

Examples:
  # Release by IP address
  wg-ip-allocation force-release --server-id 550e8400-e29b-41d4-a716-446655440000 --ip 10.88.88.50

  # Release by lease ID
  wg-ip-allocation force-release --server-id 550e8400-e29b-41d4-a716-446655440000 --lease-id abc123

  # Dry run
  wg-ip-allocation force-release --server-id 550e8400-e29b-41d4-a716-446655440000 --ip 10.88.88.50 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ServerID = serverID
			if ip == "" && leaseID == "" {
				return fmt.Errorf("either --ip or --lease-id is required")
			}
			return runForceRelease(ip, leaseID)
		},
	}

	f := cmd.Flags()
	f.StringVar(&serverID, "server-id", "", "Server ID (required)")
	f.StringVar(&ip, "ip", "", "IP address to release")
	f.StringVar(&leaseID, "lease-id", "", "Lease ID to release")

	if err := cmd.MarkFlagRequired("server-id"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}

	return cmd
}

func importPeerCmd() *cobra.Command {
	var serverID string
	var pubkey string
	var ip string

	cmd := &cobra.Command{
		Use:   "import-peer",
		Short: "Import manual peer to database",
		Long: `Import a manually configured WireGuard peer to the database.

This is used when adding peers that were created outside
the normal allocation flow.

Examples:
  # Import peer
  wg-ip-allocation import-peer --server-id 550e8400-e29b-41d4-a716-446655440000 --pubkey ABCDEF...= --ip 10.88.88.50

  # Dry run
  wg-ip-allocation import-peer --server-id 550e8400-e29b-41d4-a716-446655440000 --pubkey ABCDEF...= --ip 10.88.88.50 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ServerID = serverID
			return runImportPeer(pubkey, ip)
		},
	}

	f := cmd.Flags()
	f.StringVar(&serverID, "server-id", "", "Server ID (required)")
	f.StringVar(&pubkey, "pubkey", "", "WireGuard public key (required)")
	f.StringVar(&ip, "ip", "", "Assigned IP address (required)")

	if err := cmd.MarkFlagRequired("server-id"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}
	if err := cmd.MarkFlagRequired("pubkey"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}
	if err := cmd.MarkFlagRequired("ip"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}

	return cmd
}

func poolStatusCmd() *cobra.Command {
	var serverID string

	cmd := &cobra.Command{
		Use:   "pool-status --server-id <uuid>",
		Short: "Show pool utilization",
		Long: `Show IP pool utilization statistics.

Displays total available IPs, allocated count, utilization percentage,
and any reservation conflicts.

Examples:
  # Show pool status
  wg-ip-allocation pool-status --server-id 550e8400-e29b-41d4-a716-446655440000

  # JSON output
  wg-ip-allocation pool-status --server-id 550e8400-e29b-41d4-a716-446655440000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ServerID = serverID
			return runPoolStatus()
		},
	}

	f := cmd.Flags()
	f.StringVar(&serverID, "server-id", "", "Server ID (required)")

	if err := cmd.MarkFlagRequired("server-id"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}

	return cmd
}

func listAllocationsCmd() *cobra.Command {
	var serverID string

	cmd := &cobra.Command{
		Use:   "list --server-id <uuid>",
		Short: "List all allocations",
		Long: `List all IP allocations for a server.

Shows all active and expired leases with their status,
lease IDs, and expiration times.

Examples:
  # List allocations
  wg-ip-allocation list --server-id 550e8400-e29b-41d4-a716-446655440000

  # JSON output
  wg-ip-allocation list --server-id 550e8400-e29b-41d4-a716-446655440000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.ServerID = serverID
			return runList()
		},
	}

	f := cmd.Flags()
	f.StringVar(&serverID, "server-id", "", "Server ID (required)")

	if err := cmd.MarkFlagRequired("server-id"); err != nil {
		log.Fatalf("mark flag required: %v", err)
	}

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("wg-ip-allocation %s", Version)
			if Commit != "" {
				fmt.Printf(" (commit: %s)", Commit)
			}
			if BuildDate != "" {
				fmt.Printf(" built: %s", BuildDate)
			}
			fmt.Println()
		},
		SilenceUsage: true,
	}
}

type APIResponse struct {
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type AuditResponse struct {
	ServerID      string         `json:"server_id"`
	TotalLeases   int            `json:"total_leases"`
	DriftedLeases []DriftedLease `json:"drifted_leases,omitempty"`
	DatabaseIPs   []string       `json:"database_ips"`
	InterfaceIPs  []string       `json:"interface_ips"`
}

type DriftedLease struct {
	LeaseID   string `json:"lease_id"`
	IPAddress string `json:"ip_address"`
	Reason    string `json:"reason"`
}

func runAudit() error {
	url := cfg.BackendAPIURL + "/wireguard/audit/" + cfg.ServerID

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	setAuthHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(body))
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !result.Success && result.Error != "" {
		return fmt.Errorf("audit failed: %s", result.Error)
	}

	if cfg.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result.Data)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		fmt.Println("Audit complete")
		return nil
	}

	totalLeases := int(data["total_leases"].(float64))
	drifted := data["drifted_leases"]
	driftedSlice, ok := drifted.([]interface{})
	hasDrift := ok && len(driftedSlice) > 0

	fmt.Printf("Server: %s\n", cfg.ServerID)
	fmt.Printf("Total leases: %d\n", totalLeases)

	if hasDrift {
		fmt.Printf("Drift detected: %d issues\n", len(driftedSlice))
		fmt.Println()
		for _, d := range driftedSlice {
			dm, ok := d.(map[string]interface{})
			if !ok {
				continue
			}
			fmt.Printf("  - IP: %v  Lease: %v  Reason: %v\n", dm["ip_address"], dm["lease_id"], dm["reason"])
		}
	} else {
		fmt.Println("No drift detected")
	}

	return nil
}

func runFixDrift() error {
	url := cfg.BackendAPIURL + "/wireguard/fix-drift/" + cfg.ServerID

	var req *http.Request
	var err error

	if cfg.DryRun {
		req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, url+"?dry-run=true", nil)
	} else {
		req, err = http.NewRequestWithContext(context.Background(), http.MethodPost, url, nil)
	}
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	setAuthHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(body))
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !result.Success && result.Error != "" {
		return fmt.Errorf("fix failed: %s", result.Error)
	}

	if cfg.DryRun {
		printInfo("DRY RUN: Would fix drift for server %s", cfg.ServerID)
	} else {
		data, ok := result.Data.(map[string]interface{})
		if ok {
			if fixed, ok := data["fixed_count"].(float64); ok {
				printInfo("Fixed %d drift issues", int(fixed))
			}
		}
		printInfo("Drift fixed successfully")
	}

	return nil
}

type ReleaseRequest struct {
	ServerID string `json:"server_id"`
	IP       string `json:"ip_address,omitempty"`
	LeaseID  string `json:"lease_id,omitempty"`
	Reason   string `json:"reason"`
}

func runForceRelease(ip, leaseID string) error {
	reason := "force-release"
	if cfg.DryRun {
		reason = "force-release-dry-run"
	}

	reqBody := ReleaseRequest{
		ServerID: cfg.ServerID,
		IP:       ip,
		LeaseID:  leaseID,
		Reason:   reason,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := cfg.BackendAPIURL + "/wireguard/release-ip"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	if cfg.DryRun {
		id := ip
		if id == "" {
			id = leaseID
		}
		printInfo("DRY RUN: Would release %s (server: %s)", id, cfg.ServerID)
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(respBody))
	}

	id := ip
	if id == "" {
		id = leaseID
	}
	printInfo("Released: %s (server: %s)", id, cfg.ServerID)
	return nil
}

type ImportRequest struct {
	ServerID  string `json:"server_id"`
	PublicKey string `json:"public_key"`
	IPAddress string `json:"ip_address"`
}

func runImportPeer(pubkey, ip string) error {
	reqBody := ImportRequest{
		ServerID:  cfg.ServerID,
		PublicKey: pubkey,
		IPAddress: ip,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := cfg.BackendAPIURL + "/wireguard/import-peer"

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	setAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	if cfg.DryRun {
		pk := pubkey
		if len(pk) > 8 {
			pk = pk[:8] + "..."
		}
		printInfo("DRY RUN: Would import peer: pubkey=%s ip=%s server=%s", pk, ip, cfg.ServerID)
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(respBody))
	}

	var result APIResponse
	if err := json.Unmarshal(respBody, &result); err == nil && !result.Success {
		return fmt.Errorf("import failed: %s", result.Error)
	}

	printInfo("Imported peer: ip=%s server=%s", ip, cfg.ServerID)
	return nil
}

func runPoolStatus() error {
	url := cfg.BackendAPIURL + "/wireguard/ip-pool-status/" + cfg.ServerID

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	setAuthHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(body))
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if cfg.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result.Data)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		fmt.Println("Pool status: unknown format")
		return nil
	}

	total := int(data["total"].(float64))
	allocated := int(data["allocated"].(float64))
	available := total - allocated
	utilization := 0.0
	if total > 0 {
		utilization = float64(allocated) / float64(total) * 100
	}

	fmt.Printf("Server: %s\n", cfg.ServerID)
	fmt.Printf("Total IPs: %d\n", total)
	fmt.Printf("Allocated: %d\n", allocated)
	fmt.Printf("Available: %d\n", available)
	fmt.Printf("Utilization: %.1f%%\n", utilization)

	return nil
}

func runList() error {
	url := cfg.BackendAPIURL + "/wireguard/allocations/" + cfg.ServerID

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	setAuthHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d - %s", resp.StatusCode, string(body))
	}

	var result APIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if cfg.JSONOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result.Data)
	}

	allocations, ok := result.Data.([]interface{})
	if !ok {
		fmt.Println("No allocations found")
		return nil
	}

	fmt.Printf("Server: %s\n", cfg.ServerID)
	fmt.Printf("Total allocations: %d\n", len(allocations))
	fmt.Println()

	for i, a := range allocations {
		alloc, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		ip := alloc["ip_address"]
		leaseID := alloc["lease_id"]
		status := alloc["status"]
		expires := alloc["lease_expires_at"]

		fmt.Printf("[%d] IP: %v  Lease: %v  Status: %v  Expires: %v\n", i+1, ip, leaseID, status, expires)
	}

	return nil
}

func setAuthHeaders(req *http.Request) {
	req.Header.Set("X-Api-Key", cfg.BackendAPIKey)
	req.Header.Set("X-Timestamp", fmt.Sprint(time.Now().Unix()))
}

func printInfo(format string, args ...interface{}) {
	if cfg.JSONOutput {
		return
	}
	fmt.Printf(format+"\n", args...)
}
