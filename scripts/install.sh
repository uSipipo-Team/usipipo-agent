#!/bin/bash
set -e

# ============================================================================
# uSipipo Agent Installer v2.0
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
#   curl -fsSL .../install.sh | bash -s -- --version v0.1.10
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
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
VERSION="latest"
INSTALL_PATH="/usr/local/bin"
VERIFY_CHECKSUM=false
NEED_SUDO=false

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

print_step() {
    echo -e "${MAGENTA}🔧 $1${NC}"
}

print_package() {
    echo -e "${CYAN}📦 $1${NC}"
}

show_help() {
    head -35 "$0" | tail -30
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
        return 1
    fi
}

# ============================================================================
# Sudo Verification
# ============================================================================

verify_sudo() {
    print_step "Verifying sudo permissions..."
    
    if [ "$EUID" -eq 0 ]; then
        print_info "Running as root"
        NEED_SUDO=false
        return 0
    fi
    
    if sudo -v 2>/dev/null; then
        print_success "Sudo permissions granted"
        NEED_SUDO=true
        # Keep sudo alive
        while true; do sudo -n true; sleep 60; kill -0 "$$" 2>/dev/null || exit 0; done 2>/dev/null &
        return 0
    else
        print_warning "Sudo not available, installation may fail"
        NEED_SUDO=false
        return 0
    fi
}

# ============================================================================
# Dependency Installation
# ============================================================================

detect_package_manager() {
    if command -v apt &> /dev/null; then
        echo "apt"
    elif command -v yum &> /dev/null; then
        echo "yum"
    elif command -v dnf &> /dev/null; then
        echo "dnf"
    elif command -v apk &> /dev/null; then
        echo "apk"
    elif command -v pacman &> /dev/null; then
        echo "pacman"
    elif command -v zypper &> /dev/null; then
        echo "zypper"
    else
        echo ""
    fi
}

install_dependencies() {
    print_package "Checking dependencies..."
    
    local missing_deps=()
    
    # Check curl
    if ! require_command curl; then
        missing_deps+=("curl")
    fi
    
    # Check unzip
    if ! require_command unzip; then
        missing_deps+=("unzip")
    fi
    
    # Check sha256sum (optional, only for --verify-checksum)
    if [ "$VERIFY_CHECKSUM" = true ] && ! require_command sha256sum; then
        missing_deps+=("sha256sum")
    fi
    
    # If no missing dependencies, return
    if [ ${#missing_deps[@]} -eq 0 ]; then
        print_success "All dependencies installed"
        return 0
    fi
    
    print_warning "Missing dependencies: ${missing_deps[*]}"
    
    # Detect package manager
    local PM=$(detect_package_manager)
    
    if [ -z "$PM" ]; then
        print_error "No supported package manager found"
        print_info "Please install manually: ${missing_deps[*]}"
        exit 1
    fi
    
    print_info "Using package manager: $PM"
    
    # Install based on package manager
    case $PM in
        apt)
            install_with_apt "${missing_deps[@]}"
            ;;
        yum|dnf)
            install_with_yum "${missing_deps[@]}"
            ;;
        apk)
            install_with_apk "${missing_deps[@]}"
            ;;
        pacman)
            install_with_pacman "${missing_deps[@]}"
            ;;
        zypper)
            install_with_zypper "${missing_deps[@]}"
            ;;
    esac
    
    print_success "Dependencies installed successfully"
}

install_with_apt() {
    local deps=("$@")
    print_package "Installing with apt..."
    
    for i in 1 2 3; do
        print_info "Attempt $i/3..."
        if sudo apt update -qq && sudo apt install -y -qq "${deps[@]}"; then
            print_success "apt installation successful"
            return 0
        fi
        print_warning "Attempt $i failed, retrying in 2 seconds..."
        sleep 2
    done
    
    print_error "apt installation failed after 3 attempts"
    exit 1
}

install_with_yum() {
    local deps=("$@")
    print_package "Installing with yum/dnf..."
    
    for i in 1 2 3; do
        print_info "Attempt $i/3..."
        if sudo yum install -y -q "${deps[@]}" || sudo dnf install -y -q "${deps[@]}"; then
            print_success "yum/dnf installation successful"
            return 0
        fi
        print_warning "Attempt $i failed, retrying in 2 seconds..."
        sleep 2
    done
    
    print_error "yum/dnf installation failed after 3 attempts"
    exit 1
}

install_with_apk() {
    local deps=("$@")
    print_package "Installing with apk..."
    
    for i in 1 2 3; do
        print_info "Attempt $i/3..."
        if apk add --no-cache "${deps[@]}" 2>/dev/null; then
            print_success "apk installation successful"
            return 0
        fi
        print_warning "Attempt $i failed, retrying in 2 seconds..."
        sleep 2
    done
    
    print_error "apk installation failed after 3 attempts"
    exit 1
}

install_with_pacman() {
    local deps=("$@")
    print_package "Installing with pacman..."
    
    for i in 1 2 3; do
        print_info "Attempt $i/3..."
        if sudo pacman -Sy --noconfirm "${deps[@]}" 2>/dev/null; then
            print_success "pacman installation successful"
            return 0
        fi
        print_warning "Attempt $i failed, retrying in 2 seconds..."
        sleep 2
    done
    
    print_error "pacman installation failed after 3 attempts"
    exit 1
}

