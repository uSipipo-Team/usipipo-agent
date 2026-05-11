# WireGuard-Only Agent Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the uSipipo VPN Agent to remove ALL Outline and TrustTunnel support, keeping only WireGuard functionality optimized.

**Architecture:** Clean separation: remove entire Outline/TrustTunnel packages, endpoints, config, metrics, and documentation. Keep WireGuard core + IP allocation + reconciliation + metrics. Build clean, minimal binary.

**Tech Stack:** Go 1.21, Gin, wgctrl, gopsutil, resty

---

## Prerequisites

- Backend updated to accept agent registration without `supports_outline` field
- Backend metrics ingestion tolerant of missing `outline` and `detailed` sections
- All servers using this agent will lose Outline/TrustTunnel functionality immediately
- Version bump to v0.12.0 (breaking change)

---

## Phase 1: Code Removal - Core VPN Packages

### Task 1: Remove Outline Client

**Files:**
- **Delete:** `internal/vpn/outline.go` (324 lines)
- **Delete:** `internal/vpn/outline_test.go` (147 lines)

**Step 1:** Verify files exist

```bash
ls -la /home/mowgli/usipipo/agent/internal/vpn/outline.go
ls -la /home/mowgli/usipipo/agent/internal/vpn/outline_test.go
```

Expected: Both files exist.

**Step 2:** Delete files

```bash
rm /home/mowgli/usipipo/agent/internal/vpn/outline.go
rm /home/mowgli/usipipo/agent/internal/vpn/outline_test.go
```

**Step 3:** Verify deletion

```bash
ls /home/mowgli/usipipo/agent/internal/vpn/ | grep outline
```

Expected: No output.

**Step 4:** Commit deletion

```bash
cd /home/mowgli/usipipo/agent
git add -u internal/vpn/outline.go internal/vpn/outline_test.go
git commit -m "refactor: remove Outline client and tests"
```

---

### Task 2: Remove TrustTunnel Client

**Files:**
- **Delete:** `internal/vpn/trusttunnel.go` (387 lines)
- **Delete:** `internal/vpn/trusttunnel_metrics.go` (94 lines)
- **Delete:** `internal/vpn/trusttunnel_test.go` (163 lines)
- **Delete:** `internal/vpn/trusttunnel_metrics_test.go` (43 lines)

**Step 1:** Verify files exist

```bash
ls -la internal/vpn/trusttunnel.go
ls -la internal/vpn/trusttunnel_metrics.go
ls -la internal/vpn/trusttunnel_test.go
ls -la internal/vpn/trusttunnel_metrics_test.go
```

**Step 2:** Delete all four files

```bash
rm internal/vpn/trusttunnel.go
rm internal/vpn/trusttunnel_metrics.go
rm internal/vpn/trusttunnel_test.go
rm internal/vpn/trusttunnel_metrics_test.go
```

**Step 3:** Verify deletion

```bash
ls internal/vpn/ | grep trusttunnel
```

Expected: No output.

**Step 4:** Commit

```bash
git add -u internal/vpn/trusttunnel*.go
git commit -m "refactor: remove TrustTunnel client, metrics, and tests"
```

---

## Phase 2: API Handler Cleanup

### Task 3: Remove Outline Handlers from handlers.go

**Files:**
- **Modify:** `internal/api/handlers.go` (~592 lines → ~450 lines)

**Step 1:** Open file and locate Outline handlers

Search for:
- `CreateOutlineKeyHandler`
- `DeleteOutlineKeyHandler`
- `RegenerateOutlineKeyHandler`

**Step 2:** Remove these three handler functions entirely (approx lines 144-249)

Also remove:
- `SetOutlineClient` function (lines 32-35)
- Global var `outlineClient` (line 23)

**Step 3:** Delete the following code blocks:

```go
// GLOBAL var (top of file)
var outlineClient *vpn.OutlineClient  // DELETE LINE

// Setter function
func SetOutlineClient(client *vpn.OutlineClient) {  // DELETE ENTIRE FUNCTION
    outlineClient = client
}

// Handler functions (3 of them):
func CreateOutlineKeyHandler(c *gin.Context) { ... }      // DELETE
func DeleteOutlineKeyHandler(c *gin.Context) { ... }      // DELETE
func RegenerateOutlineKeyHandler(c *gin.Context) { ... }  // DELETE
```

**Step 4:** Remove Outline imports from `handlers.go`:

