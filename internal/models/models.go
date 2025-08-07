package models

import (
	"time"
)

// Node represents a Proxmox node
type Node struct {
	Name          string      `json:"name"`
	Status        string      `json:"status"`
	CPU           CPUInfo     `json:"cpu"`
	Memory        MemoryInfo  `json:"memory"`
	Storage       StorageInfo `json:"storage"`
	VMs           []VM        `json:"vms"`
	InMaintenance bool        `json:"in_maintenance"`
}

// VM represents a virtual machine or container
type VM struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Node      string    `json:"node"`
	Type      string    `json:"type"` // qemu or lxc
	Status    string    `json:"status"`
	CPU       float32   `json:"cpu"`
	Memory    int64     `json:"memory"`
	Tags      []string  `json:"tags"`
	Created   time.Time `json:"created"`
	LastMoved time.Time `json:"last_moved,omitempty"`
	// Load profiling
	LoadProfile *LoadProfile `json:"load_profile,omitempty"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Usage   float32 `json:"usage"` // Percentage
	Cores   int     `json:"cores"`
	Model   string  `json:"model"`
	LoadAvg float32 `json:"load_avg"`
}

// MemoryInfo represents memory information
type MemoryInfo struct {
	Total     int64   `json:"total"`     // Bytes
	Used      int64   `json:"used"`      // Bytes
	Available int64   `json:"available"` // Bytes
	Usage     float32 `json:"usage"`     // Percentage
}

// StorageInfo represents storage information
type StorageInfo struct {
	Total int64   `json:"total"` // Bytes
	Used  int64   `json:"used"`  // Bytes
	Free  int64   `json:"free"`  // Bytes
	Usage float32 `json:"usage"` // Percentage
}

// Cluster represents cluster information
type Cluster struct {
	Name    string `json:"name"`
	Nodes   []Node `json:"nodes"`
	Quorum  bool   `json:"quorum"`
	Version string `json:"version"`
}

// BalancingResult represents the result of a balancing operation
type BalancingResult struct {
	SourceNode   string    `json:"source_node"`
	TargetNode   string    `json:"target_node"`
	VM           VM        `json:"vm"`
	Reason       string    `json:"reason"`
	ResourceGain float64   `json:"resource_gain"`
	Timestamp    time.Time `json:"timestamp"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// NodeScore represents a node's score for VM placement
type NodeScore struct {
	Node    string  `json:"node"`
	Score   float64 `json:"score"`
	CPU     float32 `json:"cpu"`
	Memory  float32 `json:"memory"`
	Storage float32 `json:"storage"`
}

// AffinityGroup represents a group of VMs that should be kept together
type AffinityGroup struct {
	Tag   string   `json:"tag"`
	VMs   []VM     `json:"vms"`
	Nodes []string `json:"nodes"`
}

// AntiAffinityGroup represents a group of VMs that should be distributed
type AntiAffinityGroup struct {
	Tag   string   `json:"tag"`
	VMs   []VM     `json:"vms"`
	Nodes []string `json:"nodes"`
}

// PinnedVM represents a VM pinned to specific nodes
type PinnedVM struct {
	VM    VM       `json:"vm"`
	Nodes []string `json:"nodes"`
}

// IgnoredVM represents a VM that should be ignored by the balancer
type IgnoredVM struct {
	VM   VM       `json:"vm"`
	Tags []string `json:"tags"`
}

// ClusterStatus represents the overall status of the cluster
type ClusterStatus struct {
	TotalNodes       int       `json:"total_nodes"`
	ActiveNodes      int       `json:"active_nodes"`
	TotalVMs         int       `json:"total_vms"`
	RunningVMs       int       `json:"running_vms"`
	AverageCPU       float32   `json:"average_cpu"`
	AverageMemory    float32   `json:"average_memory"`
	AverageStorage   float32   `json:"average_storage"`
	LastBalanced     time.Time `json:"last_balanced"`
	BalancingEnabled bool      `json:"balancing_enabled"`
}

