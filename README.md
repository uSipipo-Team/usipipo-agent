# uSipipo VPN Agent

[![CI](https://github.com/uSipipo-Team/usipipo-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/uSipipo-Team/usipipo-agent/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/uSipipo-Team/usipipo-agent)](https://github.com/uSipipo-Team/usipipo-agent/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Lightweight Go agent for managing VPN servers across 200+ countries.**

Part of the **uSipipo VPN Ecosystem** - Centralized orchestration for multi-country VPN infrastructure.

---

## 🎯 Overview

The VPN Agent runs on each VPS server worldwide, providing:
- **Remote VPN Management** - Create/delete WireGuard peers via HTTPS API
- **Auto-Reporting Metrics** - Push system metrics to backend every 1 minute
- **Secure Communication** - API Key authentication + HTTPS encryption
- **Multi-Platform Support** - Linux, macOS, Windows (amd64, arm64)

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────┐
│         BACKEND CENTRAL (Orchestrator)          │
│  - Server registry & load balancing             │
│  - User authentication & billing                │
│  - Metrics storage & dashboards                 │
└─────────────────────────────────────────────────┘
                     │ HTTPS + API Key
                     │ (every 1 minute)
        ┌────────────┼────────────┬────────────┐
        │            │            │            │
   ┌────▼────┐  ┌───▼────┐  ┌───▼────┐  ┌───▼────┐
   │ USA VPS │  │ DE VPS │  │ BE VPS │  │ XX VPS │
   │ ┌─────┐ │  │ ┌────┐ │  │ ┌────┐ │  │ ┌────┐ │
   │ │Agent│ │  │ │Agent│ │  │ │Agent│ │  │ │Agent│ │
   │ └──┬──┘ │  │ └─┬──┘ │  │ └─┬──┘ │  │ └─┬──┘ │
    │ ┌──▼──────┐│ │ ┌─▼──────┐│ │ ┌─▼──────┐│ │ ┌─▼──────┐│
    │ │WireGuard││ │ │WireGuard││ │ │WireGuard││ │ │WireGuard││
    │ └─────────┘│ │ └────────┘│ │ └────────┘│ │ └────────┘│
   └─────────┘  └───────┘  └───────┘  └───────┘
```

---

## 🚀 Features

### VPN Management
- ✅ **WireGuard Integration** - Create/delete peers via `wg` commands

### Metrics & Monitoring
- ✅ **System Metrics** - CPU, memory, disk, network usage
- ✅ **VPN Metrics** - Active keys/peers, bytes transferred
- ✅ **Latency Tracking** - Average, p95, p99 latency
- ✅ **Auto-Reporting** - Push metrics to backend every 1 minute

### Security
- ✅ **API Key Authentication** - X-API-Key header validation
- ✅ **HTTPS Encryption** - Caddy + DuckDNS with Let's Encrypt
- ✅ **Encrypted API Keys at Rest** - Fernet encryption in database
- ✅ **No Hardcoded Secrets** - All secrets via environment variables

### Infrastructure
- ✅ **Multi-Platform Builds** - Linux, macOS, Windows (amd64, arm64)
- ✅ **GitHub Actions CI/CD** - Auto-build on release
- ✅ **systemd Service** - Production-ready deployment
- ✅ **Docker Support** - Containerized deployment (planned)

---

## 📦 Installation

### Download Pre-built Binary

```bash
# Linux AMD64 (most VPS)
wget https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/usipipo-agent-linux-amd64.zip
unzip usipipo-agent-linux-amd64.zip
chmod +x usipipo-agent-linux-amd64

# Linux ARM64 (Raspberry Pi, ARM VPS)
wget https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/usipipo-agent-linux-arm64.zip
unzip usipipo-agent-linux-arm64.zip
chmod +x usipipo-agent-linux-arm64
```

### Build from Source

```bash
git clone https://github.com/uSipipo-Team/usipipo-agent.git
cd usipipo-agent
go build -o agent ./cmd/agent
```

---

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `AGENT_PORT` | Port to listen on | `8080` | No |
| `AGENT_API_KEY` | API key for authentication | - | **Yes** |
| `BACKEND_URL` | Backend URL for metrics | - | **Yes** |
| `SERVER_ID` | Server identifier (UUID) | - | **Yes** |
| `WG_INTERFACE` | WireGuard interface name | `wg0` | No |
| `WG_SERVER_IP` | WireGuard server public IP | - | No |
| `WG_SERVER_PORT` | WireGuard server listening port | `51820` | No |

### Example `.env` File

```bash
# Agent configuration
AGENT_PORT=8080
AGENT_API_KEY=your-unique-api-key-here
BACKEND_URL=https://api.usipipo.duckdns.org
SERVER_ID=us-east-1

# WireGuard configuration
WG_INTERFACE=wg0
WG_SERVER_IP=your-server-public-ip
WG_SERVER_PORT=51820
```

---

## 🎛️ API Endpoints

### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check |

### Protected Endpoints (require `X-API-Key` header)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/status` | Server status |
| `GET` | `/metrics` | Detailed system + VPN metrics |
| `POST` | `/wireguard/peers` | Create WireGuard peer |
| `DELETE` | `/wireguard/peers/:name` | Delete WireGuard peer |
| `GET` | `/wireguard/peers/:name/usage` | Get peer usage stats |

### Example Usage

```bash
# Create WireGuard peer
curl -X POST -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"user-456"}' \
  https://usipipousa.duckdns.org/wireguard/peers

# Get metrics
curl -H "X-API-Key: your-api-key" \
  https://usipipousa.duckdns.org/metrics
```

---

## 🚀 Deployment

### Production Deployment (systemd)

```bash
# 1. Create directory
sudo mkdir -p /opt/usipipo-agent
sudo cp usipipo-agent-linux-amd64 /opt/usipipo-agent/agent
sudo cp .env.example /opt/usipipo-agent/.env

# 2. Configure environment
sudo nano /opt/usipipo-agent/.env

# 3. Install systemd service
sudo cp systemd/usipipo-agent.service /etc/systemd/system/

# 4. Create system user
sudo useradd -r -s /bin/false usipipo
sudo chown -R usipipo:usipipo /opt/usipipo-agent

# 5. Enable and start
sudo systemctl daemon-reload
sudo systemctl enable usipipo-agent
sudo systemctl start usipipo-agent

# 6. Check status
sudo systemctl status usipipo-agent
sudo journalctl -u usipipo-agent -f
```

### Caddy + DuckDNS Configuration

```caddyfile
# /etc/caddy/Caddyfile
usipipousa.duckdns.org {
    reverse_proxy localhost:8080
    tls {
        dns duckdns YOUR_DUCKDNS_TOKEN
    }
}

usipipode.duckdns.org {
    reverse_proxy localhost:8080
    tls {
        dns duckdns YOUR_DUCKDNS_TOKEN
    }
}
```

---

## 📊 Metrics Payload

Agents push metrics to backend every 1 minute:

```json
{
  "server_id": "us-east-1",
  "timestamp": "2026-03-28T10:00:00Z",
  "system": {
    "cpu_percent": 45.2,
    "memory_percent": 62.1,
    "disk_percent": 38.5,
    "network_rx_bytes": 1234567890,
    "network_tx_bytes": 9876543210
  },
  "vpn": {
    "wireguard": {
      "active_peers": 38,
      "total_bytes_transferred": 4500000000
    }
  },
  "latency_ms": {
    "avg": 12.5,
    "p95": 25.3,
    "p99": 45.8
  }
}
```

---

## 🧪 Testing

```bash
# Run all tests
go test -v ./...

# Run specific package tests
go test -v ./internal/api/...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 📁 Project Structure

```
usipipo-agent/
├── cmd/
│   └── agent/
│       └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go          # HTTP handlers
│   │   ├── middleware.go        # API Key auth
│   │   └── server.go            # HTTP server setup
│   ├── vpn/
│   │   └── wireguard.go         # WireGuard wrapper
│   ├── metrics/
│   │   ├── types.go             # Metrics types
│   │   └── collector.go         # Metrics collector
│   ├── reporter/
│   │   └── reporter.go          # Push metrics to backend
│   └── config/
│       └── config.go            # Configuration loader
├── systemd/
│   └── usipipo-agent.service    # systemd service file
├── .github/workflows/
│   ├── ci.yml                   # CI workflow
│   └── release.yml              # Release workflow
├── go.mod
├── go.sum
├── DEPLOYMENT.md                # Deployment guide
└── README.md                    # This file
```

---

## 🔒 Security

### API Key Encryption

Agent API keys are **encrypted at rest** in the backend database using Fernet symmetric encryption.

**Generate encryption key:**
```bash
python -c 'from cryptography.fernet import Fernet; print(Fernet.generate_key().decode())'
```

**Set in backend `.env`:**
```bash
ENCRYPTION_KEY=your-generated-key-here
```

### Best Practices

- ✅ Never commit `.env` files
- ✅ Rotate API keys every 90 days
- ✅ Use HTTPS for all communication
- ✅ Enable firewall rules (allow only backend IP)
- ✅ Monitor logs for suspicious activity

---

## 📈 Monitoring

### Health Check

```bash
# Check agent health
curl https://usipipousa.duckdns.org/health
# Expected: {"status":"healthy"}
```

### Logs

```bash
# View logs
sudo journalctl -u usipipo-agent -f

# View last 100 lines
sudo journalctl -u usipipo-agent -n 100
```

### Metrics Dashboard

Backend provides dashboards for:
- Server status (online/offline/maintenance)
- CPU, memory, disk usage per server
- Active connections per country
- Total bandwidth (GB) per server
- Latency comparison across countries

---

## 🚧 Roadmap

### Q2 2026
- [ ] Docker container support
- [ ] Automatic failover between servers
- [ ] Real-time latency monitoring

### Q3 2026
- [ ] WebSocket support for real-time metrics
- [ ] Automatic certificate renewal
- [ ] Multi-WAN support
- [ ] GeoDNS integration

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🔗 Related Projects

- **[usipipo-backend](https://github.com/uSipipo-Team/usipipo-backend)** - Central orchestrator (FastAPI)
- **[usipipo-commons](https://github.com/uSipipo-Team/usipipo-commons)** - Shared library (PyPI)
- **[usipipo-telegram-bot](https://github.com/uSipipo-Team/usipipo-telegram-bot)** - Telegram bot for user interaction
- **[usipipo-docs](https://github.com/uSipipo-Team/usipipo-docs)** - Documentation portal

---

## 📞 Support

- **Documentation:** https://github.com/uSipipo-Team/usipipo-docs
- **Issues:** https://github.com/uSipipo-Team/usipipo-agent/issues
- **Email:** dev@usipipo.com

---

**Built with ❤️ by uSipipo Team**