Currently imports:
```go
import (
    // ...
    "github.com/uSipipo-Team/usipipo-agent/internal/vpn"
)
```

No direct import to remove (uses `vpn` package). But after removing Outline handlers, verify no references to `outlineClient` remain in file:

```bash
grep -n "outlineClient" internal/api/handlers.go
```

Expected: Only maybe in MetricsHandler if Outline metrics collected. We'll fix that in next task.

**Step 5:** Save and verify file compiles (no syntax errors)

```bash
go build ./internal/api
```

Expected: Success (no Outline references).

**Step 6:** Commit

```bash
git add internal/api/handlers.go
git commit -m "refactor(api): remove Outline endpoints and client setter"
```

---

### Task 4: Remove TrustTunnel Handlers from handlers.go

**Files:**
- **Modify:** `internal/api/handlers.go`

**Step 1:** Locate TrustTunnel-related code

Search for:
- Global vars: `trusttunnelClient`, `trusttunnelMetricsCollector`
- Setter functions: `SetTrustTunnelClient`, `SetTrustTunnelMetricsCollector`
- All handler functions starting with `CreateTrustTunnel...`, `DeleteTrustTunnel...`, etc.

**Step 2:** Remove global variables (around lines 47-58):

```go
var trusttunnelClient *vpn.TrustTunnelClient
var trusttunnelMetricsCollector *vpn.TrustTunnelMetricsCollector
```

**Step 3:** Remove setter functions:

```go
func SetTrustTunnelClient(client *vpn.TrustTunnelClient) { ... }
func SetTrustTunnelMetricsCollector(collector *vpn.TrustTunnelMetricsCollector) { ... }
```

**Step 4:** Remove all TrustTunnel handler functions (approx 8 handlers, lines 395-592):

Delete:
- `CreateTrustTunnelClientHandler`
- `DeleteTrustTunnelClientHandler`
- `ListTrustTunnelClientsHandler`
- `ExportTrustTunnelClientHandler`
- `ExportTrustTunnelDeeplinkHandler`
- `GetTrustTunnelMetricsHandler`
- `AddTrustTunnelRuleHandler`
- `RemoveTrustTunnelRuleHandler`

**Step 5:** Check for any remaining `trusttunnel` references:

```bash
grep -n "trusttunnel" internal/api/handlers.go
```

Expected: No matches.

**Step 6:** Compile check:

```bash
go build ./internal/api
```

Expected: Success.

**Step 7:** Commit

```bash
git add internal/api/handlers.go
git commit -m "refactor(api): remove TrustTunnel endpoints and client"
```

---

### Task 5: Update MetricsHandler to Skip Outline/TrustTunnel Collection

**Files:**
- **Modify:** `internal/api/handlers.go` (MetricsHandler function)
- **Modify:** `internal/metrics/collector.go`
- **Modify:** `internal/metrics/types.go`

**Step 1:** Update `MetricsHandler` in handlers.go (around lines 90-130)

Current code collects Outline metrics conditionally:

```go
// Collect Outline metrics if client is available
if outlineClient != nil {
    outlineMetrics, err := metricsCollector.GetOutlineMetrics(...)
    // ...
}
```

Delete this entire `if outlineClient != nil` block (lines ~107-127).