install_with_zypper() {
    local deps=("$@")
    print_package "Installing with zypper..."
    
    for i in 1 2 3; do
        print_info "Attempt $i/3..."
        if sudo zypper install -y -q "${deps[@]}" 2>/dev/null; then
            print_success "zypper installation successful"
            return 0
        fi
        print_warning "Attempt $i failed, retrying in 2 seconds..."
        sleep 2
    done
    
    print_error "zypper installation failed after 3 attempts"
    exit 1
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

echo ""
print_step "=========================================="
print_step "  uSipipo Agent Installation"
print_step "=========================================="
echo ""

# Verify sudo first
verify_sudo

# Install dependencies
install_dependencies

# Detect platform
OS=$(detect_os)
ARCH=$(detect_arch)

print_info "Detected platform: ${CYAN}${OS}/${ARCH}${NC}"

# Determine version
if [ "$VERSION" = "latest" ]; then
    # Get latest version from GitHub API
    VERSION=$(curl -s https://api.github.com/repos/uSipipo-Team/usipipo-agent/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        print_error "Failed to fetch latest version from GitHub"
        exit 1
    fi
    print_info "Latest version: ${CYAN}$VERSION${NC}"
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
print_step "Downloading uSipipo Agent ${VERSION}..."
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/${BINARY_NAME}.zip"; then
    print_error "Failed to download binary"
    print_info "Check if the version exists: https://github.com/uSipipo-Team/usipipo-agent/releases"
    exit 1
fi

# Verify checksum if requested
if [ "$VERIFY_CHECKSUM" = true ]; then
    print_step "Verifying checksum..."
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
print_step "Extracting binary..."
unzip -q "$TMP_DIR/${BINARY_NAME}.zip" -d "$TMP_DIR"

# Prepare installation
print_step "Installing to ${INSTALL_PATH}..."

# Check if install path exists
if [ ! -d "$INSTALL_PATH" ]; then
    print_info "Creating installation directory: $INSTALL_PATH"
    if [ "$NEED_SUDO" = true ]; then
        sudo mkdir -p "$INSTALL_PATH"
    else
        mkdir -p "$INSTALL_PATH"
    fi
fi

# Install binary
if [ "$NEED_SUDO" = true ]; then
    sudo mv "$TMP_DIR/${BINARY_NAME}" "$INSTALL_PATH/usipipo-agent"
    sudo chmod +x "$INSTALL_PATH/usipipo-agent"
else
    mv "$TMP_DIR/${BINARY_NAME}" "$INSTALL_PATH/usipipo-agent"
    chmod +x "$INSTALL_PATH/usipipo-agent"
fi

# Verify installation
print_step "Verifying installation..."
if ! command -v usipipo-agent &> /dev/null; then
    print_warning "Installation directory is not in PATH"
    print_info "Add $INSTALL_PATH to your PATH:"
    echo "  ${CYAN}export PATH=\"$INSTALL_PATH:\$PATH\"${NC}"
    
    # Add to shell profile if it doesn't exist
    PROFILE_FILE="$HOME/.bashrc"
    [ -f "$HOME/.zshrc" ] && PROFILE_FILE="$HOME/.zshrc"
    
    if ! grep -q "usipipo-agent" "$PROFILE_FILE" 2>/dev/null; then
        echo "" >> "$PROFILE_FILE"
        echo "# uSipipo Agent" >> "$PROFILE_FILE"
        echo "export PATH=\"$INSTALL_PATH:\$PATH\"" >> "$PROFILE_FILE"
        print_success "Added to $PROFILE_FILE"
        print_info "Run 'source $PROFILE_FILE' or restart your terminal"
    fi
fi

# Show version
if [ -x "$INSTALL_PATH/usipipo-agent" ]; then
    VERSION_INFO=$("$INSTALL_PATH/usipipo-agent" --version 2>&1 || echo "installed")
    print_success "=========================================="
    print_success "  uSipipo Agent Installed Successfully!"
    print_success "=========================================="
    print_info "Version: $VERSION_INFO"
    print_info "Location: $INSTALL_PATH/usipipo-agent"
else
    print_error "Installation failed - binary not executable"
    exit 1
fi

# Show next steps
echo ""
print_info "${CYAN}Next steps:${NC}"
echo ""
echo "  1. Configure environment variables:"
echo "     ${YELLOW}export AGENT_API_KEY=\"your-api-key\"${NC}"
echo "     ${YELLOW}export BACKEND_URL=\"https://api.usipipo.duckdns.org\"${NC}"
echo "     ${YELLOW}export SERVER_ID=\"your-server-id\"${NC}"
echo ""
echo "  2. Run the agent:"
echo "     ${YELLOW}usipipo-agent${NC}"
echo ""
echo "  3. For systemd service, see:"
echo "     ${CYAN}https://github.com/uSipipo-Team/usipipo-agent/blob/main/DEPLOYMENT.md${NC}"
echo ""
print_success "Installation complete! 🎉"
echo ""

exit 0
