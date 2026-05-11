#!/bin/bash
set -e

# ============================================================================
# uSipipo Agent Installer v3.0
# ============================================================================
# 
# Usage:
#   curl -fsSL https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/install.sh | bash
#
# Options:
#   --version <version>    Install specific version (default: latest)
#   --path <path>          Install to custom path (default: /opt/usipipo-agent)
#   --service              Install systemd service for auto-start
#   --no-service           Skip systemd service installation
#   --verify-checksum      Verify SHA256 checksum before installation
#   --interactive, -i      Interactive mode with prompts
#   --no-color             Disable colored output
#   --help, -h             Show this help message
#
# Examples:
#   # Default installation (non-interactive)
#   curl -fsSL https://github.com/uSipipo-Team/usipipo-agent/releases/latest/download/install.sh | bash
#
#   # Interactive mode
#   curl -fsSL .../install.sh | bash -s -- --interactive
#
#   # Install with systemd service
#   curl -fsSL .../install.sh | bash -s -- --service
#
#   # Specific version
#   curl -fsSL .../install.sh | bash -s -- --version v0.1.11
#
# ============================================================================

# ============================================================================
# Color Configuration
# ============================================================================

setup_colors() {
    # Disable colors if NO_COLOR is set or not a terminal
    if [ -n "$NO_COLOR" ] || [ ! -t 1 ]; then
        RED=''
        GREEN=''
        YELLOW=''
        BLUE=''
        MAGENTA=''
        CYAN=''
        NC=''
        BOLD=''
    else
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[1;33m'
        BLUE='\033[0;34m'
        MAGENTA='\033[0;35m'
        CYAN='\033[0;36m'
        NC='\033[0m'
        BOLD='\033[1m'
    fi
}

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
    echo -e "${RED}❌ $1${NC}" >&2
}

print_step() {
    echo -e "${MAGENTA}🔧 $1${NC}"
}

print_package() {
    echo -e "${CYAN}📦 $1${NC}"
}

print_header() {
    echo ""
    echo -e "${BOLD}${CYAN}=========================================${NC}"
    echo -e "${BOLD}${CYAN}  $1${NC}"
    echo -e "${BOLD}${CYAN}=========================================${NC}"
    echo ""
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
    if [ "$EUID" -eq 0 ]; then
        NEED_SUDO=false
        return 0
    fi
    
    if sudo -v 2>/dev/null; then
        NEED_SUDO=true
        # Keep sudo alive
        while true; do sudo -n true; sleep 60; kill -0 "$$" 2>/dev/null || exit 0; done 2>/dev/null &
        return 0
    else
        NEED_SUDO=false
        return 0
    fi
}

# ============================================================================
# Create usipipo User
# ============================================================================

create_usipipo_user() {
    print_step "Creating usipipo user and group..."
    
    # Create group if not exists
    if ! getent group usipipo > /dev/null 2>&1; then
        sudo groupadd --system usipipo
        print_info "Created group: usipipo"
    else
        print_info "Group already exists: usipipo"
    fi
    
    # Create user if not exists
    if ! id usipipo > /dev/null 2>&1; then
        sudo useradd --system --no-create-home --gid usipipo --shell /bin/false usipipo
        print_info "Created user: usipipo"
    else
        print_info "User already exists: usipipo"
    fi
    
    # Add to sudo group
    sudo usermod -aG sudo usipipo
    print_info "Added usipipo to sudo group"
    
    # Set ownership of installation directory
    if [ -d "$INSTALL_PATH" ]; then
        sudo chown -R usipipo:usipipo "$INSTALL_PATH"
        print_info "Set ownership of $INSTALL_PATH to usipipo:usipipo"
    fi
}

# ============================================================================
# Update Function
# ============================================================================

