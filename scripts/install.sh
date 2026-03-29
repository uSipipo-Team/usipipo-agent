#!/bin/bash
set -e

# ============================================================================
# uSipipo Agent Installer
# ============================================================================
# 
# Usage:
#   curl -fsSL https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/install.sh | bash
#
# Options:
#   --version <version>    Install specific version (default: latest)
#   --path <path>          Install to custom path (default: /usr/local/bin)
#   --verify-checksum      Verify SHA256 checksum before installation
#   --help                 Show this help message
#
# Examples:
#   # Install latest version
#   curl -fsSL https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/install.sh | bash
#
#   # Install specific version
#   curl -fsSL .../install.sh | bash -s -- --version v0.1.8
#
#   # Install to custom path
#   curl -fsSL .../install.sh | bash -s -- --path ~/.local/bin
#
#   # Verify checksum
#   curl -fsSL .../install.sh | bash -s -- --verify-checksum
#
# ============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
VERSION="latest"
INSTALL_PATH="/usr/local/bin"
VERIFY_CHECKSUM=false

# ============================================================================
# Helper Functions
# ============================================================================

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

show_help() {
    head -30 "$0" | tail -25
    exit 0
}

detect_os() {
    local os
    case "$(uname -s)" in
        Linux*)  os="linux" ;;
        Darwin*) os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *) 
            print_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    echo "$os"
}

detect_arch() {
    local arch
    case "$(uname -m)" in
        x86_64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l)  arch="arm" ;;
        *) 
            print_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    echo "$arch"
}

require_command() {
    local cmd=$1
    if ! command -v "$cmd" &> /dev/null; then
        print_error "Required command not found: $cmd"
        print_info "Please install $cmd and try again"
        exit 1
    fi
}

# ============================================================================
# Parse Command Line Arguments
# ============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --path)
            INSTALL_PATH="$2"
            shift 2
            ;;
        --verify-checksum)
            VERIFY_CHECKSUM=true
            shift
            ;;
        --help|-h)
            show_help
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            ;;
    esac
done

# ============================================================================
# Main Installation Process
# ============================================================================

print_info "Starting uSipipo Agent installation..."

# Check required commands
require_command "curl"
require_command "unzip"

# Detect platform
OS=$(detect_os)
ARCH=$(detect_arch)

print_info "Detected platform: ${OS}/${ARCH}"

# Determine version
if [ "$VERSION" = "latest" ]; then
    # Get latest version from GitHub API
    VERSION=$(curl -s https://api.github.com/repos/uSipipo-Team/usipipo-agent/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        print_error "Failed to fetch latest version from GitHub"
        exit 1
    fi
    print_info "Latest version: $VERSION"
fi

# Build download URL
BINARY_NAME="usipipo-agent-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/uSipipo-Team/usipipo-agent/releases/download/${VERSION}/${BINARY_NAME}.zip"
CHECKSUM_URL="https://github.com/uSipipo-Team/usipipo-agent/releases/download/${VERSION}/SHA256SUMS"

print_info "Downloading from: $DOWNLOAD_URL"

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download binary
print_info "Downloading uSipipo Agent ${VERSION}..."
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/${BINARY_NAME}.zip"; then
    print_error "Failed to download binary"
    print_info "Check if the version exists: https://github.com/uSipipo-Team/usipipo-agent/releases"
    exit 1
fi

# Verify checksum if requested
if [ "$VERIFY_CHECKSUM" = true ]; then
    print_info "Verifying checksum..."
    if curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/SHA256SUMS"; then
        cd "$TMP_DIR"
        if ! sha256sum -c SHA256SUMS --ignore-missing &> /dev/null; then
            print_error "Checksum verification failed!"
            print_error "The downloaded file may be corrupted or tampered with"
            exit 1
        fi
        print_success "Checksum verified successfully"
    else
        print_warning "Failed to download checksum file, skipping verification"
    fi
fi

# Extract binary
print_info "Extracting binary..."
unzip -q "$TMP_DIR/${BINARY_NAME}.zip" -d "$TMP_DIR"

# Prepare installation
print_info "Installing to ${INSTALL_PATH}..."

# Check if install path exists
if [ ! -d "$INSTALL_PATH" ]; then
    print_info "Creating installation directory: $INSTALL_PATH"
    mkdir -p "$INSTALL_PATH"
fi

# Determine if we need sudo
NEED_SUDO=false
if [ -w "$INSTALL_PATH" ]; then
    SUDO_CMD=""
else
    NEED_SUDO=true
    SUDO_CMD="sudo"
    print_info "Installation requires sudo privileges"
fi

# Install binary
if [ "$NEED_SUDO" = true ]; then
    $SUDO_CMD mv "$TMP_DIR/${BINARY_NAME}" "$INSTALL_PATH/usipipo-agent"
    $SUDO_CMD chmod +x "$INSTALL_PATH/usipipo-agent"
else
    mv "$TMP_DIR/${BINARY_NAME}" "$INSTALL_PATH/usipipo-agent"
    chmod +x "$INSTALL_PATH/usipipo-agent"
fi

# Verify installation
print_info "Verifying installation..."
if ! command -v usipipo-agent &> /dev/null; then
    print_warning "Installation directory is not in PATH"
    print_info "Add $INSTALL_PATH to your PATH:"
    echo "  export PATH=\"$INSTALL_PATH:\$PATH\""
    
    # Add to shell profile if it doesn't exist
    PROFILE_FILE="$HOME/.bashrc"
    [ -f "$HOME/.zshrc" ] && PROFILE_FILE="$HOME/.zshrc"
    
    if ! grep -q "usipipo-agent" "$PROFILE_FILE" 2>/dev/null; then
        echo "" >> "$PROFILE_FILE"
        echo "# uSipipo Agent" >> "$PROFILE_FILE"
        echo "export PATH=\"$INSTALL_PATH:\$PATH\"" >> "$PROFILE_FILE"
        print_success "Added to $PROFILE_FILE"
    fi
fi

# Show version
if [ -x "$INSTALL_PATH/usipipo-agent" ]; then
    VERSION_INFO=$("$INSTALL_PATH/usipipo-agent" --version 2>&1 || echo "installed")
    print_success "uSipipo Agent installed successfully!"
    print_info "Version: $VERSION_INFO"
    print_info "Location: $INSTALL_PATH/usipipo-agent"
else
    print_error "Installation failed - binary not executable"
    exit 1
fi

# Show next steps
echo ""
print_info "Next steps:"
echo "  1. Configure environment variables:"
echo "     export AGENT_API_KEY=\"your-api-key\""
echo "     export BACKEND_URL=\"https://api.usipipo.duckdns.org\""
echo "     export SERVER_ID=\"your-server-id\""
echo ""
echo "  2. Run the agent:"
echo "     usipipo-agent"
echo ""
echo "  3. For systemd service, see:"
echo "     https://github.com/uSipipo-Team/usipipo-agent/blob/main/DEPLOYMENT.md"
echo ""

exit 0
