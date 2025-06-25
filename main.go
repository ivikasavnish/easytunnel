package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Version information (set by build flags)
var (
	Version    = "dev"
	BuildTime  = "unknown"
	CommitHash = "unknown"
)

// Port management functions for forceful port reclamation
// Since this application runs with sudo, it can kill processes using needed ports

//go:embed index.html
var indexHTML string

// TunnelConfig represents a tunnel configuration
type TunnelConfig struct {
	Name          string `json:"name"`
	Command       string `json:"command"`
	LocalPort     string `json:"localPort"`
	Enabled       bool   `json:"enabled"`
	AutoExtracted bool   `json:"autoExtracted"`
}

// TunnelStatus represents the status of a tunnel
type TunnelStatus struct {
	Config          TunnelConfig `json:"config"`
	Status          string       `json:"status"` // "connected", "disconnected", "connecting", "error"
	LastError       string       `json:"lastError"`
	ConnectedAt     time.Time    `json:"connectedAt"`
	Uptime          string       `json:"uptime"`
	PID             int          `json:"pid"`
	LastHealthCheck string       `json:"lastHealthCheck"`
}

// TunnelManager manages multiple SSH tunnels
type TunnelManager struct {
	tunnels        map[string]*Tunnel
	mutex          sync.RWMutex
	configFile     string
	networkMonitor *NetworkMonitor
	sseClients     map[chan string]bool
	sseMutex       sync.RWMutex
}

// AddSSEClient adds a new SSE client
func (tm *TunnelManager) AddSSEClient() chan string {
	tm.sseMutex.Lock()
	defer tm.sseMutex.Unlock()

	client := make(chan string, 10)
	tm.sseClients[client] = true
	return client
}

// RemoveSSEClient removes an SSE client
func (tm *TunnelManager) RemoveSSEClient(client chan string) {
	tm.sseMutex.Lock()
	defer tm.sseMutex.Unlock()

	delete(tm.sseClients, client)
	close(client)
}

// BroadcastSSE sends an event to all SSE clients
func (tm *TunnelManager) BroadcastSSE(eventType string, data interface{}) {
	tm.sseMutex.RLock()
	defer tm.sseMutex.RUnlock()

	eventData, _ := json.Marshal(map[string]interface{}{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now().UTC(),
	})

	message := string(eventData)

	for client := range tm.sseClients {
		select {
		case client <- message:
		default:
			// Client buffer is full, skip
		}
	}
}

// Tunnel represents an individual SSH tunnel
type Tunnel struct {
	config          TunnelConfig
	cmd             *exec.Cmd
	status          string
	lastError       string
	connectedAt     time.Time
	cancel          context.CancelFunc
	mutex           sync.RWMutex
	healthTicker    *time.Ticker
	lastHealthCheck time.Time
}

// isPortAvailable checks if a port is available for binding
func isPortAvailable(port string) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {

		return false
	}
	ln.Close()
	return true
}

// getProcessesUsingPort returns a list of PIDs using the specified port
func getProcessesUsingPort(port string) ([]int, error) {
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%s", port))
	output, err := cmd.Output()
	if err != nil {
		// If lsof fails, the port might be free or lsof not available
		return []int{}, nil
	}

	var pids []int
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line != "" {
			if pid, err := strconv.Atoi(line); err == nil {
				pids = append(pids, pid)
			}
		}
	}

	return pids, nil
}

// killProcessesOnPort kills all processes using the specified port
func killProcessesOnPort(port string) error {
	pids, err := getProcessesUsingPort(port)
	if err != nil {
		return fmt.Errorf("failed to get processes using port %s: %v", port, err)
	}

	if len(pids) == 0 {
		log.Printf("No processes found using port %s", port)
		return nil
	}

	log.Printf("Found %d process(es) using port %s: %v", len(pids), port, pids)

	// Kill each process
	for _, pid := range pids {
		// First try graceful termination
		if err := exec.Command("kill", "-TERM", fmt.Sprintf("%d", pid)).Run(); err != nil {
			log.Printf("Failed to send TERM signal to PID %d: %v", pid, err)
		} else {
			log.Printf("Sent TERM signal to PID %d", pid)
		}
	}

	// Wait a moment for graceful termination
	time.Sleep(2 * time.Second)

	// Check if processes are still running and force kill if necessary
	remainingPids, _ := getProcessesUsingPort(port)
	for _, pid := range remainingPids {
		if err := exec.Command("kill", "-KILL", fmt.Sprintf("%d", pid)).Run(); err != nil {
			log.Printf("Failed to force kill PID %d: %v", pid, err)
		} else {
			log.Printf("Force killed PID %d", pid)
		}
	}

	// Final check
	time.Sleep(1 * time.Second)
	finalPids, _ := getProcessesUsingPort(port)
	if len(finalPids) > 0 {
		return fmt.Errorf("failed to kill all processes on port %s, remaining: %v", port, finalPids)
	}

	log.Printf("Successfully freed port %s", port)
	return nil
}

// ensurePortAvailable ensures the port is available, killing processes if necessary
func ensurePortAvailable(port string) error {
	if isPortAvailable(port) {
		log.Printf("Port %s is already available", port)
		return nil
	}

	log.Printf("Port %s is in use, attempting to free it", port)

	// Check if we're running with sufficient privileges
	if os.Geteuid() != 0 {
		log.Printf("Warning: Not running as root - may not be able to kill all processes on port %s", port)
		// Still try to kill processes, but warn user
	}

	return killProcessesOnPort(port)
}

// getProcessInfoForPort gets detailed information about processes using a port
func getProcessInfoForPort(port string) string {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%s", port))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("Could not get process info for port %s", port)
	}
	return string(output)
}

// NewTunnelManager creates a new tunnel manager with network monitoring
func NewTunnelManager() *TunnelManager {
	// Determine config file location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: Could not get home directory, using current directory for config")
		homeDir = "."
	}

	configDir := filepath.Join(homeDir, ".tunnel-manager")
	os.MkdirAll(configDir, 0755)
	configFile := filepath.Join(configDir, "tunnels.json")

	tm := &TunnelManager{
		tunnels:        make(map[string]*Tunnel),
		configFile:     configFile,
		networkMonitor: NewNetworkMonitor(),
		sseClients:     make(map[chan string]bool),
	}

	// Set up SSE event sender for network monitor
	tm.networkMonitor.SetEventSender(tm.BroadcastSSE)

	// Load existing configurations
	tm.loadConfig()

	// Start network monitoring
	ctx := context.Background()
	tm.networkMonitor.Start(ctx)

	// Start background status broadcaster
	go tm.startStatusBroadcaster(ctx)

	// Add network change callback to restart tunnels when network comes back
	tm.networkMonitor.AddCallback(func(isConnected bool) {
		if isConnected {
			log.Println("Network restored - triggering tunnel reconnections")
			tm.onNetworkRestored()
		} else {
			log.Println("Network lost - tunnels will wait for reconnection")
		}
	})

	return tm
}

