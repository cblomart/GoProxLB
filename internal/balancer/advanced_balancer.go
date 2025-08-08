package balancer

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/cblomart/GoProxLB/internal/proxmox"
	"github.com/cblomart/GoProxLB/internal/rules"
)

const (
	vmStatusRunning          = "running"
	defaultTimeframe         = "day"
	criticalityLevelCritical = "Critical"
)

// AdvancedBalancer represents the advanced load balancer with profiling and capacity planning.
type AdvancedBalancer struct {
	client           proxmox.ClientInterface
	config           *config.Config
	engine           *rules.Engine
	lastRun          time.Time
	migrationHistory []models.MigrationHistory
	loadProfiles     map[int]*models.LoadProfile
	capacityMetrics  map[string]*models.CapacityMetrics
}

// NewAdvancedBalancer creates a new advanced load balancer.
func NewAdvancedBalancer(client proxmox.ClientInterface, cfg *config.Config) *AdvancedBalancer {
	return &AdvancedBalancer{
		client:           client,
		config:           cfg,
		engine:           rules.NewEngine(),
		migrationHistory: make([]models.MigrationHistory, 0),
		loadProfiles:     make(map[int]*models.LoadProfile),
		capacityMetrics:  make(map[string]*models.CapacityMetrics),
	}
}

// Run executes the advanced load balancing algorithm.
func (b *AdvancedBalancer) Run(force bool) ([]models.BalancingResult, error) {
	// Get current cluster state
	nodes, err := b.client.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Filter available nodes
	availableNodes := b.filterAvailableNodes(nodes)
	if len(availableNodes) < 2 {
		return nil, fmt.Errorf("insufficient available nodes for balancing")
	}

	// Update load profiles if enabled
	if b.config.Balancing.LoadProfiles.Enabled {
		b.updateLoadProfiles(availableNodes)
	}

	// Update capacity metrics if enabled
	if b.config.Balancing.Capacity.Enabled {
		b.updateCapacityMetrics(availableNodes)
	}

	// Check if balancing is needed
	if !force && !b.needsBalancing(availableNodes) {
		return []models.BalancingResult{}, nil
	}

	// Get aggressiveness configuration
	aggConfig := b.config.GetAggressivenessConfig()

	// Check cooldown period
	if !force && time.Since(b.lastRun) < aggConfig.CooldownPeriod {
		return []models.BalancingResult{}, nil
	}

	// Calculate node scores with advanced scoring
	nodeScores := b.calculateAdvancedNodeScores(availableNodes)

	// Find optimal migrations
	migrations := b.findOptimalMigrations(availableNodes, nodeScores, aggConfig)

	// Execute migrations
	results := b.executeMigrations(migrations)

	// Update migration history
	b.updateMigrationHistory(results)

	// Update last run time
	b.lastRun = time.Now()

	return results, nil
}

