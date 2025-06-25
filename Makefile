# Easy SSH Tunnel Manager Makefile

# Variables
BINARY_NAME=easytunnel
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags for versioning
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.CommitHash=$(COMMIT_HASH)"

# Binary names for different platforms
BINARY_UNIX=$(BINARY_NAME)-linux-amd64
BINARY_WINDOWS=$(BINARY_NAME)-windows-amd64.exe
BINARY_DARWIN_AMD64=$(BINARY_NAME)-darwin-amd64
BINARY_DARWIN_ARM64=$(BINARY_NAME)-darwin-arm64
BINARY_LINUX_ARM64=$(BINARY_NAME)-linux-arm64

# Release directory
DIST_DIR=dist
RELEASE_DIR=$(DIST_DIR)/$(VERSION)

# Default target
.PHONY: all
all: clean build

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) main.go

# Build for different platforms
.PHONY: build-linux
build-linux:
	@echo "Building for Linux AMD64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_UNIX) main.go

.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for Linux ARM64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_LINUX_ARM64) main.go

.PHONY: build-windows
build-windows:
	@echo "Building for Windows AMD64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_WINDOWS) main.go

.PHONY: build-darwin-amd64
build-darwin-amd64:
	@echo "Building for macOS AMD64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_DARWIN_AMD64) main.go

.PHONY: build-darwin-arm64  
build-darwin-arm64:
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_DARWIN_ARM64) main.go

.PHONY: build-all
build-all: build-linux build-linux-arm64 build-windows build-darwin-amd64 build-darwin-arm64

# Run the application
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Run in development mode with auto-restart (requires air)
.PHONY: dev
dev:
	@if command -v air > /dev/null; then \
		echo "Running in development mode with air..."; \
		air; \
	else \
		echo "Air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Running normally..."; \
		$(MAKE) run; \
	fi

# Test the application
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	go clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(BINARY_WINDOWS)
	rm -f $(BINARY_DARWIN_AMD64)
	rm -f $(BINARY_DARWIN_ARM64)
	rm -f $(BINARY_LINUX_ARM64)
	rm -rf $(DIST_DIR)

# Release targets
.PHONY: release
release: clean build-all package
	@echo "Release $(VERSION) ready in $(RELEASE_DIR)/"

