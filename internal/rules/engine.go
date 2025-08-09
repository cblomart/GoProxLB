// Package rules provides rule engine functionality for load balancing decisions.
package rules

import (
	"fmt"
	"strings"

	"github.com/cblomart/GoProxLB/internal/models"
)

// Engine handles VM placement rules.
type Engine struct {
	affinityGroups     map[string]*models.AffinityGroup
	antiAffinityGroups map[string]*models.AntiAffinityGroup
	pinnedVMs          map[int]*models.PinnedVM
	ignoredVMs         map[int]*models.IgnoredVM
}

// NewEngine creates a new rules engine.
func NewEngine() *Engine {
	return &Engine{
		affinityGroups:     make(map[string]*models.AffinityGroup),
		antiAffinityGroups: make(map[string]*models.AntiAffinityGroup),
		pinnedVMs:          make(map[int]*models.PinnedVM),
		ignoredVMs:         make(map[int]*models.IgnoredVM),
	}
}

// ProcessVMs processes all VMs and extracts rules.
func (e *Engine) ProcessVMs(vms []models.VM) error {
	e.affinityGroups = make(map[string]*models.AffinityGroup)
	e.antiAffinityGroups = make(map[string]*models.AntiAffinityGroup)
	e.pinnedVMs = make(map[int]*models.PinnedVM)
	e.ignoredVMs = make(map[int]*models.IgnoredVM)

	for i := range vms {
		vm := &vms[i]
		e.processVM(vm)
	}

	return nil
}

// processVM processes a single VM and extracts its rules.
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

// addVMToGroup adds a VM to a group and ensures the node is tracked.
func (e *Engine) addVMToGroup(vm *models.VM, groupName string, isAffinity bool) {
	if isAffinity {
		if e.affinityGroups[groupName] == nil {
			e.affinityGroups[groupName] = &models.AffinityGroup{
				Tag:   groupName,
				VMs:   []models.VM{},
				Nodes: []string{},
			}
		}
		e.affinityGroups[groupName].VMs = append(e.affinityGroups[groupName].VMs, *vm)
		e.addNodeToGroup(vm.Node, &e.affinityGroups[groupName].Nodes)
	} else {
		if e.antiAffinityGroups[groupName] == nil {
			e.antiAffinityGroups[groupName] = &models.AntiAffinityGroup{
				Tag:   groupName,
				VMs:   []models.VM{},
				Nodes: []string{},
			}
		}
		e.antiAffinityGroups[groupName].VMs = append(e.antiAffinityGroups[groupName].VMs, *vm)
		e.addNodeToGroup(vm.Node, &e.antiAffinityGroups[groupName].Nodes)
	}
}

// addNodeToGroup adds a node to a group's node list if not already present.
func (e *Engine) addNodeToGroup(nodeName string, nodes *[]string) {
	nodeExists := false
	for _, node := range *nodes {
		if node == nodeName {
			nodeExists = true
			break
		}
	}
	if !nodeExists {
		*nodes = append(*nodes, nodeName)
	}
}

// addAffinityRule adds a VM to an affinity group.
func (e *Engine) addAffinityRule(vm *models.VM, tag string) {
	groupName := strings.TrimPrefix(tag, "plb_affinity_")
	e.addVMToGroup(vm, groupName, true)
}

// addAntiAffinityRule adds a VM to an anti-affinity group.
func (e *Engine) addAntiAffinityRule(vm *models.VM, tag string) {
	groupName := strings.TrimPrefix(tag, "plb_anti_affinity_")
	e.addVMToGroup(vm, groupName, false)
}

// addPinningRule adds a VM to the pinned VMs list.
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

// addIgnoreRule adds a VM to the ignored VMs list.
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

// IsIgnored checks if a VM should be ignored.
func (e *Engine) IsIgnored(vmID int) bool {
	_, exists := e.ignoredVMs[vmID]
	return exists
}

// IsPinned checks if a VM is pinned to specific nodes.
func (e *Engine) IsPinned(vmID int) bool {
	_, exists := e.pinnedVMs[vmID]
	return exists
}

// GetPinnedNodes returns the nodes a VM is pinned to.
func (e *Engine) GetPinnedNodes(vmID int) []string {
	if pinned, exists := e.pinnedVMs[vmID]; exists {
		return pinned.Nodes
	}
	return nil
}

// GetAffinityGroups returns all affinity groups.
func (e *Engine) GetAffinityGroups() map[string]*models.AffinityGroup {
	return e.affinityGroups
}

// GetAntiAffinityGroups returns all anti-affinity groups.
func (e *Engine) GetAntiAffinityGroups() map[string]*models.AntiAffinityGroup {
	return e.antiAffinityGroups
}

// GetPinnedVMs returns all pinned VMs.
func (e *Engine) GetPinnedVMs() map[int]*models.PinnedVM {
	return e.pinnedVMs
}

