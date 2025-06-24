# Easy SSH Tunnel Manager

A modern web-based SSH tunnel manager that allows you to manage multiple SSH tunnels with automatic reconnection, real-time monitoring, and a beautiful web interface.

![Easy SSH Tunnel Manager](https://img.shields.io/badge/Go-v1.21+-blue) ![License](https://img.shields.io/badge/License-MIT-green) ![Status](https://img.shields.io/badge/Status-Active-brightgreen)

## ✨ Features

- **🔄 Auto-Reconnection**: Tunnels automatically reconnect when network connectivity is restored
- **📊 Real-time Monitoring**: Live status updates via Server-Sent Events (SSE)
- **🌐 Network Detection**: Automatic network connectivity monitoring
- **🔔 Desktop Notifications**: Browser notifications for tunnel status changes
- **💻 Modern Web UI**: Clean, responsive interface built with Tailwind CSS
- **⚡ Lightweight**: Single binary with embedded web interface
- **🛡️ Secure**: Supports SSH key authentication and various SSH options
- **📝 Command Parsing**: Automatically detects local ports from SSH commands
- **🔧 Health Monitoring**: Regular health checks for all tunnels

## 📋 Requirements

- **Go 1.21 or higher** (for building from source)
- **SSH client** installed on your system
- **SSH key authentication** set up for your bastion hosts
- **Web browser** with JavaScript enabled

## 🚀 Quick Start

### Option 1: Run from Source

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd easytunnel
   ```

2. **Build and run:**
   ```bash
   go mod tidy
   go run .
   ```

3. **Or build binary:**
   ```bash
   go build -o tunnel-manager .
   ./tunnel-manager
   ```

4. **Access the web interface:**
   Open http://localhost:8080 in your browser

### Option 2: Custom Port

```bash
PORT=9999 go run .
# or
PORT=9999 ./tunnel-manager
```

### Option 3: Running with sudo (if needed)

```bash
sudo PORT=10001 go run .
```

## � Usage Guide

### Adding Tunnels

1. **Open the web interface** at http://localhost:8080
2. **Fill out the form:**
   - **Tunnel Name**: A descriptive name (e.g., "Production DB")
   - **SSH Command**: Your complete SSH command with port forwarding
   - **Local Port**: (Optional) Leave empty for auto-detection

3. **Example SSH commands:**
   ```bash
   ssh -L 5432:db.internal:5432 user@bastion.example.com
   ssh -L 8080:app.internal:8080 -i ~/.ssh/my-key user@jump.example.com
   ssh -L 3306:mysql.internal:3306 -p 2222 user@bastion.example.com
   ```

4. **Click "Add Tunnel"** and the tunnel will start automatically

### Managing Tunnels

- **Start/Stop**: Use the toggle button next to each tunnel
- **Delete**: Click the delete button (confirmation required)
- **Monitor Status**: Real-time status updates with color-coded indicators
- **View Errors**: Error messages displayed in red boxes when issues occur

### Tunnel Statuses

- 🟢 **Connected**: Tunnel is active and healthy
- 🟡 **Connecting**: Tunnel is attempting to connect
- 🔴 **Error**: Connection failed or tunnel encountered an issue
- ⚪ **Disconnected**: Tunnel is stopped

## 🔧 Configuration

### SSH Key Setup

Ensure your SSH keys are properly configured:

```bash
# Generate SSH key if you don't have one
ssh-keygen -t ed25519 -C "your-email@example.com"

# Copy public key to bastion host
ssh-copy-id user@bastion.example.com

# Test connection
ssh user@bastion.example.com
```

### SSH Config File

For easier management, add hosts to your `~/.ssh/config`:

```bash
Host bastion
    HostName bastion.example.com
    User your-username
    IdentityFile ~/.ssh/id_ed25519
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

Then use simplified commands:
```bash
ssh -L 5432:db.internal:5432 bastion
```

## 🛠️ Troubleshooting

### Common Issues

#### 1. **Rapid Reconnection Loop** ⚠️ CRITICAL
**Symptoms:** Logs show constant "connecting" messages every second
```
2025/06/24 14:36:27 Connecting tunnel 'localdb' without automatic key management
2025/06/24 14:36:28 Tunnel 'localdb' connected successfully on port 5433
2025/06/24 14:36:28 Connecting tunnel 'localdb' without automatic key management
```

**🚨 IMMEDIATE ACTIONS:**
1. **Stop the application immediately:**
   ```bash
   # Press Ctrl+C (might need to press multiple times)
   # If unresponsive, force kill from another terminal:
   pkill -f "go run"
   # or find the process and kill it:
   ps aux | grep easytunnel
   sudo kill -9 <PID>
   ```

2. **Clean up orphaned SSH processes:**
   ```bash
   # Find and kill any stuck SSH tunnels
   ps aux | grep "ssh.*-L"
   pkill -f "ssh.*-L"
   
   # Check for processes using your tunnel ports
   lsof -i :5433  # replace with your port
   sudo kill -9 <PID>  # if any process is found
   ```

3. **Wait before restarting:**
   ```bash
   # Wait at least 10 seconds
   sleep 10
   ```

4. **Restart cleanly:**
   ```bash
   go run .
   ```

**🔍 ROOT CAUSE:**
This happens when:
- Multiple tunnel instances with the same name are created
- Network detection triggers rapid reconnection attempts
- SSH process exits immediately but reconnection logic doesn't have proper delays

**�️ PREVENTION:**
- Always stop the application cleanly with Ctrl+C
- Don't run multiple instances of the tunnel manager
- Ensure SSH connections are stable before adding tunnels

#### 2. **SSH Exit Status 255** 🔍 **DEBUGGING**
**Symptoms:** Tunnel connects but immediately exits with error 255
```
2025/06/24 14:45:24 Tunnel 'localdb' connected successfully on port 5433
2025/06/24 14:45:24 Tunnel 'localdb' exited with error: exit status 255
```

**🔍 DIAGNOSIS STEPS:**

1. **Run the debug script:**
   ```bash
   ./debug-ssh.sh
   ```

2. **Test SSH connection manually:**
   ```bash
   ssh -v -i ~/.ssh/id_ed25519 avnish@35.200.246.135
   ```

3. **Check common causes:**
   - **Authentication issues**: SSH key not added or wrong permissions
   - **Network/Firewall**: Connection blocked by firewall
   - **SSH server config**: Server doesn't allow tunneling
   - **Key permissions**: SSH key file should be 600

**🚨 QUICK FIXES:**

1. **Fix SSH key permissions:**
   ```bash
   chmod 600 ~/.ssh/id_ed25519
   ```

2. **Add key to SSH agent:**
   ```bash
   ssh-add ~/.ssh/id_ed25519
   ```

3. **Test basic connectivity:**
   ```bash
   ping 35.200.246.135
   telnet 35.200.246.135 22
   ```

4. **Check if port forwarding is allowed:**
   ```bash
   ssh -v -i ~/.ssh/id_ed25519 avnish@35.200.246.135 "echo 'Connection test'"
   ```

**💡 Common Exit Status 255 Causes:**
- `Permission denied (publickey)` - SSH key not properly set up
- `Connection refused` - Network/firewall blocking connection  
- `Host key verification failed` - SSH host key issues
- `AllowTcpForwarding no` - Server doesn't allow port forwarding

#### 3. **Port Already in Use**
**Error:** `bind: address already in use`

**Solutions:**
- Check what's using the port: `lsof -i :5432`
- Kill the process: `sudo kill -9 <PID>`
- Use a different local port in your SSH command

#### 4. **SSH Connection Failed**
**Error:** Various SSH authentication or connection errors

**Solutions:**
- Test SSH connection manually: `ssh user@bastion.example.com`
- Check SSH agent: `ssh-add -l`
- Add key to agent: `ssh-add ~/.ssh/id_ed25519`
- Verify network connectivity: `ping bastion.example.com`

#### 5. **Permission Denied**
**Error:** SSH authentication failures

**Solutions:**
- Ensure SSH key is added to the bastion host
- Check file permissions: `chmod 600 ~/.ssh/id_ed25519`
- Verify SSH config syntax
- Test with verbose SSH: `ssh -v user@bastion.example.com`

#### 6. **Web Interface Not Loading**
**Solutions:**
- Check if port is already in use: `lsof -i :8080`
- Try a different port: `PORT=9999 go run .`
- Check firewall settings
- Verify Go application is running: `ps aux | grep tunnel`

### Emergency Recovery

If the system becomes unresponsive due to rapid reconnection:

```bash
# 1. Force kill all related processes
sudo pkill -f "go run"
sudo pkill -f "easytunnel"
sudo pkill -f "ssh.*-L"

# 2. Clear any stuck ports (replace with your ports)
sudo lsof -ti:5432,5433,8080 | xargs sudo kill -9

# 3. Wait for cleanup
sleep 15

# 4. Restart with clean state
go run .
```

### Debug Mode

Enable verbose logging by modifying the SSH command to include debugging:

```bash
ssh -v -L 5432:db.internal:5432 user@bastion.example.com
```

## 🔒 Security Considerations

- **SSH Keys**: Use strong SSH key authentication instead of passwords
- **Network Access**: Restrict access to the web interface (default: localhost only)
- **Firewall**: Configure firewalls appropriately for your tunneled services
- **Monitoring**: Regularly monitor logs for suspicious activity

## 📁 File Locations

- **Configuration**: `~/.tunnel-manager/tunnels.json`
- **Logs**: Console output (stdout/stderr)
- **SSH Keys**: `~/.ssh/` directory

## 🔄 API Endpoints

The application provides REST API endpoints:

- `GET /`: Web interface
- `GET /api/status`: Get tunnel statuses
- `POST /api/add`: Add new tunnel
- `POST /api/toggle/{name}`: Start/stop tunnel
- `DELETE /api/delete/{name}`: Delete tunnel
- `GET /api/events`: Server-Sent Events stream

## 🏗️ Architecture

- **Backend**: Go with embedded HTTP server
- **Frontend**: HTML/JavaScript with Tailwind CSS
- **Real-time Updates**: Server-Sent Events (SSE)
- **Network Monitoring**: Built-in connectivity detection
- **Process Management**: Context-based goroutine management

## 🚀 Performance Tips

- **Limit Concurrent Tunnels**: Avoid running too many tunnels simultaneously
- **Monitor Resources**: Keep an eye on CPU and memory usage
- **Network Stability**: Ensure stable network connections to prevent reconnection loops
- **SSH Multiplexing**: Consider using SSH connection multiplexing for multiple tunnels to the same host

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🐛 Reporting Issues

When reporting issues, please include:

- Operating system and version
- Go version (`go version`)
- SSH client version (`ssh -V`)
- Complete error messages
- Steps to reproduce
- Log output

## 📞 Support

- Create an issue for bug reports
- Check existing issues before reporting
- Provide detailed information for faster resolution

---

**Built with ❤️ using Go and modern web technologies**
```

### Option 3: Download Binary

Download the latest binary from the [releases page](https://github.com/ivikasavnish/easytunnel/releases).

## 🚀 Quick Start

1. **Start the application:**
   ```bash
   ./easytunnel
   ```

2. **Open your browser:**
   Navigate to [http://localhost:10000](http://localhost:10000)

3. **Add your first tunnel:**
   - Enter a descriptive name (e.g., "Production Database")
   - Paste your SSH command (e.g., `ssh -L 5432:db.internal:5432 user@bastion.example.com`)
   - Click "Add Tunnel"

4. **Manage your tunnels:**
   - Start/Stop tunnels individually
   - Monitor connection status in real-time
   - View uptime and health information

## 📖 Usage Examples

### Basic Database Tunnel
```bash
# SSH Command to add:
ssh -L 5432:db.internal:5432 user@bastion.example.com

# After tunnel is active, connect to your database:
psql -h localhost -p 5432 -U username database_name
```

### Multiple Service Tunnels
```bash
# Database tunnel
ssh -L 5432:db.internal:5432 user@bastion.example.com

# Redis tunnel  
ssh -L 6379:redis.internal:6379 user@bastion.example.com

# Web service tunnel
ssh -L 8080:api.internal:8080 user@bastion.example.com
```

### Custom Local Ports
If you need to specify a different local port than what's in your SSH command, you can override it in the "Local Port" field when adding a tunnel.

## 🔧 Configuration

### Environment Variables

- `PORT` - Web server port (default: 10000)
  ```bash
  PORT=8080 ./easytunnel
  ```

### Configuration File

Tunnel configurations are automatically saved to `~/.easytunnel/tunnels.json` and persist between application restarts.

### SSH Key Authentication

For seamless operation, set up SSH key authentication:

1. **Generate SSH key (if not already done):**
   ```bash
   ssh-keygen -t rsa -b 4096 -C "your_email@example.com"
   ```

2. **Copy key to your servers:**
   ```bash
   ssh-copy-id user@bastion.example.com
   ```

3. **Test passwordless connection:**
   ```bash
   ssh user@bastion.example.com
   ```

## 📡 API Reference

The application provides a REST API for programmatic access:

### Get Tunnel Status
```bash
curl http://localhost:10000/api/status
```

### Add New Tunnel
```bash
curl -X POST http://localhost:10000/api/add \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Tunnel",
    "command": "ssh -L 5432:db.internal:5432 user@bastion.example.com",
    "enabled": true
  }'
