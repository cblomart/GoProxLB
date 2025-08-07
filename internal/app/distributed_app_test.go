package app

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cblomart/GoProxLB/internal/balancer"
	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/cblomart/GoProxLB/internal/proxmox"
)

// MockClient implements ClientInterface for testing
type MockDistributedClient struct {
	clusterInfo *models.Cluster
	nodes       []models.Node
	vms         []models.VM
	err         error
}

func (m *MockDistributedClient) GetClusterInfo() (*models.Cluster, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clusterInfo, nil
}

func (m *MockDistributedClient) GetNodes() ([]models.Node, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes, nil
}

func (m *MockDistributedClient) MigrateVM(vmID int, sourceNode, targetNode string) error {
	return m.err
}

func (m *MockDistributedClient) GetNodeHistoricalData(nodeName string, timeframe string) ([]proxmox.HistoricalMetric, error) {
	return nil, m.err
}

func (m *MockDistributedClient) GetVMHistoricalData(nodeName string, vmID int, vmType string, timeframe string) ([]proxmox.HistoricalMetric, error) {
	return nil, m.err
}

// MockBalancer implements BalancerInterface for testing
type MockDistributedBalancer struct {
	results []models.BalancingResult
	err     error
}

func (m *MockDistributedBalancer) Run(dryRun bool) ([]models.BalancingResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *MockDistributedBalancer) GetClusterStatus() (*models.ClusterStatus, error) {
	return &models.ClusterStatus{
		TotalNodes:       3,
		ActiveNodes:      3,
		TotalVMs:         10,
		RunningVMs:       8,
		AverageCPU:       60.0,
		AverageMemory:    70.0,
		AverageStorage:   50.0,
		BalancingEnabled: true,
	}, m.err
}

func (m *MockDistributedBalancer) GetCapacityMetrics(nodeName string) (*models.CapacityMetrics, error) {
	return &models.CapacityMetrics{
		P50:    60.0,
		P90:    80.0,
		P95:    85.0,
		P99:    90.0,
		Mean:   65.0,
		StdDev: 15.0,
	}, m.err
}

func (m *MockDistributedBalancer) PredictResourceEvolution(nodeName string, forecast string) (map[string]float64, error) {
	return map[string]float64{
		"cpu":     75.0,
		"memory":  80.0,
		"storage": 60.0,
	}, m.err
}

func (m *MockDistributedBalancer) GetResourceRecommendations(nodeName string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"cpu_cores":      4,
		"memory_gb":      16,
		"recommendation": "Increase resources",
	}, m.err
}

func (m *MockDistributedBalancer) AnalyzeVMProfile(vm models.VM) (*balancer.VMProfile, error) {
	return &balancer.VMProfile{
		WorkloadType:    "interactive",
		Pattern:         "steady",
		Criticality:     "normal",
		CPUBuffer:       50.0,
		MemoryBuffer:    50.0,
		Recommendations: []string{"Test recommendation"},
	}, m.err
}

func (m *MockDistributedBalancer) GetClusterRecommendations() (map[string]interface{}, error) {
	return map[string]interface{}{
		"total_nodes": 3,
		"recommendations": []string{
			"Add more CPU cores to node1",
			"Increase memory on node2",
		},
	}, m.err
}

// createTestDistributedApp creates a distributed app for testing with temporary directories
func createTestDistributedApp(t *testing.T, port int) (*DistributedApp, string) {
	tempDir := t.TempDir()
	configPath := tempDir + "/config.yaml"

	configContent := fmt.Sprintf(`
proxmox:
  host: "https://localhost:8006"
  insecure: true
  username: "test"
  password: "test"

raft:
  enabled: true
  node_id: "test-node"
  address: "127.0.0.1"
  port: %d
  data_dir: "%s/raft-data"
  auto_discover: false
  peers: []

balancing:
  balancer_type: "advanced"
  aggressiveness: "low"
`, port, tempDir)

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Use a shorter socket path for testing
	socketDir := "/tmp/goproxlb-test"
	app, err := NewDistributedAppWithSocketDir(configPath, socketDir)
	if err != nil {
		t.Fatalf("Failed to create distributed app: %v", err)
	}

	return app, tempDir
}

func TestNewDistributedApp(t *testing.T) {
	// Test creating distributed app with temporary socket directory
	app, _ := createTestDistributedApp(t, 7947)
	defer app.Stop()

	if app == nil {
		t.Fatal("Expected app but got nil")
	}

	if app.config == nil {
		t.Error("Expected config but got nil")
	}

	if app.client == nil {
		t.Error("Expected client but got nil")
	}

	if app.balancer == nil {
		t.Error("Expected balancer but got nil")
	}

	if app.raftNode == nil {
		t.Error("Expected raft node but got nil")
	}
}

