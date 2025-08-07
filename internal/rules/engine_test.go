package rules

import (
	"testing"

	"github.com/cblomart/GoProxLB/internal/models"
)

func TestProcessVMs(t *testing.T) {
	engine := NewEngine()

	vms := []models.VM{
		{
			ID:   1,
			Name: "vm1",
			Node: "node1",
			Tags: []string{"plb_affinity_web", "plb_ignore_dev"},
		},
		{
			ID:   2,
			Name: "vm2",
			Node: "node2",
			Tags: []string{"plb_affinity_web", "plb_anti_affinity_ntp"},
		},
		{
			ID:   3,
			Name: "vm3",
			Node: "node1",
			Tags: []string{"plb_anti_affinity_ntp", "plb_pin_node1"},
		},
		{
			ID:   4,
			Name: "vm4",
			Node: "node3",
			Tags: []string{"plb_pin_node1", "plb_pin_node2"},
		},
	}

	err := engine.ProcessVMs(vms)
	if err != nil {
		t.Fatalf("Failed to process VMs: %v", err)
	}

	// Test affinity groups
	affinityGroups := engine.GetAffinityGroups()
	if len(affinityGroups) != 1 {
		t.Errorf("Expected 1 affinity group, got %d", len(affinityGroups))
	}

	webGroup, exists := affinityGroups["web"]
	if !exists {
		t.Error("Expected affinity group 'web' not found")
	} else {
		if len(webGroup.VMs) != 2 {
			t.Errorf("Expected 2 VMs in web affinity group, got %d", len(webGroup.VMs))
		}
	}

	// Test anti-affinity groups
	antiAffinityGroups := engine.GetAntiAffinityGroups()
	if len(antiAffinityGroups) != 1 {
		t.Errorf("Expected 1 anti-affinity group, got %d", len(antiAffinityGroups))
	}

	ntpGroup, exists := antiAffinityGroups["ntp"]
	if !exists {
		t.Error("Expected anti-affinity group 'ntp' not found")
	} else {
		if len(ntpGroup.VMs) != 2 {
			t.Errorf("Expected 2 VMs in ntp anti-affinity group, got %d", len(ntpGroup.VMs))
		}
	}

	// Test pinned VMs
	pinnedVMs := engine.GetPinnedVMs()
	if len(pinnedVMs) != 2 {
		t.Errorf("Expected 2 pinned VMs, got %d", len(pinnedVMs))
	}

	// Test ignored VMs
	ignoredVMs := engine.GetIgnoredVMs()
	if len(ignoredVMs) != 1 {
		t.Errorf("Expected 1 ignored VM, got %d", len(ignoredVMs))
	}
}

func TestIsIgnored(t *testing.T) {
	engine := NewEngine()

	vms := []models.VM{
		{
			ID:   1,
			Name: "vm1",
			Tags: []string{"plb_ignore_dev"},
		},
		{
			ID:   2,
			Name: "vm2",
			Tags: []string{"plb_ignore_test"},
		},
		{
			ID:   3,
			Name: "vm3",
			Tags: []string{},
		},
	}

	err := engine.ProcessVMs(vms)
	if err != nil {
		t.Fatalf("Failed to process VMs: %v", err)
	}

	if !engine.IsIgnored(1) {
		t.Error("VM 1 should be ignored")
	}
	if !engine.IsIgnored(2) {
		t.Error("VM 2 should be ignored")
	}
	if engine.IsIgnored(3) {
		t.Error("VM 3 should not be ignored")
	}
}

