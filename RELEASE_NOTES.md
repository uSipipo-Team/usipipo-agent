## 🎉 Release v0.1.0 - Initial VPN Agent Implementation

First production-ready release of uSipipo VPN Agent for multi-country server management.

---

### 🚀 Features

#### VPN Management
- ✅ Outline Manager integration (create/delete keys)
- ✅ WireGuard integration (create/delete peers)
- ✅ HTTPS API with API Key authentication

#### Metrics & Monitoring
- ✅ System metrics (CPU, RAM, disk, network)
- ✅ VPN metrics (active keys/peers, bytes transferred)
- ✅ Latency tracking (avg, p95, p99)
- ✅ Auto-report to backend every 1 minute

#### Infrastructure
- ✅ Multi-platform builds (Linux, macOS, Windows)
- ✅ GitHub Actions CI/CD (auto-build on release)
- ✅ systemd service configuration
- ✅ Caddy + DuckDNS for HTTPS

#### Security
- ✅ API Key authentication (X-API-Key header)
- ✅ Encrypted API keys at rest (Fernet encryption)
- ✅ No hardcoded secrets
- ✅ HTTPS with Let's Encrypt

---

### 📦 Installation

#### Download Pre-built Binary

**Linux AMD64:**
```bash
wget https://github.com/uSipipo-Team/usipipo-agent/releases/download/v0.1.0/usipipo-agent-linux-amd64.zip
unzip usipipo-agent-linux-amd64.zip
chmod +x usipipo-agent-linux-amd64
./usipipo-agent-linux-amd64
```

**Linux ARM64:**
```bash
wget https://github.com/uSipipo-Team/usipipo-agent/releases/download/v0.1.0/usipipo-agent-linux-arm64.zip
unzip usipipo-agent-linux-arm64.zip
chmod +x usipipo-agent-linux-arm64
```

#### Build from Source
```bash
git clone https://github.com/uSipipo-Team/usipipo-agent.git
cd usipipo-agent
go build -o agent ./cmd/agent
```

---

### ⚙️ Configuration

Create `.env` file:
```bash
AGENT_PORT=8080
AGENT_API_KEY=your-unique-api-key-here
BACKEND_URL=https://api.usipipo.duckdns.org
SERVER_ID=us-east-1
OUTLINE_API_URL=http://localhost:8081
WG_INTERFACE=wg0
```

---

### 🎛️ API Endpoints

**Public:**
- `GET /health` - Health check

**Protected (require X-API-Key header):**
- `GET /status` - Server status
- `GET /metrics` - Detailed metrics
- `POST /outline/keys` - Create Outline key
- `DELETE /outline/keys/:id` - Delete Outline key
- `POST /wireguard/peers` - Create WireGuard peer
- `DELETE /wireguard/peers/:name` - Delete WireGuard peer
- `GET /wireguard/peers/:name/usage` - Get peer usage

---

### 📊 Metrics Payload

Agents push metrics every 1 minute:
```json
{
  "server_id": "us-east-1",
  "timestamp": "2026-03-28T10:00:00Z",
  "system": {
    "cpu_percent": 45.2,
    "memory_percent": 62.1,
    "disk_percent": 38.5
  },
  "vpn": {
    "outline": { "active_keys": 42 },
    "wireguard": { "active_peers": 38 }
  },
  "latency_ms": { "avg": 12.5, "p95": 25.3, "p99": 45.8 }
}
```

---

### 🚀 Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for:
- systemd service setup
- Caddy + DuckDNS configuration
- Production deployment guide

---

### 🔒 Security Notes

**Before first run:**
1. Generate unique API key for each server
2. Set ENCRYPTION_KEY in backend .env
3. Configure firewall (allow only backend IP)
4. Enable HTTPS with Caddy + DuckDNS

**API Key Encryption:**
```bash
python -c 'from cryptography.fernet import Fernet; print(Fernet.generate_key().decode())'
# Add to backend .env: ENCRYPTION_KEY=your-key-here
```

---

### 📁 Files Included

- `usipipo-agent-linux-amd64.zip` - Linux 64-bit (most VPS)
- `usipipo-agent-linux-arm64.zip` - Linux ARM (Raspberry Pi)
- `usipipo-agent-darwin-amd64.zip` - macOS Intel
- `usipipo-agent-darwin-arm64.zip` - macOS Apple Silicon
- `usipipo-agent-windows-amd64.zip` - Windows 64-bit

---

### 🔗 Related Projects

- **Backend:** https://github.com/uSipipo-Team/usipipo-backend
- **Commons:** https://github.com/uSipipo-Team/usipipo-commons
- **Docs:** https://github.com/uSipipo-Team/usipipo-docs
- **Telegram Bot:** https://github.com/uSipipo-Team/usipipo-telegram-bot

---

### 📈 Next Steps

1. Deploy to USA VPS (us-east-1)
2. Deploy to Germany VPS (eu-central-1)
3. Deploy to Belgium VPS (eu-west-1)
4. Configure Caddy + DuckDNS for each
5. Register servers in backend
6. End-to-end testing

---

**Full changelog:** https://github.com/uSipipo-Team/usipipo-agent/compare/v0.1.0

**Docker Hub:** Coming in v0.2.0

**License:** MIT
