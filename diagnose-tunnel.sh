#!/bin/bash

# SSH Tunnel Diagnostics Script
# Helps diagnose SSH tunnel connection issues, especially exit status 255

set -e

echo "🔍 SSH Tunnel Diagnostics"
echo "=========================="
echo ""

# Check if command is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 'ssh -i ~/.ssh/key -L port:host:port user@server'"
    echo "Example: $0 'ssh -i ~/.ssh/id_ed25519 -L 5433:10.88.145.3:5432 avnish@35.200.246.135'"
    exit 1
fi

SSH_COMMAND="$1"
echo "🚇 Testing SSH command: $SSH_COMMAND"
echo ""

# Extract components from SSH command
USER_HOST=$(echo "$SSH_COMMAND" | grep -o '[a-zA-Z0-9_-]*@[a-zA-Z0-9.-]*' | tail -1)
HOST=$(echo "$USER_HOST" | cut -d'@' -f2)
USER=$(echo "$USER_HOST" | cut -d'@' -f1)
KEY_FILE=$(echo "$SSH_COMMAND" | grep -o '\-i [^ ]*' | cut -d' ' -f2 || echo "")

echo "📝 Connection details:"
echo "   Host: $HOST"
echo "   User: $USER"
echo "   Key:  $KEY_FILE"
echo ""

# Test 1: Network connectivity
echo "1️⃣  Testing network connectivity to $HOST..."
if ping -c 3 -W 3000 "$HOST" > /dev/null 2>&1; then
    echo "   ✅ Host $HOST is reachable"
else
    echo "   ❌ Host $HOST is not reachable via ping"
    echo "   💡 This could indicate network issues or ping being blocked"
fi
echo ""

# Test 2: SSH port connectivity
echo "2️⃣  Testing SSH port (22) connectivity..."
if timeout 5 bash -c "</dev/tcp/$HOST/22" 2>/dev/null; then
    echo "   ✅ SSH port 22 is open on $HOST"
else
    echo "   ❌ SSH port 22 is not accessible on $HOST"
    echo "   💡 SSH service may not be running or firewall is blocking"
fi
echo ""

# Test 3: SSH key file
if [ -n "$KEY_FILE" ]; then
    echo "3️⃣  Testing SSH key file..."
    KEY_FILE_EXPANDED="${KEY_FILE/#\~/$HOME}"
    if [ -f "$KEY_FILE_EXPANDED" ]; then
        echo "   ✅ SSH key file exists: $KEY_FILE_EXPANDED"
        
        # Check key permissions
        KEY_PERMS=$(stat -f "%A" "$KEY_FILE_EXPANDED" 2>/dev/null || stat -c "%a" "$KEY_FILE_EXPANDED" 2>/dev/null || echo "unknown")
        if [ "$KEY_PERMS" = "600" ] || [ "$KEY_PERMS" = "400" ]; then
            echo "   ✅ SSH key permissions are correct ($KEY_PERMS)"
        else
            echo "   ⚠️  SSH key permissions are $KEY_PERMS (should be 600 or 400)"
            echo "   💡 Fix with: chmod 600 '$KEY_FILE_EXPANDED'"
        fi
    else
        echo "   ❌ SSH key file not found: $KEY_FILE_EXPANDED"
        echo "   💡 Make sure the key file exists and path is correct"
    fi
else
    echo "3️⃣  No SSH key specified - will use default key or ssh-agent"
fi
echo ""

# Test 4: SSH agent
echo "4️⃣  Testing SSH agent..."
if [ -n "$SSH_AUTH_SOCK" ] && ssh-add -l > /dev/null 2>&1; then
    echo "   ✅ SSH agent is running and has keys loaded"
    echo "   📋 Loaded keys:"
    ssh-add -l | sed 's/^/      /'
else
    echo "   ⚠️  SSH agent is not running or has no keys"
    echo "   💡 Start SSH agent with: eval \$(ssh-agent)"
    if [ -n "$KEY_FILE" ]; then
        echo "   💡 Add key with: ssh-add '$KEY_FILE_EXPANDED'"
    fi
fi
echo ""

# Test 5: Basic SSH connection
echo "5️⃣  Testing basic SSH connection..."
SSH_TEST_CMD="ssh -o ConnectTimeout=10 -o BatchMode=yes -o LogLevel=ERROR"
if [ -n "$KEY_FILE" ]; then
    SSH_TEST_CMD="$SSH_TEST_CMD -i $KEY_FILE"
fi
SSH_TEST_CMD="$SSH_TEST_CMD $USER_HOST echo 'SSH_CONNECTION_SUCCESS'"

