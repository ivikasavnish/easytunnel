# Easy SSH Tunnel Manager

A modern, web-based SSH tunnel manager that allows you to easily create, manage, and monitor multiple SSH tunnels simultaneously. Perfect for developers who need to access multiple remote services through bastion hosts or jump servers.

![Easy SSH Tunnel Manager](https://img.shields.io/badge/Go-1.23+-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

## 🚀 Features

- **Multi-Tunnel Management**: Run multiple SSH tunnels simultaneously
- **Web Interface**: Clean, modern web UI for easy management
- **Auto-Reconnection**: Automatic reconnection when tunnels fail
- **Real-time Updates**: Live status updates via Server-Sent Events (SSE)
- **Network Monitoring**: Automatic detection of network connectivity issues
- **Desktop Notifications**: System notifications for tunnel status changes
- **Port Auto-Detection**: Automatically extracts local ports from SSH commands
- **Health Monitoring**: Regular health checks for active tunnels
- **Persistent Configuration**: Tunnel configurations saved between sessions

## 📋 Prerequisites

- **Go 1.23+** - Required to build and run the application
- **SSH Client** - The `ssh` command must be available in your system PATH
- **Network Access** - Ability to connect to your SSH servers/bastion hosts
- **Modern Web Browser** - Chrome, Firefox, Safari, or Edge for the web interface

## 🛠️ Installation

### Option 1: Build from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ivikasavnish/easytunnel.git
   cd easytunnel
   ```

2. **Build the application:**
   ```bash
   go build -o easytunnel main.go
   ```

3. **Run the application:**
   ```bash
   ./easytunnel
   ```

### Option 2: Direct Go Install

```bash
go install github.com/ivikasavnish/easytunnel@latest
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