do_update() {
    print_header "Checking for Updates"
    
    # Get latest version
    LATEST=$(curl -s https://api.github.com/repos/uSipipo-Team/usipipo-agent/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    # Get current version
    CURRENT=$(/opt/usipipo-agent/usipipo-agent --version 2>&1 | grep -oP 'v\K[0-9.]+' || echo "unknown")
    
    print_info "Current version: $CURRENT"
    print_info "Latest version: $LATEST"
    
    if [ "$LATEST" = "$CURRENT" ]; then
        print_success "Already up to date ($CURRENT)"
        return 0
    fi
    
    print_info "Updating from $CURRENT to $LATEST..."
    
    # Stop service
    print_step "Stopping service..."
    sudo systemctl stop usipipo-agent || true
    
    # Download new version
    print_step "Downloading v$LATEST..."
    OS=$(detect_os)
    ARCH=$(detect_arch)
    if ! curl -fsSL -o /tmp/usipipo-agent.zip \
        "https://github.com/uSipipo-Team/usipipo-agent/releases/download/$LATEST/usipipo-agent-${OS}-${ARCH}.zip"; then
        print_error "Failed to download update"
        sudo systemctl start usipipo-agent
        exit 1
    fi
    
    # Extract and replace
    print_step "Installing new version..."
    unzip -q /tmp/usipipo-agent.zip -d /tmp
    sudo mv /tmp/usipipo-agent-${OS}-${ARCH} /opt/usipipo-agent/usipipo-agent
    sudo chmod +x /opt/usipipo-agent/usipipo-agent
    
    # Restart service
    print_step "Restarting service..."
    sudo systemctl start usipipo-agent
    sleep 2
    
    # Verify
    if sudo systemctl is-active --quiet usipipo-agent; then
        print_success "Update completed successfully!"
        print_info "Version: $LATEST"
    else
        print_error "Service failed to start. Check logs: sudo journalctl -u usipipo-agent"
        exit 1
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
# Interactive Mode
# ============================================================================

run_interactive() {
    echo ""
    print_header "Interactive Installation"
    
    # Ask for version
    read -p "Install version [latest]: " -r
    if [ -n "$REPLY" ]; then
        VERSION="$REPLY"
    fi
    
    # Ask for install path
    read -p "Install path [/opt/usipipo-agent]: " -r
    if [ -n "$REPLY" ]; then
        INSTALL_PATH="$REPLY"
    fi
    
    # Ask for systemd service
    read -p "Install systemd service for auto-start? [Y/n]: " -r
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        INSTALL_SERVICE=false
    else
        INSTALL_SERVICE=true
    fi
    
    # Ask for checksum verification
    read -p "Verify SHA256 checksum? [y/N]: " -r
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        VERIFY_CHECKSUM=true
    fi
    
    echo ""
    print_info "Configuration:"
    echo "  Version: $VERSION"
    echo "  Path: $INSTALL_PATH"
    echo "  Service: $INSTALL_SERVICE"
    echo "  Verify Checksum: $VERIFY_CHECKSUM"
    echo ""
    read -p "Continue with installation? [Y/n]: " -r
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        print_info "Installation cancelled"
        exit 0
    fi
    echo ""
}

# ============================================================================
# Systemd Service Installation
# ============================================================================

install_systemd_service() {
    print_step "Installing systemd service..."

    # Create service file
    local SERVICE_FILE="/tmp/usipipo-agent.service"

    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=uSipipo VPN Agent
After=network.target

[Service]
Type=simple
User=usipipo
Group=usipipo
WorkingDirectory=$INSTALL_PATH
EnvironmentFile=$INSTALL_PATH/.env
ExecStart=$INSTALL_PATH/usipipo-agent
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=usipipo-agent

[Install]
WantedBy=multi-user.target
EOF
    
    # Install service
    if ! sudo cp "$SERVICE_FILE" /etc/systemd/system/usipipo-agent.service; then
        print_error "Failed to install systemd service"
        return 1
    fi
    
    # Create log directory
    if ! sudo mkdir -p /var/log/usipipo-agent; then
        print_error "Failed to create log directory"
        return 1
    fi
    
    if ! sudo chown $(whoami):$(id -gn) /var/log/usipipo-agent; then
        print_error "Failed to set log directory permissions"
        return 1
    fi
    
    # Reload and enable
    if ! sudo systemctl daemon-reload; then
        print_error "Failed to reload systemd"
        return 1
    fi
    
    if ! sudo systemctl enable usipipo-agent; then
        print_error "Failed to enable service"
        return 1
    fi
    
    # Start service
    print_info "Starting usipipo-agent service..."
    if ! sudo systemctl start usipipo-agent; then
        print_warning "Service failed to start. Check logs with: sudo journalctl -u usipipo-agent"
        return 1
    fi
    
    # Wait for service to start
    sleep 2
    
    # Check status
    if sudo systemctl is-active --quiet usipipo-agent; then
        print_success "Systemd service installed and running"
        return 0
    else
        print_warning "Service installed but not running. Start manually with: sudo systemctl start usipipo-agent"
        return 1
    fi
}

# ============================================================================
# Parse Command Line Arguments
# ============================================================================

parse_arguments() {
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
            --service)
                INSTALL_SERVICE=true
                shift
                ;;
            --no-service)
                INSTALL_SERVICE=false
                shift
                ;;
            --verify-checksum)
                VERIFY_CHECKSUM=true
                shift
                ;;
            --interactive|-i)
                INTERACTIVE=true
                shift
                ;;
            --no-color)
                export NO_COLOR=1
                shift
                ;;
            --update|-u)
                UPDATE_MODE=true
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
}

# ============================================================================
# Main Installation Process
# ============================================================================

