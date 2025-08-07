package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cblomart/GoProxLB/internal/balancer"
	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/proxmox"
	"github.com/cblomart/GoProxLB/internal/raft"
)

// DistributedApp represents a distributed load balancer application with leader election
type DistributedApp struct {
	config   *config.Config
	client   ClientInterface
	balancer BalancerInterface
	raftNode *raft.RaftNode
	ctx      context.Context
	cancel   context.CancelFunc
	isLeader bool
	listener *net.UnixListener
}

// NewDistributedApp creates a new distributed load balancer application
func NewDistributedApp(configPath string) (*DistributedApp, error) {
	return NewDistributedAppWithSocketDir(configPath, "")
}

// NewDistributedAppWithSocketDir creates a new distributed load balancer application with custom socket directory
func NewDistributedAppWithSocketDir(configPath string, socketDir string) (*DistributedApp, error) {
	// Load configuration
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

	// Validate Raft configuration
	if !config.Raft.Enabled {
		return nil, fmt.Errorf("raft is not enabled in configuration")
	}

	if config.Raft.NodeID == "" {
		return nil, fmt.Errorf("raft node_id is required when raft is enabled")
	}

	// Create Proxmox client
	client := proxmox.NewClient(&config.Proxmox)

	// Create discovery service for auto-discovery
	discoveryService := proxmox.NewDiscoveryService(client, config.Raft.Port)

	// Auto-discover peers if enabled
	if config.Raft.AutoDiscover {
		fmt.Println("Auto-discovering Raft parameters from Proxmox cluster...")

		// Discover cluster nodes
		nodes, err := discoveryService.DiscoverClusterNodes()
		if err != nil {
			return nil, fmt.Errorf("failed to discover cluster nodes: %w", err)
		}
		fmt.Printf("Discovered %d cluster nodes: %v\n", len(nodes), func() []string {
			names := make([]string, len(nodes))
			for i, node := range nodes {
				names[i] = node.Name
			}
			return names
		}())

		// Get current node ID
		currentNodeID, err := discoveryService.GetCurrentNodeID()
		if err != nil {
			return nil, fmt.Errorf("failed to get current node ID: %w", err)
		}
		config.Raft.NodeID = currentNodeID
		fmt.Printf("Current node ID: %s\n", config.Raft.NodeID)

		// Get Raft peers
		peers, err := discoveryService.GetRaftPeers(config.Raft.NodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get Raft peers: %w", err)
		}
		config.Raft.Peers = peers
		fmt.Printf("Raft peers configured: %v\n", config.Raft.Peers)
	}

	// Create balancer
	var balancerInstance BalancerInterface
	if config.IsAdvancedBalancer() {
		balancerInstance = balancer.NewAdvancedBalancer(client, config)
	} else {
		balancerInstance = balancer.NewBalancer(client, config)
	}

	// Create Raft node
	fullAddress := fmt.Sprintf("%s:%d", config.Raft.Address, config.Raft.Port)
	raftNode, err := raft.NewRaftNode(config.Raft.NodeID, fullAddress, config.Raft.DataDir, config.Raft.Peers)
	if err != nil {
		return nil, fmt.Errorf("failed to create Raft node: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create Unix domain socket for status endpoint
	if socketDir == "" {
		socketDir = "/var/lib/goproxlb"
	}
	socketPath := socketDir + "/status.sock"

	// Remove existing socket file if it exists
	os.Remove(socketPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Create Unix domain socket listener
	addr := &net.UnixAddr{Name: socketPath, Net: "unix"}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Unix socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(socketPath, 0666); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	app := &DistributedApp{
		config:   config,
		client:   client,
		balancer: balancerInstance,
		raftNode: raftNode,
		ctx:      ctx,
		cancel:   cancel,
		isLeader: false,
		listener: listener,
	}

	return app, nil
}

// Start starts the distributed load balancer with leader election
func (d *DistributedApp) Start() error {
	fmt.Println("Starting GoProxLB in distributed mode...")
	fmt.Printf("Configuration loaded from: %s\n", "config.yaml")
	fmt.Printf("Proxmox host: %s\n", d.config.Proxmox.Host)
	fmt.Printf("Cluster: %s\n", d.config.Cluster.Name)
	fmt.Printf("Raft Node ID: %s\n", d.config.Raft.NodeID)
	fmt.Printf("Raft Address: %s\n", d.config.Raft.Address)
	fmt.Printf("Raft Peers: %v\n", d.config.Raft.Peers)
	fmt.Printf("Status socket: %s\n", d.listener.Addr())

	// Start Unix socket server in background
	go func() {
		for {
			conn, err := d.listener.Accept()
			if err != nil {
				if d.ctx.Err() != nil {
					// Context cancelled, shutting down
					return
				}
				fmt.Printf("Socket accept error: %v\n", err)
				continue
			}

			// Handle connection in goroutine
			go d.handleStatusRequest(conn)
		}
	}()

	// Start Raft node
	if err := d.raftNode.Start(); err != nil {
		return fmt.Errorf("failed to start raft node: %w", err)
	}

	// Wait for leader election
	fmt.Println("Waiting for leader election...")
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	if err := d.raftNode.WaitForLeader(ctx); err != nil {
		return fmt.Errorf("failed to elect leader: %w", err)
	}

	fmt.Printf("Leader elected: %s\n", d.raftNode.GetLeader())

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start leader monitoring
	go d.monitorLeadership()

	// Start balancing loop if enabled (always enabled when running)
	d.startBalancingLoop()

	fmt.Println("Distributed load balancer started. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	select {
	case <-d.ctx.Done():
		fmt.Println("Shutting down...")
	case <-sigChan:
		fmt.Println("\nReceived shutdown signal...")
		d.cancel()
	}

	return d.Stop()
}

// Stop stops the distributed application
func (d *DistributedApp) Stop() error {
	fmt.Println("Stopping distributed load balancer...")
	d.cancel()

	// Close Unix socket gracefully
	if d.listener != nil {
		d.listener.Close()
		// Remove socket file
		os.Remove("/var/lib/goproxlb/status.sock")
	}

	return d.raftNode.Stop()
}

// monitorLeadership monitors for leadership changes
func (d *DistributedApp) monitorLeadership() {
	leaderChan := d.raftNode.GetLeaderChan()

	for {
		select {
		case <-d.ctx.Done():
			return
		case isLeader := <-leaderChan:
			if isLeader && !d.isLeader {
				fmt.Println("🎉 This node is now the leader - starting load balancing...")
				d.isLeader = true
				d.startBalancingLoop()
			} else if !isLeader && d.isLeader {
				fmt.Println("📉 This node is no longer the leader - stopping load balancing...")
				d.isLeader = false
				d.stopBalancingLoop()
			}
		}
	}
}

// startBalancingLoop starts the load balancing loop
func (d *DistributedApp) startBalancingLoop() {
	// Balancing is always enabled when running

	// Get balancing interval
	interval, err := d.config.GetInterval()
	if err != nil {
		fmt.Printf("Error: invalid balancing interval: %v\n", err)
		return
	}

	fmt.Printf("Balancing interval: %v\n", interval)

	// Start balancing loop in a goroutine
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-d.ctx.Done():
				return
			case <-ticker.C:
				if d.isLeader {
					if err := d.runBalancingCycle(); err != nil {
						fmt.Printf("Error during balancing cycle: %v\n", err)
					}
				}
			}
		}
	}()
}

