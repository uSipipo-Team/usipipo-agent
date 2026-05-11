# VPN Agent Deployment Guide

## Prerequisites

- WireGuard installed and configured
- Caddy installed (for HTTPS)

## WireGuard Configuration

### 1. Install WireGuard Tools

```bash
sudo apt install wireguard  # Debian/Ubuntu
sudo yum install wireguard-tools  # RHEL/CentOS
```

### 2. Configure Sudo for Agent

The agent requires sudo privileges to execute WireGuard commands (`wg genkey`, `wg pubkey`, etc.).

```bash
# Copy sudoers configuration
sudo cp scripts/usipipo-agent.sudoers /etc/sudoers.d/
sudo chmod 440 /etc/sudoers.d/usipipo-agent

# Validate configuration
sudo visudo -c -f /etc/sudoers.d/usipipo-agent
# Expected: /etc/sudoers.d/usipipo-agent: parsed OK

# Test sudo access
sudo -u usipipo wg genkey  # Should output key without password prompt
```

### 3. Verify WireGuard Interface

```bash
# Check if wg0 exists
wg show wg0

# If not configured, see /etc/wireguard/wg0.conf
# Or use wg-quick: sudo wg-quick up wg0
```

### 4. Test WireGuard API

After agent is running:

```bash
# Create peer
curl -X POST -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-peer"}' \
  http://localhost:8080/wireguard/peers

# Verify with wg
wg show wg0 | grep test-peer

# Delete peer
curl -X DELETE -H "X-API-Key: your-api-key" \
  http://localhost:8080/wireguard/peers/test-peer
```

For detailed WireGuard setup, see [docs/WIREGUARD-SETUP.md](docs/WIREGUARD-SETUP.md).

---

## Download Pre-built Binaries

**From GitHub Releases:**
https://github.com/uSipipo-Team/usipipo-agent/releases

**Available platforms:**
- `usipipo-agent-linux-amd64.zip` - Linux 64-bit (most VPS)
- `usipipo-agent-linux-arm64.zip` - Linux ARM (Raspberry Pi, ARM VPS)
- `usipipo-agent-darwin-amd64.zip` - macOS Intel
- `usipipo-agent-darwin-arm64.zip` - macOS Apple Silicon
- `usipipo-agent-windows-amd64.zip` - Windows 64-bit

**Example: Download for Linux AMD64**
```bash
wget https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/usipipo-agent-linux-amd64.zip
unzip usipipo-agent-linux-amd64.zip
chmod +x usipipo-agent-linux-amd64
```

## Install

### 1. Create directory and copy files

```bash
sudo mkdir -p /opt/usipipo-agent
sudo cp usipipo-agent-linux-amd64 /opt/usipipo-agent/agent
sudo cp .env.example /opt/usipipo-agent/.env
sudo cp systemd/usipipo-agent.service /etc/systemd/system/
```

### 2. Edit configuration

```bash
sudo nano /opt/usipipo-agent/.env
```

Update the following variables:
- `AGENT_API_KEY` - Your unique API key (provided by backend)
- `BACKEND_URL` - Backend URL (e.g., `https://api.usipipo.duckdns.org`)
- `SERVER_ID` - Server identifier (e.g., `us-east-1`, `de-central-1`)

### 3. Create system user

```bash
sudo useradd -r -s /bin/false usipipo
sudo chown -R usipipo:usipipo /opt/usipipo-agent
```

### 4. Enable and start service

```bash
sudo systemctl daemon-reload
sudo systemctl enable usipipo-agent
sudo systemctl start usipipo-agent
```

### 5. Check status

```bash
sudo systemctl status usipipo-agent
sudo journalctl -u usipipo-agent -f
```

## Caddy Configuration

### 1. Install Caddy with DuckDNS plugin

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

### 2. Configure Caddy

Edit `/etc/caddy/Caddyfile`:

```caddyfile
usipipousa.duckdns.org {
    reverse_proxy localhost:8080
    tls {
        dns duckdns YOUR_DUCKDNS_TOKEN
    }
}
```

Replace `YOUR_DUCKDNS_TOKEN` with your DuckDNS token.

### 3. Reload Caddy

```bash
sudo systemctl reload caddy
```

## Testing

### Test health endpoint

```bash
curl https://usipipousa.duckdns.org/health
# Expected: {"status":"healthy"}
```

### Test metrics endpoint

```bash
curl -H "X-API-Key: your-api-key" https://usipipousa.duckdns.org/metrics
# Expected: JSON with system metrics
```

### Test WireGuard peer creation

```bash
curl -X POST -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-peer"}' \
  https://usipipousa.duckdns.org/wireguard/peers
```

## Troubleshooting

### Check logs

```bash
sudo journalctl -u usipipo-agent -n 100
```

### Restart service

```bash
sudo systemctl restart usipipo-agent
```

### Check if ports are in use

```bash
sudo netstat -tulpn | grep :8080
```

### Verify WireGuard interface exists

```bash
wg show wg0
```

## Updating

### Download new version

```bash
cd /opt/usipipo-agent
sudo wget https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/usipipo-agent-linux-amd64.zip
sudo unzip -o usipipo-agent-linux-amd64.zip
sudo chmod +x usipipo-agent-linux-amd64
```

### Stop service, replace binary, start

```bash
sudo systemctl stop usipipo-agent
sudo mv usipipo-agent-linux-amd64 agent
sudo systemctl start usipipo-agent
```
