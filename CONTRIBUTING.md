# Contributing to uSipipo Agent

Thank you for your interest in contributing to uSipipo Agent! This document provides guidelines and instructions for contributing.

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Prioritize security and user privacy

## Getting Started

### Prerequisites

- Go 1.21+
- Git
- GitHub account

### Setup

```bash
# Fork the repository
gh repo fork uSipipo-Team/usipipo-agent

# Clone your fork
git clone https://github.com/YOUR_USERNAME/usipipo-agent.git
cd usipipo-agent

# Install dependencies
go mod download
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

**Branch naming:**
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation
- `test/` - Tests
- `refactor/` - Code refactoring

### 2. Make Changes

- Write clean, readable code
- Add comments for complex logic
- Follow Go best practices
- Update documentation if needed

### 3. Test Your Changes

```bash
# Run tests
go test -v ./...

# Run linter
go fmt ./...
go vet ./...

# Build
go build -o usipipo-agent ./cmd/agent
```

### 4. Commit Your Changes

```bash
git add .
git commit -m "feat: add your feature description"
```

**Commit message format:**
```
type: description

[optional body]

[type]: feat, fix, docs, test, refactor, chore
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Pull Request Guidelines

### PR Title

Use conventional commits format:
- `feat: Add new feature`
- `fix: Fix bug in X`
- `docs: Update README`
- `test: Add tests for X`
- `refactor: Refactor X module`

### PR Description

Include:
1. **What** - What does this PR do?
2. **Why** - Why is this needed?
3. **How** - How does it work?
4. **Testing** - How was it tested?

### Example PR Description

```markdown
## What
Add auto-detection of package manager in install script

## Why
Users were unable to install dependencies automatically on different Linux distributions

## How
- Detect package manager (apt, yum, apk, pacman, zypper)
- Install missing dependencies with retry logic
- Add colorful output for better UX

## Testing
- Tested on Ubuntu 22.04 (apt)
- Tested on CentOS 7 (yum)
- Tested on Alpine 3.18 (apk)
```

## Code Review Process

1. **CI Checks:** All CI checks must pass
2. **Code Review:** At least 1 approval required
3. **Security Review:** Security-sensitive changes require extra review

### Review Checklist

Reviewers will check:
- [ ] Code follows project style
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No security issues introduced
- [ ] No breaking changes (or properly versioned)

## Security Considerations

### DO

- Use environment variables for secrets
- Validate all user input
- Use HTTPS for network communication
- Follow principle of least privilege

### DON'T

- Commit API keys or passwords
- Hardcode credentials
- Disable security features
- Skip input validation

## Reporting Issues

### Bug Reports

Include:
- Steps to reproduce
- Expected behavior
- Actual behavior
- System information (OS, Go version)

### Feature Requests

Include:
- Problem statement
- Proposed solution
- Use cases
- Alternatives considered

## Release Process

Releases follow semantic versioning (MAJOR.MINOR.PATCH):

- **MAJOR:** Breaking changes
- **MINOR:** New features (backwards compatible)
- **PATCH:** Bug fixes (backwards compatible)

### Release Checklist

- [ ] Update CHANGELOG.md
- [ ] Update version in code
- [ ] Run all tests
- [ ] Create release tag
- [ ] Publish GitHub release
- [ ] Update documentation

## Questions?

- **General:** Open a GitHub Discussion
- **Security:** See SECURITY.md
- **Code:** Check existing issues and PRs

---

**Thank you for contributing to uSipipo Agent! 🎉**
