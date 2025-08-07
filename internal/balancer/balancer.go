package balancer

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/cblomart/GoProxLB/internal/proxmox"
	"github.com/cblomart/GoProxLB/internal/rules"
)

// Balancer represents the load balancer
type Balancer struct {
	client  proxmox.ClientInterface
	config  *config.Config
	engine  *rules.Engine
	lastRun time.Time
}

// NewBalancer creates a new load balancer
func NewBalancer(client proxmox.ClientInterface, cfg *config.Config) *Balancer {
	return &Balancer{
		client:  client,
		config:  cfg,
		engine:  rules.NewEngine(),
		lastRun: time.Time{},
	}
}

// Run performs a load balancing cycle
func (b *Balancer) Run(force bool) ([]models.BalancingResult, error) {
	// Get current cluster state
	nodes, err := b.client.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Filter out maintenance nodes
	availableNodes := b.filterAvailableNodes(nodes)
	if len(availableNodes) < 2 {
		return nil, fmt.Errorf("insufficient available nodes for balancing (need at least 2)")
	}

	// Collect all VMs
	var allVMs []models.VM
	for _, node := range nodes {
		allVMs = append(allVMs, node.VMs...)
	}

	// Process rules
	if err := b.engine.ProcessVMs(allVMs); err != nil {
		return nil, fmt.Errorf("failed to process VM rules: %w", err)
	}

	// Check if balancing is needed
	if !force && !b.needsBalancing(nodes) {
		return nil, nil
	}

	// Calculate node scores
	nodeScores := b.calculateNodeScores(availableNodes)

	// Find VMs that need to be moved
	migrations := b.findMigrations(nodes, nodeScores)

	// Execute migrations
	var results []models.BalancingResult
	for _, migration := range migrations {
		result := b.executeMigration(migration)
		results = append(results, result)
	}

	b.lastRun = time.Now()
	return results, nil
}

// filterAvailableNodes filters out nodes in maintenance mode
func (b *Balancer) filterAvailableNodes(nodes []models.Node) []models.Node {
	var available []models.Node

	for _, node := range nodes {
		if !b.isInMaintenance(node.Name) {
			available = append(available, node)
		}
	}

	return available
}

// isInMaintenance checks if a node is in maintenance mode
func (b *Balancer) isInMaintenance(nodeName string) bool {
	for _, maintenanceNode := range b.config.Cluster.MaintenanceNodes {
		if maintenanceNode == nodeName {
			return true
		}
	}
	return false
}

// needsBalancing checks if the cluster needs balancing
func (b *Balancer) needsBalancing(nodes []models.Node) bool {
	for _, node := range nodes {
		if b.isInMaintenance(node.Name) {
			continue
		}

		if node.CPU.Usage > float32(b.config.Balancing.Thresholds.CPU) ||
			node.Memory.Usage > float32(b.config.Balancing.Thresholds.Memory) ||
			node.Storage.Usage > float32(b.config.Balancing.Thresholds.Storage) {
			return true
		}
	}
	return false
}

// calculateNodeScores calculates scores for all nodes
func (b *Balancer) calculateNodeScores(nodes []models.Node) []models.NodeScore {
	var scores []models.NodeScore

	for _, node := range nodes {
		score := b.calculateNodeScore(node)
		scores = append(scores, score)
	}

	// Sort by score (lower is better)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score < scores[j].Score
	})

	return scores
}

// calculateNodeScore calculates a score for a single node
func (b *Balancer) calculateNodeScore(node models.Node) models.NodeScore {
	// Normalize resource usage (0-1 scale)
	cpuScore := node.CPU.Usage / 100.0
	memoryScore := node.Memory.Usage / 100.0
	storageScore := node.Storage.Usage / 100.0

	// Apply weights
	weightedScore := float64(cpuScore)*b.config.Balancing.Weights.CPU +
		float64(memoryScore)*b.config.Balancing.Weights.Memory +
		float64(storageScore)*b.config.Balancing.Weights.Storage

	// Normalize by total weight
	totalWeight := b.config.Balancing.Weights.CPU +
		b.config.Balancing.Weights.Memory +
		b.config.Balancing.Weights.Storage

	finalScore := weightedScore / totalWeight

	return models.NodeScore{
		Node:    node.Name,
		Score:   finalScore,
		CPU:     cpuScore,
		Memory:  memoryScore,
		Storage: storageScore,
	}
}