// Migration represents a VM migration operation
type Migration struct {
	VM        VM         `json:"vm"`
	FromNode  string     `json:"from_node"`
	ToNode    string     `json:"to_node"`
	Status    string     `json:"status"` // pending, running, completed, failed
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// LoadProfile represents the load characteristics of a VM
type LoadProfile struct {
	// Mandatory parameters
	CPUPattern     CPUPattern     `json:"cpu_pattern"`
	MemoryPattern  MemoryPattern  `json:"memory_pattern"`
	StoragePattern StoragePattern `json:"storage_pattern"`
	Priority       Priority       `json:"priority"`
	Criticality    Criticality    `json:"criticality"`

	// Optional parameters
	NetworkPattern *NetworkPattern `json:"network_pattern,omitempty"`
	Predictability *Predictability `json:"predictability,omitempty"`
	Seasonality    *Seasonality    `json:"seasonality,omitempty"`
	Dependencies   []string        `json:"dependencies,omitempty"`
}

// CPUPattern represents CPU usage patterns
type CPUPattern struct {
	Type           string  `json:"type"`            // burst, sustained, idle
	BurstDuration  float32 `json:"burst_duration"`  // seconds
	BurstFrequency float32 `json:"burst_frequency"` // bursts per hour
	SustainedLevel float32 `json:"sustained_level"` // percentage
}

// MemoryPattern represents memory usage patterns
type MemoryPattern struct {
	Type       string  `json:"type"`        // static, growing, volatile
	GrowthRate float32 `json:"growth_rate"` // MB/hour
	Volatility float32 `json:"volatility"`  // percentage variation
	PeakUsage  float32 `json:"peak_usage"`  // percentage
}

// StoragePattern represents storage usage patterns
type StoragePattern struct {
	Type         string  `json:"type"`          // read-heavy, write-heavy, mixed
	ReadIOPs     int64   `json:"read_iops"`     // IOPS
	WriteIOPs    int64   `json:"write_iops"`    // IOPS
	ReadLatency  float32 `json:"read_latency"`  // ms
	WriteLatency float32 `json:"write_latency"` // ms
}

// NetworkPattern represents network usage patterns
type NetworkPattern struct {
	Bandwidth  float32 `json:"bandwidth"`   // Mbps
	Latency    float32 `json:"latency"`     // ms
	PacketLoss float32 `json:"packet_loss"` // percentage
}

// Predictability represents workload predictability
type Predictability struct {
	Score      float32 `json:"score"`      // 0-1, higher = more predictable
	Confidence float32 `json:"confidence"` // 0-1, confidence in prediction
}

// Seasonality represents workload seasonality
type Seasonality struct {
	Type      string `json:"type"`       // daily, weekly, monthly, none
	PeakTime  string `json:"peak_time"`  // HH:MM format
	PeakDay   string `json:"peak_day"`   // day of week
	PeakMonth string `json:"peak_month"` // month
}

// Priority represents VM priority levels
type Priority string

const (
	PriorityRealtime    Priority = "realtime"
	PriorityInteractive Priority = "interactive"
	PriorityBackground  Priority = "background"
)

// Criticality represents VM criticality levels
type Criticality string

const (
	CriticalityCritical  Criticality = "critical"
	CriticalityImportant Criticality = "important"
	CriticalityNormal    Criticality = "normal"
)

// ResourceIntensity represents resource usage intensity
type ResourceIntensity struct {
	CPUIntensive     bool    `json:"cpu_intensive"`
	MemoryIntensive  bool    `json:"memory_intensive"`
	StorageIntensive bool    `json:"storage_intensive"`
	NetworkIntensive bool    `json:"network_intensive"`
	Score            float32 `json:"score"` // 0-1, overall intensity
}

// PercentileRange represents P90 min/max range
type PercentileRange struct {
	MinP90 float32 `json:"min_p90"` // 10th percentile (ascending)
	MaxP90 float32 `json:"max_p90"` // 90th percentile (descending)
}

// CapacityMetrics represents capacity planning metrics
type CapacityMetrics struct {
	P50    float32 `json:"p50"`     // Median
	P90    float32 `json:"p90"`     // 90th percentile
	P95    float32 `json:"p95"`     // 95th percentile
	P99    float32 `json:"p99"`     // 99th percentile
	MinP90 float32 `json:"min_p90"` // 10th percentile
	MaxP90 float32 `json:"max_p90"` // 90th percentile
	Mean   float32 `json:"mean"`
	StdDev float32 `json:"std_dev"`
}

// TrendAnalysis represents trend analysis results
type TrendAnalysis struct {
	Slope      float32 `json:"slope"`      // Trend slope
	Intercept  float32 `json:"intercept"`  // Y-intercept
	Confidence float32 `json:"confidence"` // Confidence interval
	R2         float32 `json:"r2"`         // R-squared
	Trend      string  `json:"trend"`      // increasing, decreasing, stable
}

// MigrationHistory represents migration history for anti-flip-flop
type MigrationHistory struct {
	VMID      int       `json:"vmid"`
	FromNode  string    `json:"from_node"`
	ToNode    string    `json:"to_node"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

// MigrationPlan represents an optimized migration plan
type MigrationPlan struct {
	Migrations []Migration `json:"migrations"`
	TotalGain  float64     `json:"total_gain"`
	TotalCost  float64     `json:"total_cost"`
	NetBenefit float64     `json:"net_benefit"`
}

// ResourceReservation represents resource reservations
type ResourceReservation struct {
	CPU     float32 `json:"cpu"`
	Memory  float32 `json:"memory"`
	Storage float32 `json:"storage"`
}

// PlacementStrategy represents VM placement strategy
type PlacementStrategy struct {
	RealtimeNodes     []string           `json:"realtime_nodes"`
	InteractiveNodes  []string           `json:"interactive_nodes"`
	BackgroundNodes   []string           `json:"background_nodes"`
	CPUReservation    map[string]float32 `json:"cpu_reservation"`
	MemoryReservation map[string]float32 `json:"memory_reservation"`
}