.PHONY: package
package: build-all
	@echo "Packaging release $(VERSION)..."
	@mkdir -p $(RELEASE_DIR)
	
	# Package Linux AMD64
	@mkdir -p $(RELEASE_DIR)/linux-amd64
	@cp $(BINARY_UNIX) $(RELEASE_DIR)/linux-amd64/$(BINARY_NAME)
	@cp README.md LICENSE QUICKSTART.md $(RELEASE_DIR)/linux-amd64/
	@cp debug-ssh.sh diagnose-tunnel.sh install.sh $(RELEASE_DIR)/linux-amd64/
	@chmod +x $(RELEASE_DIR)/linux-amd64/*.sh
	@tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz -C $(RELEASE_DIR)/linux-amd64 .
	
	# Package Linux ARM64
	@mkdir -p $(RELEASE_DIR)/linux-arm64
	@cp $(BINARY_LINUX_ARM64) $(RELEASE_DIR)/linux-arm64/$(BINARY_NAME)
	@cp README.md LICENSE QUICKSTART.md $(RELEASE_DIR)/linux-arm64/
	@cp debug-ssh.sh diagnose-tunnel.sh install.sh $(RELEASE_DIR)/linux-arm64/
	@chmod +x $(RELEASE_DIR)/linux-arm64/*.sh
	@tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz -C $(RELEASE_DIR)/linux-arm64 .
	
	# Package Windows AMD64
	@mkdir -p $(RELEASE_DIR)/windows-amd64
	@cp $(BINARY_WINDOWS) $(RELEASE_DIR)/windows-amd64/
	@cp README.md LICENSE QUICKSTART.md $(RELEASE_DIR)/windows-amd64/
	@cp debug-ssh.sh diagnose-tunnel.sh install.sh $(RELEASE_DIR)/windows-amd64/
	@cd $(RELEASE_DIR)/windows-amd64 && zip -r ../$(BINARY_NAME)-$(VERSION)-windows-amd64.zip .
	
	# Package macOS AMD64
	@mkdir -p $(RELEASE_DIR)/darwin-amd64
	@cp $(BINARY_DARWIN_AMD64) $(RELEASE_DIR)/darwin-amd64/$(BINARY_NAME)
	@cp README.md LICENSE QUICKSTART.md $(RELEASE_DIR)/darwin-amd64/
	@cp debug-ssh.sh diagnose-tunnel.sh install.sh $(RELEASE_DIR)/darwin-amd64/
	@chmod +x $(RELEASE_DIR)/darwin-amd64/*.sh
	@tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz -C $(RELEASE_DIR)/darwin-amd64 .
	
	# Package macOS ARM64
	@mkdir -p $(RELEASE_DIR)/darwin-arm64
	@cp $(BINARY_DARWIN_ARM64) $(RELEASE_DIR)/darwin-arm64/$(BINARY_NAME)
	@cp README.md LICENSE QUICKSTART.md $(RELEASE_DIR)/darwin-arm64/
	@cp debug-ssh.sh diagnose-tunnel.sh install.sh $(RELEASE_DIR)/darwin-arm64/
	@chmod +x $(RELEASE_DIR)/darwin-arm64/*.sh
	@tar -czf $(RELEASE_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz -C $(RELEASE_DIR)/darwin-arm64 .
	
	@echo "Packaged files:"
	@ls -la $(RELEASE_DIR)/*.tar.gz $(RELEASE_DIR)/*.zip
	
	# Generate checksums
	@cd $(RELEASE_DIR) && shasum -a 256 *.tar.gz *.zip > checksums.txt
	@echo "Generated checksums:"
	@cat $(RELEASE_DIR)/checksums.txt

# Create Debian package
.PHONY: deb
deb: build-linux
	@echo "Creating Debian package..."
	@mkdir -p $(DIST_DIR)/deb/DEBIAN
	@mkdir -p $(DIST_DIR)/deb/usr/local/bin
	@mkdir -p $(DIST_DIR)/deb/usr/share/doc/$(BINARY_NAME)
	@mkdir -p $(DIST_DIR)/deb/usr/share/man/man1
	
	@cp $(BINARY_UNIX) $(DIST_DIR)/deb/usr/local/bin/$(BINARY_NAME)
	@cp README.md LICENSE QUICKSTART.md $(DIST_DIR)/deb/usr/share/doc/$(BINARY_NAME)/
	@cp debug-ssh.sh diagnose-tunnel.sh $(DIST_DIR)/deb/usr/local/bin/
	@chmod +x $(DIST_DIR)/deb/usr/local/bin/*
	
	@echo "Package: $(BINARY_NAME)" > $(DIST_DIR)/deb/DEBIAN/control
	@echo "Version: $(VERSION)" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Section: net" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Priority: optional" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Architecture: amd64" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Maintainer: Easy Tunnel Manager <easytunnel@example.com>" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Description: Easy SSH Tunnel Manager" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo " A web-based SSH tunnel manager with automatic reconnection," >> $(DIST_DIR)/deb/DEBIAN/control
	@echo " health monitoring, and network change detection." >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Homepage: https://github.com/ivikasavnish/easytunnel" >> $(DIST_DIR)/deb/DEBIAN/control
	@echo "Depends: openssh-client" >> $(DIST_DIR)/deb/DEBIAN/control
	
	@dpkg-deb --build $(DIST_DIR)/deb $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-amd64.deb
	@echo "Debian package created: $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-amd64.deb"

# Create macOS installer package
.PHONY: pkg
pkg: build-darwin-amd64 build-darwin-arm64
	@echo "Creating macOS installer packages..."
	@mkdir -p $(DIST_DIR)/macos-pkg/amd64/usr/local/bin
	@mkdir -p $(DIST_DIR)/macos-pkg/arm64/usr/local/bin
	
	# AMD64 package
	@cp $(BINARY_DARWIN_AMD64) $(DIST_DIR)/macos-pkg/amd64/usr/local/bin/$(BINARY_NAME)
	@cp debug-ssh.sh diagnose-tunnel.sh $(DIST_DIR)/macos-pkg/amd64/usr/local/bin/
	@chmod +x $(DIST_DIR)/macos-pkg/amd64/usr/local/bin/*
	
	# ARM64 package  
	@cp $(BINARY_DARWIN_ARM64) $(DIST_DIR)/macos-pkg/arm64/usr/local/bin/$(BINARY_NAME)
	@cp debug-ssh.sh diagnose-tunnel.sh $(DIST_DIR)/macos-pkg/arm64/usr/local/bin/
	@chmod +x $(DIST_DIR)/macos-pkg/arm64/usr/local/bin/*
	
	@echo "macOS package structure created in $(DIST_DIR)/macos-pkg/"
	@echo "Use 'pkgbuild' and 'productbuild' to create final .pkg files"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	@if command -v golangci-lint > /dev/null; then \
		echo "Running linter..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Install the application
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Uninstall the application
.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

# Show help
.PHONY: help
help:
	@echo "Easy SSH Tunnel Manager - Build & Release"
	@echo "========================================"
	@echo ""
	@echo "Development:"
	@echo "  build              - Build the application for current platform"
	@echo "  run                - Build and run the application"
	@echo "  dev                - Run in development mode (requires air)"
	@echo "  test               - Run tests"
	@echo "  fmt                - Format code"
	@echo "  lint               - Lint code (requires golangci-lint)"
	@echo "  deps               - Download and tidy dependencies"
	@echo ""
	@echo "Cross-platform builds:"
	@echo "  build-linux        - Build for Linux AMD64"
	@echo "  build-linux-arm64  - Build for Linux ARM64"
	@echo "  build-windows      - Build for Windows AMD64"
	@echo "  build-darwin-amd64 - Build for macOS AMD64"
	@echo "  build-darwin-arm64 - Build for macOS ARM64 (Apple Silicon)"
	@echo "  build-all          - Build for all platforms"
	@echo ""
	@echo "Release & Packaging:"
	@echo "  release            - Create full release with all platforms"
	@echo "  package            - Package all builds into archives"
	@echo "  deb                - Create Debian package (.deb)"
	@echo "  pkg                - Create macOS package structure"
	@echo ""
	@echo "Installation:"
	@echo "  install            - Install binary to /usr/local/bin"
	@echo "  uninstall          - Remove binary from /usr/local/bin"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean              - Clean build artifacts and dist folder"
	@echo "  help               - Show this help message"
	@echo ""
	@echo "Current version: $(VERSION)"
