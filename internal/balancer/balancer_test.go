package balancer

import (
	"fmt"
	"testing"
	"time"

	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/cblomart/GoProxLB/internal/proxmox"
)

// Mock client for testing.
type mockClient struct {
	nodes []models.Node
	err   error

	// For advanced balancer tests
	historicalData   map[string][]proxmox.HistoricalMetric
	vmHistoricalData map[string][]proxmox.HistoricalMetric
}

func (m *mockClient) GetClusterInfo() (*models.Cluster, error) {
	return &models.Cluster{Name: "test-cluster"}, m.err
}

func (m *mockClient) GetNodes() ([]models.Node, error) {
	return m.nodes, m.err
}

func (m *mockClient) MigrateVM(vmID int, sourceNode, targetNode string) error {
	return m.err
}

func (m *mockClient) GetNodeHistoricalData(nodeName, timeframe string) ([]proxmox.HistoricalMetric, error) {
	return m.historicalData[nodeName], m.err
}

func (m *mockClient) GetVMHistoricalData(nodeName string, vmID int, vmType, timeframe string) ([]proxmox.HistoricalMetric, error) {
	return m.vmHistoricalData[fmt.Sprintf("%s-%d-%s-%s", nodeName, vmID, vmType, timeframe)], m.err
}

// Helper function to create test nodes.
func createTestNodes() []models.Node {
	return []models.Node{
		{
			Name:   "node1",
			Status: "online",
			CPU: models.CPUInfo{
				Cores: 8,
				Usage: 85.0, // Overloaded
			},
			Memory: models.MemoryInfo{
				Total: 8589934592, // 8GB
				Used:  6871947674, // ~6.4GB (75%)
				Usage: 75.0,
			},
			Storage: models.StorageInfo{
				Total: 10737418240, // 10GB
				Used:  8589934592,  // 8GB (80%)
				Usage: 80.0,
			},
			VMs: []models.VM{
				{
					ID:     100,
					Name:   "test-vm-1",
					Status: "running",
					Node:   "node1",
					Tags:   []string{"plb_affinity_web"},
				},
				{
					ID:     101,
					Name:   "test-vm-2",
					Status: "running",
					Node:   "node1",
					Tags:   []string{"plb_anti_affinity_ntp"},
				},
			},
		},
		{
			Name:   "node2",
			Status: "online",
			CPU: models.CPUInfo{
				Cores: 8,
				Usage: 30.0, // Underloaded
			},
			Memory: models.MemoryInfo{
				Total: 8589934592, // 8GB
				Used:  2147483648, // 2GB (25%)
				Usage: 25.0,
			},
			Storage: models.StorageInfo{
				Total: 10737418240, // 10GB
				Used:  2147483648,  // 2GB (20%)
				Usage: 20.0,
			},
			VMs: []models.VM{
				{
					ID:     102,
					Name:   "test-vm-3",
					Status: "running",
					Node:   "node2",
					Tags:   []string{"plb_affinity_web"},
				},
			},
		},
		{
			Name:   "node3",
			Status: "online",
			CPU: models.CPUInfo{
				Cores: 8,
				Usage: 20.0, // Very underloaded
			},
			Memory: models.MemoryInfo{
				Total: 8589934592, // 8GB
				Used:  1073741824, // 1GB (12.5%)
				Usage: 12.5,
			},
			Storage: models.StorageInfo{
				Total: 10737418240, // 10GB
				Used:  1073741824,  // 1GB (10%)
				Usage: 10.0,
			},
			VMs: []models.VM{},
		},
	}
}