// GetClusterStatus returns the advanced cluster status.
func (b *AdvancedBalancer) GetClusterStatus() (*models.ClusterStatus, error) {
	nodes, err := b.client.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	availableNodes := b.filterAvailableNodes(nodes)
	if len(availableNodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// Calculate advanced metrics
	totalVMs := 0
	runningVMs := 0
	var cpuValues, memoryValues, storageValues []float32

	for i := range availableNodes {
		node := &availableNodes[i]
		totalVMs += len(node.VMs)
		for j := range node.VMs {
			vm := &node.VMs[j]
			if vm.Status == vmStatusRunning {
				runningVMs++
			}
		}
		cpuValues = append(cpuValues, node.CPU.Usage)
		memoryValues = append(memoryValues, node.Memory.Usage)
		storageValues = append(storageValues, node.Storage.Usage)
	}

	// Calculate percentiles
	cpuMetrics := b.calculatePercentiles(cpuValues)
	memoryMetrics := b.calculatePercentiles(memoryValues)
	storageMetrics := b.calculatePercentiles(storageValues)

	return &models.ClusterStatus{
		TotalNodes:       len(nodes),
		ActiveNodes:      len(availableNodes),
		TotalVMs:         totalVMs,
		RunningVMs:       runningVMs,
		AverageCPU:       cpuMetrics.Mean,
		AverageMemory:    memoryMetrics.Mean,
		AverageStorage:   storageMetrics.Mean,
		LastBalanced:     b.lastRun,
		BalancingEnabled: true, // Always enabled when running
	}, nil
}

// updateLoadProfiles updates load profiles for all VMs.
func (b *AdvancedBalancer) updateLoadProfiles(nodes []models.Node) {
	for i := range nodes {
		node := &nodes[i]
		for j := range node.VMs {
			vm := &node.VMs[j]
			if vm.Status == vmStatusRunning {
				profile := b.analyzeLoadProfile(vm)
				b.loadProfiles[vm.ID] = profile
			}
		}
	}
}

// analyzeLoadProfile analyzes the load profile of a VM using historical data.
func (b *AdvancedBalancer) analyzeLoadProfile(vm *models.VM) *models.LoadProfile {
	// Use simplified analysis
	cpuPattern := b.analyzeCPUPatternFromHistory()
	memoryPattern := b.analyzeMemoryPatternFromHistory()
	storagePattern := b.analyzeStoragePatternFromHistory()

	// Determine priority based on tags and usage patterns
	priority := b.determinePriority(vm, cpuPattern)

	// Determine criticality
	criticality := b.determineCriticality(vm, priority)

	return &models.LoadProfile{
		CPUPattern:     cpuPattern,
		MemoryPattern:  memoryPattern,
		StoragePattern: storagePattern,
		Priority:       priority,
		Criticality:    criticality,
	}
}



// analyzeCPUPatternFromHistory analyzes CPU usage patterns from historical data.
func (b *AdvancedBalancer) analyzeCPUPatternFromHistory() models.CPUPattern {
	// Simplified analysis without historical data

	// Example: If historical data shows a sustained high CPU usage, return sustained
	// For simplicity, we'll return a placeholder
	return models.CPUPattern{
		Type:           "sustained",
		SustainedLevel: 90.0, // Placeholder, ideally calculated from data
	}
}



// analyzeMemoryPatternFromHistory analyzes memory usage patterns from historical data.
func (b *AdvancedBalancer) analyzeMemoryPatternFromHistory() models.MemoryPattern {
	// Simplified analysis without historical data - in reality, you'd analyze historical data
	// For now, we'll just return a placeholder or a basic pattern
	// A more sophisticated analysis would involve statistical analysis of the data

	// Example: If historical data shows a sustained high memory usage, return sustained
	// For simplicity, we'll return a placeholder
	return models.MemoryPattern{
		Type:      "sustained",
		PeakUsage: 90.0, // Placeholder, ideally calculated from data
	}
}



// analyzeStoragePatternFromHistory analyzes storage usage patterns from historical data.
func (b *AdvancedBalancer) analyzeStoragePatternFromHistory() models.StoragePattern {
	// Simplified analysis without historical data
	// For now, we'll just return a placeholder or a basic pattern
	// A more sophisticated analysis would involve statistical analysis of the data

	// Example: If historical data shows a sustained high storage usage, return sustained
	// For simplicity, we'll return a placeholder
	return models.StoragePattern{
		Type:      "sustained",
		ReadIOPs:  1500, // Placeholder, ideally calculated from data
		WriteIOPs: 700,  // Placeholder, ideally calculated from data
	}
}

// determinePriority determines VM priority.
func (b *AdvancedBalancer) determinePriority(vm *models.VM, cpu models.CPUPattern) models.Priority {
	// Check for priority tags
	for _, tag := range vm.Tags {
		switch tag {
		case "realtime", "critical", "high-priority":
			return models.PriorityRealtime
		case "interactive", "user-facing":
			return models.PriorityInteractive
		case "background", "batch", "low-priority":
			return models.PriorityBackground
		}
	}

	// Determine by usage patterns
	if cpu.Type == "sustained" && cpu.SustainedLevel > 70.0 {
		return models.PriorityRealtime
	} else if cpu.Type == "burst" {
		return models.PriorityInteractive
	} else {
		return models.PriorityBackground
	}
}

// determineCriticality determines VM criticality.
func (b *AdvancedBalancer) determineCriticality(vm *models.VM, priority models.Priority) models.Criticality {
	// Check for criticality tags
	for _, tag := range vm.Tags {
		switch tag {
		case "critical", "essential":
			return models.CriticalityCritical
		case "important", "production":
			return models.CriticalityImportant
		}
	}

	// Determine by priority
	switch priority {
	case models.PriorityRealtime:
		return models.CriticalityCritical
	case models.PriorityInteractive:
		return models.CriticalityImportant
	default:
		return models.CriticalityNormal
	}
}

// updateCapacityMetrics updates capacity planning metrics.
func (b *AdvancedBalancer) updateCapacityMetrics(nodes []models.Node) {
	for i := range nodes {
		node := &nodes[i]
		// Get historical data for the node
		timeframe := defaultTimeframe // Default to 24 hours
		if forecast, err := b.config.GetCapacityForecast(); err == nil {
			if forecast >= 7*24*time.Hour {
				timeframe = "week"
			} else if forecast >= 24*time.Hour {
				timeframe = defaultTimeframe
			} else {
				timeframe = "hour"
			}
		}

		historicalData, err := b.client.GetNodeHistoricalData(node.Name, timeframe)
		if err != nil {
			// Fallback to simplified analysis if historical data is not available
			b.updateCapacityMetricsSimplified(node)
			continue
		}

		// Extract CPU and memory values from historical data
		var cpuValues, memoryValues []float32
		for _, metric := range historicalData {
			cpuValues = append(cpuValues, float32(metric.CPU))
			memoryValues = append(memoryValues, float32(metric.Memory))
		}

		// Calculate percentiles from historical data
		cpuMetrics := b.calculatePercentiles(cpuValues)
		// Memory percentiles calculated for future use
		memoryPercentiles := b.calculatePercentiles(memoryValues)
		// Store memory percentiles for potential future use
		// Currently not used in scoring but available for analysis
		_ = memoryPercentiles

		// Store metrics (currently using CPU metrics as primary)
		b.capacityMetrics[node.Name] = &models.CapacityMetrics{
			P50:    cpuMetrics.P50,
			P90:    cpuMetrics.P90,
			P95:    cpuMetrics.P95,
			P99:    cpuMetrics.P99,
			MinP90: cpuMetrics.MinP90,
			MaxP90: cpuMetrics.MaxP90,
			Mean:   cpuMetrics.Mean,
			StdDev: cpuMetrics.StdDev,
		}
	}
}

// updateCapacityMetricsSimplified provides simplified capacity metrics when historical data is not available.
func (b *AdvancedBalancer) updateCapacityMetricsSimplified(node *models.Node) {
	var cpuValues []float32

	// Use current data as fallback
	cpuValues = append(cpuValues, node.CPU.Usage)

	// Calculate percentiles (simplified - in reality, you'd store historical data)
	cpuMetrics := b.calculatePercentiles(cpuValues)

	// Store metrics
	b.capacityMetrics[node.Name] = &models.CapacityMetrics{
		P50:    cpuMetrics.P50,
		P90:    cpuMetrics.P90,
		P95:    cpuMetrics.P95,
		P99:    cpuMetrics.P99,
		MinP90: cpuMetrics.MinP90,
		MaxP90: cpuMetrics.MaxP90,
		Mean:   cpuMetrics.Mean,
		StdDev: cpuMetrics.StdDev,
	}
}

// calculatePercentiles calculates percentile metrics (optimized for performance).
func (b *AdvancedBalancer) calculatePercentiles(values []float32) models.CapacityMetrics {
	if len(values) == 0 {
		return models.CapacityMetrics{}
	}

	// Use faster sorting for all datasets
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})

	n := len(values)
	nMinus1 := float32(n - 1)

	// Calculate percentiles using integer math for better performance
	// Round to nearest integer for faster indexing
	p50Idx := int(nMinus1*0.5 + 0.5)
	p90Idx := int(nMinus1*0.9 + 0.5)
	p95Idx := int(nMinus1*0.95 + 0.5)
	p99Idx := int(nMinus1*0.99 + 0.5)
	minIdx := int(nMinus1*0.1 + 0.5)

	// Bounds checking
	if p50Idx >= n {
		p50Idx = n - 1
	}
	if p90Idx >= n {
		p90Idx = n - 1
	}
	if p95Idx >= n {
		p95Idx = n - 1
	}
	if p99Idx >= n {
		p99Idx = n - 1
	}
	if minIdx >= n {
		minIdx = n - 1
	}

	p50 := values[p50Idx]
	p90 := values[p90Idx]
	p95 := values[p95Idx]
	p99 := values[p99Idx]
	minP90 := values[minIdx]
	maxP90 := values[p90Idx]

	// Optimized mean calculation (single pass)
	var sum float32
	for _, v := range values {
		sum += v
	}
	mean := sum / float32(n)

	// Simplified variance calculation (good enough for load balancing)
	var variance float32
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}

	// Use optimized sqrt calculation
	stdDev := float32(math.Sqrt(float64(variance / float32(n))))

	return models.CapacityMetrics{
		P50:    p50,
		P90:    p90,
		P95:    p95,
		P99:    p99,
		MinP90: minP90,
		MaxP90: maxP90,
		Mean:   mean,
		StdDev: stdDev,
	}
}

