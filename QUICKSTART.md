# 🚀 Quick Start Guide

# 🚀 Quick Start Guide

## ✅ **Rapid Reconnection Issue - FIXED!**

**Good news:** The rapid reconnection loop issue has been fixed in the latest version. You should now see proper delays between connection attempts.

### 🔍 **Current Issue: SSH Exit Status 255**

You're now seeing this pattern (which is much better):
```
2025/06/24 14:45:24 Tunnel 'localdb' connected successfully on port 5433
2025/06/24 14:45:24 Tunnel 'localdb' exited with error: exit status 255
2025/06/24 14:45:29 Connecting tunnel 'localdb' without automatic key management  ← 5 second delay ✓
```

**What this means:**
- ✅ No more rapid reconnections (FIXED!)
- ⚠️ SSH connection is failing with exit status 255
- ✅ Proper delays between retry attempts

### 🛠️ **Fix SSH Exit Status 255**

1. **Run the debug script:**
   ```bash
   ./debug-ssh.sh
   ```

2. **Quick fixes to try:**
   ```bash
   # Fix SSH key permissions
   chmod 600 ~/.ssh/id_ed25519
   
   # Add key to SSH agent  
   ssh-add ~/.ssh/id_ed25519
   
   # Test SSH connection manually
   ssh -v -i ~/.ssh/id_ed25519 avnish@35.200.246.135
   ```

3. **If still failing, check:**
   - Network connectivity: `ping 35.200.246.135`
   - SSH port: `telnet 35.200.246.135 22` 
   - Firewall settings
   - SSH server allows port forwarding

## 📖 **Basic Usage**

### Step 1: Start the Application
```bash
cd easytunnel
go run .
```

### Step 2: Open Web Interface
- Open http://localhost:8080 in your browser
- If port 8080 is busy: `PORT=9999 go run .`

### Step 3: Add Your First Tunnel
1. Fill in the form:
   - **Name**: `My Database`
   - **SSH Command**: `ssh -L 5432:db.internal:5432 user@bastion.example.com`
   - **Local Port**: (leave empty - it will auto-detect 5432)

2. Click "Add Tunnel"

3. The tunnel will start automatically and show green status when connected

### Step 4: Use Your Tunnel
- Connect to `localhost:5432` to access `db.internal:5432`
- Monitor status in real-time on the web interface

## 🛡️ **SSH Key Setup (If Needed)**

```bash
# Generate SSH key
ssh-keygen -t ed25519 -C "your-email@example.com"

# Copy to bastion host
ssh-copy-id user@bastion.example.com

# Test connection
ssh user@bastion.example.com
```

## ⚡ **Quick Commands**

```bash
# Build and run
go mod tidy && go run .

# Run on different port
PORT=9999 go run .

# Emergency cleanup (if stuck)
pkill -f "go run" && pkill -f "ssh.*-L" && sleep 5

# Check what's using a port
lsof -i :8080
```

## 🔍 **Common Issues**

| Problem | Solution |
|---------|----------|
| Port already in use | `lsof -i :8080` then `sudo kill -9 <PID>` |
| SSH auth failed | Test: `ssh user@bastion.example.com` |
| Rapid reconnections | Stop app, clean processes, wait, restart |
| Can't access web UI | Try different port: `PORT=9999 go run .` |

## 📱 **Web Interface Features**

- **Real-time status** with color indicators
- **Desktop notifications** for tunnel events
- **Automatic reconnection** when network restored
- **Health monitoring** every 30 seconds
- **Start/stop tunnels** individually
- **Delete tunnels** with confirmation

## 🆘 **Get Help**

If you're still having issues:

1. **Check the README.md** for detailed troubleshooting
2. **Run system check**: Look for `make doctor` or similar in Makefile
3. **Check logs** in the terminal where you ran `go run .`
4. **Test SSH manually**: `ssh -v user@bastion.example.com`

**The most important thing**: If you see rapid reconnections, **STOP IMMEDIATELY** and follow the cleanup steps above.