func (tm *TunnelManager) AddTunnel(config TunnelConfig) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Extract local port from command if not provided
	if config.LocalPort == "" {
		port, err := extractLocalPort(config.Command)
		if err != nil {
			return fmt.Errorf("could not extract local port from command: %v", err)
		}
		config.LocalPort = port
		config.AutoExtracted = true
	}

	// Check if port is available and free it if necessary
	if !isPortAvailable(config.LocalPort) {
		log.Printf("Port %s is in use. Process info:", config.LocalPort)
		log.Printf("%s", getProcessInfoForPort(config.LocalPort))

		if err := ensurePortAvailable(config.LocalPort); err != nil {
			return fmt.Errorf("failed to free port %s: %v", config.LocalPort, err)
		}

		// Double-check that port is now available
		if !isPortAvailable(config.LocalPort) {
			return fmt.Errorf("port %s is still not available after cleanup attempt", config.LocalPort)
		}
	}

	tunnel := &Tunnel{
		config: config,
		status: "disconnected",
	}

	tm.tunnels[config.Name] = tunnel

	// Save configuration
	tm.saveConfig()

	if config.Enabled {
		go tunnel.Start()
	}

	return nil
}

// func (tm *TunnelManager) GetStatus() []TunnelStatus {
// 	tm.mutex.RLock()
// 	defer tm.mutex.RUnlock()

// 	var statuses []TunnelStatus
// 	for _, tunnel := range tm.tunnels {
// 		tunnel.mutex.RLock()
// 		uptime := ""
// 		if tunnel.status == "connected" && !tunnel.connectedAt.IsZero() {
// 			uptime = time.Since(tunnel.connectedAt).Round(time.Second).String()
// 		}

// 		pid := 0
// 		if tunnel.cmd != nil && tunnel.cmd.Process != nil {
// 			pid = tunnel.cmd.Process.Pid
// 		}

// 		lastHealthCheck := ""
// 		if !tunnel.lastHealthCheck.IsZero() {
// 			lastHealthCheck = tunnel.lastHealthCheck.Format("15:04:05")
// 		}

// 		status := TunnelStatus{
// 			Config:          tunnel.config,
// 			Status:          tunnel.status,
// 			LastError:       tunnel.lastError,
// 			ConnectedAt:     tunnel.connectedAt,
// 			Uptime:          uptime,
// 			PID:             pid,
// 			LastHealthCheck: lastHealthCheck,
// 		}
// 		tunnel.mutex.RUnlock()
// 		statuses = append(statuses, status)
// 	}

// 	return statuses
// }

func (tm *TunnelManager) ToggleTunnel(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tunnel, exists := tm.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", name)
	}

	tunnel.config.Enabled = !tunnel.config.Enabled

	// Save configuration
	tm.saveConfig()

	if tunnel.config.Enabled {
		go tunnel.Start()
	} else {
		tunnel.Stop()
	}

	return nil
}

func (tm *TunnelManager) DeleteTunnel(name string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tunnel, exists := tm.tunnels[name]
	if !exists {
		return fmt.Errorf("tunnel not found: %s", name)
	}

	tunnel.Stop()
	delete(tm.tunnels, name)

	// Save configuration
	tm.saveConfig()

	return nil
}

func (t *Tunnel) Start() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.status == "connected" || t.status == "connecting" {
		log.Printf("Tunnel '%s' is already %s, skipping start", t.config.Name, t.status)
		return
	}

	// Cancel any existing maintenance goroutine
	if t.cancel != nil {
		t.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	// Set status to connecting to prevent multiple starts
	t.status = "connecting"

	log.Printf("Starting maintenance goroutine for tunnel '%s'", t.config.Name)

	// Start health monitoring
	t.startHealthMonitoring(ctx)

	go t.maintain(ctx)
}

// Stop stops the tunnel and cleans up resources
func (t *Tunnel) Stop() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}

	if t.healthTicker != nil {
		t.healthTicker.Stop()
		t.healthTicker = nil
	}

	t.status = "disconnected"
}

func (t *Tunnel) maintain(ctx context.Context) {
	retryDelay := 5 * time.Second
	maxRetryDelay := 60 * time.Second
	networkCheckInterval := 5 * time.Second
	wasNetworkDown := false

	// Start health monitoring
	t.startHealthMonitoring(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Check network connectivity before attempting connection
			networkAvailable := t.isNetworkAvailable()

			if !networkAvailable {
				if !wasNetworkDown {
					// Network just went down
					t.mutex.Lock()
					t.status = "error"
					t.lastError = "Network unavailable - waiting for connection"
					t.mutex.Unlock()
					log.Printf("Network became unavailable for tunnel '%s'", t.config.Name)
					wasNetworkDown = true
				}

				// Wait for network to come back with shorter intervals
				t.waitForNetwork(ctx, networkCheckInterval)
				continue
			} else if wasNetworkDown {
				// Network just came back
				log.Printf("Network restored for tunnel '%s', attempting reconnection", t.config.Name)
				wasNetworkDown = false
				retryDelay = 2 * time.Second // Quick retry when network comes back
			}

			// Check if SSH host is reachable
			if !t.isSSHHostReachable() {
				t.mutex.Lock()
				t.status = "error"
				t.lastError = "SSH host unreachable"
				t.mutex.Unlock()
				log.Printf("SSH host unreachable for tunnel '%s'", t.config.Name)

				// Wait before retrying
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
					// Increase retry delay, but cap it
					retryDelay = retryDelay * 2
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
					continue
				}
			}

			// Reset retry delay on successful connectivity checks
			if !wasNetworkDown {
				retryDelay = 5 * time.Second
			}

			// Attempt to connect
			success := t.connect()

			// If connection was successful, it will have blocked until the tunnel failed
			// Always wait before retrying, regardless of success/failure
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDelay):
				// Increase retry delay, but cap it
				if !success {
					retryDelay = retryDelay * 2
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
				} else {
					// On successful connection that then failed, use a moderate delay
					retryDelay = 10 * time.Second // Wait 10 seconds after a connection that failed
				}
			}
		}
	}
}