// findMigrations finds VMs that should be migrated
func (b *Balancer) findMigrations(nodes []models.Node, nodeScores []models.NodeScore) []models.Migration {
	var migrations []models.Migration

	// Find overloaded nodes (source nodes)
	var sourceNodes []models.Node
	for _, node := range nodes {
		if b.isInMaintenance(node.Name) {
			continue
		}

		if node.CPU.Usage > float32(b.config.Balancing.Thresholds.CPU) ||
			node.Memory.Usage > float32(b.config.Balancing.Thresholds.Memory) ||
			node.Storage.Usage > float32(b.config.Balancing.Thresholds.Storage) {
			sourceNodes = append(sourceNodes, node)
		}
	}

	// For each overloaded node, find VMs to migrate
	for _, sourceNode := range sourceNodes {
		for _, vm := range sourceNode.VMs {
			// Skip ignored VMs
			if b.engine.IsIgnored(vm.ID) {
				continue
			}

			// Find best target node
			targetNode := b.findBestTargetNode(vm, nodeScores)
			if targetNode == "" {
				continue
			}

			// Calculate resource gain
			gain := b.calculateResourceGain(vm, sourceNode.Name, targetNode, nodeScores)
			if gain <= 0 {
				continue
			}

			migration := models.Migration{
				VM:        vm,
				FromNode:  sourceNode.Name,
				ToNode:    targetNode,
				Status:    "pending",
				StartTime: time.Now(),
			}

			migrations = append(migrations, migration)
		}
	}

	return migrations
}

// findBestTargetNode finds the best target node for a VM
func (b *Balancer) findBestTargetNode(vm models.VM, nodeScores []models.NodeScore) string {
	// Get valid target nodes
	var validNodes []string
	for _, score := range nodeScores {
		if score.Node != vm.Node {
			validNodes = append(validNodes, score.Node)
		}
	}

	// Filter by rules
	validNodes = b.engine.GetValidTargetNodes(vm, validNodes)
	if len(validNodes) == 0 {
		return ""
	}

	// Return the node with the best score
	for _, score := range nodeScores {
		for _, validNode := range validNodes {
			if score.Node == validNode {
				return score.Node
			}
		}
	}

	return ""
}

// calculateResourceGain calculates the resource gain from migrating a VM
func (b *Balancer) calculateResourceGain(vm models.VM, sourceNode, targetNode string, nodeScores []models.NodeScore) float64 {
	var sourceScore, targetScore models.NodeScore

	// Find scores for source and target nodes
	for _, score := range nodeScores {
		if score.Node == sourceNode {
			sourceScore = score
		}
		if score.Node == targetNode {
			targetScore = score
		}
	}

	// Calculate improvement
	improvement := sourceScore.Score - targetScore.Score
	return math.Max(0, improvement)
}

// executeMigration executes a VM migration
func (b *Balancer) executeMigration(migration models.Migration) models.BalancingResult {
	result := models.BalancingResult{
		SourceNode: migration.FromNode,
		TargetNode: migration.ToNode,
		VM:         migration.VM,
		Reason:     "load balancing",
		Timestamp:  time.Now(),
		Success:    false,
	}

	// Get current nodes for scoring
	currentNodes, err := b.client.GetNodes()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to get nodes for scoring: %v", err)
		return result
	}
	
	// Calculate resource gain
	nodeScores := b.calculateNodeScores(currentNodes)
	result.ResourceGain = b.calculateResourceGain(migration.VM, migration.FromNode, migration.ToNode, nodeScores)

	// Execute migration
	err = b.client.MigrateVM(migration.VM.ID, migration.FromNode, migration.ToNode)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	result.Success = true
	return result
}

// GetClusterStatus returns the current cluster status
func (b *Balancer) GetClusterStatus() (*models.ClusterStatus, error) {
	nodes, err := b.client.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	status := &models.ClusterStatus{
		TotalNodes:       len(nodes),
		ActiveNodes:      0,
		TotalVMs:         0,
		RunningVMs:       0,
		AverageCPU:       0,
		AverageMemory:    0,
		AverageStorage:   0,
		LastBalanced:     b.lastRun,
		BalancingEnabled: true, // Always enabled when running
	}

	var totalCPU, totalMemory, totalStorage float64
	var activeNodeCount int

	for _, node := range nodes {
		if !b.isInMaintenance(node.Name) {
			status.ActiveNodes++
			activeNodeCount++
			totalCPU += float64(node.CPU.Usage)
			totalMemory += float64(node.Memory.Usage)
			totalStorage += float64(node.Storage.Usage)
		}

		status.TotalVMs += len(node.VMs)
		for _, vm := range node.VMs {
			if vm.Status == "running" {
				status.RunningVMs++
			}
		}
	}

	if activeNodeCount > 0 {
		status.AverageCPU = float32(totalCPU / float64(activeNodeCount))
		status.AverageMemory = float32(totalMemory / float64(activeNodeCount))
		status.AverageStorage = float32(totalStorage / float64(activeNodeCount))
	}

	return status, nil
}