```

### Toggle Tunnel
```bash
curl -X POST http://localhost:10000/api/toggle/My%20Tunnel
```

### Delete Tunnel
```bash
curl -X DELETE http://localhost:10000/api/delete/My%20Tunnel
```

### Health Check
```bash
curl http://localhost:10000/health
```

### Real-time Events
Connect to Server-Sent Events for real-time updates:
```javascript
const eventSource = new EventSource('http://localhost:10000/api/events');
eventSource.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Tunnel update:', data);
};
```

## 🔒 Security Considerations

- **SSH Keys**: Use SSH key authentication instead of passwords
- **Network Access**: Run on localhost (default) for security
- **Firewall**: Consider firewall rules if exposing to network
- **SSH Config**: Use SSH config files for complex connection settings

### Example SSH Config (~/.ssh/config)
```
Host bastion
    HostName bastion.example.com
    User myuser
    IdentityFile ~/.ssh/id_rsa
    ServerAliveInterval 60
    ServerAliveCountMax 3

Host db-tunnel
    HostName bastion.example.com
    User myuser
    LocalForward 5432 db.internal:5432
    IdentityFile ~/.ssh/id_rsa
```

## 🐛 Troubleshooting

### Common Issues

1. **"Permission denied" errors:**
   - Ensure SSH keys are properly configured
   - Check SSH agent is running: `ssh-add -l`
   - Verify SSH connection works manually

2. **Port already in use:**
   - Check if another service is using the port: `lsof -i :5432`
   - Use different local ports for each tunnel
   - Kill conflicting processes if safe to do so

3. **Tunnel keeps disconnecting:**
   - Check network connectivity
   - Verify SSH server allows long-running connections
   - Consider SSH keep-alive settings

4. **Can't access web interface:**
   - Ensure port 10000 is not blocked
   - Try a different port: `PORT=8080 ./easytunnel`
   - Check if another service is using the port

### Debug Mode

Enable verbose SSH logging by modifying your SSH commands:
```bash
ssh -v -L 5432:db.internal:5432 user@bastion.example.com
```

### Logs

Application logs are printed to stdout. To save logs:
```bash
./easytunnel 2>&1 | tee easytunnel.log
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with Go and modern web technologies
- Uses Tailwind CSS for styling
- Inspired by the need for better SSH tunnel management tools

## 📞 Support

If you encounter any issues or have questions:

1. Check the [troubleshooting section](#-troubleshooting)
2. Search existing [GitHub issues](https://github.com/ivikasavnish/easytunnel/issues)
3. Create a new issue if needed

## 🚀 Roadmap

- [ ] Docker container support
- [ ] Tunnel templates and presets
- [ ] SSH config file integration
- [ ] Tunnel usage statistics
- [ ] Dark mode theme
- [ ] Mobile-responsive design improvements
- [ ] Tunnel grouping and organization
- [ ] Export/import configurations