func TestIsPinned(t *testing.T) {
	engine := NewEngine()

	vms := []models.VM{
		{
			ID:   1,
			Name: "vm1",
			Tags: []string{"plb_pin_node1"},
		},
		{
			ID:   2,
			Name: "vm2",
			Tags: []string{"plb_pin_node1", "plb_pin_node2"},
		},
		{
			ID:   3,
			Name: "vm3",
			Tags: []string{},
		},
	}

	err := engine.ProcessVMs(vms)
	if err != nil {
		t.Fatalf("Failed to process VMs: %v", err)
	}

	if !engine.IsPinned(1) {
		t.Error("VM 1 should be pinned")
	}
	if !engine.IsPinned(2) {
		t.Error("VM 2 should be pinned")
	}
	if engine.IsPinned(3) {
		t.Error("VM 3 should not be pinned")
	}

	// Test pinned nodes
	pinnedNodes := engine.GetPinnedNodes(1)
	if len(pinnedNodes) != 1 || pinnedNodes[0] != "node1" {
		t.Errorf("Expected VM 1 to be pinned to ['node1'], got %v", pinnedNodes)
	}

	pinnedNodes = engine.GetPinnedNodes(2)
	if len(pinnedNodes) != 2 {
		t.Errorf("Expected VM 2 to be pinned to 2 nodes, got %d", len(pinnedNodes))
	}
}

func TestValidatePlacement(t *testing.T) {
	engine := NewEngine()

	vms := []models.VM{
		{
			ID:   1,
			Name: "vm1",
			Node: "node1",
			Tags: []string{"plb_ignore_dev"},
		},
		{
			ID:   2,
			Name: "vm2",
			Node: "node1",
			Tags: []string{"plb_pin_node1"},
		},
		{
			ID:   3,
			Name: "vm3",
			Node: "node1",
			Tags: []string{"plb_affinity_web"},
		},
		{
			ID:   4,
			Name: "vm4",
			Node: "node2",
			Tags: []string{"plb_affinity_web"},
		},
		{
			ID:   5,
			Name: "vm5",
			Node: "node1",
			Tags: []string{"plb_anti_affinity_ntp"},
		},
		{
			ID:   6,
			Name: "vm6",
			Node: "node2",
			Tags: []string{"plb_anti_affinity_ntp"},
		},
	}

	err := engine.ProcessVMs(vms)
	if err != nil {
		t.Fatalf("Failed to process VMs: %v", err)
	}

	tests := []struct {
		name       string
		vm         models.VM
		targetNode string
		wantErr    bool
	}{
		{
			name:       "ignored VM",
			vm:         vms[0],
			targetNode: "node2",
			wantErr:    true,
		},
		{
			name:       "pinned VM to allowed node",
			vm:         vms[1],
			targetNode: "node1",
			wantErr:    false,
		},
		{
			name:       "pinned VM to disallowed node",
			vm:         vms[1],
			targetNode: "node2",
			wantErr:    true,
		},
		{
			name:       "affinity VM to node with affinity group",
			vm:         vms[2],
			targetNode: "node2",
			wantErr:    false,
		},
		{
			name:       "affinity VM to node without affinity group",
			vm:         vms[2],
			targetNode: "node3",
			wantErr:    true,
		},
		{
			name:       "anti-affinity VM to node with anti-affinity group",
			vm:         vms[4],
			targetNode: "node2",
			wantErr:    true,
		},
		{
			name:       "anti-affinity VM to node without anti-affinity group",
			vm:         vms[4],
			targetNode: "node3",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.ValidatePlacement(&tt.vm, tt.targetNode)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePlacement() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetValidTargetNodes(t *testing.T) {
	engine := NewEngine()

	vms := []models.VM{
		{
			ID:   1,
			Name: "vm1",
			Node: "node1",
			Tags: []string{"plb_pin_node1", "plb_pin_node2"},
		},
		{
			ID:   2,
			Name: "vm2",
			Node: "node1",
			Tags: []string{"plb_ignore_dev"},
		},
	}

	err := engine.ProcessVMs(vms)
	if err != nil {
		t.Fatalf("Failed to process VMs: %v", err)
	}

	availableNodes := []string{"node1", "node2", "node3"}

	// Test pinned VM
	validNodes := engine.GetValidTargetNodes(&vms[0], availableNodes)
	expected := []string{"node1", "node2"}
	if len(validNodes) != len(expected) {
		t.Errorf("Expected %d valid nodes, got %d", len(expected), len(validNodes))
	}

	// Test ignored VM
	validNodes = engine.GetValidTargetNodes(&vms[1], availableNodes)
	if len(validNodes) != 0 {
		t.Errorf("Expected 0 valid nodes for ignored VM, got %d", len(validNodes))
	}
}