// calculateAdvancedNodeScores calculates node scores with advanced algorithms including capacity planning.
func (b *AdvancedBalancer) calculateAdvancedNodeScores(nodes []models.Node) []models.NodeScore {
	var scores []models.NodeScore

	for i := range nodes {
		node := &nodes[i]
		// Calculate resource score
		resourceScore := b.calculateResourceScore(node)

		// Calculate stability score
		stabilityScore := b.calculateStabilityScore(node)

		// Calculate migration cost
		migrationCost := b.calculateMigrationCost(node)

		// Calculate capacity planning score
		capacityScore := b.calculateCapacityScore(node)

		// Calculate final score with capacity planning weighting
		// Capacity planning gets 30% weight, stability gets 20%, resource gets 40%, migration cost gets 10%
		finalScore := resourceScore*0.4 +
			stabilityScore*0.2 +
			capacityScore*0.3 +
			migrationCost*0.1

		scores = append(scores, models.NodeScore{
			Node:    node.Name,
			Score:   finalScore,
			CPU:     node.CPU.Usage,
			Memory:  node.Memory.Usage,
			Storage: node.Storage.Usage,
		})
	}

	// Sort by score (lower is better)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score < scores[j].Score
	})

	return scores
}

// calculateResourceScore calculates resource-based score with capacity planning integration.
func (b *AdvancedBalancer) calculateResourceScore(node *models.Node) float64 {
	// Get capacity metrics for predictive scoring
	metrics, exists := b.capacityMetrics[node.Name]

	// Use integer math where possible for better performance
	// Convert percentages to integers (0-10000 scale) for faster calculations
	cpuInt := int(node.CPU.Usage * 100)
	memoryInt := int(node.Memory.Usage * 100)
	storageInt := int(node.Storage.Usage * 100)

	// If capacity metrics are available, use predictive scoring
	if exists && metrics.P90 > 0 {
		// Calculate predictive scores based on P90 capacity
		predictiveCPU := float64(metrics.P90) * 100 // P90 as predictive indicator
		predictiveMemory := float64(metrics.P90) * 100

		// Blend current usage with predictive capacity (70% current, 30% predictive)
		cpuInt = int((float64(node.CPU.Usage)*0.7 + predictiveCPU*0.3) * 100)
		memoryInt = int((float64(node.Memory.Usage)*0.7 + predictiveMemory*0.3) * 100)
	}

	// Use integer weights (multiply by 1000 for precision)
	cpuWeight := int(b.config.Balancing.Weights.CPU * 1000)
	memoryWeight := int(b.config.Balancing.Weights.Memory * 1000)
	storageWeight := int(b.config.Balancing.Weights.Storage * 1000)

	// Calculate weighted sum using integer math
	weightedSum := cpuInt*cpuWeight + memoryInt*memoryWeight + storageInt*storageWeight
	totalWeight := cpuWeight + memoryWeight + storageWeight

	// Convert back to float64 and normalize
	return float64(weightedSum) / float64(totalWeight) / 100.0
}