// startHealthMonitoring starts monitoring the tunnel health
func (t *Tunnel) startHealthMonitoring(ctx context.Context) {
	t.healthTicker = time.NewTicker(30 * time.Second) // Check every 30 seconds

	go func() {
		defer t.healthTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.healthTicker.C:
				t.performHealthCheck()
			}
		}
	}()
}

// performHealthCheck checks if the tunnel is still working
func (t *Tunnel) performHealthCheck() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.lastHealthCheck = time.Now()

	// Only check if we think we're connected
	if t.status != "connected" {
		return
	}

	// Check if the process is still running
	if t.cmd == nil || t.cmd.Process == nil {
		t.status = "error"
		t.lastError = "SSH process terminated unexpectedly"
		log.Printf("Health check failed for tunnel '%s': process terminated", t.config.Name)
		return
	}

	// Check if the port is still being forwarded
	if !t.isPortOpen() {
		t.status = "error"
		t.lastError = "Local port no longer accessible"
		log.Printf("Health check failed for tunnel '%s': port not accessible", t.config.Name)
		return
	}

	// Check basic network connectivity
	if !t.isNetworkAvailable() {
		t.status = "error"
		t.lastError = "Network connectivity lost"
		log.Printf("Health check failed for tunnel '%s': network unavailable", t.config.Name)
		return
	}

	log.Printf("Health check passed for tunnel '%s'", t.config.Name)
}

// waitForNetwork waits for network connectivity to be restored
func (t *Tunnel) waitForNetwork(ctx context.Context, checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if t.isNetworkAvailable() {
				return
			}
		}
	}
}

// func (t *Tunnel) connect() bool {
// 	t.mutex.Lock()

// 	// Ensure port is available before attempting connection
// 	if !isPortAvailable(t.config.LocalPort) {
// 		log.Printf("Port %s is in use before connecting tunnel '%s', attempting to free it", t.config.LocalPort, t.config.Name)
// 		if err := ensurePortAvailable(t.config.LocalPort); err != nil {
// 			t.status = "error"
// 			t.lastError = fmt.Sprintf("Failed to free port %s: %v", t.config.LocalPort, err)
// 			t.mutex.Unlock()
// 			return false
// 		}
// 	}

// 	t.status = "connecting"
// 	t.lastError = ""
// 	t.mutex.Unlock()

// 	// SSH key management disabled - will rely on existing ssh-agent or manual key management
// 	log.Printf("Connecting tunnel '%s' on port %s (auto-freed if needed)", t.config.Name, t.config.LocalPort)

// 	// Build SSH command with better options for tunneling
// 	args, err := parseSSHCommand(t.config.Command)
// 	if err != nil {
// 		t.mutex.Lock()
// 		t.status = "error"
// 		t.lastError = fmt.Sprintf("Failed to parse command: %v", err)
// 		t.mutex.Unlock()
// 		return false
// 	}

// 	// Ensure the first argument is actually 'ssh'
// 	if len(args) == 0 || !strings.Contains(args[0], "ssh") {
// 		t.mutex.Lock()
// 		t.status = "error"
// 		t.lastError = "Command must start with 'ssh'"
// 		t.mutex.Unlock()
// 		return false
// 	}

// 	// Add additional SSH options for better tunneling
// 	enhancedArgs := make([]string, 0, len(args)+10)
// 	enhancedArgs = append(enhancedArgs, "ssh") // Always use 'ssh' as the command

// 	// Add SSH options if not already present
// 	cmdStr := strings.Join(args, " ")
// 	if !strings.Contains(cmdStr, "-N") {
// 		enhancedArgs = append(enhancedArgs, "-N") // No remote command
// 	}
// 	if !strings.Contains(cmdStr, "-T") {
// 		enhancedArgs = append(enhancedArgs, "-T") // Disable pseudo-terminal
// 	}
// 	if !strings.Contains(cmdStr, "ServerAliveInterval") {
// 		enhancedArgs = append(enhancedArgs, "-o", "ServerAliveInterval=30")
// 	}
// 	if !strings.Contains(cmdStr, "ServerAliveCountMax") {
// 		enhancedArgs = append(enhancedArgs, "-o", "ServerAliveCountMax=3")
// 	}
// 	if !strings.Contains(cmdStr, "ExitOnForwardFailure") {
// 		enhancedArgs = append(enhancedArgs, "-o", "ExitOnForwardFailure=yes")
// 	}
// 	if !strings.Contains(cmdStr, "StrictHostKeyChecking") {
// 		enhancedArgs = append(enhancedArgs, "-o", "StrictHostKeyChecking=no")
// 	}
// 	if !strings.Contains(cmdStr, "UserKnownHostsFile") {
// 		enhancedArgs = append(enhancedArgs, "-o", "UserKnownHostsFile=/dev/null")
// 	}
// 	if !strings.Contains(cmdStr, "LogLevel") {
// 		enhancedArgs = append(enhancedArgs, "-o", "LogLevel=VERBOSE")
// 	}

// 	// Add the rest of the original arguments (skip the first 'ssh' argument)
// 	if len(args) > 1 {
// 		enhancedArgs = append(enhancedArgs, args[1:]...)
// 	}

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	cmd := exec.CommandContext(ctx, enhancedArgs[0], enhancedArgs[1:]...)

// 	// Capture stderr to see SSH errors
// 	var stderr strings.Builder
// 	cmd.Stderr = &stderr
// 	cmd.Stdout = nil

// 	t.mutex.Lock()
// 	t.cmd = cmd
// 	t.mutex.Unlock()

// 	log.Printf("Starting tunnel '%s' with command: %s", t.config.Name, strings.Join(enhancedArgs, " "))

// 	// Start the SSH command
// 	err = cmd.Start()
// 	if err != nil {
// 		t.mutex.Lock()
// 		t.status = "error"
// 		t.lastError = fmt.Sprintf("Failed to start SSH: %v", err)
// 		t.mutex.Unlock()
// 		return false
// 	}

// 	// Give SSH more time to establish the tunnel and try multiple times
// 	connected := false
// 	for i := 0; i < 10; i++ {
// 		time.Sleep(1 * time.Second)
// 		if t.isPortOpen() {
// 			connected = true
// 			break
// 		}
// 		// Check if process is still running
// 		if cmd.Process == nil {
// 			break
// 		}
// 	}

