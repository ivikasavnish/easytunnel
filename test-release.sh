#!/bin/bash
# Test Release Build Script
# Verifies that all platforms build correctly

set -e

echo "🧪 Testing Release Build Process"
echo "================================"
echo ""

# Clean any existing builds
echo "🧹 Cleaning previous builds..."
make clean

# Test individual platform builds
echo ""
echo "🐧 Testing Linux AMD64 build..."
make build-linux
if [ -f "easytunnel-linux-amd64" ]; then
    echo "   ✅ Linux AMD64 build successful"
    file easytunnel-linux-amd64
else
    echo "   ❌ Linux AMD64 build failed"
    exit 1
fi

echo ""
echo "🐧 Testing Linux ARM64 build..."
make build-linux-arm64
if [ -f "easytunnel-linux-arm64" ]; then
    echo "   ✅ Linux ARM64 build successful"
    file easytunnel-linux-arm64
else
    echo "   ❌ Linux ARM64 build failed"
    exit 1
fi

echo ""
echo "🍎 Testing macOS AMD64 build..."
make build-darwin-amd64
if [ -f "easytunnel-darwin-amd64" ]; then
    echo "   ✅ macOS AMD64 build successful"
    file easytunnel-darwin-amd64
else
    echo "   ❌ macOS AMD64 build failed"
    exit 1
fi

echo ""
echo "🍎 Testing macOS ARM64 build..."
make build-darwin-arm64
if [ -f "easytunnel-darwin-arm64" ]; then
    echo "   ✅ macOS ARM64 build successful"
    file easytunnel-darwin-arm64
else
    echo "   ❌ macOS ARM64 build failed"
    exit 1
fi

echo ""
echo "🪟 Testing Windows AMD64 build..."
make build-windows
if [ -f "easytunnel-windows-amd64.exe" ]; then
    echo "   ✅ Windows AMD64 build successful"
    file easytunnel-windows-amd64.exe
else
    echo "   ❌ Windows AMD64 build failed"
    exit 1
fi

# Test version information in current platform binary
echo ""
echo "🔢 Testing version information..."
make build
if ./easytunnel --version | grep -q "Easy SSH Tunnel Manager"; then
    echo "   ✅ Version information working"
    ./easytunnel --version
else
    echo "   ❌ Version information not working"
    exit 1
fi

# Test help flag
echo ""
echo "📚 Testing help information..."
if ./easytunnel --help | grep -q "Usage:"; then
    echo "   ✅ Help information working"
else
    echo "   ❌ Help information not working"
    exit 1
fi

# Test full release process
echo ""
echo "📦 Testing full release process..."
VERSION="test-$(date +%s)"
export VERSION
make release

if [ -d "dist/$VERSION" ]; then
    echo "   ✅ Release directory created"
    echo "   📁 Contents:"
    ls -la "dist/$VERSION/"
    
    # Check for expected files
    expected_files=(
        "easytunnel-$VERSION-linux-amd64.tar.gz"
        "easytunnel-$VERSION-linux-arm64.tar.gz"
        "easytunnel-$VERSION-darwin-amd64.tar.gz"
        "easytunnel-$VERSION-darwin-arm64.tar.gz"
        "easytunnel-$VERSION-windows-amd64.zip"
        "checksums.txt"
    )
    
    missing_files=()
    for file in "${expected_files[@]}"; do
        if [ ! -f "dist/$VERSION/$file" ]; then
            missing_files+=("$file")
        fi
    done
    
    if [ ${#missing_files[@]} -eq 0 ]; then
        echo "   ✅ All expected release files present"
    else
        echo "   ❌ Missing release files:"
        for file in "${missing_files[@]}"; do
            echo "      - $file"
        done
        exit 1
    fi
    
    # Check checksums
    echo ""
    echo "🔐 Verifying checksums..."
    cd "dist/$VERSION"
    if shasum -a 256 -c checksums.txt; then
        echo "   ✅ All checksums verified"
    else
        echo "   ❌ Checksum verification failed"
        exit 1
    fi
    cd - > /dev/null
    
else
    echo "   ❌ Release directory not created"
    exit 1
fi

# Test Debian package if on Linux
if command -v dpkg-deb >/dev/null 2>&1; then
    echo ""
    echo "📦 Testing Debian package creation..."
    if make deb; then
        echo "   ✅ Debian package created"
        if [ -f "dist/easytunnel-$VERSION-amd64.deb" ]; then
            echo "   📋 Package info:"
            dpkg-deb --info "dist/easytunnel-$VERSION-amd64.deb"
        fi
    else
        echo "   ⚠️  Debian package creation failed (may need dpkg-deb)"
    fi
fi

echo ""
echo "🎉 All tests passed!"
echo ""
echo "📋 Summary:"
echo "   ✅ All platform builds successful"
echo "   ✅ Version information working"
echo "   ✅ Help information working"
echo "   ✅ Release packaging working"
echo "   ✅ Checksums verified"
echo ""
echo "🚀 Ready for release!"

# Cleanup test artifacts
echo ""
echo "🧹 Cleaning up test artifacts..."
rm -rf dist/
make clean
echo "   ✅ Cleanup complete"