func TestNewDistributedAppRaftDisabled(t *testing.T) {
	// Create temporary config file with Raft disabled
	tempDir := t.TempDir()
	configPath := tempDir + "/config.yaml"

	configContent := `
proxmox:
  host: "https://localhost:8006"
  insecure: true
  username: "test"
  password: "test"

raft:
  enabled: false

balancing:
  balancer_type: "advanced"
  aggressiveness: "low"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test creating distributed app with Raft disabled
	_, err = NewDistributedApp(configPath)
	if err == nil {
		t.Error("Expected error when Raft is disabled but got none")
	}
}

func TestNewDistributedAppMissingNodeID(t *testing.T) {
	// Create temporary config file with missing node ID
	tempDir := t.TempDir()
	configPath := tempDir + "/config.yaml"

	configContent := `
proxmox:
  host: "https://localhost:8006"
  insecure: true
  username: "test"
  password: "test"

raft:
  enabled: true
  address: "127.0.0.1"
  port: 7948
  data_dir: "` + tempDir + `/raft-data"
  auto_discover: false
  peers: []

balancing:
  balancer_type: "advanced"
  aggressiveness: "low"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test creating distributed app with missing node ID
	_, err = NewDistributedApp(configPath)
	if err == nil {
		t.Error("Expected error when node ID is missing but got none")
	}
}

func TestDistributedAppGetStatus(t *testing.T) {
	app, _ := createTestDistributedApp(t, 7949)
	defer app.Stop()

	// Test getting status
	status := app.GetStatus()
	if status == nil {
		t.Fatal("Expected status but got nil")
	}

	// Check required fields
	requiredFields := []string{"node_id", "address", "is_leader", "raft_state", "leader", "peers", "balancing_enabled"}
	for _, field := range requiredFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Status missing required field: %s", field)
		}
	}

	// Check specific values
	if status["node_id"] != "test-node" {
		t.Errorf("Expected node_id 'test-node', got %v", status["node_id"])
	}

	if status["address"] != "127.0.0.1" {
		t.Errorf("Expected address '127.0.0.1', got %v", status["address"])
	}

	if status["balancing_enabled"] != true {
		t.Errorf("Expected balancing_enabled true, got %v", status["balancing_enabled"])
	}
}

func TestDistributedAppRunBalancingCycle(t *testing.T) {
	app, _ := createTestDistributedApp(t, 7950)
	defer app.Stop()

	// Test running balancing cycle when not leader
	err := app.runBalancingCycle()
	if err == nil {
		t.Error("Expected error when not leader but got none")
	}

	// Set as leader and test again
	app.isLeader = true

	// Mock the balancer to return success
	app.balancer = &MockDistributedBalancer{
		results: []models.BalancingResult{
			{
				SourceNode:   "node1",
				TargetNode:   "node2",
				VM:           models.VM{ID: 1, Name: "test-vm"},
				Success:      true,
				ResourceGain: 10.5,
			},
		},
	}

	err = app.runBalancingCycle()
	if err != nil {
		t.Errorf("Unexpected error running balancing cycle: %v", err)
	}
}

func TestDistributedAppRunBalancingCycleError(t *testing.T) {
	app, _ := createTestDistributedApp(t, 7951)
	defer app.Stop()

	// Set as leader
	app.isLeader = true

	// Mock the balancer to return error
	app.balancer = &MockDistributedBalancer{
		err: fmt.Errorf("balancing failed"),
	}

	err := app.runBalancingCycle()
	if err == nil {
		t.Error("Expected error from balancer but got none")
	}
}

func TestDistributedAppStartBalancingLoop(t *testing.T) {
	app, _ := createTestDistributedApp(t, 7952)
	defer app.Stop()

	// Test starting balancing loop
	app.startBalancingLoop()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel to stop the loop
	app.cancel()
}

func TestDistributedAppStartBalancingLoopDisabled(t *testing.T) {
	app, _ := createTestDistributedApp(t, 7953)
	defer app.Stop()

	// Test starting balancing loop
	app.startBalancingLoop()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel to stop the loop
	app.cancel()
}

func TestDistributedAppStopBalancingLoop(t *testing.T) {
	app, _ := createTestDistributedApp(t, 7954)
	defer app.Stop()

	// Test stopping balancing loop
	app.stopBalancingLoop()

	// Should not panic or error
}