// calculateStabilityScore calculates stability-based score (optimized for performance).
func (b *AdvancedBalancer) calculateStabilityScore(node *models.Node) float64 {
	// Cache current time to avoid multiple calls
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	// Count recent migrations for this node (optimized loop)
	recentMigrations := 0
	for _, migration := range b.migrationHistory {
		// Use direct comparison instead of After() for better performance
		if (migration.FromNode == node.Name || migration.ToNode == node.Name) &&
			migration.Timestamp.After(oneHourAgo) {
			recentMigrations++
		}
	}

	// Optimized VM age calculation
	var totalAge float64
	vmCount := 0

	for i := range node.VMs {
		vm := &node.VMs[i]
		if !vm.LastMoved.IsZero() {
			// Use faster time calculation
			age := now.Sub(vm.LastMoved).Hours()
			totalAge += age
			vmCount++
		}
	}

	// Simplified average age calculation
	averageAge := 0.0
	if vmCount > 0 {
		averageAge = totalAge / float64(vmCount)
	}

	// Optimized stability score calculation
	// Use integer math where possible for better performance
	migrationPenalty := float64(recentMigrations) * 10.0

	// Simplified age bonus calculation (good enough for load balancing)
	ageBonus := 0.0
	if averageAge > 24.0 {
		ageBonus = 20.0 // Max bonus for 24h+ stability
	} else {
		ageBonus = (averageAge / 24.0) * 20.0
	}

	return migrationPenalty - ageBonus
}

// calculateMigrationCost calculates migration cost for a node (optimized for performance).
func (b *AdvancedBalancer) calculateMigrationCost(node *models.Node) float64 {
	// Use integer math for better performance
	// Convert percentages to integers for faster calculations
	cpuInt := int(node.CPU.Usage)
	memoryInt := int(node.Memory.Usage)

	// Base cost calculation using integer math
	baseCost := float64(cpuInt+memoryInt) / 200.0

	// Simplified high-load check (good enough for load balancing)
	if cpuInt > 80 || memoryInt > 80 {
		baseCost += 10.0
	}

	return baseCost
}

// calculateCapacityScore calculates capacity planning score for a node (optimized for performance).
func (b *AdvancedBalancer) calculateCapacityScore(node *models.Node) float64 {
	// Get current capacity metrics for the node
	metrics, exists := b.capacityMetrics[node.Name]
	if !exists {
		// Fallback to simplified analysis if capacity metrics are not available
		return b.calculateCapacityScoreSimplified(node)
	}

	// Get aggressiveness configuration
	aggConfig := b.config.GetAggressivenessConfig()

	// Calculate score based on percentile usage
	cpuScore := 0.0
	if metrics.P90 > 0 {
		cpuScore = 100.0 - float64(metrics.P90) // Lower P90 is better
	}

	memoryScore := 0.0
	if metrics.P90 > 0 {
		memoryScore = 100.0 - float64(metrics.P90) // Lower P90 is better
	}

	// Combine scores, with capacity planning getting more weight
	capacityScore := cpuScore*0.6 + memoryScore*0.4

	// Apply aggressiveness weighting
	return capacityScore * aggConfig.CapacityWeight
}

