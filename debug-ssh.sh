#!/bin/bash

# SSH Tunnel Debugging Script for Easy Tunnel Manager
# This script helps debug SSH connection issues

echo "🔍 SSH Tunnel Debugging Script"
echo "=============================="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Your tunnel configuration
SSH_HOST="avnish@35.200.246.135"
SSH_KEY="/Users/vikasavnish/.ssh/id_ed25519"
LOCAL_PORT="5433"
REMOTE_HOST="10.88.145.3"
REMOTE_PORT="5432"

echo -e "${YELLOW}Testing SSH configuration for tunnel:${NC}"
echo "  Host: $SSH_HOST"
echo "  Key: $SSH_KEY"
echo "  Tunnel: localhost:$LOCAL_PORT -> $REMOTE_HOST:$REMOTE_PORT"
echo ""

# Test 1: Check SSH key file
echo "1. Checking SSH key file..."
if [ -f "$SSH_KEY" ]; then
    echo -e "${GREEN}✓ SSH key file exists${NC}"
    
    # Check permissions
    PERMS=$(stat -f "%A" "$SSH_KEY" 2>/dev/null || stat -c "%a" "$SSH_KEY" 2>/dev/null)
    if [ "$PERMS" = "600" ] || [ "$PERMS" = "400" ]; then
        echo -e "${GREEN}✓ SSH key permissions are secure ($PERMS)${NC}"
    else
        echo -e "${RED}⚠ SSH key permissions should be 600 (currently $PERMS)${NC}"
        echo "  Fix with: chmod 600 $SSH_KEY"
    fi
else
    echo -e "${RED}✗ SSH key file not found: $SSH_KEY${NC}"
    exit 1
fi

# Test 2: Check if key is in SSH agent
echo ""
echo "2. Checking SSH agent..."
if command -v ssh-add >/dev/null 2>&1; then
    if ssh-add -l >/dev/null 2>&1; then
        if ssh-add -l | grep -q "$(basename "$SSH_KEY" .pub)"; then
            echo -e "${GREEN}✓ SSH key is loaded in agent${NC}"
        else
            echo -e "${YELLOW}⚠ SSH key not in agent, adding it...${NC}"
            ssh-add "$SSH_KEY"
        fi
    else
        echo -e "${YELLOW}⚠ SSH agent not running or no keys loaded${NC}"
        echo "  Start agent: eval \$(ssh-agent)"
        echo "  Add key: ssh-add $SSH_KEY"
    fi
else
    echo -e "${YELLOW}⚠ ssh-add not available${NC}"
fi

# Test 3: Test basic SSH connectivity
echo ""
echo "3. Testing basic SSH connectivity..."
ssh -o ConnectTimeout=10 \
    -o BatchMode=yes \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -i "$SSH_KEY" \
    "$SSH_HOST" \
    "echo 'SSH connection successful'" 2>/dev/null

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Basic SSH connection successful${NC}"
else
    echo -e "${RED}✗ Basic SSH connection failed${NC}"
    echo ""
    echo "Trying with verbose output..."
    ssh -v -o ConnectTimeout=10 \
        -o BatchMode=yes \
        -o StrictHostKeyChecking=no \
        -o UserKnownHostsFile=/dev/null \
        -i "$SSH_KEY" \
        "$SSH_HOST" \
        "echo 'SSH connection test'" 2>&1 | head -20
fi

# Test 4: Check if local port is available
echo ""
echo "4. Checking local port availability..."
if lsof -i :$LOCAL_PORT >/dev/null 2>&1; then
    echo -e "${RED}⚠ Port $LOCAL_PORT is already in use:${NC}"
    lsof -i :$LOCAL_PORT
    echo "  Kill with: sudo kill -9 <PID>"
else
    echo -e "${GREEN}✓ Port $LOCAL_PORT is available${NC}"
fi

# Test 5: Test SSH tunnel manually
echo ""
echo "5. Testing SSH tunnel manually (will run for 10 seconds)..."
echo "Command: ssh -N -T -o ConnectTimeout=10 -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o ExitOnForwardFailure=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $SSH_KEY -L $LOCAL_PORT:$REMOTE_HOST:$REMOTE_PORT $SSH_HOST"

# Run tunnel in background for testing
timeout 10s ssh -N -T \
    -o ConnectTimeout=10 \
    -o ServerAliveInterval=30 \
    -o ServerAliveCountMax=3 \
    -o ExitOnForwardFailure=yes \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -i "$SSH_KEY" \
    -L "$LOCAL_PORT:$REMOTE_HOST:$REMOTE_PORT" \
    "$SSH_HOST" &

TUNNEL_PID=$!
sleep 3

# Test if tunnel is working
if nc -z localhost $LOCAL_PORT 2>/dev/null; then
    echo -e "${GREEN}✓ SSH tunnel is working!${NC}"
    echo "  Test connection: nc localhost $LOCAL_PORT"
else
    echo -e "${RED}✗ SSH tunnel failed to establish${NC}"
fi

# Clean up
kill $TUNNEL_PID 2>/dev/null
wait $TUNNEL_PID 2>/dev/null

# Test 6: Network connectivity
echo ""
echo "6. Testing network connectivity..."
if ping -c 1 8.8.8.8 >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Internet connectivity working${NC}"
else
    echo -e "${RED}✗ No internet connectivity${NC}"
fi

# Test 7: DNS resolution
echo ""
echo "7. Testing DNS resolution..."
if nslookup $(echo $SSH_HOST | cut -d'@' -f2) >/dev/null 2>&1; then
    echo -e "${GREEN}✓ DNS resolution working for SSH host${NC}"
else
    echo -e "${RED}✗ DNS resolution failed for SSH host${NC}"
fi

echo ""
echo "=============================="
echo "🔍 Debugging complete!"
echo ""
echo "If issues persist:"
echo "1. Check firewall settings"
echo "2. Verify SSH server configuration" 
echo "3. Check network connectivity to remote host"
echo "4. Try connecting from a different network"
echo ""
echo "For more help, run SSH with verbose output:"
echo "ssh -vv -i $SSH_KEY $SSH_HOST"
