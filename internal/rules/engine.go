package rules

import (
	"fmt"
	"strings"

	"github.com/cblomart/GoProxLB/internal/models"
)

// Engine handles VM placement rules
type Engine struct {
	affinityGroups     map[string]*models.AffinityGroup
	antiAffinityGroups map[string]*models.AntiAffinityGroup
	pinnedVMs          map[int]*models.PinnedVM
	ignoredVMs         map[int]*models.IgnoredVM
}

// NewEngine creates a new rules engine
func NewEngine() *Engine {
	return &Engine{
		affinityGroups:     make(map[string]*models.AffinityGroup),
		antiAffinityGroups: make(map[string]*models.AntiAffinityGroup),
		pinnedVMs:          make(map[int]*models.PinnedVM),
		ignoredVMs:         make(map[int]*models.IgnoredVM),
	}
}

// ProcessVMs processes all VMs and extracts rules
func (e *Engine) ProcessVMs(vms []models.VM) error {
	e.affinityGroups = make(map[string]*models.AffinityGroup)
	e.antiAffinityGroups = make(map[string]*models.AntiAffinityGroup)
	e.pinnedVMs = make(map[int]*models.PinnedVM)
	e.ignoredVMs = make(map[int]*models.IgnoredVM)

	for _, vm := range vms {
		e.processVM(&vm)
	}

	return nil
}

// processVM processes a single VM and extracts its rules
func (e *Engine) processVM(vm *models.VM) {
	for _, tag := range vm.Tags {
		tag = strings.TrimSpace(tag)

		switch {
		case strings.HasPrefix(tag, "plb_affinity_"):
			e.addAffinityRule(vm, tag)
		case strings.HasPrefix(tag, "plb_anti_affinity_"):
			e.addAntiAffinityRule(vm, tag)
		case strings.HasPrefix(tag, "plb_pin_"):
			e.addPinningRule(vm, tag)
		case strings.HasPrefix(tag, "plb_ignore_"):
			e.addIgnoreRule(vm, tag)
		}
	}
}

