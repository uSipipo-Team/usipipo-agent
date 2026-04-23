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