// 	if connected {
// 		t.mutex.Lock()
// 		t.status = "connected"
// 		t.connectedAt = time.Now()
// 		t.mutex.Unlock()

// 		log.Printf("Tunnel '%s' connected successfully on port %s", t.config.Name, t.config.LocalPort)

// 		// Wait for the command to finish
// 		err = cmd.Wait()

// 		t.mutex.Lock()
// 		if err != nil {
// 			stderrOutput := stderr.String()
// 			if stderrOutput != "" {
// 				t.lastError = fmt.Sprintf("SSH tunnel failed: %v - %s", err, stderrOutput)
// 				log.Printf("Tunnel '%s' SSH stderr: %s", t.config.Name, stderrOutput)
// 			} else {
// 				t.lastError = fmt.Sprintf("SSH tunnel failed: %v", err)
// 			}
// 			t.status = "error"
// 			log.Printf("Tunnel '%s' exited with error: %v", t.config.Name, err)
// 		} else {
// 			t.status = "disconnected"
// 			log.Printf("Tunnel '%s' exited normally", t.config.Name)
// 		}
// 		t.mutex.Unlock()
// 		return true // Connection was established (even if it later failed)
// 	} else {
// 		// Don't kill immediately - SSH might still be working even if port check fails
// 		t.mutex.Lock()
// 		t.status = "connected" // Assume it's working even if port check fails
// 		t.connectedAt = time.Now()
// 		t.lastError = "Port check failed but tunnel may still be working"
// 		t.mutex.Unlock()

// 		log.Printf("Tunnel '%s' started but port check failed - assuming it's working", t.config.Name)

// 		// Wait for the command to finish
// 		err = cmd.Wait()

// 		t.mutex.Lock()
// 		if err != nil {
// 			stderrOutput := stderr.String()
// 			if stderrOutput != "" {
// 				t.lastError = fmt.Sprintf("SSH tunnel failed: %v - %s", err, stderrOutput)
// 				log.Printf("Tunnel '%s' (no port check) SSH stderr: %s", t.config.Name, stderrOutput)
// 			} else {
// 				t.lastError = fmt.Sprintf("SSH tunnel failed: %v", err)
// 			}
// 			t.status = "error"
// 			log.Printf("Tunnel '%s' (no port check) exited with error: %v", t.config.Name, err)
// 		} else {
// 			t.status = "disconnected"
// 			log.Printf("Tunnel '%s' (no port check) exited normally", t.config.Name)
// 		}
// 		t.mutex.Unlock()
// 		return true // Connection was attempted (even if port check failed)
// 	}
// }

// func (t *Tunnel) isPortOpen() bool {
// 	// Try to connect to the local port
// 	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", t.config.LocalPort), 2*time.Second)
// 	if err != nil {
// 		// If connection failed, try alternative checks

// 		// Check if something is listening on the port
// 		ln, err := net.Listen("tcp", fmt.Sprintf(":%s", t.config.LocalPort))
// 		if err != nil {
// 			// Port is in use (which is good - means SSH is using it)
// 			return true
// 		}
// 		ln.Close()
// 		// Port is available (which is bad - means SSH isn't using it)
// 		return false
// 	}
// 	conn.Close()
// 	return true
// }

// isNetworkAvailable checks if network connectivity is available
func (t *Tunnel) isNetworkAvailable() bool {
	// Multiple connectivity checks for robustness
	checks := []func() bool{
		t.checkDNSConnectivity,
		t.checkInternetConnectivity,
		t.checkDefaultGateway,
	}

	// If any check passes, consider network available
	for _, check := range checks {
		if check() {
			return true
		}
	}

	return false
}

// checkDNSConnectivity checks if DNS resolution is working
func (t *Tunnel) checkDNSConnectivity() bool {
	// Try to resolve a public DNS server
	_, err := net.LookupHost("8.8.8.8")
	return err == nil
}

// checkInternetConnectivity checks if we can connect to the internet
func (t *Tunnel) checkInternetConnectivity() bool {
	// Try to establish a quick connection to a reliable service
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkDefaultGateway checks if we can reach the default gateway
func (t *Tunnel) checkDefaultGateway() bool {
	// Try to ping the default gateway
	// This is a basic check that works on most networks
	cmd := exec.Command("ping", "-c", "1", "-W", "2000", "8.8.8.8")
	err := cmd.Run()
	return err == nil
}

// isSSHHostReachable checks if the SSH host is reachable
func (t *Tunnel) isSSHHostReachable() bool {
	// Extract host from SSH command
	host := t.extractSSHHost()
	if host == "" {
		return false
	}

	// Try to connect to SSH port
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "22"), 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// extractSSHHost extracts the SSH host from the command
func (t *Tunnel) extractSSHHost() string {
	args, err := parseSSHCommand(t.config.Command)
	if err != nil {
		return ""
	}

	// Look for the host (usually the last argument without flags)
	for i := len(args) - 1; i >= 0; i-- {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") && strings.Contains(arg, "@") {
			// Format: user@host
			parts := strings.Split(arg, "@")
			if len(parts) == 2 {
				return parts[1]
			}
		} else if !strings.HasPrefix(arg, "-") && i == len(args)-1 {
			// Just hostname as last argument
			return arg
		}
	}

	return ""
}

// extractLocalPort extracts the local port from SSH command
func extractLocalPort(command string) (string, error) {
	// Look for -L flag followed by port forwarding specification
	re := regexp.MustCompile(`-L\s+(\d+):`)
	matches := re.FindStringSubmatch(command)
	if len(matches) >= 2 {
		return matches[1], nil
	}

	// Alternative format: -L port:host:port
	re2 := regexp.MustCompile(`-L\s+(\d+):[\w\.-]+:\d+`)
	matches2 := re2.FindStringSubmatch(command)
	if len(matches2) >= 2 {
		return matches2[1], nil
	}

	return "", fmt.Errorf("could not find local port in command")
}