// addAffinityRule adds a VM to an affinity group
func (e *Engine) addAffinityRule(vm *models.VM, tag string) {
	groupName := strings.TrimPrefix(tag, "plb_affinity_")

	if e.affinityGroups[groupName] == nil {
		e.affinityGroups[groupName] = &models.AffinityGroup{
			Tag:   groupName,
			VMs:   []models.VM{},
			Nodes: []string{},
		}
	}

	e.affinityGroups[groupName].VMs = append(e.affinityGroups[groupName].VMs, *vm)

	// Add node if not already present
	nodeExists := false
	for _, node := range e.affinityGroups[groupName].Nodes {
		if node == vm.Node {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		e.affinityGroups[groupName].Nodes = append(e.affinityGroups[groupName].Nodes, vm.Node)
	}
}

// addAntiAffinityRule adds a VM to an anti-affinity group
func (e *Engine) addAntiAffinityRule(vm *models.VM, tag string) {
	groupName := strings.TrimPrefix(tag, "plb_anti_affinity_")

	if e.antiAffinityGroups[groupName] == nil {
		e.antiAffinityGroups[groupName] = &models.AntiAffinityGroup{
			Tag:   groupName,
			VMs:   []models.VM{},
			Nodes: []string{},
		}
	}

	e.antiAffinityGroups[groupName].VMs = append(e.antiAffinityGroups[groupName].VMs, *vm)

	// Add node if not already present
	nodeExists := false
	for _, node := range e.antiAffinityGroups[groupName].Nodes {
		if node == vm.Node {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		e.antiAffinityGroups[groupName].Nodes = append(e.antiAffinityGroups[groupName].Nodes, vm.Node)
	}
}

// addPinningRule adds a VM to the pinned VMs list
func (e *Engine) addPinningRule(vm *models.VM, tag string) {
	nodeName := strings.TrimPrefix(tag, "plb_pin_")

	if e.pinnedVMs[vm.ID] == nil {
		e.pinnedVMs[vm.ID] = &models.PinnedVM{
			VM:    *vm,
			Nodes: []string{},
		}
	}

	// Add node if not already present
	nodeExists := false
	for _, node := range e.pinnedVMs[vm.ID].Nodes {
		if node == nodeName {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		e.pinnedVMs[vm.ID].Nodes = append(e.pinnedVMs[vm.ID].Nodes, nodeName)
	}
}

// addIgnoreRule adds a VM to the ignored VMs list
func (e *Engine) addIgnoreRule(vm *models.VM, tag string) {
	ignoreTag := strings.TrimPrefix(tag, "plb_ignore_")

	if e.ignoredVMs[vm.ID] == nil {
		e.ignoredVMs[vm.ID] = &models.IgnoredVM{
			VM:   *vm,
			Tags: []string{},
		}
	}

	e.ignoredVMs[vm.ID].Tags = append(e.ignoredVMs[vm.ID].Tags, ignoreTag)
}

// IsIgnored checks if a VM should be ignored
func (e *Engine) IsIgnored(vmID int) bool {
	_, exists := e.ignoredVMs[vmID]
	return exists
}

// IsPinned checks if a VM is pinned to specific nodes
func (e *Engine) IsPinned(vmID int) bool {
	_, exists := e.pinnedVMs[vmID]
	return exists
}

// GetPinnedNodes returns the nodes a VM is pinned to
func (e *Engine) GetPinnedNodes(vmID int) []string {
	if pinned, exists := e.pinnedVMs[vmID]; exists {
		return pinned.Nodes
	}
	return nil
}

// GetAffinityGroups returns all affinity groups
func (e *Engine) GetAffinityGroups() map[string]*models.AffinityGroup {
	return e.affinityGroups
}

// GetAntiAffinityGroups returns all anti-affinity groups
func (e *Engine) GetAntiAffinityGroups() map[string]*models.AntiAffinityGroup {
	return e.antiAffinityGroups
}

// GetPinnedVMs returns all pinned VMs
func (e *Engine) GetPinnedVMs() map[int]*models.PinnedVM {
	return e.pinnedVMs
}

// GetIgnoredVMs returns all ignored VMs
func (e *Engine) GetIgnoredVMs() map[int]*models.IgnoredVM {
	return e.ignoredVMs
}

// ValidatePlacement validates if a VM can be placed on a specific node
func (e *Engine) ValidatePlacement(vm *models.VM, targetNode string) error {
	// Check if VM is ignored
	if e.IsIgnored(vm.ID) {
		return fmt.Errorf("VM %s is ignored and cannot be moved", vm.Name)
	}

	// Check if VM is pinned to specific nodes
	if e.IsPinned(vm.ID) {
		pinnedNodes := e.GetPinnedNodes(vm.ID)
		nodeAllowed := false
		for _, node := range pinnedNodes {
			if node == targetNode {
				nodeAllowed = true
				break
			}
		}
		if !nodeAllowed {
			return fmt.Errorf("VM %s is pinned to nodes %v, cannot move to %s", vm.Name, pinnedNodes, targetNode)
		}
	}

	// Check affinity rules
	for _, group := range e.affinityGroups {
		for _, groupVM := range group.VMs {
			if groupVM.ID == vm.ID {
				// This VM is part of an affinity group
				// Check if any other VM in the group is on the target node
				hasAffinityVM := false
				for _, otherVM := range group.VMs {
					if otherVM.ID != vm.ID && otherVM.Node == targetNode {
						hasAffinityVM = true
						break
					}
				}
				if !hasAffinityVM {
					// Check if there are other VMs in the group that are not on the target node
					otherVMsOnDifferentNodes := false
					for _, otherVM := range group.VMs {
						if otherVM.ID != vm.ID && otherVM.Node != targetNode {
							otherVMsOnDifferentNodes = true
							break
						}
					}
					if otherVMsOnDifferentNodes {
						return fmt.Errorf("VM %s is part of affinity group %s, but no other VMs in the group are on %s", vm.Name, group.Tag, targetNode)
					}
				}
				break
			}
		}
	}

	// Check anti-affinity rules
	for _, group := range e.antiAffinityGroups {
		for _, groupVM := range group.VMs {
			if groupVM.ID == vm.ID {
				// This VM is part of an anti-affinity group
				// Check if any other VM in the group is on the target node
				for _, otherVM := range group.VMs {
					if otherVM.ID != vm.ID && otherVM.Node == targetNode {
						return fmt.Errorf("VM %s is part of anti-affinity group %s, but another VM in the group is already on %s", vm.Name, group.Tag, targetNode)
					}
				}
				break
			}
		}
	}

	return nil
}

// GetValidTargetNodes returns all valid target nodes for a VM
func (e *Engine) GetValidTargetNodes(vm *models.VM, availableNodes []string) []string {
	var validNodes []string

	for _, node := range availableNodes {
		if err := e.ValidatePlacement(vm, node); err == nil {
			validNodes = append(validNodes, node)
		}
	}

	return validNodes
}