main() {
    # Initialize
    setup_colors
    parse_arguments "$@"

    # Print header
    print_header "uSipipo Agent Installation"

    # Update mode
    if [ "$UPDATE_MODE" = true ]; then
        do_update
        exit 0
    fi

    # Interactive mode
    if [ "$INTERACTIVE" = true ]; then
        run_interactive
    fi
    
    # Verify sudo
    print_step "Verifying sudo permissions..."
    verify_sudo
    if [ "$NEED_SUDO" = true ]; then
        print_success "Sudo permissions granted"
    else
        print_info "Running without sudo (some features may be limited)"
    fi
    
    # Install dependencies
    install_dependencies
    
    # Create usipipo user
    print_step "Creating usipipo user..."
    create_usipipo_user
    
    # Detect platform
    OS=$(detect_os)
    ARCH=$(detect_arch)
    print_info "Detected platform: ${CYAN}${OS}/${ARCH}${NC}"
    
    # Determine version
    if [ "$VERSION" = "latest" ]; then
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
    if ! unzip -q "$TMP_DIR/${BINARY_NAME}.zip" -d "$TMP_DIR"; then
        print_error "Failed to extract binary"
        exit 1
    fi
    
    # Prepare installation directory
    print_step "Installing to ${INSTALL_PATH}..."
    
    # Create directory
    if [ "$NEED_SUDO" = true ]; then
        if ! sudo mkdir -p "$INSTALL_PATH"; then
            print_error "Failed to create installation directory"
            exit 1
        fi
        if ! sudo chown $(whoami):$(id -gn) "$INSTALL_PATH"; then
            print_error "Failed to set directory ownership"
            exit 1
        fi
    else
        if ! mkdir -p "$INSTALL_PATH"; then
            print_error "Failed to create installation directory"
            exit 1
        fi
    fi
    
    # Install binary
    if ! mv "$TMP_DIR/${BINARY_NAME}" "$INSTALL_PATH/usipipo-agent"; then
        print_error "Failed to install binary"
        exit 1
    fi
    
    if ! chmod +x "$INSTALL_PATH/usipipo-agent"; then
        print_error "Failed to make binary executable"
        exit 1
    fi
    
    # Create .env from template if not exists
    if [ ! -f "$INSTALL_PATH/.env" ]; then
        print_step "Creating configuration file..."
        
        # Generate unique server ID
        local SERVER_ID="$(hostname)-$(date +%s)"
        local TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        
        # Create .env from template
        cat > "$INSTALL_PATH/.env" << EOF
# uSipipo Agent Configuration
# Generated: $TIMESTAMP

# Agent API Key (for backend communication)
AGENT_API_KEY=your-api-key-here

# Backend URL (HTTP or HTTPS)
BACKEND_URL=http://localhost:8001

# Server ID (unique identifier for this VPS)
SERVER_ID=$SERVER_ID

        # WireGuard Configuration
WG_INTERFACE=wg0
WG_SERVER_IP=localhost
WG_SERVER_PORT=51820

# Advanced Settings
AGENT_PORT=8080
METRICS_INTERVAL=60
LOG_LEVEL=INFO
EOF
        chmod 600 "$INSTALL_PATH/.env"
        print_success "Configuration file created: $INSTALL_PATH/.env"
        print_warning "Please edit $INSTALL_PATH/.env with your settings"
    fi
    
    # Verify installation
    print_step "Verifying installation..."
    if [ ! -x "$INSTALL_PATH/usipipo-agent" ]; then
        print_error "Installation failed - binary not executable"
        exit 1
    fi
    
    # Show version
    VERSION_INFO=$("$INSTALL_PATH/usipipo-agent" --version 2>&1 || echo "installed")
    print_success "=========================================="
    print_success "  uSipipo Agent Installed Successfully!"
    print_success "=========================================="
    print_info "Version: $VERSION_INFO"
    print_info "Location: $INSTALL_PATH/usipipo-agent"
    
    # Install systemd service if requested
    if [ "$INSTALL_SERVICE" = true ]; then
        echo ""
        if install_systemd_service; then
            print_success "Systemd service configured"
        else
            print_warning "Systemd service installation failed"
        fi
    fi
    
    # Show next steps
    echo ""
    print_info "${CYAN}Next steps:${NC}"
    echo ""
    echo "  1. Configure environment variables:"
    echo "     ${YELLOW}nano $INSTALL_PATH/.env${NC}"
    echo ""
    echo "  2. Edit the following settings:"
    echo "     ${YELLOW}AGENT_API_KEY${NC} - Your API key from backend"
    echo "     ${YELLOW}BACKEND_URL${NC} - Backend API URL"
    echo "     ${YELLOW}SERVER_ID${NC} - Unique server identifier"
    echo ""
    echo "  3. Run the agent:"
    if [ "$INSTALL_SERVICE" = true ]; then
        echo "     ${YELLOW}sudo systemctl start usipipo-agent${NC}"
        echo "     ${YELLOW}sudo systemctl status usipipo-agent${NC}"
    else
        echo "     ${YELLOW}cd $INSTALL_PATH${NC}"
        echo "     ${YELLOW}source .env && usipipo-agent${NC}"
    fi
    echo ""
    echo "  4. For advanced configuration, see:"
    echo "     ${CYAN}https://github.com/uSipipo-Team/usipipo-agent/blob/main/DEPLOYMENT.md${NC}"
    echo ""
    print_success "Installation complete! 🎉"
    echo ""
    
    exit 0
}

# Default values
VERSION="latest"
INSTALL_PATH="/opt/usipipo-agent"
INSTALL_SERVICE=false
VERIFY_CHECKSUM=false
INTERACTIVE=false
UPDATE_MODE=false
NEED_SUDO=false

# Run main
main "$@"