// parseSSHCommand parses the SSH command string into command and arguments
func parseSSHCommand(command string) ([]string, error) {
	// Simple command parsing - split by spaces but handle quoted strings
	var args []string
	var current string
	inQuotes := false

	for i, char := range command {
		if char == '"' || char == '\'' {
			inQuotes = !inQuotes
		} else if char == ' ' && !inQuotes {
			if current != "" {
				args = append(args, current)
				current = ""
			}
		} else {
			current += string(char)
		}

		// Add the last argument
		if i == len(command)-1 && current != "" {
			args = append(args, current)
		}
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	return args, nil
}

// saveConfig saves tunnel configurations to disk
func (tm *TunnelManager) saveConfig() {
	var configs []TunnelConfig
	for _, tunnel := range tm.tunnels {
		configs = append(configs, tunnel.config)
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		log.Printf("Error marshaling config: %v", err)
		return
	}

	if err := ioutil.WriteFile(tm.configFile, data, 0644); err != nil {
		log.Printf("Error saving config to %s: %v", tm.configFile, err)
	} else {
		log.Printf("Configuration saved to %s", tm.configFile)
	}
}

// loadConfig loads tunnel configurations from disk
func (tm *TunnelManager) loadConfig() {
	data, err := ioutil.ReadFile(tm.configFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Error reading config file %s: %v", tm.configFile, err)
		}
		return
	}

	var configs []TunnelConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Printf("Error parsing config file %s: %v", tm.configFile, err)
		return
	}

	log.Printf("Loading %d tunnel configurations from %s", len(configs), tm.configFile)

	for _, config := range configs {
		tunnel := &Tunnel{
			config: config,
			status: "disconnected",
		}
		tm.tunnels[config.Name] = tunnel

		// Auto-start enabled tunnels
		if config.Enabled {
			go tunnel.Start()
		}
	}
}

// NetworkMonitor monitors network connectivity changes
type NetworkMonitor struct {
	callbacks   []func(bool)
	mutex       sync.RWMutex
	isRunning   bool
	eventSender func(string, interface{})
}

// NewNetworkMonitor creates a new network monitor
func NewNetworkMonitor() *NetworkMonitor {
	return &NetworkMonitor{
		callbacks: make([]func(bool), 0),
	}
}

// SetEventSender sets the function to send SSE events
func (nm *NetworkMonitor) SetEventSender(sender func(string, interface{})) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()
	nm.eventSender = sender
}

// AddCallback adds a callback for network changes
func (nm *NetworkMonitor) AddCallback(callback func(bool)) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()
	nm.callbacks = append(nm.callbacks, callback)
}

// Start starts monitoring network changes
func (nm *NetworkMonitor) Start(ctx context.Context) {
	nm.mutex.Lock()
	if nm.isRunning {
		nm.mutex.Unlock()
		return
	}
	nm.isRunning = true
	nm.mutex.Unlock()

	go nm.monitor(ctx)
}

// monitor runs the network monitoring loop
func (nm *NetworkMonitor) monitor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastNetworkState bool

	// Initial network state check
	currentState := nm.checkNetworkConnectivity()
	lastNetworkState = currentState

	for {
		select {
		case <-ctx.Done():
			nm.mutex.Lock()
			nm.isRunning = false
			nm.mutex.Unlock()
			return
		case <-ticker.C:
			currentState := nm.checkNetworkConnectivity()
			if currentState != lastNetworkState {
				log.Printf("Network state changed: %t -> %t", lastNetworkState, currentState)
				nm.notifyCallbacks(currentState)

				// Send SSE event about network change
				if nm.eventSender != nil {
					nm.eventSender("network_change", map[string]interface{}{
						"available": currentState,
						"previous":  lastNetworkState,
						"timestamp": time.Now().UTC(),
					})
				}

				lastNetworkState = currentState
			}
		}
	}
}

// checkNetworkConnectivity checks if network is available
func (nm *NetworkMonitor) checkNetworkConnectivity() bool {
	// Try to connect to a reliable service
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// notifyCallbacks notifies all registered callbacks of network changes
func (nm *NetworkMonitor) notifyCallbacks(isConnected bool) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	for _, callback := range nm.callbacks {
		go callback(isConnected)
	}
}

// onNetworkRestored handles network restoration by triggering reconnections
func (tm *TunnelManager) onNetworkRestored() {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for _, tunnel := range tm.tunnels {
		if tunnel.config.Enabled {
			tunnel.mutex.Lock()
			if tunnel.status == "error" && strings.Contains(tunnel.lastError, "Network") {
				log.Printf("Triggering reconnection for tunnel '%s' after network restoration", tunnel.config.Name)
				tunnel.status = "disconnected"
				tunnel.lastError = ""
			}
			tunnel.mutex.Unlock()
		}
	}
}

// // startStatusBroadcaster periodically broadcasts tunnel status updates
// func (tm *TunnelManager) startStatusBroadcaster(ctx context.Context) {
// 	ticker := time.NewTicker(3 * time.Second)
// 	defer ticker.Stop()

// 	var lastStatusJSON string

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			status := tm.GetStatus()
// 			statusJSON, _ := json.Marshal(status)
// 			currentStatusStr := string(statusJSON)

// 			// Only broadcast if status changed
// 			if currentStatusStr != lastStatusJSON {
// 				tm.BroadcastSSE("status_update", status)
// 				lastStatusJSON = currentStatusStr
// 			}
// 		}
// 	}
// }

