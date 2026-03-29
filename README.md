# uSipipo VPN Agent

Lightweight Go agent for managing VPN servers (Outline, WireGuard, Trust Tunnel).

## Features

- 🚀 Create/delete VPN keys via HTTPS API
- 📊 Auto-report metrics to backend every 1 minute
- 🔐 API Key authentication
- 🔒 HTTPS with Caddy + DuckDNS
- 📈 System metrics (CPU, memory, disk, network)
- 🌍 Multi-platform support (Linux, macOS, Windows)

## Quick Start

### Download Pre-built Binary

```bash
# Linux AMD64 (most VPS)
wget https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/usipipo-agent-linux-amd64.zip
unzip usipipo-agent-linux-amd64.zip
chmod +x usipipo-agent-linux-amd64
```

### Build from Source

```bash
go build -o agent ./cmd/agent
```

### Run

```bash
export AGENT_API_KEY="your-api-key"
export BACKEND_URL="https://api.usipipo.duckdns.org"
export SERVER_ID="us-east-1"
./agent
```

## Configuration

| Environment Variable | Description | Default | Required |
|---------------------|-------------|---------|----------|
| `AGENT_PORT` | Port to listen on | `8080` | No |
| `AGENT_API_KEY` | API key for authentication | - | **Yes** |
| `BACKEND_URL` | Backend URL for metrics | - | **Yes** |
| `SERVER_ID` | Server identifier | - | **Yes** |
| `OUTLINE_API_URL` | Outline Manager API URL | `http://localhost:8081` | No |
| `WG_INTERFACE` | WireGuard interface name | `wg0` | No |

## API Endpoints

### Public

- `GET /health` - Health check

### Protected (requires X-API-Key header)

- `GET /status` - Server status
- `GET /metrics` - Detailed system metrics
- `POST /outline/keys` - Create Outline key
- `DELETE /outline/keys/:id` - Delete Outline key
- `POST /wireguard/peers` - Create WireGuard peer
- `DELETE /wireguard/peers/:name` - Delete WireGuard peer
- `GET /wireguard/peers/:name/usage` - Get peer usage

## Deployment

See [DEPLOYMENT.md](./DEPLOYMENT.md) for systemd service setup and Caddy configuration.

## License

MIT License - see [LICENSE](./LICENSE)
