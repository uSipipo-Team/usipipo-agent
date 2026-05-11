# Agent Auto-Registration Guide

**Version:** 0.2.0+
**Last Updated:** 2026-03-29

---

## Overview

Starting from version 0.2.0, VPN agents automatically register with the backend on first startup. This eliminates the need for manual database entries and simplifies大规模 deployment.

---

## How It Works

### Registration Flow

```
1. Admin generates API key via backend admin endpoint
2. Admin installs agent with AGENT_API_KEY in .env
3. Agent starts → collects system metadata (hostname, IP, country, etc.)
4. Agent POST /api/v1/servers/register-agent → Backend
5. Backend validates API key → creates server record → returns UUID
6. Agent saves UUID to .env (SERVER_ID=...)
7. Agent sends metrics every 1 minute using UUID
```

### Architecture

```
┌──────────────────────────────────────────────────────────┐
│  ADMIN                                                   │
│  1. Generate API key via /admin/agent-api-keys          │
│  2. Copy key to agent .env                              │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  AGENT (VPS)                                             │
│  1. Start → read AGENT_API_KEY from .env                │
│  2. Collect metadata (IP, country, version, etc.)       │
│  3. POST /api/v1/servers/register-agent                 │
│  4. Save SERVER_ID to .env                              │
│  5. Send metrics every 1 minute                         │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  BACKEND                                                 │
│  1. Validate API key (hash match, not used/expired)     │
│  2. Create vpn_servers record with metadata             │
│  3. Mark API key as "used"                              │
│  4. Return server_id (UUID)                             │
│  5. Accept metrics at /api/v1/metrics/agents/{id}       │
└──────────────────────────────────────────────────────────┘
```

---

## Admin Setup

### Step 1: Generate API Key

Use the backend admin API to generate a unique API key for each VPS:

```bash
curl -X POST https://usipipo.duckdns.org/api/v1/admin/agent-api-keys \
  -H "Authorization: Bearer <admin_jwt_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "USA East VPS #1 (165.140.241.96)",
    "expires_in_days": 365
  }'
```

**Response:**
```json
{
  "id": "uuid-of-key-record",
  "api_key": "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "status": "active",
  "created_at": "2026-03-29T19:30:00Z",
  "expires_at": "2027-03-29T19:30:00Z"
}
```

**⚠️ IMPORTANT:** Save the `api_key` value - it's only shown once!

---

### Step 2: Install Agent on VPS

SSH into your VPS:

```bash
ssh root@your-vps-ip
```

Download and install the agent:

```bash
# Download latest release
wget https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/install.sh
chmod +x install.sh

# Install agent
sudo ./install.sh
```

---

### Step 3: Configure Agent

Edit the agent configuration:

```bash
sudo nano /opt/usipipo-agent/.env
```

Set the following variables:

```env
# Backend URL (HTTP, not HTTPS - backend runs behind Caddy)
BACKEND_URL=http://usipipo.duckdns.org

# Agent API Key (from Step 1)
AGENT_API_KEY=agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

# Server ID (leave empty - will be auto-filled)
SERVER_ID=

# Agent public URL
AGENT_URL=http://usipipousa.duckdns.org:8080

# WireGuard configuration
WG_INTERFACE=wg0
```

Save and exit (`Ctrl+X`, `Y`, `Enter`).

---

### Step 4: Start Agent

```bash
# Start the service
sudo systemctl start usipipo-agent

# Enable on boot
sudo systemctl enable usipipo-agent

# Check status
sudo systemctl status usipipo-agent

# View logs
sudo journalctl -u usipipo-agent -f
```

**Expected logs:**
```
Mar 29 19:21:04 usipipo usipipo-agent[1045106]: Starting VPN Agent on port 8080
Mar 29 19:21:04 usipipo usipipo-agent[1045106]: Server ID not set or invalid, attempting registration...
Mar 29 19:21:05 usipipo usipipo-agent[1045106]: Registered with server_id: 1bc5c426-29de-4440-9ec6-ada7866e2c08
Mar 29 19:21:05 usipipo usipipo-agent[1045106]: Metrics sent successfully to backend
```

---

### Step 5: Verify Registration