Also remove any `trusttunnelMetricsCollector` usage in MetricsHandler (there shouldn't be any after Task 4, but verify).

**Result:** MetricsHandler now only collects system + WireGuard metrics.

**Step 2:** Remove Outline methods from `internal/metrics/collector.go`

Open `internal/metrics/collector.go`. Search for:
- `GetOutlineMetrics` method
- `GetDetailedOutlineMetrics` method
- Any fields related to `outlineCache`, `outlineCacheTime`, `outlineTTL`
- Any TrustTunnel metrics collection

Delete these methods and fields entirely.

Keep:
- `GetMetrics()` - but ensure it doesn't reference outline/detailed
- `GetWireGuardMetrics()`
- WireGuard cache fields

**Step 3:** Verify removal:

```bash
grep -n "Outline" internal/metrics/collector.go
```

Expected: No matches (except maybe in comments).

**Step 4:** Update `internal/metrics/types.go`

Search for structs:
```go
type OutlineMetrics struct { ... }
type DetailedOutlineMetrics struct { ... }
type TrustTunnelMetrics struct { ... }
```

Delete these structs entirely.

Also in `VPNMetrics` struct, remove:
```go
Outline     *OutlineMetrics     `json:"outline,omitempty"`
Detailed    *DetailedMetrics    `json:"detailed,omitempty"`  // Outline detailed
TrustTunnel *TrustTunnelMetrics `json:"trusttunnel,omitempty"`
```

Keep only:
```go
WireGuard *WireGuardMetrics `json:"wireguard"`
```

**Step 5:** Compile check:

```bash
go build ./internal/metrics
go build ./internal/api
```

Expected: Success.

**Step 6:** Commit

```bash
git add internal/metrics/collector.go internal/metrics/types.go internal/api/handlers.go
git commit -m "refactor(metrics): remove Outline and TrustTunnel metrics"
```

---

## Phase 3: Configuration Cleanup

### Task 6: Remove Outline/TrustTunnel Config Variables

**Files:**
- **Modify:** `internal/config/config.go`
- **Modify:** `.env.example`

**Step 1:** Open `internal/config/config.go`

Locate struct:

```go
type Config struct {
    // ...
    OutlineAPIURL      string
    OutlineVerifySSL   bool
    SupportsOutline    bool  // derived
    
    TrustTunnelBinary    string
    TrustTunnelConfigDir string
    TrustTunnelDomain    string
    TrustTunnelPort      int
    TrustTunnelPublicPort int
    // ...
}
```

Delete all these fields.

Also in `Load()` function, remove:
- `viper.GetString("OUTLINE_API_URL")`
- `viper.GetBool("OUTLINE_VERIFY_SSL")`
- `viper.GetString("TRUSTTUNNEL_BINARY")`
- etc. (all 8 variables)

**Step 2:** Remove derived `SupportsOutline` logic (probably set after parsing).

**Step 3:** Compile check:

```bash
go build ./internal/config
```

Expected: Success.

**Step 4:** Update `.env.example`

Open `.env.example`. Delete entire sections:

```
# =============================================================================
# VPN CONFIGURATION
# =============================================================================
# Outline Manager API URL
OUTLINE_API_URL=http://localhost:8081

# =============================================================================
# TRUSTTUNNEL CONFIGURATION
# =============================================================================
# TrustTunnel binary path
TRUSTTUNNEL_BINARY=/opt/trusttunnel/trusttunnel_endpoint
...
```

Keep only WireGuard section:

```
# WireGuard interface name
WG_INTERFACE=wg0

# WireGuard server public IP
WG_SERVER_IP=your-server-public-ip

# WireGuard server listening port
WG_SERVER_PORT=51820
```

**Step 5:** Commit config changes

```bash
git add internal/config/config.go .env.example
git commit -m "refactor(config): remove Outline and TrustTunnel configuration"
```

---

## Phase 4: Main Initialization Cleanup

### Task 7: Update main.go to Not Initialize Outline/TrustTunnel

**Files:**
- **Modify:** `cmd/agent/main.go`

**Step 1:** Open `cmd/agent/main.go`

**Step 2:** Remove Outline client initialization (around lines 75-77):

```go
// Initialize Outline client (commented out or removed)
if cfg.OutlineAPIURL != "" {
    outlineClient = vpn.NewOutlineClient(cfg.OutlineAPIURL, cfg.OutlineVerifySSL)
    log.Info().Str("url", cfg.OutlineAPIURL).Msg("Outline client initialized")
}
```

Delete this block.

**Step 3:** Remove TrustTunnel client initialization (lines ~121-140):

```go
// Initialize TrustTunnel client if binary exists
if cfg.TrustTunnelBinary != "" {
    // ... stats check
    trusttunnelClient = vpn.NewTrustTunnelClient(...)
    trusttunnelMetricsCollector = vpn.NewTrustTunnelMetricsCollector(...)
}
```

Delete entire block.

**Step 4:** Remove `SetOutlineClient()` and `SetTrustTunnelClient()` calls (if any). Verify only `SetWireGuardClient` and `SetMetricsCollector` remain.

**Step 5:** Check that global vars `outlineClient` and `trusttunnelClient` are no longer referenced anywhere else (compiler will catch).

**Step 6:** Compile check:

```bash
go build -o agent ./cmd/agent
```

Expected: Success. Binary size should be smaller (~12-13MB vs ~15MB).

**Step 7:** Commit

```bash
git add cmd/agent/main.go
git commit -m "refactor(main): remove Outline/TrustTunnel initialization"
```

---

### Task 8: Update Registrar to Remove SupportsOutline

**Files:**
- **Modify:** `internal/registrar/registrar.go`

**Step 1:** Open `registrar.go`

Find `RegistrationMetadata` struct (or similar):

```go
type RegistrationMetadata struct {
    Hostname          string `json:"hostname"`
    IPAddress         string `json:"ip_address"`
    CountryCode       string `json:"country_code"`
    CountryName       string `json:"country_name"`
    Region            string `json:"region"`
    City              string `json:"city"`
    AgentVersion      string `json:"agent_version"`
    OSType            string `json:"os_type"`
    OSArch            string `json:"os_arch"`
    AgentURL          string `json:"agent_url"`
    SupportsOutline   bool   `json:"supports_outline"`    // DELETE
    SupportsWireGuard bool   `json:"supports_wireguard"` // KEEP
}
```

Delete `SupportsOutline` field.

**Step 2:** Remove logic that sets `SupportsOutline`. Probably in `collectMetadata()`:

```go
metadata := RegistrationMetadata{
    // ...
    SupportsOutline:   cfg.OutlineAPIURL != "",  // DELETE THIS LINE
    SupportsWireGuard: true,
}
```

Remove line.

**Step 3:** Verify no other references to `SupportsOutline`.

```bash
grep -rn "SupportsOutline" internal/registrar/
```

Expected: No matches.

**Step 4:** Compile check:

```bash
go build ./internal/registrar
```

Expected: Success.

**Step 5:** Commit

```bash
git add internal/registrar/registrar.go
git commit -m "refactor(registrar): remove supports_outline from registration metadata"
```

---

## Phase 5: Dependency Cleanup

### Task 9: Remove Unused Dependency go-toml/v2

**Files:**
- **Modify:** `go.mod`, `go.sum`

**Step 1:** Check go-toml is truly unused:

```bash
grep -rn "pelletier/go-toml" --include="*.go" .
```

Expected: Only in `go.mod`, `go.sum`, maybe README. No code references.

**Step 2:** Remove dependency:

```bash
go mod edit -drop github.com/pelletier/go-toml/v2
```

**Step 3:** Tidy go.mod:

```bash
go mod tidy
```

**Step 4:** Verify no unused deps remain:

```bash
go mod verify
go mod graph | grep go-toml
```

Expected: No output.

**Step 5:** Commit dependency removal

```bash
git add go.mod go.sum
git commit -m "chore(deps): remove unused go-toml/v2 dependency"
```

---

## Phase 6: Documentation Updates

### Task 10: Update README.md

**Files:**
- **Modify:** `README.md`

**Step 1:** Open README.md

**Step 2:** Remove "TrustTunnel Support" section entirely (approx 20 lines).

**Step 3:** Update "Features" section:

**Before:**
```markdown
## Features

### VPN Management
- ✅ **Outline Manager Integration**
- ✅ **WireGuard Integration**
- ✅ **TrustTunnel Support**
```

**After:**
```markdown
## Features

### VPN Management
- ✅ **WireGuard Integration** - Create/delete peers via wg commands
```

Remove Outline/TrustTunnel bullets.

**Step 4:** Update "Architecture" diagram if it shows three VPN types. Simplify to show only WireGuard.

**Step 5:** Update any mentions in README:
- Search "Outline" → remove or rephrase
- Search "TrustTunnel" → remove
- Example `.env` snippet → remove Outline/TrustTunnel lines

**Step 6:** Commit

```bash
git add README.md
git commit -m "docs: remove Outline and TrustTunnel from README"
```

---

### Task 11: Update DEPLOYMENT.md

**Files:**
- **Modify:** `DEPLOYMENT.md`

**Step 1:** Remove entire "TrustTunnel Configuration" section.

**Step 2:** Remove any "Outline" prerequisites and setup steps.

**Step 3:** Keep only WireGuard-related deployment instructions.

**Step 4:** Commit

```bash
git add DEPLOYMENT.md
git commit -m "docs: remove Outline/TrustTunnel from deployment guide"
```

---

### Task 12: Update .env.example

**Files:**
- **Modify:** `.env.example`

**Step 1:** Remove these environment variable blocks:
- `# Outline Manager API URL`
- `# Outline configuration` (any Outline section)
- Entire `# =============================================================================`
- `# TRUSTTUNNEL CONFIGURATION` section

**Step 2:** Result should only have:

```
AGENT_PORT=8080
LOG_LEVEL=INFO
LOG_FORMAT=json
BACKEND_URL=http://your-backend-url.com
AGENT_API_KEY=agent_your-api-key-here
SERVER_ID=
AGENT_URL=http://localhost:8080
SUPPORTS_WIREGUARD=true  # or remove entirely if no longer needed
WG_INTERFACE=wg0
WG_SERVER_IP=your-server-public-ip
WG_SERVER_PORT=51820
# WireGuard network config
WIREGUARD_NETWORK_CIDR=10.0.0.0/24
WIREGUARD_START_IP=2
WIREGUARD_END_IP=254
# Security & rate limiting...
```

**Step 3:** Commit

```bash
git add .env.example
git commit -m "config: remove Outline/TrustTunnel env vars from example"
```

---

### Task 13: Update AUTO-REGISTRATION-GUIDE.md

**Files:**
- **Modify:** `AUTO-REGISTRATION-GUIDE.md`

**Step 1:** Open file. Search for "supports_outline".

Found in section "Metadata Collected" table:

| Field | Description |
|-------|-------------|
| `supports_outline` | Outline support |

Delete this row.

Also delete `supports_outline` from example JSON payloads:

```json
{
  "supports_outline": true,   // DELETE THIS LINE
  "supports_wireguard": true
}
```

**Step 2:** Remove any text mentioning Outline in the guide (except possibly historical notes).

**Step 3:** Ensure guide now only discusses WireGuard agent.

**Step 4:** Commit

```bash
git add AUTO-REGISTRATION-GUIDE.md
git commit -m "docs: update auto-registration guide for WireGuard-only agent"
```

---

### Task 14: Update CHANGELOG.md

**Files:**
- **Modify:** `CHANGELOG.md`

**Step 1:** Add new version at top (after [Unreleased]):

```markdown
## [0.12.0] - 2026-05-10

### BREAKING CHANGES

- **Removed Outline VPN support** — Outline Manager integration completely removed
- **Removed TrustTunnel support** — AdGuard TrustTunnel client eliminated
- **WireGuard-only agent** — Agent now manages WireGuard exclusively

### Removed
- 3 Outline API endpoints (`/outline/keys*`)
- 8 TrustTunnel API endpoints (`/trusttunnel/*`)
- `OUTLINE_API_URL`, `OUTLINE_VERIFY_SSL`, `SUPPORTS_OUTLINE` config vars
- All `TRUSTTUNNEL_*` configuration variables
- `github.com/pelletier/go-toml/v2` dependency
- Outline metrics collection (`outline`, `detailed` fields)
- TrustTunnel metrics and export features

### Migration
Existing deployments must:
1. Update backend to accept agents without `supports_outline` field
2. Deploy new agent binary (v0.12.0+)
3. Clean up TrustTunnel config files if present (`/opt/trusttunnel/`)
```

**Step 2:** Ensure `[Unreleased]` section is empty or references future work.

**Step 3:** Commit

```bash
git add CHANGELOG.md
git commit -m "chore: add v0.12.0 changelog with breaking changes"
```

---

### Task 15: Remove/Update Other Documentation

**Files:**
- Delete or archive TrustTunnel-specific docs
- Keep WireGuard docs

**Step 1:** List TrustTunnel/Outline-specific docs:

```bash
find docs -name "*.md" -exec grep -l "TrustTunnel\|Outline" {} \;
```

Expected files:
- `docs/ops/` (directory) - likely TrustTunnel runbooks
- `docs/runbooks/` - possibly TrustTunnel
- `docs/api/` - possibly TrustTunnel API docs

**Step 2:** Remove TrustTunnel-specific documentation:

```bash
# Remove ops/ and runbooks/ if they only contain TrustTunnel content
rm -rf docs/ops
rm -rf docs/runbooks
# Remove any TrustTunnel files in docs/api/
find docs/api -name "*trusttunnel*" -delete
find docs/api -name "*outline*" -delete
```

**Step 3:** Keep WireGuard docs:
- `docs/WIREGUARD-SETUP.md` — keep
- `docs/ip-allocation-implementation-guide.md` — keep
- `docs/alerting/wireguard-ip-allocation-alerts.yaml` — keep

**Step 4:** Commit doc cleanup

```bash
git rm -r docs/ops docs/runbooks
git add -u
git commit -m "docs: remove TrustTunnel/Outline-specific documentation"
```

---

### Task 16: Update scripts/install.sh

**Files:**
- **Modify:** `scripts/install.sh`

**Step 1:** Open install script.

**Step 2:** Remove any sections that:
- Install TrustTunnel binary
- Configure TrustTunnel paths
- Setup Outline API URL

**Step 3:** Ensure script only:
- Creates `/opt/usipipo-agent`
- Copies binary
- Installs systemd service
- Creates usipipo user
- Sets permissions

**Step 4:** Commit

```bash
git add scripts/install.sh
git commit -m "scripts(install): remove TrustTunnel/Outline setup steps"
```

---

## Phase 7: Testing & Validation

### Task 17: Run Full Test Suite

**Objective:** All WireGuard tests pass, no Outline/TrustTunnel tests remain.

**Step 1:** Run all tests:

```bash
cd /home/mowgli/usipipo/agent
go test ./... -v
```

Expected:
- All WireGuard tests pass
- No failures from missing Outline/TrustTunnel code
- No compilation errors

**Step 2:** If failures, debug until clean.

**Step 3:** Run integration tests (if build tag needed):

```bash
go test -tags=integration ./internal/vpn/... -v
```

Expected: Pass.

**Step 4:** Commit test run (no code changes needed, just verification):

```bash
git commit --allow-empty -m "test: verify all tests pass post-refactor"
```

---

### Task 18: Build Binary and Manual Smoke Test

**Step 1:** Build binary for current platform:

```bash
go build -o agent-linux-amd64 ./cmd/agent
```

**Step 2:** Check size:

```bash
ls -lh agent-linux-amd64
```

Compare with pre-refactor (~15MB). Expected: ~12MB or less.

**Step 3:** Create minimal test `.env`:

```bash
cat > test.env <<EOF
AGENT_PORT=8081
BACKEND_URL=http://localhost:8000
AGENT_API_KEY=agent_test_key_123456789012345678901234567890
SERVER_ID=test-server-1
WG_INTERFACE=wg0
WG_SERVER_IP=127.0.0.1
WG_SERVER_PORT=51820
WIREGUARD_NETWORK_CIDR=10.0.0.0/24
WIREGUARD_START_IP=2
WIREGUARD_END_IP=254
EOF
```

**Step 4:** Run agent with test config:

```bash
AGENT_PORT=8081 \
BACKEND_URL=http://localhost:8000 \
AGENT_API_KEY=agent_test_key_123456789012345678901234567890 \
SERVER_ID=test-001 \
WG_INTERFACE=wg0 \
WG_SERVER_IP=127.0.0.1 \
WG_SERVER_PORT=51820 \
WIREGUARD_NETWORK_CIDR=10.0.0.0/24 \
WIREGUARD_START_IP=2 \
WIREGUARD_END_IP=254 \
./agent-linux-amd64 &
AGENT_PID=$!
sleep 2
```

**Step 5:** Test health endpoint:

```bash
curl -s http://localhost:8081/health | jq .
```

Expected:
```json
{
  "status": "healthy",
  "agent_status": "online",
  "wireguard": "online",
  "timestamp": 1234567890
}
```
Note: No `outline` field.

**Step 6:** Test metrics endpoint:

```bash
curl -s http://localhost:8081/metrics | jq .
```

Expected:
- Has `system`, `vpn`, `wireguard` fields
- NO `outline` field at top level
- NO `detailed` field

**Step 7:** Kill agent:

```bash
kill $AGENT_PID
wait $AGENT_PID 2>/dev/null
```

**Step 8:** Commit smoke test results

```bash
git commit --allow-empty -m "test: smoke test WireGuard-only agent"
```

---

## Phase 8: Release Preparation

### Task 19: Update go.mod for Version Bump

**Files:**
- **Modify:** `go.mod` (module version comment optional)

**Step 1:** Update version in `cmd/agent/main.go` if hardcoded:

Search for `const Version` or `var Version`. If exists, update to `v0.12.0`.

**Step 2:** If version is injected via ldflags in build script, no change needed.

**Step 3:** Ensure module path unchanged.

**Step 4:** Commit

```bash
git add cmd/agent/main.go go.mod
git commit -m "chore: bump version to v0.12.0-wireguard-only"
```

---

### Task 20: Create GitHub Release Notes

**Files:**
- **Create:** `RELEASE_NOTES_v0.12.0.md` (optional, or use GitHub release page)

**Content:**

```markdown
## 🎉 Release v0.12.0 — WireGuard Only

This is a **breaking change** release. The agent now supports **WireGuard exclusively**. Outline and TrustTunnel support have been completely removed.

### Breaking Changes

- ❌ `POST /outline/keys` — removed
- ❌ `DELETE /outline/keys/:id` — removed
- ❌ All `/trusttunnel/*` endpoints — removed
- ❌ Configuration vars: `OUTLINE_API_URL`, `OUTLINE_VERIFY_SSL`, `SUPPORTS_OUTLINE`, all `TRUSTTUNNEL_*` — removed
- ❌ `SupportsOutline` field in registration metadata — removed
- ❌ `outline` and `detailed` fields in metrics payload — removed

### What Stays

- ✅ WireGuard peer management (`/wireguard/peers`)
- ✅ DB-backed IP allocation (if enabled)
- ✅ Metrics collection and reporting
- ✅ Auto-registration
- ✅ Rate limiting, security logging

### Migration Steps

1. **Update backend** to accept agents without Outline fields (must be done first)
2. Deploy new agent binary to all servers
3. Verify health checks: `GET /health` returns `wireguard: "online"`
4. Clean up old TrustTunnel data: `rm -rf /opt/trusttunnel/`

### Binary Size

Reduced from ~15MB to ~12MB (20% smaller).

### Changelog

See [CHANGELOG.md](CHANGELOG.md) for full details.

---

**Download:** [v0.12.0 release assets](https://github.com/uSipipo-Team/usipipo-agent/releases/tag/v0.12.0)
```

**Step:** Save to `RELEASE_NOTES_v0.12.0.md` in repo root.

**Step 5:** Commit

```bash
git add RELEASE_NOTES_v0.12.0.md
git commit -m "release: prepare v0.12.0 notes"
```

---

## Phase 9: Final Verification

### Task 21: Final Build All Platforms

**Step 1:** Build all release binaries via CI or locally:

```bash
# If using goreleaser or Makefile, use that
# Otherwise manual builds:
GOOS=linux GOARCH=amd64 go build -o agent-linux-amd64 ./cmd/agent
GOOS=linux GOARCH=arm64 go build -o agent-linux-arm64 ./cmd/agent
GOOS=darwin GOARCH=amd64 go build -o agent-darwin-amd64 ./cmd/agent
GOOS=darwin GOARCH=arm64 go build -o agent-darwin-arm64 ./cmd/agent
GOOS=windows GOARCH=amd64 go build -o agent-windows-amd64.exe ./cmd/agent
```

**Step 2:** Verify each binary runs (at least check `--version` or `--help` if available):

```bash
./agent-linux-amd64 --version 2>&1 || ./agent-linux-amd64 &
```

**Step 3:** Check binary sizes are reasonable (~12MB).

**Step 4:** Commit (if build scripts changed)

```bash
git add .
git commit -m "build: verify all platform builds succeed"
```

---

### Task 22: Final Repository State Check

**Checklist:**

- [ ] No Outline/TrustTunnel `.go` files remain
- [ ] `go.mod` has no `go-toml/v2`
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `README.md` updated
- [ ] `.env.example` cleaned
- [ ] `CHANGELOG.md` has v0.12.0 entry
- [ ] All 11 endpoints removed (verify in handlers.go)
- [ ] Health response JSON has no `outline` field
- [ ] Metrics response JSON has no `outline` or `detailed`
- [ ] No references to `trusttunnel` or `outline` in any `.go` file:

```bash
grep -r "outline\|trusttunnel" --include="*.go" . && echo "FOUND" || echo "CLEAN"
```

Expected output: `CLEAN`

**Step 1:** Run the grep check:

```bash
cd /home/mowgli/usipipo/agent
grep -ri "outline" --include="*.go" . | grep -v "WireGuard\|wireguard\|Wireguard" && echo "FOUND OUTLINE REFERENCES" || echo "OUTLINE CLEAN"
grep -ri "trusttunnel" --include="*.go" . && echo "FOUND TRUSTTUNNEL REFERENCES" || echo "TRUSTTUNNEL CLEAN"
```

**Step 2:** If any found, investigate and remove.

**Step 3:** Final commit:

```bash
git add -A
git commit -m "refactor: complete WireGuard-only agent removal of Outline/TrustTunnel"
```

---

## Phase 10: Merge & Release

### Task 23: Create Pull Request

**Prerequisites:**
- All tasks complete
- Tests passing
- Branch pushed to GitHub

**Steps:**

1. Push branch to remote:

```bash
git push origin refactor/wireguard-only
```

2. Create PR via gh CLI or GitHub UI:

```bash
gh pr create --title "refactor: Remove Outline and TrustTunnel support (WireGuard-only agent)" \
  --body "Breaking change: removes Outline and TrustTunnel VPN protocols. Agent is now WireGuard-only.

## Changes
- Delete Outline client and 3 API endpoints
- Delete TrustTunnel client and 8 API endpoints
- Remove 8 config vars and go-toml/v2 dependency
- Update docs, README, CHANGELOG, deployment guide

## Migration
Backend must be updated first to accept agents without `supports_outline` field.

## Testing
- [x] All WireGuard tests pass
- [x] Build succeeds on all platforms
- [x] Manual smoke test passed
- [x] No outline/trusttunnel references in code

Fixes: #<issue-number-if-any>"
```

3. Assign reviewers, add label `breaking-change`, `refactor`

---

### Task 24: Post-Merge Actions

After PR merged to `main`:

**Step 1:** Tag release:

```bash
git tag -a v0.12.0 -m "WireGuard-only agent - remove Outline/TrustTunnel"
git push origin v0.12.0
```

**Step 2:** Create GitHub Release:

```bash
gh release create v0.12.0 \
  --title "v0.12.0 — WireGuard Only" \
  --notes-file RELEASE_NOTES_v0.12.0.md \
  --draft
```

Review and publish.

**Step 3:** Update backend documentation link to new agent version.

**Step 4:** Notify team in #dev-channel about breaking change.

---

## Appendix: Quick Reference

### Files to Delete (8)

```
internal/vpn/outline.go
internal/vpn/outline_test.go
internal/vpn/trusttunnel.go
internal/vpn/trusttunnel_metrics.go
internal/vpn/trusttunnel_test.go
internal/vpn/trusttunnel_metrics_test.go
# Plus docs/ops/, docs/runbooks/, docs/api/trusttunnel*, docs/api/outline*
```

### Files to Modify (10)

```
internal/api/handlers.go           # Remove handlers + global vars
internal/metrics/collector.go      # Remove Outline/TT metric methods
internal/metrics/types.go          # Remove Outline/TT structs
internal/config/config.go          # Remove 8 config vars
internal/registrar/registrar.go    # Remove SupportsOutline
cmd/agent/main.go                  # Remove initialization blocks
README.md
DEPLOYMENT.md
.env.example
AUTO-REGISTRATION-GUIDE.md
CHANGELOG.md
scripts/install.sh
go.mod (dependency only)
```

### Dependencies

```
REMOVE: github.com/pelletier/go-toml/v2
KEEP: Everything else
```

### API Endpoints Removed (11)

```
POST   /outline/keys
DELETE /outline/keys/:id
POST   /outline/keys/:id/regenerate

POST   /trusttunnel/clients
DELETE /trusttunnel/clients/:username
GET    /trusttunnel/clients
POST   /trusttunnel/clients/:username/export
POST   /trusttunnel/clients/:username/export-deeplink
GET    /trusttunnel/metrics
POST   /trusttunnel/rules
DELETE /trusttunnel/rules
```

---

## Checklist for Completion

- [ ] Phase 1: Outline/TrustTunnel packages deleted (Tasks 1-2)
- [ ] Phase 2: Handlers removed (Tasks 3-5)
- [ ] Phase 3: Config vars removed (Task 6)
- [ ] Phase 4: main.go cleaned (Task 7), registrar updated (Task 8)
- [ ] Phase 5: go-toml/v2 removed (Task 9)
- [ ] Phase 6: All docs updated (Tasks 10-16)
- [ ] Phase 7: Tests pass, smoke test passes (Tasks 17-18)
- [ ] Phase 8: Version bumped, release notes ready (Tasks 19-20)
- [ ] Phase 9: Final verification (Task 21-22)
- [ ] Phase 10: PR created, merged, tagged (Tasks 23-24)

---

**Plan Version:** 1.0  
**Estimated Tasks:** 24  
**Estimated Time:** 4-6 hours (including review, testing)  
**Risk Level:** Medium (requires backend coordination)
