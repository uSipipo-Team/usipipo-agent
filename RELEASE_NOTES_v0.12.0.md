## 🎉 Release v0.12.0 — WireGuard Only

This is a **breaking change** release. The agent now supports **WireGuard exclusively**.

### Breaking Changes
- ❌ `POST /outline/keys`, `DELETE /outline/keys/:id` — removed
- ❌ All `/trusttunnel/*` endpoints — removed
- ❌ Config vars: `OUTLINE_API_URL`, `OUTLINE_VERIFY_SSL`, `SUPPORTS_OUTLINE`, all `TRUSTTUNNEL_*` — removed
- ❌ `SupportsOutline` field in registration metadata — removed
- ❌ `outline` and `detailed` fields in metrics payload — removed
- ❌ `github.com/pelletier/go-toml/v2` dependency — removed

### What Stays
- ✅ WireGuard peer management (`/wireguard/peers`)
- ✅ DB-backed IP allocation
- ✅ Metrics collection and reporting
- ✅ Auto-registration
- ✅ Rate limiting, security logging

### Migration
1. Update backend to accept agents without Outline fields (must be done first)
2. Deploy new agent binary to all servers
3. Verify health: `GET /health` returns `wireguard: "online"`
4. Clean up old TrustTunnel data: `rm -rf /opt/trusttunnel/`

### Binary Size
Reduced from ~15MB to ~12MB (20% smaller).

See [CHANGELOG.md](CHANGELOG.md) for full details.