echo "   Running: $SSH_TEST_CMD"
if SSH_OUTPUT=$(eval "$SSH_TEST_CMD" 2>&1); then
    if echo "$SSH_OUTPUT" | grep -q "SSH_CONNECTION_SUCCESS"; then
        echo "   ✅ Basic SSH connection successful"
    else
        echo "   ⚠️  SSH connection succeeded but got unexpected output:"
        echo "$SSH_OUTPUT" | sed 's/^/      /'
    fi
else
    echo "   ❌ Basic SSH connection failed:"
    echo "$SSH_OUTPUT" | sed 's/^/      /'
    
    # Analyze common error patterns
    if echo "$SSH_OUTPUT" | grep -q "Permission denied"; then
        echo "   💡 Authentication failed - check SSH key or password"
    elif echo "$SSH_OUTPUT" | grep -q "Connection refused"; then
        echo "   💡 SSH service not running or port blocked"
    elif echo "$SSH_OUTPUT" | grep -q "No route to host"; then
        echo "   💡 Network routing issue"
    elif echo "$SSH_OUTPUT" | grep -q "Connection timed out"; then
        echo "   💡 Network timeout - host may be unreachable"
    fi
fi
echo ""

# Test 6: SSH tunnel test (if basic connection works)
echo "6️⃣  Testing SSH tunnel capability..."
LOCAL_PORT=$(echo "$SSH_COMMAND" | grep -o '\-L [0-9]*:' | cut -d' ' -f2 | cut -d':' -f1)
if [ -n "$LOCAL_PORT" ]; then
    echo "   Local port: $LOCAL_PORT"
    
    # Check if port is already in use
    if lsof -i ":$LOCAL_PORT" > /dev/null 2>&1; then
        echo "   ⚠️  Port $LOCAL_PORT is already in use:"
        lsof -i ":$LOCAL_PORT" | sed 's/^/      /'
        echo "   💡 Stop the process using this port or choose a different port"
    else
        echo "   ✅ Port $LOCAL_PORT is available"
    fi
    
    # Test tunnel with short duration
    echo "   Testing tunnel for 5 seconds..."
    TUNNEL_CMD=$(echo "$SSH_COMMAND" | sed 's/ssh/timeout 5 ssh -o ConnectTimeout=10/')
    if eval "$TUNNEL_CMD" > /dev/null 2>&1; then
        echo "   ✅ SSH tunnel test completed (no immediate errors)"
    else
        EXIT_CODE=$?
        echo "   ❌ SSH tunnel test failed with exit code $EXIT_CODE"
        if [ $EXIT_CODE -eq 255 ]; then
            echo "   💡 Exit code 255 typically indicates SSH authentication or connection issues"
        fi
    fi
else
    echo "   ⚠️  Could not extract local port from command"
fi
echo ""

# Test 7: DNS resolution
echo "7️⃣  Testing DNS resolution..."
if nslookup "$HOST" > /dev/null 2>&1; then
    echo "   ✅ DNS resolution for $HOST works"
    IP=$(nslookup "$HOST" | awk '/^Address: / { print $2 }' | tail -1)
    if [ -n "$IP" ]; then
        echo "   📍 Resolved to: $IP"
    fi
else
    echo "   ❌ DNS resolution for $HOST failed"
    echo "   💡 Check DNS settings or use IP address instead"
fi
echo ""

echo "🎯 Recommendations:"
echo "==================="

# Generate recommendations based on findings
if ! ping -c 1 -W 3000 "$HOST" > /dev/null 2>&1; then
    echo "• Check network connectivity to $HOST"
fi

if ! timeout 5 bash -c "</dev/tcp/$HOST/22" 2>/dev/null; then
    echo "• Verify SSH service is running on $HOST"
    echo "• Check firewall settings on $HOST"
fi

if [ -n "$KEY_FILE" ] && [ ! -f "${KEY_FILE/#\~/$HOME}" ]; then
    echo "• Create or fix the SSH key file path: $KEY_FILE"
fi

if [ -z "$SSH_AUTH_SOCK" ] || ! ssh-add -l > /dev/null 2>&1; then
    echo "• Start SSH agent: eval \$(ssh-agent)"
    if [ -n "$KEY_FILE" ]; then
        echo "• Add SSH key: ssh-add '$KEY_FILE'"
    fi
fi

echo "• Try running SSH with verbose output: ssh -vvv ..."
echo "• Check SSH server logs on $HOST for authentication errors"
echo "• Verify the remote port forwarding destination is reachable from $HOST"

echo ""
echo "🔧 For more detailed debugging, run:"
echo "   ssh -vvv [your-ssh-options] $USER_HOST"
echo ""
echo "✅ Diagnostics complete!"