Check that `.env` was updated:

```bash
sudo cat /opt/usipipo-agent/.env | grep SERVER_ID
# Expected: SERVER_ID=1bc5c426-29de-4440-9ec6-ada7866e2c08
```

Check backend for server registration:

```bash
curl -s https://usipipo.duckdns.org/api/v1/admin/servers \
  -H "Authorization: Bearer <admin_jwt_token>" | jq .
```

Expected: Server with matching UUID and metadata.

---

## Metadata Collected

During registration, the agent automatically collects and sends:

| Field | Description | Example |
|-------|-------------|---------|
| `hostname` | VPS hostname | `us-east-1-vps-165-140-241-96` |
| `ip_address` | Public IP (from GeoIP API) | `165.140.241.96` |
| `country_code` | ISO country code | `US` |
| `country_name` | Country name | `United States` |
| `region` | State/region | `Virginia` |
| `city` | City | `Ashburn` |
| `agent_version` | Agent version | `0.2.0` |
| `os_type` | Operating system | `linux` |
| `os_arch` | Architecture | `amd64` |
| `agent_url` | Agent API URL | `http://usipipousa.duckdns.org:8080` |
| `supports_wireguard` | WireGuard support | `true` |

---

## API Key Management

### List All API Keys

```bash
curl -s https://usipipo.duckdns.org/api/v1/admin/agent-api-keys \
  -H "Authorization: Bearer <admin_jwt_token>" | jq .
```

**Response:**
```json
{
  "keys": [
    {
      "id": "uuid-1",
      "status": "active",
      "description": "USA East VPS #1",
      "created_at": "2026-03-29T19:30:00Z",
      "server_id": null
    },
    {
      "id": "uuid-2",
      "status": "used",
      "description": "Germany VPS #1",
      "created_at": "2026-03-28T10:00:00Z",
      "used_at": "2026-03-28T10:05:00Z",
      "server_id": "uuid-of-german-server"
    }
  ],
  "total": 2
}
```

### Revoke an API Key

If an API key is compromised or VPS is decommissioned:

```bash
curl -X PATCH https://usipipo.duckdns.org/api/v1/admin/agent-api-keys/{key_id}/revoke \
  -H "Authorization: Bearer <admin_jwt_token>"
```

---

## Troubleshooting

### Registration Fails with "Invalid API Key"

**Symptoms:**
```
Failed to register: Invalid or expired API key
```

**Causes:**
1. API key typo in `.env`
2. API key already used
3. API key expired

**Solution:**
```bash
# Verify API key in .env
sudo cat /opt/usipipo-agent/.env | grep AGENT_API_KEY

# Check key status in backend
curl -s https://usipipo.duckdns.org/api/v1/admin/agent-api-keys \
  -H "Authorization: Bearer <admin_jwt_token>" | jq '.keys[] | select(.status=="active")'

# Generate new key if needed
curl -X POST https://usipipo.duckdns.org/api/v1/admin/agent-api-keys \
  -H "Authorization: Bearer <admin_jwt_token>" \
  -d '{"description": "Replacement key for VPS #1"}'
```

---

### Registration Fails with "Already Used"

**Symptoms:**
```
Failed to register: API key already used for registration
```

**Cause:** API key can only register once.

**Solution:**
1. Check if server already exists:
   ```bash
   curl -s "https://usipipo.duckdns.org/api/v1/servers/register-agent?api_key=agent_..." \
     -H "Authorization: Bearer <admin_jwt_token>"
   ```

2. If server exists, reuse existing `SERVER_ID` in `.env`

3. If server deleted, generate new API key

---

### SERVER_ID Not Saved to .env

**Symptoms:**
```
Warning: Could not save SERVER_ID to .env: permission denied
```

**Cause:** Agent user (`usipipo`) can't write to `.env`

**Solution:**
```bash
# Fix permissions
sudo chown usipipo:usipipo /opt/usipipo-agent/.env
sudo chmod 600 /opt/usipipo-agent/.env

# Restart agent
sudo systemctl restart usipipo-agent
```

**Note:** Agent will still function and send metrics, but will re-register on each restart.

---

### GeoIP Lookup Fails