// GetIgnoredVMs returns all ignored VMs.
func (e *Engine) GetIgnoredVMs() map[int]*models.IgnoredVM {
	return e.ignoredVMs
}

// ValidatePlacement validates if a VM can be placed on a specific node.
func (e *Engine) ValidatePlacement(vm *models.VM, targetNode string) error {
	if err := e.validateIgnoreRules(vm); err != nil {
		return err
	}

	if err := e.validatePinningRules(vm, targetNode); err != nil {
		return err
	}

	if err := e.validateAffinityRules(vm, targetNode); err != nil {
		return err
	}

	if err := e.validateAntiAffinityRules(vm, targetNode); err != nil {
		return err
	}

	return nil
}

// GetValidTargetNodes returns all valid target nodes for a VM.
func (e *Engine) GetValidTargetNodes(vm *models.VM, availableNodes []string) []string {
	var validNodes []string

	for _, node := range availableNodes {
		if err := e.ValidatePlacement(vm, node); err == nil {
			validNodes = append(validNodes, node)
		}
	}

	return validNodes
}

// validateIgnoreRules validates if a VM is ignored.
func (e *Engine) validateIgnoreRules(vm *models.VM) error {
	if e.IsIgnored(vm.ID) {
		return fmt.Errorf("VM %s is ignored and cannot be moved", vm.Name)
	}
	return nil
}

// validatePinningRules validates if a VM can be moved to a target node based on pinning rules.
func (e *Engine) validatePinningRules(vm *models.VM, targetNode string) error {
	if !e.IsPinned(vm.ID) {
		return nil
	}

	pinnedNodes := e.GetPinnedNodes(vm.ID)
	for _, node := range pinnedNodes {
		if node == targetNode {
			return nil // Node is allowed
		}
	}

	return fmt.Errorf("VM %s is pinned to nodes %v, cannot move to %s", vm.Name, pinnedNodes, targetNode)
}

// validateAffinityRules validates affinity rules for VM placement.
func (e *Engine) validateAffinityRules(vm *models.VM, targetNode string) error {
	for _, group := range e.affinityGroups {
		if vmGroup := e.findVMInAffinityGroup(vm.ID, group); vmGroup != nil {
			return e.checkAffinityConstraints(vm, targetNode, group)
		}
	}
	return nil
}

// validateAntiAffinityRules validates anti-affinity rules for VM placement.
func (e *Engine) validateAntiAffinityRules(vm *models.VM, targetNode string) error {
	for _, group := range e.antiAffinityGroups {
		if vmGroup := e.findVMInAntiAffinityGroup(vm.ID, group); vmGroup != nil {
			return e.checkAntiAffinityConstraints(vm, targetNode, group)
		}
	}
	return nil
}

// findVMInAffinityGroup finds a VM in an affinity group.
func (e *Engine) findVMInAffinityGroup(vmID int, group *models.AffinityGroup) *models.VM {
	for i := range group.VMs {
		groupVM := &group.VMs[i]
		if groupVM.ID == vmID {
			return groupVM
		}
	}
	return nil
}

// checkAffinityConstraints checks if a VM can be placed on a target node based on affinity rules.
func (e *Engine) checkAffinityConstraints(vm *models.VM, targetNode string, group *models.AffinityGroup) error {
	// Check if any other VM in the group is on the target node
	hasAffinityVM := false
	for j := range group.VMs {
		otherVM := &group.VMs[j]
		if otherVM.ID != vm.ID && otherVM.Node == targetNode {
			hasAffinityVM = true
			break
		}
	}

	if hasAffinityVM {
		return nil // Affinity constraint satisfied
	}

	// Check if there are other VMs in the group that are not on the target node
	for k := range group.VMs {
		otherVM := &group.VMs[k]
		if otherVM.ID != vm.ID && otherVM.Node != targetNode {
			return fmt.Errorf("VM %s is part of affinity group %s, but no other VMs in the group are on %s", vm.Name, group.Tag, targetNode)
		}
	}

	return nil // No constraint violation
}

// findVMInAntiAffinityGroup finds a VM in an anti-affinity group.
func (e *Engine) findVMInAntiAffinityGroup(vmID int, group *models.AntiAffinityGroup) *models.VM {
	for i := range group.VMs {
		groupVM := &group.VMs[i]
		if groupVM.ID == vmID {
			return groupVM
		}
	}
	return nil
}

// checkAntiAffinityConstraints checks if a VM can be placed on a target node based on anti-affinity rules.
func (e *Engine) checkAntiAffinityConstraints(vm *models.VM, targetNode string, group *models.AntiAffinityGroup) error {
	// Check if any other VM in the group is on the target node
	for j := range group.VMs {
		otherVM := &group.VMs[j]
		if otherVM.ID != vm.ID && otherVM.Node == targetNode {
			return fmt.Errorf("VM %s is part of anti-affinity group %s, but another VM in the group is already on %s", vm.Name, group.Tag, targetNode)
		}
	}
	return nil
}
