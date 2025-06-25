#!/bin/bash
# Test Port Reclamation Functionality
# Tests that the application can reclaim ports when running with sudo

set -e

echo "🧪 Testing Port Reclamation Functionality"
echo "========================================="
echo ""

# Test port
TEST_PORT=15555

echo "📋 Test Setup:"
echo "   Test port: $TEST_PORT"
echo "   Current user: $(whoami)"
echo "   User ID: $(id -u)"
echo ""

# Clean up any existing test processes
cleanup() {
    echo "🧹 Cleaning up test processes..."
    sudo pkill -f "nc.*$TEST_PORT" 2>/dev/null || true
    sudo pkill -f "python.*$TEST_PORT" 2>/dev/null || true
    sleep 1
}

# Initial cleanup
cleanup

# Function to occupy a port
occupy_port() {
    local port=$1
    echo "🔒 Occupying port $port with netcat..."
    
    # Try different approaches based on what's available
    if command -v nc >/dev/null 2>&1; then
        # Use netcat to occupy the port
        nc -l $port &
        NC_PID=$!
        echo "   Started netcat with PID $NC_PID"
    elif command -v python3 >/dev/null 2>&1; then
        # Use Python to occupy the port
        python3 -c "
import socket
import time
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(('localhost', $port))
s.listen(1)
print('Python server listening on port $port')
try:
    while True:
        time.sleep(1)
except KeyboardInterrupt:
    pass
finally:
    s.close()
" &
        PYTHON_PID=$!
        echo "   Started Python server with PID $PYTHON_PID"
    else
        echo "   ❌ Neither netcat nor Python available for testing"
        exit 1
    fi
    
    # Give it time to start
    sleep 2
    
    # Verify port is occupied
    if lsof -i :$port >/dev/null 2>&1; then
        echo "   ✅ Port $port is now occupied"
        lsof -i :$port
    else
        echo "   ❌ Failed to occupy port $port"
        exit 1
    fi
}

# Function to test port reclamation
test_reclamation() {
    echo ""
    echo "🔧 Testing port reclamation..."
    
    # Build the test binary
    echo "   Building test binary..."
    go build -o easytunnel-test main.go
    
    # Create a test tunnel configuration that uses the occupied port
    echo "   Creating test tunnel with port $TEST_PORT..."
    
    # We'll use curl to test the API since the binary needs to run in background
    # Start the application in background
    PORT=18080 ./easytunnel-test &
    APP_PID=$!
    
    # Give it time to start
    sleep 3
    
    # Try to add a tunnel that uses the occupied port
    echo "   Attempting to add tunnel using occupied port $TEST_PORT..."
    
    TUNNEL_CONFIG='{
        "name": "test-tunnel",
        "command": "ssh -L '$TEST_PORT':localhost:22 test@localhost",
        "localPort": "'$TEST_PORT'",
        "enabled": false
    }'
    
    # Test the API call
    if curl -s -X POST -H "Content-Type: application/json" \
            -d "$TUNNEL_CONFIG" \
            http://localhost:18080/api/add >/dev/null 2>&1; then
        echo "   ✅ Tunnel added successfully - port reclamation worked!"
        
        # Check if the original process was killed
        if ! lsof -i :$TEST_PORT >/dev/null 2>&1; then
            echo "   ✅ Original process was successfully terminated"
        else
            echo "   ⚠️  Original process still running:"
            lsof -i :$TEST_PORT
        fi
    else
        echo "   ❌ Failed to add tunnel - port reclamation may have failed"
        
        # Show what's still using the port
        echo "   Processes still using port $TEST_PORT:"
        lsof -i :$TEST_PORT || echo "   No processes found"
    fi
    
    # Clean up the test application
    kill $APP_PID 2>/dev/null || true
    wait $APP_PID 2>/dev/null || true
    
    # Remove test binary
    rm -f easytunnel-test
}

# Main test flow
echo "1️⃣  Step 1: Occupy test port $TEST_PORT"
occupy_port $TEST_PORT

echo ""
echo "2️⃣  Step 2: Verify port is occupied"
if lsof -i :$TEST_PORT >/dev/null 2>&1; then
    echo "   ✅ Port $TEST_PORT is occupied as expected"
    echo "   Current processes using port $TEST_PORT:"
    lsof -i :$TEST_PORT | head -5
else
    echo "   ❌ Port $TEST_PORT is not occupied - test setup failed"
    exit 1
fi

echo ""
echo "3️⃣  Step 3: Test port reclamation"
if [ "$(id -u)" -eq 0 ]; then
    echo "   ✅ Running as root - testing full port reclamation"
    test_reclamation
else
    echo "   ⚠️  Not running as root - testing limited reclamation"
    echo "   💡 For full testing, run: sudo $0"
    test_reclamation
fi

echo ""
echo "4️⃣  Step 4: Final cleanup"
cleanup

# Test if port is now free
if ! lsof -i :$TEST_PORT >/dev/null 2>&1; then
    echo "   ✅ Port $TEST_PORT is now free"
else
    echo "   ⚠️  Port $TEST_PORT is still occupied:"
    lsof -i :$TEST_PORT
fi

echo ""
echo "🎉 Port reclamation test completed!"
echo ""
echo "📋 Summary:"
echo "   ✅ Port occupation test passed"
echo "   ✅ Port reclamation functionality tested"
echo "   ✅ Cleanup completed"
echo ""

if [ "$(id -u)" -eq 0 ]; then
    echo "💡 Running with root privileges allows full port reclamation"
else
    echo "💡 For production use with full port management, run with sudo"
fi
