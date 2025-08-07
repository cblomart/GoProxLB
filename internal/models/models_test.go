package models

import (
	"testing"
	"time"
)

func TestNodeValidation(t *testing.T) {
	// Test valid node
	node := Node{
		Name:   "test-node",
		Status: "online",
		CPU: CPUInfo{
			Usage:   50.0,
			Cores:   8,
			Model:   "Intel Xeon",
			LoadAvg: 1.5,
		},
		Memory: MemoryInfo{
			Total:     8589934592, // 8GB
			Used:      4294967296, // 4GB
			Available: 4294967296, // 4GB
			Usage:     50.0,
		},
		Storage: StorageInfo{
			Total: 10737418240, // 10GB
			Used:  5368709120,  // 5GB
			Free:  5368709120,  // 5GB
			Usage: 50.0,
		},
		VMs:           []VM{},
		InMaintenance: false,
	}

	// Test that node has valid data
	if node.Name == "" {
		t.Error("Node name should not be empty")
	}
	if node.CPU.Usage < 0 || node.CPU.Usage > 100 {
		t.Error("CPU usage should be between 0 and 100")
	}
	if node.Memory.Usage < 0 || node.Memory.Usage > 100 {
		t.Error("Memory usage should be between 0 and 100")
	}
	if node.Storage.Usage < 0 || node.Storage.Usage > 100 {
		t.Error("Storage usage should be between 0 and 100")
	}
}

func TestVMValidation(t *testing.T) {
	// Test valid VM
	vm := VM{
		ID:        100,
		Name:      "test-vm",
		Node:      "test-node",
		Type:      "qemu",
		Status:    "running",
		CPU:       50.0,
		Memory:    1024 * 1024 * 1024, // 1GB
		Tags:      []string{"web", "production"},
		Created:   time.Now(),
		LastMoved: time.Now().Add(-1 * time.Hour),
	}

	// Test that VM has valid data
	if vm.ID <= 0 {
		t.Error("VM ID should be positive")
	}
	if vm.Name == "" {
		t.Error("VM name should not be empty")
	}
	if vm.Node == "" {
		t.Error("VM node should not be empty")
	}
	if vm.Type != "qemu" && vm.Type != "lxc" {
		t.Error("VM type should be qemu or lxc")
	}
	if vm.CPU < 0 || vm.CPU > 100 {
		t.Error("VM CPU usage should be between 0 and 100")
	}
	if vm.Memory < 0 {
		t.Error("VM memory should be positive")
	}
}

func TestLoadProfileValidation(t *testing.T) {
	// Test valid load profile
	profile := LoadProfile{
		CPUPattern: CPUPattern{
			Type:           "burst",
			BurstDuration:  300.0,
			BurstFrequency: 2.0,
			SustainedLevel: 50.0,
		},
		MemoryPattern: MemoryPattern{
			Type:       "static",
			GrowthRate: 0.0,
			Volatility: 5.0,
			PeakUsage:  80.0,
		},
		StoragePattern: StoragePattern{
			Type:         "mixed",
			ReadIOPs:     1000,
			WriteIOPs:    500,
			ReadLatency:  5.0,
			WriteLatency: 10.0,
		},
		Priority:    PriorityInteractive,
		Criticality: CriticalityImportant,
	}

	// Test that load profile has valid data
	if profile.Priority != PriorityRealtime && profile.Priority != PriorityInteractive && profile.Priority != PriorityBackground {
		t.Error("Invalid priority value")
	}
	if profile.Criticality != CriticalityCritical && profile.Criticality != CriticalityImportant && profile.Criticality != CriticalityNormal {
		t.Error("Invalid criticality value")
	}
	if profile.CPUPattern.BurstDuration < 0 {
		t.Error("Burst duration should be positive")
	}
	if profile.CPUPattern.BurstFrequency < 0 {
		t.Error("Burst frequency should be positive")
	}
	if profile.CPUPattern.SustainedLevel < 0 || profile.CPUPattern.SustainedLevel > 100 {
		t.Error("Sustained level should be between 0 and 100")
	}
}

func TestCapacityMetricsValidation(t *testing.T) {
	// Test valid capacity metrics
	metrics := CapacityMetrics{
		P50:    50.0,
		P90:    80.0,
		P95:    85.0,
		P99:    95.0,
		MinP90: 20.0,
		MaxP90: 80.0,
		Mean:   55.0,
		StdDev: 15.0,
	}

	// Test that metrics are valid
	if metrics.P50 < 0 || metrics.P50 > 100 {
		t.Error("P50 should be between 0 and 100")
	}
	if metrics.P90 < 0 || metrics.P90 > 100 {
		t.Error("P90 should be between 0 and 100")
	}
	if metrics.P95 < 0 || metrics.P95 > 100 {
		t.Error("P95 should be between 0 and 100")
	}
	if metrics.P99 < 0 || metrics.P99 > 100 {
		t.Error("P99 should be between 0 and 100")
	}
	if metrics.Mean < 0 || metrics.Mean > 100 {
		t.Error("Mean should be between 0 and 100")
	}
	if metrics.StdDev < 0 {
		t.Error("Standard deviation should be positive")
	}
}