**Symptoms:**
```
Failed to collect metadata: failed to fetch geo location
```

**Cause:** Can't reach ip-api.com (firewall, network issue)

**Solution:**
```bash
# Test GeoIP API
curl http://ip-api.com/json/

# Expected response:
# {"query":"1.2.3.4","countryCode":"US","countryName":"United States",...}

# If blocked by firewall, allow outbound HTTP to ip-api.com
```

Agent will use defaults (`XX`, `Unknown`) if GeoIP fails.

---

## Security Considerations

### API Key Storage

- ✅ API keys are **hashed** (SHA-256) before storage in database
- ✅ Keys are **only shown once** during generation
- ✅ Each key can only be used **once** for registration
- ✅ Keys can have **expiration dates** (optional)
- ✅ Keys can be **revoked** at any time

### Agent Authentication

- ✅ All agent endpoints require `X-API-Key` header
- ✅ Invalid keys return `401 Unauthorized`
- ✅ Expired keys return `401 Unauthorized`
- ✅ Revoked keys return `401 Unauthorized`

### Best Practices

1. **Generate unique keys per VPS** - Don't reuse keys
2. **Set expiration dates** - Especially for temporary deployments
3. **Revoke unused keys** - When decommissioning VPS
4. **Monitor registration logs** - Watch for failed attempts
5. **Use HTTPS** - For all backend communication (via Caddy)

---

## Migration from Manual Registration

If you have existing agents with manually registered servers:

### Option 1: Keep Existing Setup

No action needed. Existing agents continue working with their `SERVER_ID`.

### Option 2: Migrate to Auto-Registration

1. Generate new API key for each existing VPS
2. Update agent `.env`:
   ```env
   AGENT_API_KEY=agent_new_key_here
   SERVER_ID=  # Clear existing UUID
   ```
3. Restart agent - it will re-register automatically
4. Old server record becomes orphan (can be cleaned up manually)

---

## API Reference

### Generate API Key

**Endpoint:** `POST /api/v1/admin/agent-api-keys`

**Headers:**
- `Authorization: Bearer <admin_jwt_token>`
- `Content-Type: application/json`

**Request:**
```json
{
  "description": "USA East VPS #1",
  "expires_in_days": 365
}
```

**Response (201):**
```json
{
  "id": "uuid-of-key-record",
  "api_key": "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "status": "active",
  "created_at": "2026-03-29T19:30:00Z",
  "expires_at": "2027-03-29T19:30:00Z"
}
```

---

### Register Agent

**Endpoint:** `POST /api/v1/servers/register-agent`

**Headers:**
- `X-API-Key: agent_...`
- `Content-Type: application/json`

**Request:**
```json
{
  "hostname": "us-east-1-vps",
  "ip_address": "165.140.241.96",
  "country_code": "US",
  "country_name": "United States",
  "region": "Virginia",
  "city": "Ashburn",
  "agent_version": "0.2.0",
  "os_type": "linux",
  "os_arch": "amd64",
  "agent_url": "http://usipipousa.duckdns.org:8080",
  "supports_wireguard": true,
  "agent_api_key": "agent_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
}
```

**Response (201):**
```json
{
  "server_id": "1bc5c426-29de-4440-9ec6-ada7866e2c08",
  "status": "registered",
  "message": "Server registered successfully"
}
```

---

### Check Registration Status

**Endpoint:** `GET /api/v1/servers/register-agent?api_key=agent_...`

**Response (200):**
```json
{
  "server_id": "1bc5c426-29de-4440-9ec6-ada7866e2c08",
  "status": "registered"
}
```

---

## Related Documentation

- [Agent Deployment Guide](DEPLOYMENT.md)
- [Backend API Documentation](https://github.com/uSipipo-Team/usipipo-backend/wiki)
- [Auto-Registration Design Doc](../../usipipo-docs/plans/agent/2026-03-29-agent-auto-registration-design.md)
- [Auto-Registration Implementation Plan](../../usipipo-docs/plans/agent/2026-03-29-agent-auto-registration-plan.md)

---

**Support:** For issues or questions, open an issue at https://github.com/uSipipo-Team/usipipo-agent/issues
