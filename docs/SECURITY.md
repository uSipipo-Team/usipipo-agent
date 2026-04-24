# Security Configuration

This document describes security-related configuration options for the uSipipo VPN Agent.

## WireGuard Key Validation (Issue #27)

### Overview

WireGuard private keys are generated using `crypto/rand` by default. However, in environments with weak or predictable random number generators (RNG), cryptographic keys may have insufficient entropy, making them vulnerable to brute-force attacks.

The **WireGuard Key Validation** feature ensures that generated private keys have sufficient randomness before they are accepted.

### Configuration

| Environment Variable | Type | Default | Description |
|----------------------|------|---------|-------------|
| `WG_VALIDATE_KEYS` | boolean | `true` | Enable entropy validation for WireGuard private key generation |

**Example**:
```bash
export WG_VALIDATE_KEYS=true
```

### How It Works

1. When `WG_VALIDATE_KEYS=true` (default), each generated WireGuard private key undergoes entropy validation
2. The validation checks that byte distribution is sufficiently random (≥95% unique bytes)
3. If a key fails entropy validation, the system retries up to 3 times
4. If all retries fail, a warning is logged and the key is still used (graceful degradation)
5. When `WG_VALIDATE_KEYS=false`, no entropy validation is performed

### Security Impact

- **Enabled (recommended)**: Protects against weak RNG systems by ensuring high-entropy keys
- **Disabled**: Faster key generation, but vulnerable to weak randomness attacks

**Best Practice**: Keep `WG_VALIDATE_KEYS=true` in all environments except controlled testing.

### Logging

When entropy validation fails, you'll see:
```
WARN: Private key failed entropy validation (attempt 1/3), retrying...
ERROR: Private key entropy validation failed after 3 attempts, proceeding anyway
```

If you see these warnings frequently, investigate your system's RNG:
```bash
# Check RNG availability
cat /proc/sys/kernel/random/entropy_avail  # Linux only
```

## Configuration File Permissions (Issue #28)

### Overview

Sensitive configuration files like `.env` may contain secrets (API keys, tokens). If these files have overly permissive permissions (e.g., world-readable), other users on the system could access credentials.

The **Configuration File Security** feature checks file permissions at startup and warns about insecure settings.

### Supported Files

| File | Required Permissions | Reason |
|------|----------------------|--------|
| `.env` | `0600` (rw-------) | Contains API keys and secrets |
| Agent config files (future) | Configurable | Prevents unauthorized access |

### Configuration

| Environment Variable | Type | Default | Description |
|----------------------|------|---------|-------------|
| `CONFIG_STRICT_PERMS` | boolean | `false` | Enforce strict permission checks at startup |

**Example**:
```bash
export CONFIG_STRICT_PERMS=true
```

### Permission Requirements

- **`.env` file must be readable and writable only by the owner (0600)**
- If permissions are more permissive (e.g., `0644`, `0666`), the agent will log a warning

### Setting Correct Permissions

```bash
# Set .env to 0600
chmod 600 .env

# Verify
ls -la .env
# -rw------- 1 user user ... .env
```

### Security Impact

- **Strict perms enabled**: Prevents other system users from reading secrets
- **Disabled**: No startup check (not recommended for production)

**Best Practice**: Always set `CONFIG_STRICT_PERMS=true` in production and ensure `.env` is `0600`.

### Startup Checks

At startup, the agent logs:

**When secure**:
```
INFO: Configuration file permissions are secure (0600)
```

**When insecure**:
```
SECURITY WARNING: .env file has insecure permissions (0644)
SECURITY WARNING: Expected 0600 (owner read/write only), got 0644
SECURITY WARNING: Other users on this system may be able to read secrets!
```
