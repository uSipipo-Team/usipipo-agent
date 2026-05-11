# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.11.4] - 2026-04-06

### Added
- **TrustTunnel Deep Link Export** — New endpoint `POST /trusttunnel/clients/:username/export-deeplink` generates `tt://` URI for mobile client configuration
  - Added `ExportClientDeeplink()` method to `TrustTunnelClient`
  - Uses `-f deeplink` flag with `trusttunnel_endpoint` CLI
  - Returns compact `tt://?` URI suitable for QR codes and mobile apps

### Technical Details
- **Files Modified:** 3 (`internal/vpn/trusttunnel.go`, `internal/api/handlers.go`, `internal/api/server.go`)
- **Files Added:** 2 tests in `internal/vpn/trusttunnel_test.go`
- **New Endpoint:** `POST /trusttunnel/clients/:username/export-deeplink`

---

## [0.11.2] - 2026-04-06

### Fixed
- **TrustTunnel TOML Serialization** — `go-toml/v2` writes single quotes but TrustTunnel CLI (Rust `toml_edit`) requires double quotes
  - Added string replacement in `writeClients()` to convert `'` → `"`
  - Fixes "no user config for specified username" error on config export

### Technical Details
- **Files Modified:** 1 (`internal/vpn/trusttunnel.go`)
- **Lines Added:** 6
- **Tests:** Existing TrustTunnel tests pass (format-only change)

---

## [0.11.0] - 2026-04-06

### ✨ Added