// Helper function to create test config.
func createTestConfig() *config.Config {
	return &config.Config{
		Proxmox: config.ProxmoxConfig{
			Host:     "https://test-host:8006",
			Username: "test-user@pve",
			Password: "test-password",
			Insecure: true,
		},
		Cluster: config.ClusterConfig{
			Name:             "test-cluster",
			MaintenanceNodes: []string{},
		},
		Balancing: config.BalancingConfig{
			Interval: "5m",
			Thresholds: config.ResourceThresholds{
				CPU:     80,
				Memory:  85,
				Storage: 90,
			},
			Weights: config.ResourceWeights{
				CPU:     1.0,
				Memory:  1.0,
				Storage: 0.5,
			},
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

func TestNewBalancer(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{}

	balancer := NewBalancer(client, cfg)
	if balancer == nil {
		t.Fatal("Expected balancer to be created")
	}

	// Test balancer properties after nil check
	testBalancerProperties(t, balancer, cfg, client)
}

// testBalancerProperties tests that the balancer has the expected properties.
func testBalancerProperties(t *testing.T, balancer *Balancer, cfg *config.Config, client proxmox.ClientInterface) {
	if balancer.config != cfg {
		t.Error("Expected config to be set")
	}

	if balancer.client != client {
		t.Error("Expected client to be set")
	}
}

func TestFilterAvailableNodes(t *testing.T) {
	cfg := createTestConfig()
	cfg.Cluster.MaintenanceNodes = []string{"node2"}

	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	available := balancer.filterAvailableNodes(client.nodes)

	if len(available) != 2 {
		t.Errorf("Expected 2 available nodes, got %d", len(available))
	}

	// Check that node2 is filtered out
	for _, node := range available {
		if node.Name == "node2" {
			t.Error("Expected node2 to be filtered out")
		}
	}
}

func TestIsInMaintenance(t *testing.T) {
	cfg := createTestConfig()
	cfg.Cluster.MaintenanceNodes = []string{"node1", "node3"}

	client := &mockClient{}
	balancer := NewBalancer(client, cfg)

	if !balancer.isInMaintenance("node1") {
		t.Error("Expected node1 to be in maintenance")
	}

	if balancer.isInMaintenance("node2") {
		t.Error("Expected node2 to not be in maintenance")
	}

	if !balancer.isInMaintenance("node3") {
		t.Error("Expected node3 to be in maintenance")
	}
}

func TestNeedsBalancing(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	needs := balancer.needsBalancing(client.nodes)
	if !needs {
		t.Error("Expected balancing to be needed (node1 is overloaded)")
	}

	// Test with lower thresholds
	cfg.Balancing.Thresholds.CPU = 90
	cfg.Balancing.Thresholds.Memory = 90
	cfg.Balancing.Thresholds.Storage = 90

	needs = balancer.needsBalancing(client.nodes)
	if needs {
		t.Error("Expected balancing to not be needed with higher thresholds")
	}
}

func TestCalculateNodeScore(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{}
	balancer := NewBalancer(client, cfg)

	node := models.Node{
		Name: "test-node",
		CPU: models.CPUInfo{
			Cores: 8,
			Usage: 50.0, // 50%
		},
		Memory: models.MemoryInfo{
			Total: 8589934592,
			Used:  4294967296, // 50%
			Usage: 50.0,
		},
		Storage: models.StorageInfo{
			Total: 10737418240,
			Used:  5368709120, // 50%
			Usage: 50.0,
		},
	}

	score := balancer.calculateNodeScore(&node)

	// The score is calculated as: (cpu*weight + memory*weight + storage*weight) / total_weight
	// (0.5*1.0 + 0.5*1.0 + 0.5*0.5) / (1.0 + 1.0 + 0.5) = (0.5 + 0.5 + 0.25) / 2.5 = 1.25 / 2.5 = 0.5
	expectedScore := 0.5
	if score.Score != expectedScore {
		t.Errorf("Expected score %.1f, got %.1f", expectedScore, score.Score)
	}

	if score.Node != "test-node" {
		t.Errorf("Expected node name 'test-node', got %s", score.Node)
	}
}

func TestCalculateNodeScores(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	scores := balancer.calculateNodeScores(client.nodes)

	if len(scores) != 3 {
		t.Errorf("Expected 3 node scores, got %d", len(scores))
	}

	// Check that scores are sorted (lower is better)
	for i := 1; i < len(scores); i++ {
		if scores[i].Score < scores[i-1].Score {
			t.Errorf("Scores not sorted correctly: %.1f < %.1f", scores[i].Score, scores[i-1].Score)
		}
	}
}

func TestFindMigrations(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	// Process rules first
	allVMs := []models.VM{}
	for _, node := range client.nodes {
		allVMs = append(allVMs, node.VMs...)
	}
	_ = balancer.engine.ProcessVMs(allVMs)

	nodeScores := balancer.calculateNodeScores(client.nodes)
	migrations := balancer.findMigrations(client.nodes, nodeScores)

	// Should find migrations from overloaded node1 to underloaded nodes
	if len(migrations) == 0 {
		t.Error("Expected to find migrations")
	}

	// Check that migrations are from overloaded nodes
	for _, migration := range migrations {
		if migration.FromNode != "node1" {
			t.Errorf("Expected migration from node1, got %s", migration.FromNode)
		}

		if migration.ToNode == "node1" {
			t.Error("Expected migration to different node")
		}
	}
}

func TestFindBestTargetNode(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{}
	balancer := NewBalancer(client, cfg)

	vm := models.VM{
		ID:   100,
		Name: "test-vm",
		Node: "node1",
	}

	nodeScores := []models.NodeScore{
		{Node: "node1", Score: 85.0}, // Overloaded
		{Node: "node2", Score: 30.0}, // Good target
		{Node: "node3", Score: 20.0}, // Best target
	}

	target := balancer.findBestTargetNode(&vm, nodeScores)

	if target != "node2" {
		t.Errorf("Expected best target to be node2, got %s", target)
	}
}

func TestCalculateResourceGain(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{}
	balancer := NewBalancer(client, cfg)

	nodeScores := []models.NodeScore{
		{Node: "node1", Score: 85.0}, // Source (overloaded)
		{Node: "node2", Score: 30.0}, // Target (underloaded)
	}

	gain := balancer.calculateResourceGain("node1", "node2", nodeScores)

	if gain <= 0 {
		t.Errorf("Expected positive resource gain, got %.2f", gain)
	}
}

func TestRunWithNoBalancingNeeded(t *testing.T) {
	cfg := createTestConfig()
	// Set high thresholds so no balancing is needed
	cfg.Balancing.Thresholds.CPU = 90
	cfg.Balancing.Thresholds.Memory = 90
	cfg.Balancing.Thresholds.Storage = 90

	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	results, err := balancer.Run(false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected no migrations, got %d", len(results))
	}
}

func TestRunWithInsufficientNodes(t *testing.T) {
	cfg := createTestConfig()
	// Put all nodes in maintenance
	cfg.Cluster.MaintenanceNodes = []string{"node1", "node2", "node3"}

	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	_, err := balancer.Run(false)
	if err == nil {
		t.Fatal("Expected error for insufficient available nodes")
	}
}

func TestRunWithClientError(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{err: fmt.Errorf("API error")}
	balancer := NewBalancer(client, cfg)

	_, err := balancer.Run(false)
	if err == nil {
		t.Fatal("Expected error from client")
	}
}

func TestGetClusterStatus(t *testing.T) {
	cfg := createTestConfig()
	client := &mockClient{nodes: createTestNodes()}
	balancer := NewBalancer(client, cfg)

	status, err := balancer.GetClusterStatus()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if status == nil {
		t.Fatal("Expected cluster status")
	}

	// Test status properties after nil check
	testStatusProperties(t, status)
}

// testStatusProperties tests that the status has the expected properties.
func testStatusProperties(t *testing.T, status *models.ClusterStatus) {
	if status.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", status.TotalNodes)
	}

	if status.TotalVMs != 3 {
		t.Errorf("Expected 3 total VMs, got %d", status.TotalVMs)
	}
}

func TestAdvancedBalancerRun(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.LoadProfiles.Enabled = true
	config.Balancing.Capacity.Enabled = true

	balancer := NewAdvancedBalancer(client, config)

	results, err := balancer.Run(false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have migrations since node1 is overloaded
	if len(results) == 0 {
		t.Error("Expected migrations since node1 is overloaded")
	}
}

func TestAdvancedBalancerRunWithOverloadedNodes(t *testing.T) {
	// Create overloaded nodes
	overloadedNodes := []models.Node{
		{
			Name:   "node1",
			Status: "online",
			CPU: models.CPUInfo{
				Usage: 90.0, // Overloaded
			},
			Memory: models.MemoryInfo{
				Usage: 85.0, // Overloaded
			},
			Storage: models.StorageInfo{
				Usage: 80.0,
			},
			VMs: []models.VM{
				{
					ID:     100,
					Name:   "test-vm-1",
					Status: "running",
					Node:   "node1",
					CPU:    50.0,
					Memory: 1024 * 1024 * 1024,
					Tags:   []string{},
				},
			},
		},
		{
			Name:   "node2",
			Status: "online",
			CPU: models.CPUInfo{
				Usage: 30.0, // Underloaded
			},
			Memory: models.MemoryInfo{
				Usage: 25.0, // Underloaded
			},
			Storage: models.StorageInfo{
				Usage: 20.0,
			},
			VMs: []models.VM{},
		},
	}

	client := &mockClient{
		nodes: overloadedNodes,
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.LoadProfiles.Enabled = true
	config.Balancing.Capacity.Enabled = true

	balancer := NewAdvancedBalancer(client, config)

	results, err := balancer.Run(false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should have at least one migration
	if len(results) == 0 {
		t.Error("Expected at least one migration for overloaded nodes")
	}
}

func TestAdvancedBalancerLoadProfiling(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.LoadProfiles.Enabled = true

	balancer := NewAdvancedBalancer(client, config)

	// Test load profiling
	nodes := createTestNodes()
	balancer.updateLoadProfiles(nodes)

	// Check that load profiles were created
	if len(balancer.loadProfiles) == 0 {
		t.Error("Expected load profiles to be created")
	}
}

func TestAdvancedBalancerCapacityMetrics(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.Capacity.Enabled = true

	balancer := NewAdvancedBalancer(client, config)

	// Test capacity metrics
	nodes := createTestNodes()
	balancer.updateCapacityMetrics(nodes)

	// Check that capacity metrics were created
	if len(balancer.capacityMetrics) == 0 {
		t.Error("Expected capacity metrics to be created")
	}
}

func TestAdvancedBalancerHistoricalData(t *testing.T) {
	// Create client with historical data
	historicalData := map[string][]proxmox.HistoricalMetric{
		"node1-day": {
			{
				Timestamp: time.Now().Add(-1 * time.Hour),
				CPU:       80.0,
				Memory:    2048 * 1024 * 1024,
				LoadAvg:   2.5,
			},
			{
				Timestamp: time.Now(),
				CPU:       85.0,
				Memory:    2304 * 1024 * 1024,
				LoadAvg:   3.0,
			},
		},
	}
	vmHistoricalData := map[string][]proxmox.HistoricalMetric{
		"node1-100-qemu-day": {
			{
				Timestamp: time.Now().Add(-1 * time.Hour),
				CPU:       60.0,
				Memory:    1024 * 1024 * 1024,
				Disk:      20 * 1024 * 1024 * 1024,
			},
			{
				Timestamp: time.Now(),
				CPU:       70.0,
				Memory:    1280 * 1024 * 1024,
				Disk:      22 * 1024 * 1024 * 1024,
			},
		},
	}

	client := &mockClient{
		nodes:            createTestNodes(),
		err:              nil,
		historicalData:   historicalData,
		vmHistoricalData: vmHistoricalData,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.LoadProfiles.Enabled = true
	config.Balancing.Capacity.Enabled = true

	balancer := NewAdvancedBalancer(client, config)

	// Test with historical data
	nodes := createTestNodes()
	balancer.updateLoadProfiles(nodes)
	balancer.updateCapacityMetrics(nodes)

	// Check that historical data was used
	if len(balancer.loadProfiles) == 0 {
		t.Error("Expected load profiles to be created with historical data")
	}
	if len(balancer.capacityMetrics) == 0 {
		t.Error("Expected capacity metrics to be created with historical data")
	}
}

func TestAdvancedBalancerMigrationHistory(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"

	balancer := NewAdvancedBalancer(client, config)

	// Add some migration history
	history := models.MigrationHistory{
		VMID:      100,
		FromNode:  "node1",
		ToNode:    "node2",
		Timestamp: time.Now().Add(-30 * time.Minute),
		Reason:    "load_balancing",
	}
	balancer.migrationHistory = append(balancer.migrationHistory, history)

	// Test that migration history is maintained
	if len(balancer.migrationHistory) != 1 {
		t.Errorf("Expected 1 migration history entry, got %d", len(balancer.migrationHistory))
	}
}

func TestAdvancedBalancerStabilityScoring(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"

	balancer := NewAdvancedBalancer(client, config)

	// Add migration history for stability testing
	history := models.MigrationHistory{
		VMID:      100,
		FromNode:  "node1",
		ToNode:    "node2",
		Timestamp: time.Now().Add(-30 * time.Minute),
		Reason:    "load_balancing",
	}
	balancer.migrationHistory = append(balancer.migrationHistory, history)

	// Test stability scoring
	nodes := createTestNodes()
	scores := balancer.calculateAdvancedNodeScores(nodes)

	if len(scores) == 0 {
		t.Error("Expected node scores to be calculated")
	}

	// Check that stability is considered in scoring
	for _, score := range scores {
		if score.Score < 0 {
			t.Errorf("Expected positive score, got %f", score.Score)
		}
	}
}

func TestAdvancedBalancerAntiFlipFlop(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"

	balancer := NewAdvancedBalancer(client, config)

	// Add recent migration history to test anti-flip-flop
	history := models.MigrationHistory{
		VMID:      100,
		FromNode:  "node1",
		ToNode:    "node2",
		Timestamp: time.Now().Add(-10 * time.Minute), // Very recent
		Reason:    "load_balancing",
	}
	balancer.migrationHistory = append(balancer.migrationHistory, history)

	// Test that VM with recent migration cannot be migrated again
	vm := models.VM{
		ID:     100,
		Name:   "test-vm",
		Node:   "node2",
		Status: "running",
	}

	canMigrate := balancer.canMigrateVM(&vm, "node2")
	if canMigrate {
		t.Error("Expected VM with recent migration to be blocked from migrating")
	}
}

func TestAdvancedBalancerPercentileCalculation(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"

	balancer := NewAdvancedBalancer(client, config)

	// Test percentile calculation
	originalValues := []float32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	values := make([]float32, len(originalValues))
	copy(values, originalValues)
	metrics := balancer.calculatePercentiles(values)

	// Check that percentiles are calculated correctly
	// For 10 values with optimized calculation: P50=60, P90=90, P95=100, P99=100
	// The optimized calculation uses different rounding logic
	if metrics.P50 != 60 {
		t.Errorf("Expected P50 to be 60 (optimized), got %f", metrics.P50)
	}
	if metrics.P90 != 90 {
		t.Errorf("Expected P90 to be 90 (optimized), got %f", metrics.P90)
	}
	if metrics.P95 != 100 {
		t.Errorf("Expected P95 to be 100 (optimized), got %f", metrics.P95)
	}
	if metrics.P99 != 100 {
		t.Errorf("Expected P99 to be 100 (optimized), got %f", metrics.P99)
	}
}

func TestAdvancedBalancerResourceGainCalculation(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"

	balancer := NewAdvancedBalancer(client, config)

	// Create node scores for testing
	nodeScores := []models.NodeScore{
		{
			Node:  "node1",
			Score: 0.8, // High load
			CPU:   80.0,
		},
		{
			Node:  "node2",
			Score: 0.3, // Low load
			CPU:   30.0,
		},
	}

	// Test resource gain calculation
	gain := balancer.calculateResourceGain("node1", "node2", nodeScores)
	if gain <= 0 {
		t.Errorf("Expected positive resource gain, got %f", gain)
	}
}

func TestAdvancedBalancerGetClusterStatus(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"

	balancer := NewAdvancedBalancer(client, config)

	status, err := balancer.GetClusterStatus()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Check status fields
	if status.TotalNodes != 3 {
		t.Errorf("Expected 3 total nodes, got %d", status.TotalNodes)
	}
	if status.ActiveNodes != 3 {
		t.Errorf("Expected 3 active nodes, got %d", status.ActiveNodes)
	}
	if status.TotalVMs != 3 {
		t.Errorf("Expected 3 total VMs, got %d", status.TotalVMs)
	}
	if status.RunningVMs != 3 {
		t.Errorf("Expected 3 running VMs, got %d", status.RunningVMs)
	}
}

func TestAdvancedBalancerWithHistoricalDataError(t *testing.T) {
	// Create client that returns error for historical data
	client := &mockClient{
		nodes: createTestNodes(),
		err:   fmt.Errorf("historical data not available"),
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.LoadProfiles.Enabled = true
	config.Balancing.Capacity.Enabled = true

	balancer := NewAdvancedBalancer(client, config)

	// Test that it falls back gracefully when historical data is not available
	nodes := createTestNodes()
	balancer.updateLoadProfiles(nodes)
	balancer.updateCapacityMetrics(nodes)

	// Should still work with fallback to simplified analysis
	if len(balancer.loadProfiles) == 0 {
		t.Error("Expected load profiles to be created even with historical data error")
	}
}

func TestAdvancedBalancerAggressivenessConfig(t *testing.T) {
	client := &mockClient{
		nodes: createTestNodes(),
		err:   nil,
	}
	config := createTestConfig()
	config.Balancing.BalancerType = "advanced"
	config.Balancing.Aggressiveness = "high"

	_ = NewAdvancedBalancer(client, config) // Create balancer but don't use it

	// Test that aggressiveness config is applied
	aggConfig := config.GetAggressivenessConfig()
	if aggConfig.CooldownPeriod != 30*time.Minute {
		t.Errorf("Expected high aggressiveness cooldown to be 30 minutes, got %v", aggConfig.CooldownPeriod)
	}
	if aggConfig.MinImprovement != 5.0 {
		t.Errorf("Expected high aggressiveness min improvement to be 5.0, got %f", aggConfig.MinImprovement)
	}
}