// calculateCapacityScoreSimplified provides simplified capacity score when historical data is not available.
func (b *AdvancedBalancer) calculateCapacityScoreSimplified(node *models.Node) float64 {
	// Use current data as fallback
	cpuScore := 0.0
	if node.CPU.Usage > 0 {
		cpuScore = 100.0 - float64(node.CPU.Usage) // Lower usage is better
	}

	memoryScore := 0.0
	if node.Memory.Usage > 0 {
		memoryScore = 100.0 - float64(node.Memory.Usage) // Lower usage is better
	}

	// Combine scores, with capacity planning getting more weight
	capacityScore := cpuScore*0.6 + memoryScore*0.4

	// Apply aggressiveness weighting
	aggConfig := b.config.GetAggressivenessConfig()
	return capacityScore * aggConfig.CapacityWeight
}

// findOptimalMigrations finds optimal migration plan (optimized for performance).
func (b *AdvancedBalancer) findOptimalMigrations(nodes []models.Node, nodeScores []models.NodeScore, aggConfig config.AggressivenessConfig) []models.Migration {
	// Pre-allocate slice with reasonable capacity to reduce allocations
	migrations := make([]models.Migration, 0, 5) // Most clusters won't need more than 5 migrations

	// Pre-calculate thresholds as integers for faster comparison
	cpuThreshold := int(b.config.Balancing.Thresholds.CPU) //nolint:unconvert
	memoryThreshold := int(b.config.Balancing.Thresholds.Memory) //nolint:unconvert
	storageThreshold := int(b.config.Balancing.Thresholds.Storage) //nolint:unconvert

	// Find overloaded nodes (optimized loop)
	overloadedNodes := make([]models.Node, 0, len(nodes)/2) // Pre-allocate with reasonable capacity
	for i := range nodes {
		node := &nodes[i]
		// Use integer comparisons for better performance
		if int(node.CPU.Usage) > cpuThreshold ||
			int(node.Memory.Usage) > memoryThreshold ||
			int(node.Storage.Usage) > storageThreshold {
			overloadedNodes = append(overloadedNodes, *node)
		}
	}

	// For each overloaded node, find VMs to migrate
	for i := range overloadedNodes {
		overloadedNode := &overloadedNodes[i]
		for j := range overloadedNode.VMs {
			vm := &overloadedNode.VMs[j]
			// Early exit for non-running VMs
			if vm.Status != "running" {
				continue
			}

			// Check if VM can be migrated
			if !b.canMigrateVM(vm, overloadedNode.Name) {
				continue
			}

			// Find best target node
			targetNode := b.findBestTargetNode(vm, nodeScores, overloadedNode.Name)
			if targetNode == "" {
				continue
			}

			// Calculate resource gain
			gain := b.calculateResourceGain(overloadedNode.Name, targetNode, nodeScores)

			// Check if gain meets minimum improvement threshold
			if gain < aggConfig.MinImprovement {
				continue
			}

			// Create migration
			migration := models.Migration{
				VM:        *vm,
				FromNode:  overloadedNode.Name,
				ToNode:    targetNode,
				Status:    "pending",
				StartTime: time.Now(),
			}

			migrations = append(migrations, migration)

			// Limit number of migrations per cycle
			if len(migrations) >= 5 {
				return migrations
			}
		}
	}

	return migrations
}

// canMigrateVM checks if a VM can be migrated (optimized for performance).
func (b *AdvancedBalancer) canMigrateVM(vm *models.VM, sourceNode string) bool {
	// Cache current time to avoid multiple calls
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	// Check if VM was recently migrated
	if !vm.LastMoved.IsZero() && vm.LastMoved.After(oneHourAgo) {
		return false
	}

	// Check migration history for flip-flopping (optimized loop)
	for _, migration := range b.migrationHistory {
		if migration.VMID == vm.ID && migration.Timestamp.After(oneHourAgo) {
			return false
		}
	}

	// Check rules engine
	return b.engine.ValidatePlacement(vm, sourceNode) == nil
}

// findBestTargetNode finds the best target node for a VM.
func (b *AdvancedBalancer) findBestTargetNode(vm *models.VM, nodeScores []models.NodeScore, sourceNode string) string {
	// Get available nodes for validation
	var availableNodes []string
	for _, score := range nodeScores {
		if score.Node != sourceNode {
			availableNodes = append(availableNodes, score.Node)
		}
	}

	// Get valid target nodes from rules engine
	validNodes := b.engine.GetValidTargetNodes(vm, availableNodes)

	// Find the best valid node
	for _, score := range nodeScores {
		if score.Node == sourceNode {
			continue
		}

		// Check if node is valid
		isValid := false
		for _, validNode := range validNodes {
			if validNode == score.Node {
				isValid = true
				break
			}
		}

		if isValid {
			return score.Node
		}
	}

	return ""
}