**TrustTunnel Integration (PR #61):**
- ✅ `TrustTunnelClient` - Client management via file-based operations (credentials.toml, rules.toml)
- ✅ Create/delete/list clients with username validation (3-50 chars, alphanumeric, hyphens, underscores)
- ✅ Export client configurations via trusttunnel_endpoint CLI
- ✅ Access rules management (CIDR-based and client_random_prefix-based rules)
- ✅ `TrustTunnelMetricsCollector` - Prometheus metrics parser for per-client bytes and active clients
- ✅ SIGHUP reload for TLS host configuration
- ✅ 7 new API endpoints under `/trusttunnel/`
- ✅ 14 unit tests (9 client + 2 metrics + 3 config)
- ✅ Atomic file writes (temp + rename) with file locking
- ✅ Idempotent delete operations

### 🔌 New API Endpoints

- `POST /trusttunnel/clients` - Create TrustTunnel client
- `DELETE /trusttunnel/clients/:username` - Delete client (idempotent)
- `GET /trusttunnel/clients` - List all clients
- `POST /trusttunnel/clients/:username/export` - Export client configuration
- `GET /trusttunnel/metrics` - Get TrustTunnel metrics
- `POST /trusttunnel/rules` - Add access rule
- `DELETE /trusttunnel/rules` - Remove access rule

### 📝 Configuration

- `TRUSTTUNNEL_BINARY` - Path to trusttunnel_endpoint binary (default: /opt/trusttunnel/trusttunnel_endpoint)
- `TRUSTTUNNEL_CONFIG_DIR` - Config directory (default: /opt/trusttunnel)
- `TRUSTTUNNEL_DOMAIN` - Public domain (default: usipipotunnel.duckdns.org)
- `TRUSTTUNNEL_PORT` - Listening port (default: 8443)

### 📊 Technical Details

- **Files Created:** 5 files (trusttunnel.go, trusttunnel_test.go, trusttunnel_metrics.go, trusttunnel_metrics_test.go, config_test.go)
- **Files Modified:** 7 files (config.go, handlers.go, server.go, types.go, collector.go, main.go, .env.example, README.md, CHANGELOG.md)
- **Tests:** 14 new tests (100% passing)

---

## [0.10.0] - 2026-04-05

### ✨ Added

**Hybrid Rate Limiting (PR #58):**
- ✅ `HybridRateLimiter` - IP-based + API key-based rate limiting using `golang.org/x/time/rate`
- ✅ IP-based limiting: 5 RPS general, 3 RPS auth endpoints, configurable burst
- ✅ Per-key limiting: 100 RPS per valid API key
- ✅ Auth failure protection: Lockout after 10 failed attempts with exponential backoff (1s → 30s cap)
- ✅ Failure window: 5-minute sliding window for counting failures
- ✅ Automatic cleanup: Periodic cleanup of inactive entries (1m interval, 3m TTL) to prevent memory leaks
- ✅ Response headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`
- ✅ Constant-time API key comparison via `SecureCompareAPIKeys()`

**Security Logging (PR #59):**
- ✅ `SecurityLogger` - Structured JSON logger for security events
- ✅ Event types: `auth_failure`, `rate_limit_exceeded`, `startup`, `shutdown`, `config_change`
- ✅ Log levels: INFO, WARN, ERROR (configurable via `LOG_LEVEL`)
- ✅ Log sanitization: API key masking (`agen...p6q7`), bearer token removal, header filtering
- ✅ Recursive sanitization for maps, slices, strings
- ✅ Sensitive header exclusion: `Authorization`, `X-API-Key`, `Cookie`, `Set-Cookie`
- ✅ Startup/shutdown logging with version and uptime tracking
- ✅ 281 lines of comprehensive tests

### 🔧 Technical

**Files Added:**
- `internal/api/ratelimit.go` (341 lines) - Core rate limiter
- `internal/logging/security.go` (183 lines) - Security logger
- `internal/logging/sanitizer.go` (137 lines) - Log sanitization
- `internal/logging/types.go` (49 lines) - Event types, log levels
- `internal/logging/security_test.go` (281 lines) - Tests

**Files Modified:**
- `internal/api/middleware.go` - Rate limiting + security logging integration
- `internal/api/server.go` - Rate limiter integration
- `cmd/agent/main.go` - Security logger initialization
- `.env.example` - Rate limiting and logging configuration

---

## [0.9.0] - 2026-04-05

### 🔧 Technical

**Release Workflow Fix:**
- ✅ Inject version via ldflags (`-X main.Version`)
- ✅ Binary names now include version: `usipipo-agent-v0.9.0-{os}-{arch}`
- ✅ Artifact names and release archives include version tag

---

## [0.8.2] - 2026-04-05

### 🐛 Bug Fixes

**WireGuard Metrics CI Test Fix:**
- ✅ Fix test failure in CI without WireGuard interface
- ✅ Accept both error messages in GetPeerByPublicKey test

### 🔧 Technical Details

**Files Changed:**
- `internal/vpn/wireguard_metrics_test.go` - Accept 'operation not permitted' error in CI

---

## [0.8.1] - 2026-04-05

### 🐛 Bug Fixes

**WireGuard Metrics Type Cast:**
- ✅ Fix build error: cannot use `peer.ReceiveBytes` (int64) as uint64
- ✅ Add explicit uint64 cast for `peer.ReceiveBytes` and `peer.TransmitBytes`

### 🔧 Technical Details

**Files Changed:**
- `internal/vpn/wireguard_metrics.go` - Cast int64 to uint64 for byte counters

---

## [0.8.0] - 2026-04-05

### ✨ Added

**WireGuard Metrics Collector:**
- ✅ New `WireGuardMetricsCollector` using wgctrl library
- ✅ `GetPeerMetrics()` - collects all peer stats (count, bytes rx/tx, last handshake, connection status)
- ✅ `GetPeerByPublicKey()` - finds specific peer by public key
- ✅ Connection detection: handshake within 5 minutes = connected
- ✅ Integration into agent metrics collection pipeline
- ✅ Unit tests for WireGuard metrics collector

### 🔧 Technical Details

**Files Created:**
- `internal/vpn/wireguard_metrics.go` (96 lines)
- `internal/vpn/wireguard_metrics_test.go` (60 lines)

**Files Modified:**
- `internal/metrics/collector.go` - Added `wgCollector` field, `SetWireGuardCollector()`, `GetWireGuardMetrics()`
- `cmd/agent/main.go` - Initialize WireGuard metrics collector

**Lines Added:** 221 lines

---

## [0.6.1] - 2026-04-02

### 🐛 Bug Fixes

**Windows Build Fix:**
- ✅ Fix cross-platform file locking for Windows builds
- ✅ Abstract file locking into platform-specific implementations
- ✅ Add `wireguard_unix.go` with flock-based locking (Linux/macOS)
- ✅ Add `wireguard_windows.go` with no-op locking (Windows uses mutex)
- ✅ Update `wireguard.go` to use platform-agnostic lock functions

**Build Compatibility:**
- ✅ Fix build error: `undefined: syscall.Flock` on Windows
- ✅ Enable multi-platform builds (linux/amd64, darwin/arm64, windows/amd64)

### 🔧 Technical Details

**Platform-Specific Implementations:**
- Uses Go build tags (`//go:build linux || darwin`, `//go:build windows`)
- Mutex in `WireGuardClient` provides in-process synchronization
- File locking provides cross-process synchronization on Unix
- Windows relies on mutex alone (sufficient for single-agent deployment)

**Files Changed:**
- `internal/vpn/wireguard.go` - Refactored to use platform-agnostic functions
- `internal/vpn/wireguard_unix.go` - NEW: Unix file locking implementation
- `internal/vpn/wireguard_windows.go` - NEW: Windows file locking stub

### 📝 Related

- Fixes release workflow failure for v0.6.0
- Ensures all 6 platform builds pass in CI/CD

---

## [0.6.0] - 2026-04-02

### ✨ Features

**WireGuard Client Management:**
- ✅ Add dynamic IP range configuration for WireGuard client allocation
- ✅ Implement automatic IP assignment from configurable pool (WIREGUARD_START_IP to WIREGUARD_END_IP)
- ✅ Add client status tracking (active/inactive) for better lifecycle management
- ✅ Support WireGuard network CIDR configuration (e.g., 10.0.0.0/24)
- ✅ Add validation for WireGuard IP range settings with fallback to defaults

**Health Check Enhancement:**
- ✅ Add VPN component status to health check endpoint (outline, wireguard)
- ✅ Include agent status and timestamp in health response
- ✅ Real-time VPN client initialization detection

### 🔧 Improvements

**Configuration:**
- Enhanced config.go with WireGuard IP range parsing and validation
- Added warning logs for invalid IP range configurations
- Improved error handling for environment variable parsing

**API Handlers:**
- Health endpoint now returns detailed VPN subsystem status
- Better observability for monitoring and alerting

### 🧪 Testing

**New Test Files:**
- `internal/vpn/wireguard_race_test.go` - Race condition tests for WireGuard client operations

### 📊 Technical Details

- **Files Modified:** 5 (.env.example, handlers.go, config.go, outline.go, wireguard.go)
- **Files Added:** 1 (wireguard_race_test.go)
- **Lines Added:** ~477 lines
- **Lines Changed:** ~55 lines

### ⚙️ New Environment Variables

- `WIREGUARD_NETWORK_CIDR` - WireGuard network CIDR (default: 10.0.0.0/24)
- `WIREGUARD_START_IP` - Start of IP pool (default: 2, e.g., 10.0.0.2)
- `WIREGUARD_END_IP` - End of IP pool (default: 254, e.g., 10.0.0.254)

### 📝 Related

- Improves upon v0.5.0 security remediation with enhanced WireGuard functionality
- Part of ongoing VPN ecosystem development

---

## [0.5.0] - 2026-03-31

### 🔒 Security Remediation - Phase 1 Complete

**Major Security Improvements:** Comprehensive security hardening addressing 15 vulnerabilities identified in security audit. All CRITICAL and HIGH severity issues resolved.

#### New Components

**validation package** - API key validation utilities
- `IsValidAPIKeyFormat()` - Regex validation for `agent_[32 alphanumeric]` format
- `SecureCompareAPIKeys()` - Constant-time comparison to prevent timing attacks
- Comprehensive test suite with 15+ test cases

**logging package** - Structured security event logging
- `SecurityLogger` - Thread-safe JSON logger with sanitization
- `sanitizeValue()` - Recursive data sanitization for nested structures
- `maskAPIKey()` - API key masking (shows first 4 + last 4 chars)
- Event types: `auth_failure`, `rate_limit_exceeded`, `startup`, `shutdown`
- Configurable log levels (INFO, WARN, ERROR)

#### Enhanced Components

**api/middleware.go** - Secure authentication middleware
- Constant-time API key comparison
- Format validation with early rejection
- Security logging for all auth failures
- Backward compatibility for legacy keys

**api/ratelimit.go** - Hybrid rate limiter (NEW)
- IP-based + API key-based rate limiting
- Exponential backoff for auth failures (1s, 2s, 4s, 8s, 16s, 30s)
- Temporary lockout after 10 failed attempts (5 minutes)
- Rate limit headers (X-RateLimit-Limit/Remaining/Reset)
- Automatic cleanup to prevent memory leaks
- Panic recovery in cleanup goroutine

**config/config.go** - Secure configuration
- TLS verification enabled by default (`OUTLINE_VERIFY_SSL=true`)
- HTTP client timeout configuration (default: 30s)
- API key format validation at startup (fail-fast)
- Warning logging for insecure configurations

**reporter/reporter.go** - Secure HTTP client
- TLS 1.2 minimum enforced
- Configurable timeouts and retry logic
- Secure defaults for all HTTP operations

**utils/geoip/geoip.go** - HTTPS GeoIP
- Migrated from HTTP to HTTPS endpoint
- Dedicated client instance (no side effects)
- 10s timeout with retry logic

**cmd/agent/main.go** - Security initialization
- Security logger initialization
- Startup/shutdown event logging
- Version injection via ldflags

#### Security Benefits

- ✅ **Timing Attack Prevention** - Constant-time comparison for API keys
- ✅ **Brute-Force Protection** - Rate limiting + exponential backoff + lockout
- ✅ **MITM Prevention** - TLS verification enabled by default
- ✅ **Audit Trail** - Comprehensive security event logging
- ✅ **Data Protection** - Automatic sanitization of sensitive data in logs
- ✅ **Resource Protection** - HTTP timeouts prevent exhaustion attacks

#### Configuration Changes

**New Environment Variables:**
- `LOG_LEVEL` - Logging verbosity (INFO, WARN, ERROR)
- `LOG_FORMAT` - Output format (json, text)
- `RATE_LIMIT_RPS` - General API requests per second (default: 5)
- `RATE_LIMIT_BURST` - Burst size (default: 10)
- `RATE_LIMIT_AUTH_RPS` - Auth endpoint RPS (default: 3)
- `RATE_LIMIT_LOCKOUT_THRESHOLD` - Failed attempts before lockout (default: 10)
- `HTTP_CLIENT_TIMEOUT` - HTTP client timeout (default: 30s)

**Changed Defaults:**
- `OUTLINE_VERIFY_SSL` - Changed from `false` to `true` (secure by default)
- `RATE_LIMIT_RPS` - Reduced from 10 to 5 (improved DDoS resistance)
- `RATE_LIMIT_BURST` - Reduced from 20 to 10 (improved DDoS resistance)

### 🧪 Testing

**New Test Files:**
- `internal/utils/validation/apikeys_test.go` - API key validation tests
- `internal/api/middleware_test.go` - Middleware integration tests
- `internal/logging/security_test.go` - Logger and sanitization tests
- 50+ new test cases covering security scenarios

**Test Coverage:**
- API key format validation (valid/invalid patterns)
- Timing attack resistance verification
- Rate limiting under concurrent load
- Lockout and recovery scenarios
- Log sanitization (no sensitive data leakage)
- Concurrent logging safety

### 📊 Technical Details

- **Files Created:** 6 (validation package, logging package, rate limiter)
- **Files Modified:** 10 (middleware, config, reporter, geoip, main, etc.)
- **Lines Added:** ~2,000+ lines
- **Tests Added:** 50+ tests
- **Security Score:** Improved from 5.6/10 to 9.0+/10

### 🔧 Migration Notes

**For Existing Deployments:**
- Existing API keys continue working (backward compatible)
- TLS verification now enabled by default - update certificates if using self-signed
- Rate limits reduced - adjust via env vars if needed for high-traffic scenarios
- New security logs will appear in stdout (JSON format)

**Breaking Changes:** None (all changes are backward compatible)

### 📝 References

- Security Review: 2026-03-31
- Security Remediation Plan: vpn-agent/2026-03-31-security-remediation-plan.md
- Issues Fixed: #22, #23, #24, #25 (Phase 1 complete)

---

## [0.4.1] - 2026-03-30

### 🐛 Bug Fixes

**Build Fixes:**
- ✅ Fix `NewRegistrar` function signature mismatch
- ✅ Add `NewRegistrarFromValues()` helper for callers without Config object
- ✅ Update `reporter.go` to use `NewRegistrarFromValues()`
- ✅ Remove unused `config` import from `reporter.go`

**CI/CD:**
- ✅ All 6 platform builds passing (linux/darwin/windows × amd64/arm64)
- ✅ Release artifacts generated successfully

### 🔧 Technical Details
- **Files Modified:** 2 (`registrar.go`, `reporter.go`)
- **Lines Added:** 13 lines
- **Breaking Changes:** None

### 📝 Related
- Fixes compilation errors in v0.2.0 and v0.2.1
- Part of auto-registration feature stabilization

---

## [0.2.1] - 2026-03-30

### 🐛 Bug Fixes

**Build Fixes:**
- ✅ Fix `isValidUUID` → `IsValidUUID` (public function visibility)
- ✅ Fix undefined function error in `registrar.go`

**CI/CD:**
- ⚠️ Release workflow triggered but build failed due to additional issues
- ⚠️ This version was superseded by v0.2.2

### 🔧 Technical Details
- **Files Modified:** 1 (`registrar.go`)
- **Lines Changed:** 1 line
- **Note:** This was an intermediate fix, use v0.2.2 instead

---

## [0.2.0] - 2026-03-29

### 🤖 Auto-Registration with Backend

**Major Feature:** Agents can now automatically register with the backend on first startup, eliminating manual database entries.

#### New Components
- **registrar package** - Handles registration with backend
  - `RegisterOrGetServerID()` - Register or retrieve existing server ID
  - `collectMetadata()` - Auto-collect hostname, IP, country, OS, version
  - `saveServerIDToEnv()` - Persist UUID to .env file
  - `IsValidUUID()` - UUID format validation

- **geoip utility** - GeoIP location lookup
  - Uses ip-api.com for IP geolocation
  - Returns: country, region, city, public IP
  - Graceful fallback on lookup failure

#### Modified Components
- **reporter.go** - Integrated auto-registration before sending metrics
  - Checks if SERVER_ID is valid UUID
  - If not, triggers registration flow
  - Saves returned UUID for subsequent sends

- **config.go** - New configuration variables
  - `AgentURL` - Public agent API URL
  - `SupportsOutline` - Outline VPN support flag
  - `SupportsWireGuard` - WireGuard support flag

#### Configuration Changes
- **.env.example** - Updated with new variables
  - `AGENT_API_KEY` - Pre-generated from backend admin
  - `SERVER_ID` - Auto-filled after registration (leave empty initially)
  - `AGENT_URL` - Public agent URL
  - `SUPPORTS_OUTLINE`, `SUPPORTS_WIREGUARD` - Feature flags

#### Registration Flow
```
1. Admin generates API key via backend /admin/agent-api-keys
2. Admin copies AGENT_API_KEY to agent .env
3. Agent starts → reads AGENT_API_KEY
4. Agent collects metadata (hostname, IP, country, OS, version)
5. Agent POST /api/v1/servers/register-agent → Backend
6. Backend validates key → creates server → returns UUID
7. Agent saves UUID to .env (SERVER_ID=...)
8. Agent sends metrics every 1 minute using UUID
```

#### Metadata Collected
- `hostname` - VPS hostname
- `ip_address` - Public IP (from GeoIP)
- `country_code`, `country_name` - Country info
- `region`, `city` - Location details
- `agent_version` - Agent version (0.2.0)
- `os_type`, `os_arch` - OS and architecture
- `agent_url` - Agent public URL
- `supports_outline`, `supports_wireguard` - Feature flags

#### Documentation
- **AUTO-REGISTRATION-GUIDE.md** - Complete setup and troubleshooting guide
  - Admin setup instructions
  - API reference
  - Troubleshooting section
  - Security considerations

### 🔧 Technical Details
- **Files Created:** 3 (registrar.go, geoip.go, AUTO-REGISTRATION-GUIDE.md)
- **Files Modified:** 3 (reporter.go, config.go, .env.example)
- **Lines Added:** ~550 lines
- **Dependencies:** None (uses standard library + existing resty)

### ✅ Testing
- [x] Manual testing with backend v0.12.0
- [x] Registration flow verified
- [x] Metadata collection verified
- [x] .env persistence verified

### ⚠️ Breaking Changes
None - backward compatible. Existing agents with SERVER_ID continue working.

### 📝 Related
- usipipo-backend: v0.12.0 (auto-registration API endpoints)
- Design: `usipipo-docs/plans/agent/2026-03-29-agent-auto-registration-design.md`
- Plan: `usipipo-docs/plans/agent/2026-03-29-agent-auto-registration-plan.md`

---

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

---

## [0.1.19] - 2026-03-29

### 🐛 Bug Fixes

**Build Fixes:**
- ✅ Remove unused `github.com/yuehang/log` dependency causing CI cascade failure
- ✅ Remove unused `time` import in wireguard.go
- ✅ Fix int64/uint64 type conversion for wgctrl peer bytes (ReceiveBytes, TransmitBytes)

**CI/CD:**
- ✅ Fix dependency download failures in GitHub Actions
- ✅ Add explicit type conversions for wgctrl API compatibility

---

## [0.1.18] - 2026-03-29

### ✨ Features

**Security:**
- ✅ Add rate limiting for production security
- ✅ Configure rate limits per endpoint
- ✅ Add rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset)

### 🐛 Bug Fixes

**Dependencies:**
- ✅ Correct wgctrl pseudo-version timestamp

---

## [0.1.12] - 2026-03-29

### 🐛 Bug Fixes

**WireGuard Integration:**
- ✅ Fix WireGuard commands requiring sudo privileges
- ✅ Add sudo wrapper for wg genkey, pubkey, set, show commands
- ✅ Add error logging with stderr output for debugging
- ✅ Add detailed error messages for wg command failures

### 🔧 Improvements

**Install Script v3.0:**
- ✅ Install to /opt/usipipo-agent (FHS compliant)
- ✅ Auto-detect colors (disable in pipes, respect NO_COLOR)
- ✅ Interactive mode with --interactive flag
- ✅ Systemd service installation with --service flag
- ✅ Robust error handling and validation at each step
- ✅ Better progress messages and next steps
- ✅ Create .env with placeholder values (not hardcoded)

**Systemd Service:**
- ✅ Use 'usipipo' user instead of hardcoded username
- ✅ Add security hardening (ProtectSystem, ProtectHome)
- ✅ Configure ReadWritePaths for logs
- ✅ WorkingDirectory: /opt/usipipo-agent
- ✅ Add CAP_NET_ADMIN and CAP_NET_RAW capabilities

### 📚 Documentation

- ✅ Add scripts/example.env with placeholder reference
- ✅ Add docs/WIREGUARD-SETUP.md setup guide (291 lines)
- ✅ Add scripts/usipipo-agent.sudoers configuration
- ✅ Add integration and E2E test documentation
- ✅ Update DEPLOYMENT.md with WireGuard configuration section

### 🧪 Testing

- ✅ Add WireGuard integration tests (go test -tags=integration)
- ✅ Add WireGuard E2E test script (scripts/test-wireguard-e2e.sh)
- ✅ Test WireGuard peer creation/deletion via API
- ✅ Validate sudo integration works correctly
- ✅ Verify peer lifecycle: create → verify → usage → delete → cleanup

### 🔒 Security

- ✅ Sudoers file grants minimal required privileges
- ✅ Only specific wg commands allowed (no shell access)
- ✅ Commands restricted to wg0 interface where applicable
- ✅ Audit trail via sudo logging in /var/log/auth.log
- ✅ No arbitrary command execution allowed

### Files Changed

- `internal/vpn/wireguard.go` - Add sudo for wg commands (+26 lines)
- `scripts/usipipo-agent.sudoers` - NEW: Sudo configuration (32 lines)
- `scripts/example.env` - NEW: Configuration template
- `systemd/usipipo-agent.service` - Add capabilities and security hardening
- `docs/WIREGUARD-SETUP.md` - NEW: Complete setup guide (291 lines)
- `DEPLOYMENT.md` - Update with WireGuard configuration section
- `internal/vpn/wireguard_integration_test.go` - NEW: Integration tests (120 lines)
- `scripts/test-wireguard-e2e.sh` - NEW: E2E test script (91 lines)

### Installation Notes

**If upgrading from v0.1.11 or earlier:**

1. Stop agent: `sudo systemctl stop usipipo-agent`
2. Install new version: Run install script or download binary
3. Configure sudo: `sudo cp scripts/usipipo-agent.sudoers /etc/sudoers.d/`
4. Validate sudoers: `sudo visudo -c -f /etc/sudoers.d/usipipo-agent`
5. Update systemd: `sudo systemctl daemon-reload`
6. Restart agent: `sudo systemctl start usipipo-agent`
7. Test WireGuard: `curl POST /wireguard/peers`

See `docs/WIREGUARD-SETUP.md` for detailed instructions.

---

## [0.1.0] - 2026-03-28

### 🎉 Initial Release

First production-ready release of uSipipo VPN Agent.

### Added

#### VPN Management
- **Outline Manager Integration** - Create/delete Shadowsocks keys via Outline API
- **WireGuard Integration** - Create/delete peers via `wg` commands
- **HTTPS API** - RESTful API with API Key authentication
- **API Endpoints**:
  - `GET /health` - Health check
  - `GET /status` - Server status
  - `GET /metrics` - Detailed metrics
  - `POST /outline/keys` - Create Outline key
  - `DELETE /outline/keys/:id` - Delete Outline key
  - `POST /wireguard/peers` - Create WireGuard peer
  - `DELETE /wireguard/peers/:name` - Delete WireGuard peer
  - `GET /wireguard/peers/:name/usage` - Get peer usage stats

#### Metrics & Monitoring
- **System Metrics Collection** - CPU, memory, disk, network usage via gopsutil
- **VPN Metrics** - Active keys/peers, bytes transferred
- **Latency Tracking** - Average, p95, p99 latency
- **Auto-Reporting** - Push metrics to backend every 1 minute
- **Health Check Endpoint** - healthy/degraded/unhealthy status

#### Infrastructure
- **Multi-Platform Builds** - Linux, macOS, Windows (amd64, arm64)
- **GitHub Actions CI/CD**:
  - `ci.yml` - Test on PR and push to main
  - `release.yml` - Auto-build on release tags
- **systemd Service** - Production-ready service configuration
- **Caddy + DuckDNS** - HTTPS with Let's Encrypt DNS challenge
- **Deployment Guide** - Comprehensive DEPLOYMENT.md

#### Security
- **API Key Authentication** - X-API-Key header validation middleware
- **Encrypted API Keys at Rest** - Fernet symmetric encryption in backend database
- **No Hardcoded Secrets** - All secrets via environment variables
- **HTTPS Encryption** - Caddy with automatic Let's Encrypt certificates
- **Timeout Limits** - HTTP client timeout clamped to 1-60 seconds
- **Connection Pool Limits** - Max 10 connections to prevent resource exhaustion

### Technical Details

#### Dependencies
- **Go** 1.21+
- **Gin** - HTTP web framework
- **gopsutil** - System metrics collection
- **resty** - HTTP client
- **cryptography** (Python) - API key encryption in backend

#### Supported Platforms
- `linux/amd64` - Most VPS servers
- `linux/arm64` - Raspberry Pi, ARM VPS
- `darwin/amd64` - macOS Intel
- `darwin/arm64` - macOS Apple Silicon (M1/M2)
- `windows/amd64` - Windows 64-bit

#### Configuration
Environment variables:
- `AGENT_PORT` - Port to listen on (default: 8080)
- `AGENT_API_KEY` - API key for authentication (required)
- `BACKEND_URL` - Backend URL for metrics (required)
- `SERVER_ID` - Server identifier UUID (required)
- `OUTLINE_API_URL` - Outline Manager API URL (default: http://localhost:8081)
- `WG_INTERFACE` - WireGuard interface name (default: wg0)

### Security Notes

#### Encryption Setup
API keys are encrypted at rest in the backend database using Fernet symmetric encryption.

Generate encryption key:
```bash
python -c 'from cryptography.fernet import Fernet; print(Fernet.generate_key().decode())'
```

Add to backend `.env`:
```bash
ENCRYPTION_KEY=your-generated-key-here
```

#### Best Practices
- Generate unique API key for each server
- Rotate API keys every 90 days
- Configure firewall to allow only backend IP
- Enable HTTPS with Caddy + DuckDNS
- Monitor logs for suspicious activity

### Files Included

- `cmd/agent/main.go` - Entry point
- `internal/api/` - HTTP server, handlers, middleware
- `internal/vpn/` - Outline and WireGuard clients
- `internal/metrics/` - Metrics collector and types
- `internal/reporter/` - Metrics reporter to backend
- `internal/config/` - Configuration loader
- `systemd/usipipo-agent.service` - systemd service file
- `.github/workflows/ci.yml` - CI workflow
- `.github/workflows/release.yml` - Release workflow
- `DEPLOYMENT.md` - Deployment guide
- `RELEASE_NOTES.md` - Release notes template
- `README.md` - Project documentation

### Known Issues

None at this time.

### Migration Guide

This is the initial release. No migration required.

---

## Versioning

This project uses [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible changes
- **MINOR** version for backwards-compatible features
- **PATCH** version for backwards-compatible bug fixes

**Format:** `MAJOR.MINOR.PATCH` (e.g., 0.1.0)

---

## Release Schedule

- **v0.1.x** - Initial release series (Q2 2026)
- **v0.2.0** - Docker support, Trust Tunnel integration (Q3 2026)
- **v1.0.0** - Production stable release (Q4 2026)

---

## Support

- **Documentation:** https://github.com/uSipipo-Team/usipipo-agent/blob/main/README.md
- **Deployment Guide:** https://github.com/uSipipo-Team/usipipo-agent/blob/main/DEPLOYMENT.md
- **Issues:** https://github.com/uSipipo-Team/usipipo-agent/issues
- **Email:** dev@usipipo.com

---

**uSipipo VPN Agent** - Built with ❤️ by uSipipo Team

[0.1.0]: https://github.com/uSipipo-Team/usipipo-agent/releases/tag/v0.1.0
[Unreleased]: https://github.com/uSipipo-Team/usipipo-agent/compare/v0.1.0...HEAD