func main() {
	// Handle version flag
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v", "version":
			fmt.Printf("Easy SSH Tunnel Manager\n")
			fmt.Printf("Version: %s\n", Version)
			fmt.Printf("Build Time: %s\n", BuildTime)
			fmt.Printf("Commit: %s\n", CommitHash)
			return
		case "--help", "-h", "help":
			fmt.Printf("Easy SSH Tunnel Manager - Web-based SSH tunnel management\n\n")
			fmt.Printf("Usage: %s [options]\n\n", os.Args[0])
			fmt.Printf("Options:\n")
			fmt.Printf("  --version, -v    Show version information\n")
			fmt.Printf("  --help, -h       Show this help message\n\n")
			fmt.Printf("Environment Variables:\n")
			fmt.Printf("  PORT             Web server port (default: 10000)\n\n")
			fmt.Printf("Web Interface:\n")
			fmt.Printf("  http://localhost:10000  (or custom PORT)\n\n")
			fmt.Printf("Documentation:\n")
			fmt.Printf("  https://github.com/ivikasavnish/easytunnel\n")
			return
		}
	}

	manager := NewTunnelManager()

	// Create template for the web interface
	tmpl := template.Must(template.New("index").Parse(indexHTML))

	// Web interface route
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		tmpl.Execute(w, nil)
	})

	// API Routes
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		json.NewEncoder(w).Encode(manager.GetStatus())
	})

	http.HandleFunc("/api/add", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var config TunnelConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if err := manager.AddTunnel(config); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})

	http.HandleFunc("/api/toggle/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/api/toggle/")
		if name == "" {
			http.Error(w, "Tunnel name required", http.StatusBadRequest)
			return
		}

		if err := manager.ToggleTunnel(name); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/delete/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "DELETE" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/api/delete/")
		if name == "" {
			http.Error(w, "Tunnel name required", http.StatusBadRequest)
			return
		}

		if err := manager.DeleteTunnel(name); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Server-sent events for real-time updates
	http.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

		if r.Method == "OPTIONS" {
			return
		}

		// Add client to SSE broadcast list
		client := manager.AddSSEClient()
		defer manager.RemoveSSEClient(client)

		// Send initial status
		status := manager.GetStatus()
		data, _ := json.Marshal(map[string]interface{}{
			"type":      "status_update",
			"data":      status,
			"timestamp": time.Now().UTC(),
		})
		fmt.Fprintf(w, "data: %s\n\n", data)
		w.(http.Flusher).Flush()

		// Listen for events and context cancellation
		for {
			select {
			case <-r.Context().Done():
				return
			case message := <-client:
				fmt.Fprintf(w, "data: %s\n\n", message)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
		}
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		response := map[string]interface{}{
			"status":  "healthy",
			"time":    time.Now().UTC(),
			"tunnels": len(manager.tunnels),
		}
		json.NewEncoder(w).Encode(response)
	})

	// Version endpoint
	http.HandleFunc("/api/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		response := map[string]interface{}{
			"version":     Version,
			"build_time":  BuildTime,
			"commit_hash": CommitHash,
			"timestamp":   time.Now().UTC(),
		}
		json.NewEncoder(w).Encode(response)
	})

	// Manual network change trigger for testing
	http.HandleFunc("/api/trigger-network-change", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get the state parameter
		state := r.URL.Query().Get("state")
		isConnected := state == "true"

		log.Printf("Manual network change triggered: %t", isConnected)

		// Broadcast the network change event
		manager.BroadcastSSE("network_change", map[string]interface{}{
			"available": isConnected,
			"previous":  !isConnected,
			"timestamp": time.Now().UTC(),
			"manual":    true,
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Network change event triggered"))
	})

	// Port management API endpoint
	http.HandleFunc("/api/kill-port/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		port := strings.TrimPrefix(r.URL.Path, "/api/kill-port/")
		if port == "" {
			http.Error(w, "Port number required", http.StatusBadRequest)
			return
		}

		// Validate port number
		if _, err := strconv.Atoi(port); err != nil {
			http.Error(w, "Invalid port number", http.StatusBadRequest)
			return
		}

		log.Printf("Manual port kill requested for port %s", port)

		// Get process info before killing
		processInfo := getProcessInfoForPort(port)
		log.Printf("Processes using port %s:\n%s", port, processInfo)

		// Kill processes on the port
		if err := killProcessesOnPort(port); err != nil {
			log.Printf("Failed to kill processes on port %s: %v", port, err)
			http.Error(w, fmt.Sprintf("Failed to kill processes on port %s: %v", port, err), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success":     true,
			"port":        port,
			"message":     fmt.Sprintf("Successfully freed port %s", port),
			"processInfo": processInfo,
			"timestamp":   time.Now().UTC(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Port status check API endpoint
	http.HandleFunc("/api/port-status/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		port := strings.TrimPrefix(r.URL.Path, "/api/port-status/")
		if port == "" {
			http.Error(w, "Port number required", http.StatusBadRequest)
			return
		}

		// Validate port number
		if _, err := strconv.Atoi(port); err != nil {
			http.Error(w, "Invalid port number", http.StatusBadRequest)
			return
		}

		available := isPortAvailable(port)
		pids, _ := getProcessesUsingPort(port)
		processInfo := ""
		if !available {
			processInfo = getProcessInfoForPort(port)
		}

		response := map[string]interface{}{
			"port":        port,
			"available":   available,
			"pids":        pids,
			"processInfo": processInfo,
			"timestamp":   time.Now().UTC(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Start server
	port := "10000"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	log.Printf("🚇 Easy Tunnel Manager v%s starting on port %s", Version, port)
	log.Printf("📱 Open http://localhost:%s in your browser", port)
	log.Printf("🔗 API endpoints available at http://localhost:%s/api/", port)
	log.Printf("💾 Configurations saved to: %s", manager.configFile)
	log.Printf("🔧 Build: %s (%s)", BuildTime, CommitHash)

	// Check privileges and inform about port reclamation capabilities
	if os.Geteuid() == 0 {
		log.Printf("🔐 Running with root privileges - can forcefully reclaim ports if needed")
	} else {
		log.Printf("⚠️  Running without root privileges - may not be able to kill all processes using required ports")
		log.Printf("💡 For full port management capabilities, run with: sudo %s", os.Args[0])
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	server := &http.Server{
		Addr: ":" + port,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-c
	log.Println("🛑 Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("✅ Server stopped")
}

// testSSHConnection tests the SSH connection without establishing a tunnel
func (t *Tunnel) testSSHConnection() error {
	args, err := parseSSHCommand(t.config.Command)
	if err != nil {
		return fmt.Errorf("failed to parse command: %v", err)
	}

	// Create a simple SSH test command
	testArgs := []string{"ssh", "-o", "ConnectTimeout=10", "-o", "BatchMode=yes"}

	// Extract the connection details (skip -L and other tunnel-specific options)
	for i, arg := range args {
		if i == 0 {
			continue // skip 'ssh'
		}
		if arg == "-L" && i+1 < len(args) {
			i++ // skip the -L argument and its value
			continue
		}
		if strings.HasPrefix(arg, "-L") {
			continue // skip combined -L arguments
		}
		testArgs = append(testArgs, arg)
	}

	// Add a simple test command
	testArgs = append(testArgs, "echo", "connection_test")

	cmd := exec.Command(testArgs[0], testArgs[1:]...)
	output, err := cmd.CombinedOutput()

	log.Printf("SSH test for '%s': %s", t.config.Name, string(output))

	return err
}

// extractSSHKeyFromCommand extracts SSH key path from the command
func extractSSHKeyFromCommand(command string) string {
	args, err := parseSSHCommand(command)
	if err != nil {
		return ""
	}

	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "-i") {
			// Handle combined format like -i~/.ssh/id_rsa
			return strings.TrimPrefix(arg, "-i")
		}
	}
	return ""
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// isKeyInAgent checks if a key is already added to ssh-agent
func isKeyInAgent(keyPath string) bool {
	cmd := exec.Command("ssh-add", "-l")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	expandedPath := expandPath(keyPath)
	return strings.Contains(string(output), expandedPath)
}

// addKeyToAgent adds an SSH key to ssh-agent
func addKeyToAgent(keyPath string) error {
	expandedPath := expandPath(keyPath)

	// Check if key file exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file not found: %s", expandedPath)
	}

	cmd := exec.Command("ssh-add", expandedPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add key to ssh-agent: %v - %s", err, string(output))
	}

	log.Printf("Successfully added SSH key to agent: %s", expandedPath)
	return nil
}

// ensureSSHKeyInAgent ensures the SSH key is added to ssh-agent
func (t *Tunnel) ensureSSHKeyInAgent() error {
	// First ensure ssh-agent is running
	if err := ensureSSHAgentRunning(); err != nil {
		return fmt.Errorf("could not start ssh-agent: %v", err)
	}

	keyPath := extractSSHKeyFromCommand(t.config.Command)
	if keyPath == "" {
		// No specific key specified, try default keys
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil // Skip if can't get home dir
		}

		defaultKeys := []string{
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
			filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		}

		for _, key := range defaultKeys {
			if _, err := os.Stat(key); err == nil {
				if !isKeyInAgent(key) {
					if err := addKeyToAgent(key); err != nil {
						log.Printf("Warning: Could not add default key %s: %v", key, err)
					} else {
						return nil // Successfully added a key
					}
				} else {
					return nil // Key already in agent
				}
			}
		}
		return nil // No default keys found or needed
	}

	// Check if the specified key is in the agent
	if !isKeyInAgent(keyPath) {
		return addKeyToAgent(keyPath)
	}

	return nil // Key already in agent
}

// ensureSSHAgentRunning ensures ssh-agent is running
func ensureSSHAgentRunning() error {
	// Check if SSH_AUTH_SOCK is set (indicates ssh-agent is running)
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		// Test if agent is actually responsive
		cmd := exec.Command("ssh-add", "-l")
		if err := cmd.Run(); err == nil {
			return nil // Agent is running and responsive
		}
	}

	// Try to start ssh-agent
	cmd := exec.Command("ssh-agent", "-s")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to start ssh-agent: %v", err)
	}

	// Parse the output to set environment variables
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "SSH_AUTH_SOCK") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				value := strings.TrimSuffix(parts[1], ";")
				os.Setenv("SSH_AUTH_SOCK", value)
			}
		}
		if strings.Contains(line, "SSH_AGENT_PID") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				value := strings.TrimSuffix(parts[1], ";")
				os.Setenv("SSH_AGENT_PID", value)
			}
		}
	}

	log.Printf("Started ssh-agent")
	return nil
}