// stopBalancingLoop stops the load balancing loop
func (d *DistributedApp) stopBalancingLoop() {
	// The balancing loop will automatically stop when d.isLeader becomes false
	fmt.Println("Load balancing stopped (no longer leader)")
}

// runBalancingCycle runs a single balancing cycle
func (d *DistributedApp) runBalancingCycle() error {
	if !d.isLeader {
		return fmt.Errorf("not the leader, skipping balancing cycle")
	}

	fmt.Printf("[%s] Running balancing cycle (Leader: %s)...\n",
		time.Now().Format("2006-01-02 15:04:05"), d.config.Raft.NodeID)

	results, err := d.balancer.Run(false)
	if err != nil {
		return fmt.Errorf("balancing cycle failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No balancing actions needed")
		return nil
	}

	fmt.Printf("Executed %d migrations:\n", len(results))
	for _, result := range results {
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

// handleStatusRequest handles status requests from Unix socket clients
func (d *DistributedApp) handleStatusRequest(conn net.Conn) {
	defer conn.Close()

	// Get current status
	status := d.GetStatus()

	// Encode status as JSON
	statusData, err := json.Marshal(status)
	if err != nil {
		fmt.Printf("Error marshaling status: %v\n", err)
		return
	}

	// Send status response
	response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s", len(statusData), string(statusData))
	_, err = io.WriteString(conn, response)
	if err != nil {
		fmt.Printf("Error writing status response: %v\n", err)
	}
}

// GetStatus returns the current status of the distributed application
func (d *DistributedApp) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"node_id":           d.config.Raft.NodeID,
		"address":           d.config.Raft.Address,
		"is_leader":         d.isLeader,
		"raft_state":        d.raftNode.GetState().String(),
		"leader":            d.raftNode.GetLeader(),
		"peers":             d.raftNode.GetPeers(),
		"balancing_enabled": true, // Always enabled when running
	}
}