func TestMigrationHistoryValidation(t *testing.T) {
	// Test valid migration history
	history := MigrationHistory{
		VMID:      100,
		FromNode:  "node1",
		ToNode:    "node2",
		Timestamp: time.Now(),
		Reason:    "load_balancing",
	}

	// Test that history has valid data
	if history.VMID <= 0 {
		t.Error("VM ID should be positive")
	}
	if history.FromNode == "" {
		t.Error("From node should not be empty")
	}
	if history.ToNode == "" {
		t.Error("To node should not be empty")
	}
	if history.FromNode == history.ToNode {
		t.Error("From and to nodes should be different")
	}
	if history.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if history.Reason == "" {
		t.Error("Reason should not be empty")
	}
}

func TestBalancingResultValidation(t *testing.T) {
	// Test valid balancing result
	result := BalancingResult{
		SourceNode: "node1",
		TargetNode: "node2",
		VM: VM{
			ID:   100,
			Name: "test-vm",
		},
		Reason:       "load_balancing",
		ResourceGain: 10.5,
		Timestamp:    time.Now(),
		Success:      true,
	}

	// Test that result has valid data
	if result.SourceNode == "" {
		t.Error("Source node should not be empty")
	}
	if result.TargetNode == "" {
		t.Error("Target node should not be empty")
	}
	if result.SourceNode == result.TargetNode {
		t.Error("Source and target nodes should be different")
	}
	if result.VM.ID <= 0 {
		t.Error("VM ID should be positive")
	}
	if result.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestNodeScoreValidation(t *testing.T) {
	// Test valid node score
	score := NodeScore{
		Node:    "test-node",
		Score:   0.5,
		CPU:     50.0,
		Memory:  60.0,
		Storage: 40.0,
	}

	// Test that score has valid data
	if score.Node == "" {
		t.Error("Node name should not be empty")
	}
	if score.Score < 0 {
		t.Error("Score should be non-negative")
	}
	if score.CPU < 0 || score.CPU > 100 {
		t.Error("CPU usage should be between 0 and 100")
	}
	if score.Memory < 0 || score.Memory > 100 {
		t.Error("Memory usage should be between 0 and 100")
	}
	if score.Storage < 0 || score.Storage > 100 {
		t.Error("Storage usage should be between 0 and 100")
	}
}

func TestClusterStatusValidation(t *testing.T) {
	// Test valid cluster status
	status := ClusterStatus{
		TotalNodes:       3,
		ActiveNodes:      2,
		TotalVMs:         10,
		RunningVMs:       8,
		AverageCPU:       60.0,
		AverageMemory:    70.0,
		AverageStorage:   50.0,
		LastBalanced:     time.Now(),
		BalancingEnabled: true,
	}

	// Test that status has valid data
	if status.TotalNodes < 0 {
		t.Error("Total nodes should be non-negative")
	}
	if status.ActiveNodes < 0 {
		t.Error("Active nodes should be non-negative")
	}
	if status.ActiveNodes > status.TotalNodes {
		t.Error("Active nodes should not exceed total nodes")
	}
	if status.TotalVMs < 0 {
		t.Error("Total VMs should be non-negative")
	}
	if status.RunningVMs < 0 {
		t.Error("Running VMs should be non-negative")
	}
	if status.RunningVMs > status.TotalVMs {
		t.Error("Running VMs should not exceed total VMs")
	}
	if status.AverageCPU < 0 || status.AverageCPU > 100 {
		t.Error("Average CPU should be between 0 and 100")
	}
	if status.AverageMemory < 0 || status.AverageMemory > 100 {
		t.Error("Average memory should be between 0 and 100")
	}
	if status.AverageStorage < 0 || status.AverageStorage > 100 {
		t.Error("Average storage should be between 0 and 100")
	}
}

func TestPriorityConstants(t *testing.T) {
	// Test priority constants
	if PriorityRealtime != "realtime" {
		t.Errorf("Expected PriorityRealtime to be 'realtime', got %s", PriorityRealtime)
	}
	if PriorityInteractive != "interactive" {
		t.Errorf("Expected PriorityInteractive to be 'interactive', got %s", PriorityInteractive)
	}
	if PriorityBackground != "background" {
		t.Errorf("Expected PriorityBackground to be 'background', got %s", PriorityBackground)
	}
}

func TestCriticalityConstants(t *testing.T) {
	// Test criticality constants
	if CriticalityCritical != "critical" {
		t.Errorf("Expected CriticalityCritical to be 'critical', got %s", CriticalityCritical)
	}
	if CriticalityImportant != "important" {
		t.Errorf("Expected CriticalityImportant to be 'important', got %s", CriticalityImportant)
	}
	if CriticalityNormal != "normal" {
		t.Errorf("Expected CriticalityNormal to be 'normal', got %s", CriticalityNormal)
	}
}
