# WireGuard Setup Guide for uSipipo Agent

This guide covers WireGuard integration with the uSipipo Agent.

## Prerequisites

- WireGuard tools installed (`wg`, `wg-quick`)
- WireGuard interface configured (e.g., `wg0`)
- Agent installed and running as `usipipo` user

## Quick Start

```bash
# 1. Install WireGuard tools
sudo apt install wireguard  # Debian/Ubuntu
sudo yum install wireguard-tools  # RHEL/CentOS

# 2. Configure sudo for agent
sudo cp scripts/usipipo-agent.sudoers /etc/sudoers.d/
sudo chmod 440 /etc/sudoers.d/usipipo-agent
sudo visudo -c -f /etc/sudoers.d/usipipo-agent  # Validate

# 3. Test sudo access
sudo -u usipipo wg genkey  # Should output key without password

# 4. Restart agent
sudo systemctl restart usipipo-agent

# 5. Test WireGuard API
curl -X POST -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-peer"}' \
  http://localhost:8080/wireguard/peers
```

## Sudo Configuration

The agent requires sudo privileges to execute WireGuard commands. Follow these steps:

### 1. Copy Sudoers File

```bash
sudo cp scripts/usipipo-agent.sudoers /etc/sudoers.d/
sudo chmod 440 /etc/sudoers.d/usipipo-agent
```

### 2. Validate Sudoers Configuration

```bash
sudo visudo -c -f /etc/sudoers.d/usipipo-agent
# Expected output: /etc/sudoers.d/usipipo-agent: parsed OK
```

### 3. Test Sudo Access

```bash
# Test each command
sudo -u usipipo wg genkey
sudo -u usipipo wg pubkey
sudo -u usipipo wg show wg0
sudo -u usipipo wg show wg0 dump

# All should work without password prompt
```

### 4. Verify Audit Logging

Sudo commands are logged to `/var/log/auth.log`:

```bash
sudo grep usipipo /var/log/auth.log | grep wg
```

## Systemd Service Configuration

The agent service requires capabilities for WireGuard operations:

```ini
[Service]
User=usipipo
Group=usipipo
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW
```

These settings are already included in `systemd/usipipo-agent.service`.

## Testing WireGuard Integration

### Run Integration Tests

```bash
cd /opt/usipipo-agent
WIREGUARD_TEST_INTERFACE=wg0 go test -tags=integration ./internal/vpn/... -v
```

Expected output:
```
=== RUN   TestWireGuardGenKey
--- PASS: TestWireGuardGenKey (0.01s)
=== RUN   TestWireGuardPubkey
--- PASS: TestWireGuardPubkey (0.01s)
=== RUN   TestWireGuardShow
--- PASS: TestWireGuardShow (0.01s)
=== RUN   TestWireGuardShowDump
--- PASS: TestWireGuardShowDump (0.01s)
PASS
```

### Run E2E Tests

```bash
export AGENT_API_KEY="your-api-key"
./scripts/test-wireguard-e2e.sh
```

Expected output:
```
=========================================
uSipipo Agent - WireGuard E2E Test
=========================================

Test 1: Creating WireGuard peer...
✅ PASSED: Peer created

Test 2: Verifying peer exists...
✅ PASSED: Peer found in wg show

Test 3: Getting peer usage...
✅ PASSED: Usage retrieved

Test 4: Deleting peer...
✅ PASSED: Peer deleted

Test 5: Verifying cleanup...
✅ PASSED: Peer cleaned up successfully

=========================================
All WireGuard E2E tests passed! ✅
=========================================
```

### Manual Testing

```bash
# Create peer
curl -X POST -H "X-API-Key: your-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-peer"}' \
  http://localhost:8080/wireguard/peers

# Expected response:
# {"public_key":"...","name":"test-peer","ip_address":"10.0.0.X","config":"[Interface]..."}

# Verify with wg
wg show wg0

# Should show new peer with public key

# Get peer usage
curl -H "X-API-Key: your-key" \
  http://localhost:8080/wireguard/peers/test-peer/usage

# Expected response:
# {"name":"test-peer","bytes_used":0}

# Delete peer
curl -X DELETE -H "X-API-Key: your-key" \
  http://localhost:8080/wireguard/peers/test-peer

# Expected: HTTP 204 No Content
```

## Troubleshooting

### Issue: "wg command failed: exit status 1"

**Cause:** Sudo not configured or permissions denied

**Fix:**
```bash
# Check sudoers
sudo visudo -c -f /etc/sudoers.d/usipipo-agent

# Test sudo access
sudo -u usipipo wg genkey

# Check agent logs
sudo journalctl -u usipipo-agent -n 50

# Look for lines like:
# "Executing wg command: wg genkey"
# "command failed: ..."
```

### Issue: "Operation not permitted"

**Cause:** Missing capabilities in systemd service

**Fix:**
```bash
# Verify service has capabilities
sudo systemctl cat usipipo-agent | grep -A 2 AmbientCapabilities

# If missing, update service file and reload:
sudo systemctl daemon-reload
sudo systemctl restart usipipo-agent
```

### Issue: "peer already exists"

**Cause:** Peer with same name already exists in WireGuard config

**Fix:**
```bash
# List existing peers
wg show wg0 peers

# Manually remove if needed
sudo wg set wg0 peer <public-key> remove

# Or delete via API
curl -X DELETE -H "X-API-Key: your-key" \
  http://localhost:8080/wireguard/peers/<peer-name>
```

### Issue: "sudo: no valid sudoers sources found"

**Cause:** Sudoers file not in correct location or wrong permissions

**Fix:**
```bash
# Verify file location
ls -la /etc/sudoers.d/usipipo-agent

# Should be: -r--r----- 1 root root

# Fix permissions
sudo chmod 440 /etc/sudoers.d/usipipo-agent
sudo chown root:root /etc/sudoers.d/usipipo-agent

# Validate
sudo visudo -c -f /etc/sudoers.d/usipipo-agent
```

## Security Considerations

### Sudoers Security

The sudoers file grants **minimal required privileges**:

- ✅ Only specific wg commands allowed
- ✅ No shell access (`/bin/bash`, `/bin/sh` not allowed)
- ✅ No arbitrary command execution
- ✅ Commands restricted to wg0 interface where applicable
- ✅ Audit trail via sudo logging

### Audit Trail

All sudo commands are logged:

```bash
# View WireGuard commands executed by agent
sudo grep usipipo /var/log/auth.log | grep wg

# Example output:
# Mar 29 10:00:00 server sudo: usipipo : TTY=unknown ; PWD=/opt/usipipo-agent ;
# USER=root ; COMMAND=/usr/bin/wg genkey
```

### Best Practices

1. **Regular Audits:** Review sudo logs weekly
2. **Minimal Privileges:** Only grant required commands
3. **Interface Restrictions:** Limit to wg0 where possible
4. **No Wildcards:** Avoid `ALL` in sudoers (use specific commands)

## Next Steps

- Configure WireGuard interface (see `/etc/wireguard/wg0.conf`)
- Set up peer management via API
- Monitor peer usage and statistics
- Configure firewall rules for WireGuard traffic
- Set up monitoring and alerting

## Related Documentation

- [DEPLOYMENT.md](../DEPLOYMENT.md) - General deployment guide
- [README.md](../README.md) - Installation and usage
- [scripts/usipipo-agent.sudoers](../scripts/usipipo-agent.sudoers) - Sudoers template
- [scripts/test-wireguard-e2e.sh](../scripts/test-wireguard-e2e.sh) - E2E test script