// calculateResourceGain calculates resource gain from migration (optimized for performance).
func (b *AdvancedBalancer) calculateResourceGain(sourceNode, targetNode string, nodeScores []models.NodeScore) float64 {
	// Use map for O(1) lookup instead of O(n) search
	nodeScoreMap := make(map[string]float64, len(nodeScores))
	for _, score := range nodeScores {
		nodeScoreMap[score.Node] = score.Score
	}

	// Get scores with default fallback
	sourceScore, sourceExists := nodeScoreMap[sourceNode]
	targetScore, targetExists := nodeScoreMap[targetNode]

	// If either node is not found, return 0 gain
	if !sourceExists || !targetExists {
		return 0.0
	}

	// Calculate gain (improvement in balance)
	return sourceScore - targetScore
}

// executeMigrations executes the migration plan.
func (b *AdvancedBalancer) executeMigrations(migrations []models.Migration) []models.BalancingResult {
	var results []models.BalancingResult

	for i := range migrations {
		migration := &migrations[i]
		// Execute migration via Proxmox API
		err := b.client.MigrateVM(migration.VM.ID, migration.FromNode, migration.ToNode)

		result := models.BalancingResult{
			SourceNode:   migration.FromNode,
			TargetNode:   migration.ToNode,
			VM:           migration.VM,
			Reason:       "load_balancing",
			ResourceGain: 10.0, // Simplified
			Timestamp:    time.Now(),
			Success:      err == nil,
		}

		if err != nil {
			result.ErrorMessage = err.Error()
		}

		results = append(results, result)
	}

	return results
}

// updateMigrationHistory updates migration history.
func (b *AdvancedBalancer) updateMigrationHistory(results []models.BalancingResult) {
	for i := range results {
		result := &results[i]
		if result.Success {
			history := models.MigrationHistory{
				VMID:      result.VM.ID,
				FromNode:  result.SourceNode,
				ToNode:    result.TargetNode,
				Timestamp: result.Timestamp,
				Reason:    result.Reason,
			}
			b.migrationHistory = append(b.migrationHistory, history)
		}
	}

	// Keep only recent history (last 24 hours)
	oneDayAgo := time.Now().Add(-24 * time.Hour)
	var recentHistory []models.MigrationHistory
	for _, history := range b.migrationHistory {
		if history.Timestamp.After(oneDayAgo) {
			recentHistory = append(recentHistory, history)
		}
	}
	b.migrationHistory = recentHistory
}

// filterAvailableNodes filters out maintenance nodes.
func (b *AdvancedBalancer) filterAvailableNodes(nodes []models.Node) []models.Node {
	var available []models.Node

	for i := range nodes {
		node := &nodes[i]
		if node.Status == "online" && !b.isInMaintenance(node.Name) {
			available = append(available, *node)
		}
	}

	return available
}

// isInMaintenance checks if a node is in maintenance mode.
func (b *AdvancedBalancer) isInMaintenance(nodeName string) bool {
	for _, maintenanceNode := range b.config.Cluster.MaintenanceNodes {
		if maintenanceNode == nodeName {
			return true
		}
	}
	return false
}

// needsBalancing checks if balancing is needed.
func (b *AdvancedBalancer) needsBalancing(nodes []models.Node) bool {
	for i := range nodes {
		node := &nodes[i]
		if node.CPU.Usage > float32(b.config.Balancing.Thresholds.CPU) ||
			node.Memory.Usage > float32(b.config.Balancing.Thresholds.Memory) ||
			node.Storage.Usage > float32(b.config.Balancing.Thresholds.Storage) {
			return true
		}
	}
	return false
}

// GetCapacityMetrics returns capacity metrics for a specific node.
func (b *AdvancedBalancer) GetCapacityMetrics(nodeName string) (*models.CapacityMetrics, bool) {
	metrics, exists := b.capacityMetrics[nodeName]
	return metrics, exists
}

// PredictResourceEvolution predicts resource usage evolution for a given period.
func (b *AdvancedBalancer) PredictResourceEvolution(nodeName, resourceType string, forecastDuration time.Duration) float64 {
	metrics, exists := b.capacityMetrics[nodeName]
	if !exists {
		return 0.0
	}

	// Simple linear prediction based on P90 and current trend
	// In a real implementation, you'd use more sophisticated time series analysis
	baseUsage := metrics.P90

	// Calculate trend factor (simplified)
	trendFactor := 1.0
	if metrics.StdDev > 0 {
		// Higher standard deviation suggests more variability
		trendFactor = 1.0 + float64(metrics.StdDev/100.0)*0.1
	}

	// Apply forecast duration scaling
	weeks := forecastDuration.Hours() / (7 * 24)
	predictedUsage := float64(baseUsage) * trendFactor * (1.0 + weeks*0.05) // 5% growth per week

	// Cap at 100%
	if predictedUsage > 100.0 {
		predictedUsage = 100.0
	}

	return predictedUsage
}