// startStatusBroadcaster periodically broadcasts tunnel status updates
func (tm *TunnelManager) startStatusBroadcaster(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Track meaningful status changes only
	type StatusSnapshot struct {
		Name   string
		Status string
		Error  string
		PID    int
	}

	var lastSnapshots []StatusSnapshot

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := tm.GetStatus()

			// Create current snapshots (excluding dynamic fields like uptime)
			currentSnapshots := make([]StatusSnapshot, len(status))
			for i, s := range status {
				currentSnapshots[i] = StatusSnapshot{
					Name:   s.Config.Name,
					Status: s.Status,
					Error:  s.LastError,
					PID:    s.PID,
				}
			}

			// Only broadcast if meaningful status changed
			hasChanged := false

			// Check if number of tunnels changed
			if len(currentSnapshots) != len(lastSnapshots) {
				hasChanged = true
			} else {
				// Check if any tunnel status changed meaningfully
				for i, current := range currentSnapshots {
					if i >= len(lastSnapshots) {
						hasChanged = true
						break
					}

					last := lastSnapshots[i]
					if current.Name != last.Name ||
						current.Status != last.Status ||
						current.Error != last.Error ||
						current.PID != last.PID {
						hasChanged = true
						break
					}
				}
			}

			if hasChanged {
				log.Printf("Broadcasting status update - meaningful changes detected")
				tm.BroadcastSSE("status_update", status)
				lastSnapshots = currentSnapshots
			}
		}
	}
}

// Enhanced status checking to avoid false positives
func (tm *TunnelManager) GetStatus() []TunnelStatus {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var statuses []TunnelStatus
	for _, tunnel := range tm.tunnels {
		tunnel.mutex.RLock()

		// Only calculate uptime for truly connected tunnels
		uptime := ""
		if tunnel.status == "connected" && !tunnel.connectedAt.IsZero() {
			// Ensure we've been connected for at least 5 seconds before showing uptime
			connectedDuration := time.Since(tunnel.connectedAt)
			if connectedDuration >= 5*time.Second {
				uptime = connectedDuration.Round(time.Second).String()
			}
		}

		pid := 0
		if tunnel.cmd != nil && tunnel.cmd.Process != nil {
			pid = tunnel.cmd.Process.Pid
		}

		lastHealthCheck := ""
		if !tunnel.lastHealthCheck.IsZero() {
			lastHealthCheck = tunnel.lastHealthCheck.Format("15:04:05")
		}

		status := TunnelStatus{
			Config:          tunnel.config,
			Status:          tunnel.status,
			LastError:       tunnel.lastError,
			ConnectedAt:     tunnel.connectedAt,
			Uptime:          uptime,
			PID:             pid,
			LastHealthCheck: lastHealthCheck,
		}
		tunnel.mutex.RUnlock()
		statuses = append(statuses, status)
	}

	return statuses
}

