#!/bin/bash
# Universal Easy SSH Tunnel Manager Installer
# Supports Linux, macOS, and Windows (via WSL/Git Bash)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="ivikasavnish"
REPO_NAME="easytunnel"
BINARY_NAME="easytunnel"
INSTALL_DIR="/usr/local/bin"

# Detect platform
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        linux*)
            OS="linux"
            ;;
        darwin*)
            OS="darwin"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            echo -e "${RED}Unsupported operating system: $os${NC}"
            exit 1
            ;;
    esac
    
    case $arch in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}Unsupported architecture: $arch${NC}"
            exit 1
            ;;
    esac
    
    PLATFORM="${OS}-${ARCH}"
    echo -e "${BLUE}Detected platform: $PLATFORM${NC}"
}

# Get latest release version
get_latest_version() {
    echo -e "${BLUE}Fetching latest release information...${NC}"
    
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -s "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest" | \
                  grep '"tag_name":' | \
                  sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest" | \
                  grep '"tag_name":' | \
                  sed -E 's/.*"([^"]+)".*/\1/')
    else
        echo -e "${RED}Error: curl or wget is required${NC}"
        exit 1
    fi
    
    if [ -z "$VERSION" ]; then
        echo -e "${RED}Error: Could not fetch latest version${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Latest version: $VERSION${NC}"
}

# Download and install
install_binary() {
    local temp_dir=$(mktemp -d)
    local file_ext="tar.gz"
    
    if [ "$OS" = "windows" ]; then
        file_ext="zip"
    fi
    
    local download_url="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/$VERSION/${BINARY_NAME}-${VERSION}-${PLATFORM}.${file_ext}"
    local archive_file="$temp_dir/${BINARY_NAME}-${VERSION}-${PLATFORM}.${file_ext}"
    
    echo -e "${BLUE}Downloading $download_url${NC}"
    
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "$archive_file" "$download_url"
    else
        wget -O "$archive_file" "$download_url"
    fi
    
    if [ ! -f "$archive_file" ]; then
        echo -e "${RED}Error: Download failed${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Extracting archive...${NC}"
    cd "$temp_dir"
    
    if [ "$file_ext" = "zip" ]; then
        unzip -q "$archive_file"
    else
        tar -xzf "$archive_file"
    fi
    
    # Find the binary
    local binary_path=""
    if [ -f "$BINARY_NAME" ]; then
        binary_path="$BINARY_NAME"
    elif [ -f "${BINARY_NAME}.exe" ]; then
        binary_path="${BINARY_NAME}.exe"
    else
        echo -e "${RED}Error: Binary not found in archive${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Installing to $INSTALL_DIR...${NC}"
    
    # Check if we need sudo
    if [ ! -w "$INSTALL_DIR" ]; then
        echo -e "${YELLOW}Admin privileges required for installation${NC}"
        sudo mkdir -p "$INSTALL_DIR"
        sudo cp "$binary_path" "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
        
        # Install helper scripts if they exist
        for script in debug-ssh.sh diagnose-tunnel.sh; do
            if [ -f "$script" ]; then
                sudo cp "$script" "$INSTALL_DIR/"
                sudo chmod +x "$INSTALL_DIR/$script"
            fi
        done
    else
        mkdir -p "$INSTALL_DIR"
        cp "$binary_path" "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
        
        # Install helper scripts if they exist
        for script in debug-ssh.sh diagnose-tunnel.sh; do
            if [ -f "$script" ]; then
                cp "$script" "$INSTALL_DIR/"
                chmod +x "$INSTALL_DIR/$script"
            fi
        done
    fi
    
    # Cleanup
    rm -rf "$temp_dir"
    
    echo -e "${GREEN}Installation complete!${NC}"
}

# Verify installation
verify_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        local installed_version=$($BINARY_NAME --version 2>/dev/null || echo "unknown")
        echo -e "${GREEN}✓ $BINARY_NAME installed successfully${NC}"
        echo -e "${BLUE}Version: $installed_version${NC}"
        echo -e "${BLUE}Location: $(which $BINARY_NAME)${NC}"
    else
        echo -e "${YELLOW}Warning: $BINARY_NAME not found in PATH${NC}"
        echo -e "${YELLOW}You may need to add $INSTALL_DIR to your PATH${NC}"
        echo -e "${YELLOW}Run: export PATH=\"$INSTALL_DIR:\$PATH\"${NC}"
    fi
}

# Show usage instructions
show_usage() {
    echo ""
    echo -e "${GREEN}🚇 Easy SSH Tunnel Manager installed!${NC}"
    echo ""
    echo -e "${BLUE}Quick Start:${NC}"
    echo "  $BINARY_NAME                    # Start the application"
    echo "  $BINARY_NAME --help             # Show help"
    echo "  debug-ssh.sh 'ssh command'      # Debug SSH issues"
    echo "  diagnose-tunnel.sh 'ssh cmd'    # Diagnose tunnel problems"
    echo ""
    echo -e "${BLUE}Web Interface:${NC}"
    echo "  Open http://localhost:10000 in your browser"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo "  https://github.com/$REPO_OWNER/$REPO_NAME"
}

# Uninstall function
uninstall() {
    echo -e "${YELLOW}Uninstalling Easy SSH Tunnel Manager...${NC}"
    
    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        if [ -w "$INSTALL_DIR" ]; then
            rm -f "$INSTALL_DIR/$BINARY_NAME"
            rm -f "$INSTALL_DIR/debug-ssh.sh"
            rm -f "$INSTALL_DIR/diagnose-tunnel.sh"
        else
            sudo rm -f "$INSTALL_DIR/$BINARY_NAME"
            sudo rm -f "$INSTALL_DIR/debug-ssh.sh"
            sudo rm -f "$INSTALL_DIR/diagnose-tunnel.sh"
        fi
        echo -e "${GREEN}Uninstallation complete${NC}"
    else
        echo -e "${YELLOW}$BINARY_NAME not found in $INSTALL_DIR${NC}"
    fi
}

# Main installation flow
main() {
    echo -e "${GREEN}🚇 Easy SSH Tunnel Manager Installer${NC}"
    echo -e "${BLUE}=====================================${NC}"
    echo ""
    
    # Check for uninstall flag
    if [ "$1" = "--uninstall" ] || [ "$1" = "uninstall" ]; then
        uninstall
        exit 0
    fi
    
    # Check for help flag
    if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
        echo "Usage: $0 [--uninstall|--help]"
        echo ""
        echo "Options:"
        echo "  --uninstall    Uninstall Easy SSH Tunnel Manager"
        echo "  --help         Show this help message"
        echo ""
        echo "This script will automatically:"
        echo "  1. Detect your platform (OS and architecture)"
        echo "  2. Download the latest release"
        echo "  3. Install to $INSTALL_DIR"
        echo "  4. Make the binary executable"
        exit 0
    fi
    
    detect_platform
    get_latest_version
    install_binary
    verify_installation
    show_usage
}

# Run main function
main "$@"