// GetResourceRecommendations provides resource recommendations for a node.
func (b *AdvancedBalancer) GetResourceRecommendations(nodeName string, detailed bool) []string {
	var recommendations []string

	metrics, exists := b.capacityMetrics[nodeName]
	if !exists {
		recommendations = append(recommendations, "No historical data available for recommendations")
		return recommendations
	}

	// Analyze P90 usage patterns
	if metrics.P90 > 90 {
		recommendations = append(recommendations, "⚠️  High P90 usage (>90%) - Consider adding resources or redistributing VMs")
	} else if metrics.P90 > 80 {
		recommendations = append(recommendations, "⚠️  Elevated P90 usage (>80%) - Monitor closely and plan for capacity expansion")
	} else if metrics.P90 < 30 {
		recommendations = append(recommendations, "💡 Low P90 usage (<30%) - Consider consolidating VMs or reducing resources")
	}

	// Analyze variability
	if metrics.StdDev > 20 {
		recommendations = append(recommendations, "📊 High resource variability - Consider burst-capable resources or over-provisioning")
	} else if metrics.StdDev < 5 {
		recommendations = append(recommendations, "📊 Low resource variability - Can optimize with tighter resource allocation")
	}

	if detailed {
		// Detailed recommendations
		if metrics.P95 > 95 {
			recommendations = append(recommendations, "🚨 Critical P95 usage (>95%) - Immediate action required")
		}

		if metrics.P99 > 98 {
			recommendations = append(recommendations, "🚨 Extreme P99 usage (>98%) - Emergency capacity planning needed")
		}

		// Trend analysis
		if metrics.P90 > metrics.P50*1.5 {
			recommendations = append(recommendations, "📈 High P90/P50 ratio - Consider burst-capable resources")
		}
	}

	return recommendations
}

// VMProfile represents a VM's workload profile and recommendations.
type VMProfile struct {
	WorkloadType    string
	Pattern         string
	Criticality     string
	CPUBuffer       float64
	MemoryBuffer    float64
	Recommendations []string
}

