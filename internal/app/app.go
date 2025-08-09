// Package app provides the main application logic and command handling for GoProxLB.
package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cblomart/GoProxLB/internal/balancer"
	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/cblomart/GoProxLB/internal/proxmox"
)

const (
	vmStatusRunning   = "running"
	balancerThreshold = "threshold"
	balancerAdvanced  = "advanced"
)

// App represents the main application.
type App struct {
	config   *config.Config
	client   ClientInterface
	balancer BalancerInterface
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewApp creates a new application instance.
func NewApp(configPath string) (*App, error) {
	config, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Auto-detect cluster name if not specified
	if config.Cluster.Name == "" {
		client := proxmox.NewClient(&config.Proxmox)
		if err := config.AutoDetectClusterName(client); err != nil {
			return nil, fmt.Errorf("failed to auto-detect cluster name: %w", err)
		}
		fmt.Printf("Auto-detected cluster name: %s\n", config.Cluster.Name)
	}

	client := proxmox.NewClient(&config.Proxmox)

	var balancerInstance BalancerInterface
	if config.IsAdvancedBalancer() {
		balancerInstance = balancer.NewAdvancedBalancer(client, config)
	} else {
		balancerInstance = balancer.NewBalancer(client, config)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		config:   config,
		client:   client,
		balancer: balancerInstance,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// NewAppWithDependencies creates a new application instance with custom dependencies.
func NewAppWithDependencies(configPath string, configLoader ConfigLoaderInterface, client ClientInterface, balancerInstance BalancerInterface) (*App, error) {
	var cfg *config.Config
	var err error

	// Load configuration
	if configLoader != nil {
		cfg, err = configLoader.Load(configPath)
	} else {
		cfg, err = config.Load(configPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create Proxmox client if not provided
	if client == nil {
		client = proxmox.NewClient(&cfg.Proxmox)
	}

	// Create balancer if not provided
	if balancerInstance == nil {
		if cfg.IsAdvancedBalancer() {
			balancerInstance = balancer.NewAdvancedBalancer(client, cfg)
		} else {
			balancerInstance = balancer.NewBalancer(client, cfg)
		}
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		config:   cfg,
		client:   client,
		balancer: balancerInstance,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// NewAppWithDefaults creates a new application instance with default configuration.
func NewAppWithDefaults() (*App, error) {
	config, err := config.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("failed to load default config: %w", err)
	}

	// Auto-detect cluster name from Proxmox API
	client := proxmox.NewClient(&config.Proxmox)
	if err := config.AutoDetectClusterName(client); err != nil {
		return nil, fmt.Errorf("failed to auto-detect cluster name: %w", err)
	}
	fmt.Printf("Auto-detected cluster name: %s\n", config.Cluster.Name)

	var balancerInstance BalancerInterface
	if config.IsAdvancedBalancer() {
		balancerInstance = balancer.NewAdvancedBalancer(client, config)
	} else {
		balancerInstance = balancer.NewBalancer(client, config)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		config:   config,
		client:   client,
		balancer: balancerInstance,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start starts the load balancer daemon with default balancer type.
func Start(configPath string) error {
	return StartWithBalancerType(configPath, "")
}

// StartWithBalancerType starts the load balancer daemon with a specific balancer type.
func StartWithBalancerType(configPath, balancerType string) error {
	app, err := NewApp(configPath)
	if err != nil {
		return err
	}
	defer app.cancel()

	// Override balancer type if specified
	if balancerType != "" {
		if balancerType != balancerThreshold && balancerType != balancerAdvanced {
			return fmt.Errorf("invalid balancer type: %s (must be 'threshold' or 'advanced')", balancerType)
		}
		app.config.Balancing.BalancerType = balancerType

		// Recreate the balancer with the new type
		client := app.client
		if app.config.IsAdvancedBalancer() {
			app.balancer = balancer.NewAdvancedBalancer(client, app.config)
		} else {
			app.balancer = balancer.NewBalancer(client, app.config)
		}
	}

	fmt.Println("Starting GoProxLB...")
	fmt.Printf("Configuration loaded from: %s\n", configPath)
	fmt.Printf("Proxmox host: %s\n", app.config.Proxmox.Host)
	fmt.Printf("Cluster: %s\n", app.config.Cluster.Name)
	fmt.Printf("Balancing enabled: true\n")
	fmt.Printf("Balancer type: %s\n", app.config.Balancing.BalancerType)
	fmt.Printf("Aggressiveness: %s\n", app.config.Balancing.Aggressiveness)

	// Get balancing interval
	interval, err := app.config.GetInterval()
	if err != nil {
		return fmt.Errorf("invalid balancing interval: %w", err)
	}

	fmt.Printf("Balancing interval: %v\n", interval)
	fmt.Printf("Balancing enabled: true\n")

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start balancing loop
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	fmt.Println("Load balancer started. Press Ctrl+C to stop.")

	for {
		select {
		case <-app.ctx.Done():
			fmt.Println("Shutting down...")
			return nil
		case <-sigChan:
			fmt.Println("\nReceived shutdown signal...")
			app.cancel()
			return nil
		case <-ticker.C:
			if err := app.runBalancingCycle(); err != nil {
				fmt.Printf("Error during balancing cycle: %v\n", err)
			}
		}
	}
}

// runBalancingCycle runs a single balancing cycle.
func (app *App) runBalancingCycle() error {
	fmt.Printf("[%s] Running balancing cycle...\n", time.Now().Format("2006-01-02 15:04:05"))

	results, err := app.balancer.Run(false)
	if err != nil {
		return fmt.Errorf("balancing cycle failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No balancing actions needed")
		return nil
	}

	fmt.Printf("Executed %d migrations:\n", len(results))
	for i := range results {
		result := &results[i]
		if result.Success {
			fmt.Printf("  ✓ Migrated VM %s (%d) from %s to %s (gain: %.2f)\n",
				result.VM.Name, result.VM.ID, result.SourceNode, result.TargetNode, result.ResourceGain)
		} else {
			fmt.Printf("  ✗ Failed to migrate VM %s (%d): %s\n",
				result.VM.Name, result.VM.ID, result.ErrorMessage)
		}
	}

	return nil
}

// ShowStatus shows the current status of the load balancer.
func ShowStatus(configPath string) error {
	var app *App
	var err error

	if configPath == "" {
		app, err = NewAppWithDefaults()
	} else {
		app, err = NewApp(configPath)
	}

	if err != nil {
		return err
	}
	defer app.cancel()

	// Get cluster status
	status, err := app.balancer.GetClusterStatus()
	if err != nil {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	fmt.Println("=== GoProxLB Status ===")
	fmt.Printf("Total Nodes: %d\n", status.TotalNodes)
	fmt.Printf("Active Nodes: %d\n", status.ActiveNodes)
	fmt.Printf("Total VMs: %d\n", status.TotalVMs)
	fmt.Printf("Running VMs: %d\n", status.RunningVMs)
	fmt.Printf("Balancing Enabled: %v\n", status.BalancingEnabled)
	fmt.Printf("Last Balanced: %v\n", status.LastBalanced)
	fmt.Printf("Average CPU Usage: %.1f%%\n", status.AverageCPU)
	fmt.Printf("Average Memory Usage: %.1f%%\n", status.AverageMemory)
	fmt.Printf("Average Storage Usage: %.1f%%\n", status.AverageStorage)

	return nil
}

// ShowClusterInfo shows detailed cluster information.
func ShowClusterInfo(configPath string) error {
	var app *App
	var err error

	if configPath == "" {
		app, err = NewAppWithDefaults()
	} else {
		app, err = NewApp(configPath)
	}

	if err != nil {
		return err
	}
	defer app.cancel()

	// Get cluster status
	status, err := app.balancer.GetClusterStatus()
	if err != nil {
		return fmt.Errorf("failed to get cluster status: %w", err)
	}

	fmt.Println("=== Cluster Information ===")
	fmt.Printf("Total Nodes: %d\n", status.TotalNodes)
	fmt.Printf("Active Nodes: %d\n", status.ActiveNodes)
	fmt.Printf("Total VMs: %d\n", status.TotalVMs)
	fmt.Printf("Running VMs: %d\n", status.RunningVMs)
	fmt.Printf("Average CPU Usage: %.1f%%\n", status.AverageCPU)
	fmt.Printf("Average Memory Usage: %.1f%%\n", status.AverageMemory)
	fmt.Printf("Average Storage Usage: %.1f%%\n", status.AverageStorage)

	// Get detailed node information
	nodes, err := app.client.GetNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	fmt.Println("\n=== Node Details ===")
	for i := range nodes {
		node := &nodes[i]
		fmt.Printf("Node: %s\n", node.Name)
		fmt.Printf("  Status: %s\n", node.Status)
		fmt.Printf("  CPU: %.1f%% (%d cores)\n", node.CPU.Usage, node.CPU.Cores)
		fmt.Printf("  Memory: %.1f%% (%.1f GB used / %.1f GB total)\n",
			node.Memory.Usage,
			float64(node.Memory.Used)/1024/1024/1024,
			float64(node.Memory.Total)/1024/1024/1024)
		fmt.Printf("  Storage: %.1f%% (%.1f GB used / %.1f GB total)\n",
			node.Storage.Usage,
			float64(node.Storage.Used)/1024/1024/1024,
			float64(node.Storage.Total)/1024/1024/1024)
		fmt.Printf("  VMs: %d\n", len(node.VMs))
		fmt.Println()
	}

	return nil
}

// ListVMs lists all VMs in the cluster.
func ListVMs(configPath string) error {
	var app *App
	var err error

	if configPath == "" {
		app, err = NewAppWithDefaults()
	} else {
		app, err = NewApp(configPath)
	}

	if err != nil {
		return err
	}
	defer app.cancel()

	// Get nodes and their VMs
	nodes, err := app.client.GetNodes()
	if err != nil {
		return fmt.Errorf("failed to get nodes: %w", err)
	}

	fmt.Println("=== Virtual Machines ===")
	totalVMs := 0
	runningVMs := 0

	for i := range nodes {
		node := &nodes[i]
		fmt.Printf("\nNode: %s\n", node.Name)
		fmt.Printf("  Status: %s\n", node.Status)

		if len(node.VMs) == 0 {
			fmt.Println("  No VMs")
			continue
		}

		fmt.Printf("  VMs (%d):\n", len(node.VMs))
		for j := range node.VMs {
			vm := &node.VMs[j]
			totalVMs++
			status := "stopped"
			if vm.Status == vmStatusRunning {
				status = vmStatusRunning
				runningVMs++
			}

			fmt.Printf("    %d: %s (%s) - %s\n", vm.ID, vm.Name, vm.Type, status)
			if vm.Status == vmStatusRunning {
				fmt.Printf("      CPU: %.1f%%, Memory: %.1f GB\n",
					vm.CPU, float64(vm.Memory)/1024/1024/1024)
			}
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Total VMs: %d\n", totalVMs)
	fmt.Printf("Running VMs: %d\n", runningVMs)
	fmt.Printf("Stopped VMs: %d\n", totalVMs-runningVMs)

	return nil
}

// ForceBalance forces a balancing operation.
func ForceBalance(configPath string, force bool) error {
	app, err := NewApp(configPath)
	if err != nil {
		return err
	}
	defer app.cancel()

	fmt.Printf("Forcing balance operation (force=%v)...\n", force)

	results, err := app.balancer.Run(force)
	if err != nil {
		return fmt.Errorf("balance operation failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No balancing actions performed")
		return nil
	}

	fmt.Printf("Balance operation completed. %d migrations executed:\n", len(results))
	for i := range results {
		result := &results[i]
		if result.Success {
			fmt.Printf("  ✓ Migrated VM %d from %s to %s\n", result.VM.ID, result.SourceNode, result.TargetNode)
		} else {
			fmt.Printf("  ✗ Failed to migrate VM %d: %s\n", result.VM.ID, result.ErrorMessage)
		}
	}

	return nil
}

// ForceBalanceWithBalancerType forces a balancing operation with a specific balancer type.
func ForceBalanceWithBalancerType(configPath string, force bool, balancerType string) error {
	app, err := NewApp(configPath)
	if err != nil {
		return err
	}
	defer app.cancel()

	// Override balancer type if specified
	if balancerType != "" {
		if balancerType != balancerThreshold && balancerType != balancerAdvanced {
			return fmt.Errorf("invalid balancer type: %s (must be 'threshold' or 'advanced')", balancerType)
		}
		app.config.Balancing.BalancerType = balancerType

		// Recreate the balancer with the new type
		client := app.client
		if app.config.IsAdvancedBalancer() {
			app.balancer = balancer.NewAdvancedBalancer(client, app.config)
		} else {
			app.balancer = balancer.NewBalancer(client, app.config)
		}
	}

	fmt.Printf("Forcing balance operation (force=%v, balancer=%s)...\n", force, app.config.Balancing.BalancerType)

	results, err := app.balancer.Run(force)
	if err != nil {
		return fmt.Errorf("balance operation failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No balancing actions performed")
		return nil
	}

	fmt.Printf("Balance operation completed. %d migrations executed:\n", len(results))
	for i := range results {
		result := &results[i]
		if result.Success {
			fmt.Printf("  ✓ Migrated VM %d from %s to %s\n", result.VM.ID, result.SourceNode, result.TargetNode)
		} else {
			fmt.Printf("  ✗ Failed to migrate VM %d: %s\n", result.VM.ID, result.ErrorMessage)
		}
	}

	return nil
}

// ShowCapacityPlanning shows detailed capacity planning information.
func ShowCapacityPlanning(configPath string, detailed bool, forecast, csvOutput string) error {
	context, err := setupCapacityPlanningContext(configPath, forecast, csvOutput)
	if err != nil {
		return err
	}

	printCapacityPlanningHeader(context.forecastDuration)

	// Analyze all nodes and collect recommendations
	adaptationRecommendations := analyzeNodesForCapacityPlanning(context, detailed)

	// Display final results
	return displayCapacityPlanningResults(context, adaptationRecommendations)
}

// writeCSVFile writes the CSV data to a file.
func writeCSVFile(filename string, data [][]string) error {
	// Validate filename to prevent path traversal attacks
	cleanFilename := filepath.Clean(filename)
	if !filepath.IsAbs(cleanFilename) {
		// If relative path, make it relative to current working directory
		if wd, err := os.Getwd(); err == nil {
			cleanFilename = filepath.Join(wd, cleanFilename)
		}
	}

	// Ensure we're not trying to write outside allowed directories
	if strings.Contains(cleanFilename, "..") {
		return fmt.Errorf("invalid filename: path traversal not allowed")
	}

	file, err := os.Create(cleanFilename)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck // file is being written, close error not actionable

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, row := range data {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// ShowRaftStatus shows detailed Raft cluster status information.
func ShowRaftStatus(configPath string) error {
	app, err := initializeApp(configPath)
	if err != nil {
		return err
	}
	defer app.cancel()

	if !app.config.Raft.Enabled {
		displaySingleNodeStatus()
		return nil
	}

	status, err := fetchRaftStatus()
	if err != nil {
		displayServiceNotFound(configPath)
		return nil
	}

	displayRaftClusterStatus(status)
	displayClusterHealth(status)
	displayAutoDiscoveryStatus(app)
	displayRaftConfiguration(app)

	return nil
}

// initializeApp creates a new app instance with the given config path.
func initializeApp(configPath string) (*App, error) {
	if configPath == "" {
		return NewAppWithDefaults()
	}
	return NewApp(configPath)
}

// displaySingleNodeStatus shows status for single-node deployments.
func displaySingleNodeStatus() {
		fmt.Println("=== Raft Status ===")
		fmt.Println("Raft is not enabled in configuration")
		fmt.Println("This is a single-node deployment")
}

// displayServiceNotFound shows message when GoProxLB service is not running.
func displayServiceNotFound(configPath string) {
		fmt.Println("=== Raft Status ===")
		fmt.Println("⚠️  No running GoProxLB service found")
		fmt.Println("Please start GoProxLB in distributed mode first:")
		if configPath == "" {
			fmt.Println("  goproxlb --config config.yaml")
		} else {
			fmt.Printf("  goproxlb --config %s\n", configPath)
		}
		fmt.Println()
		fmt.Println("The service will expose status at:")
	fmt.Printf("  %s\n", "/var/lib/goproxlb/status.sock")
}

// fetchRaftStatus connects to the Unix socket and retrieves Raft status.
func fetchRaftStatus() (map[string]interface{}, error) {
	socketPath := "/var/lib/goproxlb/status.sock"
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close() //nolint:errcheck // connection cleanup, error not actionable

	// Send HTTP GET request
	request := "GET /status HTTP/1.1\r\nHost: localhost\r\n\r\n"
	if _, err = conn.Write([]byte(request)); err != nil {
		return nil, fmt.Errorf("error writing request: %w", err)
	}

	// Read and parse response
	response, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	return parseHTTPResponse(response)
}

// parseHTTPResponse extracts JSON from HTTP response.
func parseHTTPResponse(response []byte) (map[string]interface{}, error) {
	parts := bytes.Split(response, []byte("\r\n\r\n"))
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid response format")
	}

	var status map[string]interface{}
	if err := json.Unmarshal(parts[1], &status); err != nil {
		return nil, fmt.Errorf("error parsing status response: %w", err)
	}

	return status, nil
}

// displayRaftClusterStatus shows basic Raft cluster information.
func displayRaftClusterStatus(status map[string]interface{}) {
	fmt.Println("=== Raft Cluster Status ===")
	fmt.Printf("Node ID: %s\n", status["node_id"])
	fmt.Printf("Address: %s\n", status["address"])
	fmt.Printf("Current State: %s\n", status["raft_state"])
	fmt.Printf("Is Leader: %v\n", status["is_leader"])
	fmt.Printf("Current Leader: %s\n", status["leader"])
}

// displayClusterHealth shows cluster health information including quorum status.
func displayClusterHealth(status map[string]interface{}) {
	peers, ok := status["peers"].([]interface{})
	if !ok {
		return
	}

		peerStrings := make([]string, len(peers))
		for i, peer := range peers {
			peerStrings[i] = peer.(string)
		}
		fmt.Printf("Peers (%d): %v\n", len(peerStrings), peerStrings)

		fmt.Println("\n=== Cluster Health ===")
		if len(peerStrings) == 0 {
			fmt.Println("⚠️  No peers configured - single node cluster")
		return
	}

			quorumSize := (len(peerStrings)+1)/2 + 1 // +1 for current node
	totalNodes := len(peerStrings) + 1
			fmt.Printf("Quorum size: %d nodes\n", quorumSize)
	fmt.Printf("Total nodes: %d nodes\n", totalNodes)

	if totalNodes >= quorumSize {
				fmt.Println("✅ Cluster has quorum")
			} else {
				fmt.Println("❌ Cluster does not have quorum")
		}
	}

// displayAutoDiscoveryStatus shows auto-discovery configuration status.
func displayAutoDiscoveryStatus(app *App) {
	fmt.Println("\n=== Auto-Discovery ===")
	if app.config.Raft.AutoDiscover {
		fmt.Println("✅ Auto-discovery enabled")
		fmt.Println("Peers are automatically discovered from Proxmox cluster")
	} else {
		fmt.Println("❌ Auto-discovery disabled")
		fmt.Println("Peers must be manually configured")
	}
	}

// displayRaftConfiguration shows Raft configuration details.
func displayRaftConfiguration(app *App) {
	fmt.Println("\n=== Raft Configuration ===")
	fmt.Printf("Data Directory: %s\n", app.config.Raft.DataDir)
	fmt.Printf("Port: %d\n", app.config.Raft.Port)
	fmt.Printf("Auto-Discover: %v\n", app.config.Raft.AutoDiscover)
}

// InstallService installs the GoProxLB service as a systemd service.
func InstallService(user, group, configPath string, enableService bool) error {
	serviceName := "goproxlb"
	serviceDescription := "GoProxLB Load Balancer"
	
	// Check if we're running as root (required for systemd installation)
	if os.Geteuid() != 0 {
		fmt.Println("⚠️  Warning: This command requires root privileges to install systemd services.")
		fmt.Println("   Running in dry-run mode to show what would be installed.")
		fmt.Println("   Run with 'sudo' to perform actual installation.")
		fmt.Println()
		return installServiceDryRun(user, group, configPath, enableService)
	}
	
	// Determine executable path
	execPath := os.Args[0]
	if !filepath.IsAbs(execPath) {
		// If relative path, try to find the absolute path
		if absPath, err := exec.LookPath(execPath); err == nil {
			execPath = absPath
		}
	}
	
	// Build service command
	var serviceExec string
	if configPath != "" {
		serviceExec = fmt.Sprintf("%s --config %s", execPath, configPath)
	} else {
		serviceExec = execPath
	}

	// Create the service file content
	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=/var/lib/goproxlb
ExecStart=%s
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=goproxlb

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/goproxlb

[Install]
WantedBy=multi-user.target
`, serviceDescription, user, group, serviceExec)

	// Define the service file path
	serviceFilePath := "/etc/systemd/system/" + serviceName + ".service"

	// Create required directories
	dirs := []string{
		"/var/lib/goproxlb",
		"/etc/goproxlb",
		"/var/log/goproxlb",
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create user and group if they don't exist
	createUserAndGroup(user, group)

	// Write the service file
	//nolint:gosec // Systemd service files need to be readable by systemd (0644 is correct)
	if err := os.WriteFile(serviceFilePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file %s: %w", serviceFilePath, err)
	}

	// Set proper ownership
	setOwnership(user, group, dirs)

	// Reload systemd daemon
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	// Enable and start service if requested
	if enableService {
		// Enable service
		if err := exec.Command("systemctl", "enable", serviceName).Run(); err != nil {
			return fmt.Errorf("failed to enable service: %w", err)
		}
		
		// Start service
		if err := exec.Command("systemctl", "start", serviceName).Run(); err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}
		
		fmt.Printf("✅ Service enabled and started successfully.\n")
	}

	fmt.Printf("✅ Service file %s created successfully.\n", serviceFilePath)
	fmt.Printf("✅ User '%s' and group '%s' created.\n", user, group)
	fmt.Printf("✅ Directories created with proper permissions.\n")
	
	if !enableService {
		fmt.Printf("\n📋 Next steps:\n")
		fmt.Printf("1. Enable service: sudo systemctl enable %s\n", serviceName)
		fmt.Printf("2. Start service: sudo systemctl start %s\n", serviceName)
		fmt.Printf("3. Check status: sudo systemctl status %s\n", serviceName)
		fmt.Printf("4. View logs: sudo journalctl -u %s -f\n", serviceName)
	} else {
		fmt.Printf("\n📋 Service is now running:\n")
		fmt.Printf("1. Check status: sudo systemctl status %s\n", serviceName)
		fmt.Printf("2. View logs: sudo journalctl -u %s -f\n", serviceName)
		fmt.Printf("3. Stop service: sudo systemctl stop %s\n", serviceName)
	}

	return nil
}

// installServiceDryRun shows what would be installed without actually doing it.
func installServiceDryRun(user, group, configPath string, enableService bool) error {
	serviceName := "goproxlb"
	serviceDescription := "GoProxLB Load Balancer"
	
	// Determine executable path
	execPath := os.Args[0]
	if !filepath.IsAbs(execPath) {
		// If relative path, try to find the absolute path
		if absPath, err := exec.LookPath(execPath); err == nil {
			execPath = absPath
		}
	}
	
	// Build service command
	var serviceExec string
	if configPath != "" {
		serviceExec = fmt.Sprintf("%s --config %s", execPath, configPath)
	} else {
		serviceExec = execPath
	}

	// Create the service file content
	serviceContent := fmt.Sprintf(`[Unit]
Description=%s
After=network.target
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=/var/lib/goproxlb
ExecStart=%s
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=goproxlb

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/goproxlb

[Install]
WantedBy=multi-user.target
`, serviceDescription, user, group, serviceExec)

	fmt.Println("🔍 DRY-RUN MODE - What would be installed:")
	fmt.Println()
	fmt.Printf("📁 Directories to create:\n")
	fmt.Printf("   /var/lib/goproxlb\n")
	fmt.Printf("   /etc/goproxlb\n")
	fmt.Printf("   /var/log/goproxlb\n")
	fmt.Println()
	fmt.Printf("👤 User/Group to create:\n")
	fmt.Printf("   User: %s\n", user)
	fmt.Printf("   Group: %s\n", group)
	fmt.Println()
	fmt.Printf("📄 Service file to create: /etc/systemd/system/%s.service\n", serviceName)
	fmt.Println()
	fmt.Printf("⚙️  Service configuration:\n")
	fmt.Printf("   Executable: %s\n", execPath)
	fmt.Printf("   Command: %s\n", serviceExec)
	fmt.Printf("   User: %s\n", user)
	fmt.Printf("   Group: %s\n", group)
	fmt.Println()
	fmt.Printf("📋 Service file content:\n")
	fmt.Printf("---\n%s---\n", serviceContent)
	fmt.Println()
	fmt.Printf("🚀 To install for real, run: sudo ./goproxlb install --config %s\n", configPath)
	if enableService {
		fmt.Printf("🚀 To install and start automatically, run: sudo ./goproxlb install --config %s --enable\n", configPath)
	}

	return nil
}

// createUserAndGroup creates the specified user and group if they don't exist.
func createUserAndGroup(user, group string) {
	// Check if group exists
	if _, err := exec.LookPath("groupadd"); err == nil {
		cmd := exec.Command("groupadd", "-r", group)
		if err := cmd.Run(); err != nil {
			// Group might already exist, which is fine
			_ = err // Suppress unused variable warning
		}
	}

	// Check if user exists
	if _, err := exec.LookPath("useradd"); err == nil {
		cmd := exec.Command("useradd", "-r", "-g", group, "-d", "/var/lib/goproxlb", "-s", "/bin/false", user)
		if err := cmd.Run(); err != nil {
			// User might already exist, which is fine
			_ = err // Suppress unused variable warning
		}
	}
}

// setOwnership sets the ownership of directories to the specified user and group.
func setOwnership(user, group string, dirs []string) {
	for _, dir := range dirs {
		//nolint:gosec // This is a legitimate system administration task
		cmd := exec.Command("chown", user+":"+group, dir)
		if err := cmd.Run(); err != nil {
			// Ignore ownership errors, might not have permissions
			_ = err // Suppress unused variable warning
		}
	}
}

// capacityPlanningContext holds the context for capacity planning analysis.
type capacityPlanningContext struct {
	cfg              *config.Config
	client           ClientInterface
	balancer         BalancerInterface
	nodes            []models.Node
	forecastDuration time.Duration
	csvData          [][]string
	csvOutput        string
}

// setupCapacityPlanningContext initializes the context for capacity planning.
func setupCapacityPlanningContext(configPath, forecast, csvOutput string) (*capacityPlanningContext, error) {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create Proxmox client
	client := proxmox.NewClient(&cfg.Proxmox)

	// Get cluster information
	nodes, err := client.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Create advanced balancer for capacity analysis
	balancerInstance := balancer.NewAdvancedBalancer(client, cfg)

	// Parse forecast period
	forecastDuration := parseForecastDuration(forecast)

	// Prepare CSV data if output is requested
	var csvData [][]string
	if csvOutput != "" {
		csvData = [][]string{{
			"Type", "Name", "ID", "Status", "WorkloadType", "CurrentCPU%", "CurrentMemory%", "CurrentStorage%",
			"P90CPU%", "P95CPU%", "P99CPU%", "PredictedCPU%", "PredictedMemory%", "CurrentCPUCores", "CurrentMemoryGB",
			"RecommendedCPUCores", "RecommendedMemoryGB", "Criticality", "Pattern", "Recommendations",
		}}
	}

	return &capacityPlanningContext{
		cfg:              cfg,
		client:           client,
		balancer:         balancerInstance,
		nodes:            nodes,
		forecastDuration: forecastDuration,
		csvData:          csvData,
		csvOutput:        csvOutput,
	}, nil
}

// parseForecastDuration parses the forecast string into a duration.
func parseForecastDuration(forecast string) time.Duration {
	forecastDuration, err := time.ParseDuration(forecast)
	if err != nil {
		// Try parsing as weeks/months
		if strings.HasSuffix(forecast, "w") {
			weeks, _ := strconv.Atoi(strings.TrimSuffix(forecast, "w")) //nolint:errcheck // default value used on error
			forecastDuration = time.Duration(weeks) * 7 * 24 * time.Hour
		} else if strings.HasSuffix(forecast, "m") {
			months, _ := strconv.Atoi(strings.TrimSuffix(forecast, "m")) //nolint:errcheck // default value used on error
			forecastDuration = time.Duration(months) * 30 * 24 * time.Hour
		} else {
			forecastDuration = 7 * 24 * time.Hour // Default to 1 week
		}
	}
	return forecastDuration
}

// printCapacityPlanningHeader prints the analysis header.
func printCapacityPlanningHeader(forecastDuration time.Duration) {
	fmt.Printf("🔍 Capacity Planning Analysis\n")
	fmt.Printf("============================\n")
	fmt.Printf("Forecast Period: %s\n", forecastDuration.String())
	fmt.Printf("Analysis Date: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
}

// analyzeNodesForCapacityPlanning analyzes all nodes and returns adaptation recommendations.
func analyzeNodesForCapacityPlanning(context *capacityPlanningContext, detailed bool) []string {
	var adaptationRecommendations []string
	recommendationCounter := 1

	// Analyze each node
	for i := range context.nodes {
		node := &context.nodes[i]
		fmt.Printf("📊 Node: %s\n", node.Name)
		fmt.Printf("   Status: %s\n", node.Status)

		nodeRecommendations := analyzeNodeCapacity(context, node, &recommendationCounter, detailed)
		adaptationRecommendations = append(adaptationRecommendations, nodeRecommendations...)

		// Analyze VMs on this node
		vmRecommendations := analyzeNodeVMs(context, node, &recommendationCounter, detailed)
		adaptationRecommendations = append(adaptationRecommendations, vmRecommendations...)

		fmt.Println()
	}

	return adaptationRecommendations
}

// analyzeNodeCapacity analyzes a single node's capacity and returns recommendations.
func analyzeNodeCapacity(context *capacityPlanningContext, node *models.Node, recommendationCounter *int, detailed bool) []string {
	var recommendations []string

	// Get capacity metrics (only available on AdvancedBalancer)
	advancedBalancer, ok := context.balancer.(*balancer.AdvancedBalancer)
	if !ok {
		fmt.Printf("   Current CPU: %.1f%% | Memory: %.1f%% | Storage: %.1f%%\n",
			node.CPU.Usage, node.Memory.Usage, node.Storage.Usage)
		fmt.Printf("   ⚠️  Advanced capacity planning requires advanced balancer\n")
		return recommendations
	}

	metrics, hasMetrics := advancedBalancer.GetCapacityMetrics(node.Name)
	if hasMetrics {
		fmt.Printf("   Current CPU: %.1f%% | Memory: %.1f%% | Storage: %.1f%%\n",
			node.CPU.Usage, node.Memory.Usage, node.Storage.Usage)
		fmt.Printf("   P90 CPU: %.1f%% | P95 CPU: %.1f%% | P99 CPU: %.1f%%\n",
			metrics.P90, metrics.P95, metrics.P99)

		// Predict evolution
		predictedCPU := advancedBalancer.PredictResourceEvolution(node.Name, "cpu", context.forecastDuration)
		predictedMemory := advancedBalancer.PredictResourceEvolution(node.Name, "memory", context.forecastDuration)

		fmt.Printf("   Predicted CPU (%s): %.1f%% | Memory: %.1f%%\n",
			context.forecastDuration.String(), predictedCPU, predictedMemory)

		// Generate node adaptation recommendations
		recommendations = append(recommendations, generateNodeRecommendations(node, float32(predictedCPU), float32(predictedMemory), recommendationCounter)...)

		// Get and display recommendations
		resourceRecommendations := advancedBalancer.GetResourceRecommendations(node.Name, detailed)
		fmt.Printf("   Recommendations:\n")
		for _, rec := range resourceRecommendations {
			fmt.Printf("     • %s\n", rec)
		}

		// Add node data to CSV
		addNodeToCSV(context, node, metrics, float32(predictedCPU), float32(predictedMemory), resourceRecommendations)
	} else {
		fmt.Printf("   Current CPU: %.1f%% | Memory: %.1f%% | Storage: %.1f%%\n",
			node.CPU.Usage, node.Memory.Usage, node.Storage.Usage)
		fmt.Printf("   ⚠️  No historical data available for capacity planning\n")

		// Add node data to CSV (without historical metrics)
		addNodeToCSVWithoutMetrics(context, node)
	}

	return recommendations
}

// generateNodeRecommendations generates CPU and memory recommendations for a node.
func generateNodeRecommendations(node *models.Node, predictedCPU, predictedMemory float32, recommendationCounter *int) []string {
	var recommendations []string

	if predictedCPU > 90 {
		currentCores := node.CPU.Cores
		recommendedCores := int(float64(currentCores) * (float64(predictedCPU) / 80.0)) // Target 80% usage
		if recommendedCores > currentCores {
			recommendations = append(recommendations,
				fmt.Sprintf("%d. Node %s: Increase CPU from %d to %d cores",
					*recommendationCounter, node.Name, currentCores, recommendedCores))
			*recommendationCounter++
		}
	}

	if predictedMemory > 90 {
		currentMemoryGB := float64(node.Memory.Total) / 1024 / 1024 / 1024
		recommendedMemoryGB := currentMemoryGB * (float64(predictedMemory) / 80.0) // Target 80% usage
		if recommendedMemoryGB > currentMemoryGB {
			recommendations = append(recommendations,
				fmt.Sprintf("%d. Node %s: Increase memory from %.1f to %.1f GB",
					*recommendationCounter, node.Name, currentMemoryGB, recommendedMemoryGB))
			*recommendationCounter++
		}
	}

	return recommendations
}

// analyzeNodeVMs analyzes VMs on a node and returns recommendations.
func analyzeNodeVMs(context *capacityPlanningContext, node *models.Node, recommendationCounter *int, detailed bool) []string {
	var recommendations []string

	if len(node.VMs) == 0 {
		return recommendations
	}

	fmt.Printf("   VMs (%d):\n", len(node.VMs))

	// Group VMs by workload type for cleaner output
	workloadGroups := make(map[string][]models.VM)

	// Get advanced balancer for VM profile analysis
	advancedBalancer, ok := context.balancer.(*balancer.AdvancedBalancer)
	if !ok {
		// Fall back to grouping all VMs as "Standard" if not using advanced balancer
		workloadGroups["Standard"] = append(workloadGroups["Standard"], node.VMs...)
	} else {
		for j := range node.VMs {
			vm := &node.VMs[j]
			vmProfile := advancedBalancer.AnalyzeVMProfile(vm, node.Name)
			workloadType := vmProfile.WorkloadType
			workloadGroups[workloadType] = append(workloadGroups[workloadType], *vm)
		}
	}

	// Show VMs grouped by workload type and generate VM adaptation recommendations
	for workloadType, vms := range workloadGroups {
		fmt.Printf("     %s (%d VMs):\n", workloadType, len(vms))
		for k := range vms {
			vm := &vms[k]
			vmRecommendations := analyzeVMCapacity(context, vm, node.Name, workloadType, recommendationCounter, detailed)
			recommendations = append(recommendations, vmRecommendations...)
		}
	}

	return recommendations
}

// analyzeVMCapacity analyzes a single VM's capacity and returns recommendations.
func analyzeVMCapacity(context *capacityPlanningContext, vm *models.VM, nodeName, workloadType string, recommendationCounter *int, detailed bool) []string {
	var recommendations []string

	// Get VM profile (only available with advanced balancer)
	var vmProfile balancer.VMProfile
	if advancedBalancer, ok := context.balancer.(*balancer.AdvancedBalancer); ok {
		vmProfile = advancedBalancer.AnalyzeVMProfile(vm, nodeName)
	} else {
		// Fallback profile for basic balancer
		vmProfile = balancer.VMProfile{
			WorkloadType:    workloadType,
			Pattern:         "Standard",
			Criticality:     "Normal",
			Recommendations: []string{"Using basic balancer - limited analysis available"},
		}
	}
	fmt.Printf("       🖥️  %s (ID: %d) - %s\n", vm.Name, vm.ID, vm.Status)

	// Generate VM-specific adaptation recommendations
	currentCPU := int(vm.CPU)
	currentMemoryGB := float64(vm.Memory) / 1024 / 1024 / 1024

	// Calculate recommended resources based on workload type
	recommendedCPU, recommendedMemoryGB := calculateVMRecommendations(currentCPU, currentMemoryGB, workloadType, vmProfile.Criticality)

	// Only add recommendation if there's a significant difference
	if recommendedCPU > currentCPU || recommendedMemoryGB > currentMemoryGB {
		recommendations = append(recommendations,
			fmt.Sprintf("%d. VM %s (%s): CPU %d→%d cores, Memory %.1f→%.1f GB",
				*recommendationCounter, vm.Name, workloadType,
				currentCPU, recommendedCPU, currentMemoryGB, recommendedMemoryGB))
		*recommendationCounter++
	}

	// Add VM data to CSV
	addVMToCSV(context, vm, workloadType, currentCPU, currentMemoryGB, recommendedCPU, recommendedMemoryGB, vmProfile)

	if detailed {
		fmt.Printf("         Pattern: %s | Criticality: %s\n", vmProfile.Pattern, vmProfile.Criticality)
		if len(vmProfile.Recommendations) > 0 {
			fmt.Printf("         Recommendations:\n")
			for _, rec := range vmProfile.Recommendations {
				fmt.Printf("           • %s\n", rec)
			}
		}
	}

	return recommendations
}

// calculateVMRecommendations calculates recommended CPU and memory for a VM.
func calculateVMRecommendations(currentCPU int, currentMemoryGB float64, workloadType, criticality string) (recommendedCPU int, recommendedMemoryGB float64) {
	switch workloadType {
	case "Burst":
		recommendedCPU = int(float64(currentCPU) * 1.4) // 40% more for burst
		recommendedMemoryGB = currentMemoryGB * 1.3     // 30% more for burst
	case "Sustained":
		recommendedCPU = int(float64(currentCPU) * 1.2) // 20% more for sustained
		recommendedMemoryGB = currentMemoryGB * 1.2     // 20% more for sustained
	case "Idle":
		recommendedCPU = int(float64(currentCPU) * 1.1) // 10% more for idle
		recommendedMemoryGB = currentMemoryGB * 1.1     // 10% more for idle
	default:
		recommendedCPU = int(float64(currentCPU) * 1.25) // 25% more default
		recommendedMemoryGB = currentMemoryGB * 1.25     // 25% more default
	}

	// Add priority adjustments
	if criticality == "Critical" {
		recommendedCPU = int(float64(recommendedCPU) * 1.2) // 20% more for critical
		recommendedMemoryGB *= 1.2                          // 20% more for critical
	}

	return recommendedCPU, recommendedMemoryGB
}

// displayCapacityPlanningResults displays the final capacity planning results.
func displayCapacityPlanningResults(context *capacityPlanningContext, adaptationRecommendations []string) error {
	// Show numbered adaptation recommendations
	if len(adaptationRecommendations) > 0 {
		fmt.Printf("🔧 Resource Adaptation Recommendations\n")
		fmt.Printf("=====================================\n")
		for _, rec := range adaptationRecommendations {
			fmt.Printf("%s\n", rec)
		}
		fmt.Println()
	} else {
		fmt.Printf("✅ No resource adaptations needed based on current analysis\n\n")
	}

	// Show cluster-wide recommendations
	fmt.Printf("🎯 Cluster-Wide Recommendations\n")
	fmt.Printf("===============================\n")

	var clusterRecommendations []string
	if advancedBalancer, ok := context.balancer.(*balancer.AdvancedBalancer); ok {
		clusterRecommendations = advancedBalancer.GetClusterRecommendations(context.forecastDuration)
	} else {
		clusterRecommendations = []string{
			"📊 Monitor resource distribution across nodes for optimal balance",
			"🔄 Regular capacity planning reviews recommended",
			"⚠️  Advanced cluster recommendations require advanced balancer",
		}
	}
	for _, rec := range clusterRecommendations {
		fmt.Printf("• %s\n", rec)
	}

	// Write CSV file if requested
	if context.csvOutput != "" {
		if err := writeCSVFile(context.csvOutput, context.csvData); err != nil {
			return fmt.Errorf("failed to write CSV file: %w", err)
		}
		fmt.Printf("📊 CSV report written to: %s\n", context.csvOutput)
	}

	return nil
}

// addNodeToCSV adds node data to CSV output.
func addNodeToCSV(context *capacityPlanningContext, node *models.Node, metrics interface{}, predictedCPU, predictedMemory float32, recommendations []string) {
	if context.csvOutput == "" {
		return
	}

	currentMemoryGB := float64(node.Memory.Total) / 1024 / 1024 / 1024
	recommendedCores := node.CPU.Cores
	recommendedMemoryGB := currentMemoryGB

	if predictedCPU > 90 {
		recommendedCores = int(float64(node.CPU.Cores) * (float64(predictedCPU) / 80.0))
	}
	if predictedMemory > 90 {
		recommendedMemoryGB = currentMemoryGB * (float64(predictedMemory) / 80.0)
	}

	// Extract metrics values (using interface{} for compatibility)
	p90, p95, p99 := "", "", ""
	if m, ok := metrics.(struct{ P90, P95, P99 float32 }); ok {
		p90 = fmt.Sprintf("%.1f", m.P90)
		p95 = fmt.Sprintf("%.1f", m.P95)
		p99 = fmt.Sprintf("%.1f", m.P99)
	}

	context.csvData = append(context.csvData, []string{
		"Node", node.Name, "", node.Status, "",
		fmt.Sprintf("%.1f", node.CPU.Usage), fmt.Sprintf("%.1f", node.Memory.Usage), fmt.Sprintf("%.1f", node.Storage.Usage),
		p90, p95, p99,
		fmt.Sprintf("%.1f", predictedCPU), fmt.Sprintf("%.1f", predictedMemory),
		fmt.Sprintf("%d", node.CPU.Cores), fmt.Sprintf("%.1f", currentMemoryGB),
		fmt.Sprintf("%d", recommendedCores), fmt.Sprintf("%.1f", recommendedMemoryGB),
		"", "", strings.Join(recommendations, "; "),
	})
}

// addNodeToCSVWithoutMetrics adds node data to CSV without historical metrics.
func addNodeToCSVWithoutMetrics(context *capacityPlanningContext, node *models.Node) {
	if context.csvOutput == "" {
		return
	}

	currentMemoryGB := float64(node.Memory.Total) / 1024 / 1024 / 1024
	context.csvData = append(context.csvData, []string{
		"Node", node.Name, "", node.Status, "",
		fmt.Sprintf("%.1f", node.CPU.Usage), fmt.Sprintf("%.1f", node.Memory.Usage), fmt.Sprintf("%.1f", node.Storage.Usage),
		"", "", "", "", "",
		fmt.Sprintf("%d", node.CPU.Cores), fmt.Sprintf("%.1f", currentMemoryGB),
		fmt.Sprintf("%d", node.CPU.Cores), fmt.Sprintf("%.1f", currentMemoryGB),
		"", "", "No historical data available",
	})
}

// addVMToCSV adds VM data to CSV output.
func addVMToCSV(context *capacityPlanningContext, vm *models.VM, workloadType string, currentCPU int, currentMemoryGB float64, recommendedCPU int, recommendedMemoryGB float64, vmProfile interface{}) {
	if context.csvOutput == "" {
		return
	}

	// Extract vmProfile values (using interface{} for compatibility)
	criticality, pattern, recommendations := "", "", ""
	if p, ok := vmProfile.(struct {
		Criticality, Pattern string
		Recommendations      []string
	}); ok {
		criticality = p.Criticality
		pattern = p.Pattern
		recommendations = strings.Join(p.Recommendations, "; ")
	}

	context.csvData = append(context.csvData, []string{
		"VM", vm.Name, fmt.Sprintf("%d", vm.ID), vm.Status, workloadType,
		fmt.Sprintf("%.1f", vm.CPU), fmt.Sprintf("%.1f", float64(vm.Memory)/1024/1024/1024), "",
		"", "", "", "", "",
		fmt.Sprintf("%d", currentCPU), fmt.Sprintf("%.1f", currentMemoryGB),
		fmt.Sprintf("%d", recommendedCPU), fmt.Sprintf("%.1f", recommendedMemoryGB),
		criticality, pattern, recommendations,
	})
}