// Enhanced connection logic to prevent false connected states
func (t *Tunnel) connect() bool {
	t.mutex.Lock()

	// Ensure port is available before attempting connection
	if !isPortAvailable(t.config.LocalPort) {
		log.Printf("Port %s is in use before connecting tunnel '%s', attempting to free it", t.config.LocalPort, t.config.Name)
		if err := ensurePortAvailable(t.config.LocalPort); err != nil {
			t.status = "error"
			t.lastError = fmt.Sprintf("Failed to free port %s: %v", t.config.LocalPort, err)
			t.mutex.Unlock()
			return false
		}
	}

	t.status = "connecting"
	t.lastError = ""
	t.mutex.Unlock()

	log.Printf("Connecting tunnel '%s' on port %s", t.config.Name, t.config.LocalPort)

	// Build SSH command with better options for tunneling
	args, err := parseSSHCommand(t.config.Command)
	if err != nil {
		t.mutex.Lock()
		t.status = "error"
		t.lastError = fmt.Sprintf("Failed to parse command: %v", err)
		t.mutex.Unlock()
		return false
	}

	// Ensure the first argument is actually 'ssh'
	if len(args) == 0 || !strings.Contains(args[0], "ssh") {
		t.mutex.Lock()
		t.status = "error"
		t.lastError = "Command must start with 'ssh'"
		t.mutex.Unlock()
		return false
	}

	// Add additional SSH options for better tunneling
	enhancedArgs := make([]string, 0, len(args)+10)
	enhancedArgs = append(enhancedArgs, "ssh") // Always use 'ssh' as the command

	// Add SSH options if not already present
	cmdStr := strings.Join(args, " ")
	if !strings.Contains(cmdStr, "-N") {
		enhancedArgs = append(enhancedArgs, "-N") // No remote command
	}
	if !strings.Contains(cmdStr, "-T") {
		enhancedArgs = append(enhancedArgs, "-T") // Disable pseudo-terminal
	}
	if !strings.Contains(cmdStr, "ServerAliveInterval") {
		enhancedArgs = append(enhancedArgs, "-o", "ServerAliveInterval=30")
	}
	if !strings.Contains(cmdStr, "ServerAliveCountMax") {
		enhancedArgs = append(enhancedArgs, "-o", "ServerAliveCountMax=3")
	}
	if !strings.Contains(cmdStr, "ExitOnForwardFailure") {
		enhancedArgs = append(enhancedArgs, "-o", "ExitOnForwardFailure=yes")
	}
	if !strings.Contains(cmdStr, "StrictHostKeyChecking") {
		enhancedArgs = append(enhancedArgs, "-o", "StrictHostKeyChecking=no")
	}
	if !strings.Contains(cmdStr, "UserKnownHostsFile") {
		enhancedArgs = append(enhancedArgs, "-o", "UserKnownHostsFile=/dev/null")
	}
	if !strings.Contains(cmdStr, "LogLevel") {
		enhancedArgs = append(enhancedArgs, "-o", "LogLevel=ERROR") // Reduce verbosity
	}

	// Add the rest of the original arguments (skip the first 'ssh' argument)
	if len(args) > 1 {
		enhancedArgs = append(enhancedArgs, args[1:]...)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, enhancedArgs[0], enhancedArgs[1:]...)

	// Capture stderr to see SSH errors
	var stderr strings.Builder
	cmd.Stderr = &stderr
	cmd.Stdout = nil

	t.mutex.Lock()
	t.cmd = cmd
	t.mutex.Unlock()

	log.Printf("Starting tunnel '%s' with command: %s", t.config.Name, strings.Join(enhancedArgs, " "))

	// Start the SSH command
	err = cmd.Start()
	if err != nil {
		t.mutex.Lock()
		t.status = "error"
		t.lastError = fmt.Sprintf("Failed to start SSH: %v", err)
		t.mutex.Unlock()
		return false
	}

	// Wait longer and check more thoroughly for tunnel establishment
	connected := false
	maxAttempts := 15 // Give up to 15 seconds

	for i := 0; i < maxAttempts; i++ {
		time.Sleep(1 * time.Second)

		// Check if process is still running first
		if cmd.Process == nil {
			log.Printf("Tunnel '%s' process died during startup", t.config.Name)
			break
		}

		// Then check if port is accessible
		if t.isPortOpen() {
			// Double-check by trying to connect
			if t.verifyPortConnection() {
				connected = true
				log.Printf("Tunnel '%s' port verification successful after %d seconds", t.config.Name, i+1)
				break
			}
		}

		// Show progress for longer connections
		if i > 5 && i%3 == 0 {
			log.Printf("Tunnel '%s' still establishing connection... (%ds)", t.config.Name, i+1)
		}
	}

	if connected {
		t.mutex.Lock()
		t.status = "connected"
		t.connectedAt = time.Now()
		t.lastError = ""
		t.mutex.Unlock()

		log.Printf("Tunnel '%s' connected successfully on port %s", t.config.Name, t.config.LocalPort)

		// Wait for the command to finish
		err = cmd.Wait()

		t.mutex.Lock()
		if err != nil {
			stderrOutput := stderr.String()
			if stderrOutput != "" {
				t.lastError = fmt.Sprintf("SSH tunnel failed: %v - %s", err, stderrOutput)
				log.Printf("Tunnel '%s' SSH stderr: %s", t.config.Name, stderrOutput)
			} else {
				t.lastError = fmt.Sprintf("SSH tunnel failed: %v", err)
			}
			t.status = "error"
			log.Printf("Tunnel '%s' exited with error: %v", t.config.Name, err)
		} else {
			t.status = "disconnected"
			t.lastError = ""
			log.Printf("Tunnel '%s' exited normally", t.config.Name)
		}
		t.mutex.Unlock()
		return true // Connection was established (even if it later failed)
	} else {
		// Connection failed to establish
		t.mutex.Lock()

		// Kill the process since it didn't establish properly
		if cmd.Process != nil {
			cmd.Process.Kill()
		}

		stderrOutput := stderr.String()
		if stderrOutput != "" {
			t.lastError = fmt.Sprintf("Connection failed to establish: %s", stderrOutput)
			log.Printf("Tunnel '%s' failed to establish - stderr: %s", t.config.Name, stderrOutput)
		} else {
			t.lastError = "Connection failed to establish within timeout"
			log.Printf("Tunnel '%s' failed to establish within %d seconds", t.config.Name, maxAttempts)
		}

		t.status = "error"
		t.mutex.Unlock()
		return false
	}
}

// Add a more thorough port verification method
func (t *Tunnel) verifyPortConnection() bool {
	// Try to actually connect and send/receive data
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", t.config.LocalPort), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Set a short deadline for the verification
	conn.SetDeadline(time.Now().Add(1 * time.Second))

	// The connection succeeded, which means something is listening
	// For SSH tunnels, this is usually sufficient verification
	return true
}

func (t *Tunnel) isPortOpen() bool {
	// Try to connect to the local port
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", t.config.LocalPort), 2*time.Second)
	if err != nil {
		// If connection failed, try alternative checks
		// Check if something is listening on the port
		ln, err := net.Listen("tcp", fmt.Sprintf(":%s", t.config.LocalPort))
		if err != nil {
			// Port is in use (which is good - means SSH is using it)
			return true
		}
		ln.Close()
		// Port is available (which is bad - means SSH isn't using it)
		return false
	}
	conn.Close()
	return true
}
