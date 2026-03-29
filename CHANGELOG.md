# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Planned
- Trust Tunnel (AdGuard) integration
- Docker container support
- Automatic failover between servers
- Real-time latency monitoring via WebSocket
- Automatic certificate renewal
- Multi-WAN support
- GeoDNS integration

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