// AnalyzeVMProfile analyzes a VM's workload profile and provides recommendations.
func (b *AdvancedBalancer) AnalyzeVMProfile(vm models.VM, nodeName string) VMProfile {
	profile := VMProfile{
		WorkloadType: "Unknown",
		Pattern:      "Unknown",
		Criticality:  "Normal",
		CPUBuffer:    50.0, // Default buffer
		MemoryBuffer: 50.0,
	}

	// Get VM's load profile
	loadProfile, exists := b.loadProfiles[vm.ID]
	if exists {
		// Analyze CPU pattern
		switch loadProfile.CPUPattern.Type {
		case "burst":
			profile.WorkloadType = "Burst"
			profile.Pattern = "CPU Burst"
			profile.CPUBuffer = 70.0 // High buffer for burst workloads
			profile.Recommendations = append(profile.Recommendations, "High CPU buffer (70%) recommended for burst workloads")
		case "sustained":
			profile.WorkloadType = "Sustained"
			profile.Pattern = "CPU Sustained"
			profile.CPUBuffer = 30.0 // Lower buffer for sustained workloads
			profile.Recommendations = append(profile.Recommendations, "Moderate CPU buffer (30%) sufficient for sustained workloads")
		case "idle":
			profile.WorkloadType = "Idle"
			profile.Pattern = "CPU Idle"
			profile.CPUBuffer = 20.0 // Minimal buffer for idle workloads
			profile.Recommendations = append(profile.Recommendations, "Minimal CPU buffer (20%) for idle workloads")
		}

		// Analyze memory pattern
		switch loadProfile.MemoryPattern.Type {
		case "static":
			profile.MemoryBuffer = 30.0
			profile.Recommendations = append(profile.Recommendations, "Static memory usage - minimal buffer (30%) sufficient")
		case "growing":
			profile.MemoryBuffer = 50.0
			profile.Recommendations = append(profile.Recommendations, "Growing memory usage - moderate buffer (50%) recommended")
		case "volatile":
			profile.MemoryBuffer = 60.0
			profile.Recommendations = append(profile.Recommendations, "Volatile memory usage - high buffer (60%) recommended")
		}

		// Analyze priority and criticality
		switch loadProfile.Priority {
		case models.PriorityRealtime:
			profile.Criticality = criticalityLevelCritical
			profile.CPUBuffer += 20.0 // Extra buffer for realtime
			profile.Recommendations = append(profile.Recommendations, "Realtime priority - extra CPU buffer recommended")
		case models.PriorityInteractive:
			profile.Criticality = "Important"
			profile.CPUBuffer += 10.0 // Extra buffer for interactive
			profile.Recommendations = append(profile.Recommendations, "Interactive priority - moderate extra buffer recommended")
		}

		switch loadProfile.Criticality {
		case models.CriticalityCritical:
			profile.Criticality = criticalityLevelCritical
			profile.Recommendations = append(profile.Recommendations, "Critical VM - ensure high availability and redundancy")
		case models.CriticalityImportant:
			profile.Criticality = "Important"
			profile.Recommendations = append(profile.Recommendations, "Important VM - monitor closely and ensure adequate resources")
		}
	} else {
		// Fallback analysis based on VM tags and type
		profile.WorkloadType = "Standard"
		profile.Pattern = "Unknown (no historical data)"

		// Analyze tags for hints
		for _, tag := range vm.Tags {
			if strings.Contains(tag, "critical") || strings.Contains(tag, "essential") {
				profile.Criticality = criticalityLevelCritical
				profile.CPUBuffer = 70.0
				profile.MemoryBuffer = 60.0
				profile.Recommendations = append(profile.Recommendations, "Critical VM based on tags - high buffer recommended")
			} else if strings.Contains(tag, "web") || strings.Contains(tag, "app") {
				profile.WorkloadType = "Web/Application"
				profile.CPUBuffer = 50.0
				profile.MemoryBuffer = 40.0
				profile.Recommendations = append(profile.Recommendations, "Web/Application VM - moderate buffer recommended")
			} else if strings.Contains(tag, "db") || strings.Contains(tag, "database") {
				profile.WorkloadType = "Database"
				profile.CPUBuffer = 40.0
				profile.MemoryBuffer = 50.0
				profile.Recommendations = append(profile.Recommendations, "Database VM - memory-focused buffer recommended")
			}
		}

		profile.Recommendations = append(profile.Recommendations, "No historical data available - using tag-based analysis")
	}

	// Cap buffers at 100%
	if profile.CPUBuffer > 100.0 {
		profile.CPUBuffer = 100.0
	}
	if profile.MemoryBuffer > 100.0 {
		profile.MemoryBuffer = 100.0
	}

	return profile
}

// GetClusterRecommendations provides cluster-wide capacity planning recommendations.
func (b *AdvancedBalancer) GetClusterRecommendations(forecastDuration time.Duration) []string {
	var recommendations []string

	// Get all nodes
	nodes, err := b.client.GetNodes()
	if err != nil {
		recommendations = append(recommendations, "Unable to get cluster data for recommendations")
		return recommendations
	}

	// Analyze cluster-wide patterns
	nodesWithData := 0
	highUsageNodes := 0
	lowUsageNodes := 0

	for i := range nodes {
		node := &nodes[i]
		_, exists := b.capacityMetrics[node.Name]
		if exists {
			nodesWithData++

			// Predict future usage
			predictedCPU := b.PredictResourceEvolution(node.Name, "cpu", forecastDuration)
			predictedMemory := b.PredictResourceEvolution(node.Name, "memory", forecastDuration)

			if predictedCPU > 90 || predictedMemory > 90 {
				highUsageNodes++
			} else if predictedCPU < 30 && predictedMemory < 30 {
				lowUsageNodes++
			}
		}
	}

	// Generate cluster-wide recommendations
	if nodesWithData == 0 {
		recommendations = append(recommendations, "⚠️  No historical data available for cluster analysis")
		return recommendations
	}

	highUsagePercentage := float64(highUsageNodes) / float64(nodesWithData) * 100
	lowUsagePercentage := float64(lowUsageNodes) / float64(nodesWithData) * 100

	if highUsagePercentage > 50 {
		recommendations = append(recommendations, "🚨 High predicted usage on majority of nodes - consider cluster expansion")
	} else if highUsagePercentage > 25 {
		recommendations = append(recommendations, "⚠️  Elevated predicted usage on significant portion of nodes - plan for capacity expansion")
	}

	if lowUsagePercentage > 50 {
		recommendations = append(recommendations, "💡 Low predicted usage on majority of nodes - consider VM consolidation")
	}

	// Resource distribution recommendations
	recommendations = append(recommendations, "📊 Monitor resource distribution across nodes for optimal balance")
	recommendations = append(recommendations, "🔄 Regular capacity planning reviews recommended")

	// Forecast-specific recommendations
	weeks := forecastDuration.Hours() / (7 * 24)
	if weeks > 4 {
		recommendations = append(recommendations, "📈 Long-term forecast - consider seasonal patterns and growth trends")
	}

	return recommendations
}
