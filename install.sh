#!/bin/bash

# Easy SSH Tunnel Manager Installation Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="easytunnel"
INSTALL_DIR="/usr/local/bin"
REPO_URL="https://github.com/ivikasavnish/easytunnel.git"

# Functions
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.23+ from https://golang.org/dl/"
        exit 1
    fi
    
    GO_VERSION=$(go version | cut -d' ' -f3 | cut -d'o' -f2)
    print_status "Found Go version: $GO_VERSION"
}

# Check if git is installed
check_git() {
    if ! command -v git &> /dev/null; then
        print_error "Git is not installed. Please install Git first."
        exit 1
    fi
}

# Build the application
build_app() {
    print_status "Building Easy SSH Tunnel Manager..."
    
    if [ -f "main.go" ]; then
        go build -o "$BINARY_NAME" main.go
        if [ $? -eq 0 ]; then
            print_success "Build completed successfully"
        else
            print_error "Build failed"
            exit 1
        fi
    else
        print_error "main.go not found. Please run this script from the project directory."
        exit 1
    fi
}

# Install the binary
install_binary() {
    print_status "Installing $BINARY_NAME to $INSTALL_DIR..."
    
    if [ ! -f "$BINARY_NAME" ]; then
        print_error "Binary not found. Please build first."
        exit 1
    fi
    
    if [ -w "$INSTALL_DIR" ]; then
        cp "$BINARY_NAME" "$INSTALL_DIR/"
    else
        print_status "Requesting sudo access to install to $INSTALL_DIR..."
        sudo cp "$BINARY_NAME" "$INSTALL_DIR/"
    fi
    
    if [ $? -eq 0 ]; then
        print_success "Installation completed successfully"
        print_status "You can now run: $BINARY_NAME"
    else
        print_error "Installation failed"
        exit 1
    fi
}

# Create config directory
create_config_dir() {
    CONFIG_DIR="$HOME/.easytunnel"
    if [ ! -d "$CONFIG_DIR" ]; then
        print_status "Creating configuration directory at $CONFIG_DIR..."
        mkdir -p "$CONFIG_DIR"
        print_success "Configuration directory created"
    else
        print_status "Configuration directory already exists"
    fi
}

# Check SSH
check_ssh() {
    if ! command -v ssh &> /dev/null; then
        print_warning "SSH client not found. Please install OpenSSH client."
    else
        print_success "SSH client found"
    fi
}

# Main installation function
main() {
    echo -e "${GREEN}"
    echo "========================================"
    echo "  Easy SSH Tunnel Manager Installer"
    echo "========================================"
    echo -e "${NC}"
    
    print_status "Starting installation..."
    
    # Run checks
    check_go
    check_git
    check_ssh
    
    # Build and install
    build_app
    create_config_dir
    install_binary
    
    echo ""
    echo -e "${GREEN}🚇 Installation completed successfully!${NC}"
    echo ""
    echo "Quick start:"
    echo "  1. Run: $BINARY_NAME"
    echo "  2. Open: http://localhost:10000"
    echo "  3. Add your SSH tunnels"
    echo ""
    echo "For more information, see the README.md file or visit:"
    echo "  https://github.com/ivikasavnish/easytunnel"
    echo ""
}

# Check if running from correct directory
if [ ! -f "main.go" ] && [ ! -f "go.mod" ]; then
    print_error "This script must be run from the easytunnel project directory."
    print_status "To clone and install:"
    echo ""
    echo "  git clone $REPO_URL"
    echo "  cd easytunnel"
    echo "  ./install.sh"
    echo ""
    exit 1
fi

# Run main installation
main
