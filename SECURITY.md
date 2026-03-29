# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | ✅                 |
| < 0.1   | ❌                 |

## Reporting a Vulnerability

We take the security of uSipipo Agent seriously. If you believe you have found a security vulnerability, please report it to us as described below.

**Please do NOT report security vulnerabilities through public GitHub issues.**

### How to Report

1. **Email:** Send an email to `security@usipipo.com` (if available) or create a draft security advisory on GitHub
2. **Include:**
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

### What to Expect

- **Initial Response:** Within 48 hours
- **Status Update:** Within 5 business days
- **Resolution:** Depends on severity and complexity

### Security Best Practices

#### For Users

1. **API Keys:** Never commit API keys to version control
2. **Environment Variables:** Use `.env` files and add to `.gitignore`
3. **HTTPS:** Always use HTTPS for agent communication
4. **Firewall:** Restrict access to agent port (default 8080)

#### For Developers

1. **Code Review:** All changes require PR review
2. **CI/CD:** All tests must pass before merge
3. **Dependencies:** Keep dependencies up to date
4. **Secrets:** Use GitHub Secrets for sensitive data

## Security Features

### Current Implementation

- ✅ **API Key Authentication:** All endpoints require X-API-Key header
- ✅ **HTTPS Support:** Works with Caddy + Let's Encrypt
- ✅ **Encrypted API Keys:** API keys encrypted at rest in backend database
- ✅ **No Hardcoded Secrets:** All secrets via environment variables
- ✅ **Branch Protection:** Main branch requires PR review
- ✅ **CI/CD Checks:** All PRs must pass CI tests

### Planned Improvements

- [ ] **mTLS Support:** Mutual TLS for agent-backend communication
- [ ] **Rate Limiting:** Prevent brute force attacks
- [ ] **Audit Logging:** Log all API requests
- [ ] **Secret Rotation:** Automatic API key rotation

## Known Limitations

1. **Agent Port:** Default port 8080 should be firewalled
2. **API Key Storage:** Keys stored encrypted but require secure backend
3. **No RBAC:** All-or-nothing API key access

## Security Audit

Last security review: 2026-03-29

- ✅ Code review completed
- ✅ Dependency audit passed
- ✅ No hardcoded secrets found
- ✅ Branch protection enabled
- ✅ CI/CD pipeline configured

---

**Thank you for helping keep uSipipo Agent secure!**
