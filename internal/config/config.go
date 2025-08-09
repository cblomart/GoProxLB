// Package config handles configuration loading and validation for GoProxLB.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/spf13/viper"
)

// Config represents the application configuration.
type Config struct {
	Proxmox   ProxmoxConfig   `mapstructure:"proxmox"`
	Cluster   ClusterConfig   `mapstructure:"cluster"`
	Balancing BalancingConfig `mapstructure:"balancing"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Raft      RaftConfig      `mapstructure:"raft"`
}

// ProxmoxConfig holds Proxmox connection settings.
type ProxmoxConfig struct {
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Token    string `mapstructure:"token"`
	Insecure bool   `mapstructure:"insecure"`
}

// ClusterConfig holds cluster-specific settings.
type ClusterConfig struct {
	Name             string   `mapstructure:"name"`
	MaintenanceNodes []string `mapstructure:"maintenance_nodes"`
}

// BalancingConfig holds load balancing configuration.
type BalancingConfig struct {
	Interval       string             `mapstructure:"interval"`
	BalancerType   string             `mapstructure:"balancer_type"`  // "threshold" or "advanced"
	Aggressiveness string             `mapstructure:"aggressiveness"` // low, medium, high
	Cooldown       string             `mapstructure:"cooldown"`       // Duration string (e.g., "2h") - now linked to aggressiveness
	Thresholds     ResourceThresholds `mapstructure:"thresholds"`
	Weights        ResourceWeights    `mapstructure:"weights"`

	// Advanced features
	LoadProfiles LoadProfilesConfig `mapstructure:"load_profiles"`
	Capacity     CapacityConfig     `mapstructure:"capacity"`
}

// ResourceThresholds defines when to trigger rebalancing.
type ResourceThresholds struct {
	CPU     int `mapstructure:"cpu"`
	Memory  int `mapstructure:"memory"`
	Storage int `mapstructure:"storage"`
}

// ResourceWeights defines the importance of each resource type.
type ResourceWeights struct {
	CPU     float64 `mapstructure:"cpu"`
	Memory  float64 `mapstructure:"memory"`
	Storage float64 `mapstructure:"storage"`
}

// LoadProfilesConfig holds load profiling settings.
type LoadProfilesConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Window  string `mapstructure:"window"` // Duration string (e.g., "24h")
}

// CapacityConfig holds capacity planning settings.
type CapacityConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Forecast string `mapstructure:"forecast"` // Duration string (e.g., "7d")
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// RaftConfig holds Raft leader election configuration.
type RaftConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	NodeID       string   `mapstructure:"node_id"`
	Address      string   `mapstructure:"address"`
	DataDir      string   `mapstructure:"data_dir"`
	Peers        []string `mapstructure:"peers"`
	AutoDiscover bool     `mapstructure:"auto_discover"` // Auto-discover peers from Proxmox cluster
	Port         int      `mapstructure:"port"`          // Raft communication port
}

// Load reads configuration from file.
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Set defaults
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// LoadDefault creates a default configuration with sensible defaults.
func LoadDefault() (*Config, error) {
	// Set up viper with defaults
	viper.Reset()
	setDefaults()

	// Create a default config
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal default config: %w", err)
	}

	// Validate the default config
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("default config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values.
func setDefaults() {
	// Set Proxmox defaults
	viper.SetDefault("proxmox.host", "https://localhost:8006")
	viper.SetDefault("proxmox.username", "")
	viper.SetDefault("proxmox.password", "")
	viper.SetDefault("proxmox.token", "")
	viper.SetDefault("proxmox.insecure", true) // Allow self-signed certs for localhost by default

	// Set cluster defaults
	viper.SetDefault("cluster.name", "pve")
	viper.SetDefault("cluster.maintenance_nodes", []string{})

	// Set balancing defaults - SIMPLIFIED for MLP
	viper.SetDefault("balancing.interval", "5m")
	viper.SetDefault("balancing.balancer_type", "advanced") // Advanced by default
	viper.SetDefault("balancing.aggressiveness", "low")     // LOW by default - trust must be earned
	// Note: cooldown is now linked to aggressiveness level, not set here

	// Set threshold defaults (for threshold balancer - kept for compatibility)
	viper.SetDefault("balancing.thresholds.cpu", 80)
	viper.SetDefault("balancing.thresholds.memory", 85)
	viper.SetDefault("balancing.thresholds.storage", 90)

	// Set weight defaults (for advanced balancer - SIMPLIFIED)
	viper.SetDefault("balancing.weights.cpu", 1.0)
	viper.SetDefault("balancing.weights.memory", 1.0)
	viper.SetDefault("balancing.weights.storage", 0.5)

	// Set advanced features defaults - ENABLED by default
	viper.SetDefault("balancing.load_profiles.enabled", true)
	viper.SetDefault("balancing.load_profiles.window", "24h")
	viper.SetDefault("balancing.capacity.enabled", true)
	viper.SetDefault("balancing.capacity.forecast", "168h") // 7 days

	// Set aggressiveness level defaults - CONSERVATIVE by default
	viper.SetDefault("balancing.aggressiveness_levels.low.capacity_weight", 0.2)
	viper.SetDefault("balancing.aggressiveness_levels.medium.capacity_weight", 0.5)
	viper.SetDefault("balancing.aggressiveness_levels.high.capacity_weight", 0.8)

	// Set Raft defaults for distributed mode
	viper.SetDefault("raft.enabled", false)                // Single-node mode by default
	viper.SetDefault("raft.node_id", "")                   // Auto-detected if empty
	viper.SetDefault("raft.address", "0.0.0.0")            // Listen on all interfaces
	viper.SetDefault("raft.data_dir", "/var/lib/goproxlb") // Standard system directory
	viper.SetDefault("raft.auto_discover", true)           // Enable auto-discovery by default
	viper.SetDefault("raft.port", 7946)                    // Standard Serf port
	viper.SetDefault("raft.peers", []string{})

	// Set logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
}

// validateConfig validates the configuration.
func validateConfig(config *Config) error {
	if err := validateProxmoxConfig(&config.Proxmox); err != nil {
		return err
	}

	if err := validateBalancingConfig(&config.Balancing); err != nil {
		return err
	}

	return nil
}

// GetInterval returns the balancing interval as a time.Duration.
func (c *Config) GetInterval() (time.Duration, error) {
	return time.ParseDuration(c.Balancing.Interval)
}

// GetCooldown returns the cooldown period as a time.Duration.
func (c *Config) GetCooldown() (time.Duration, error) {
	return time.ParseDuration(c.Balancing.Cooldown)
}

// GetLoadProfilesWindow returns the load profiles window as a time.Duration.
func (c *Config) GetLoadProfilesWindow() (time.Duration, error) {
	return time.ParseDuration(c.Balancing.LoadProfiles.Window)
}

// GetCapacityForecast returns the capacity forecast period as a time.Duration.
func (c *Config) GetCapacityForecast() (time.Duration, error) {
	return time.ParseDuration(c.Balancing.Capacity.Forecast)
}

// IsAdvancedBalancer returns true if advanced balancer is enabled.
func (c *Config) IsAdvancedBalancer() bool {
	return c.Balancing.BalancerType == "advanced"
}

// GetAggressivenessConfig returns the aggressiveness configuration.
// Cooldown is per-VM: "don't touch this VM because we already moved it less than X ago".
func (c *Config) GetAggressivenessConfig() AggressivenessConfig {
	switch c.Balancing.Aggressiveness {
	case "low":
		return AggressivenessConfig{
			CooldownPeriod:  4 * time.Hour, // 4h cooldown - very conservative
			MinImprovement:  15.0,          // High improvement threshold
			StabilityWeight: 0.8,           // High stability weight
			CapacityWeight:  0.2,           // Conservative capacity planning
		}
	case "high":
		return AggressivenessConfig{
			CooldownPeriod:  30 * time.Minute, // 30m cooldown - aggressive
			MinImprovement:  5.0,              // Low improvement threshold
			StabilityWeight: 0.4,              // Lower stability weight
			CapacityWeight:  0.8,              // Aggressive capacity planning
		}
	default: // medium
		return AggressivenessConfig{
			CooldownPeriod:  2 * time.Hour, // 2h cooldown - balanced
			MinImprovement:  10.0,          // Medium improvement threshold
			StabilityWeight: 0.6,           // Balanced stability weight
			CapacityWeight:  0.5,           // Balanced capacity planning
		}
	}
}

// AggressivenessConfig holds aggressiveness-specific settings.
type AggressivenessConfig struct {
	CooldownPeriod  time.Duration
	MinImprovement  float64
	StabilityWeight float64
	CapacityWeight  float64
}

// AutoDetectClusterName detects the cluster name from Proxmox API.
func (c *Config) AutoDetectClusterName(client interface{}) error {
	if c.Cluster.Name != "" {
		return nil // Already specified
	}

	// Try to get cluster info from Proxmox API
	if proxmoxClient, ok := client.(interface {
		GetClusterInfo() (*models.Cluster, error)
	}); ok {
		cluster, err := proxmoxClient.GetClusterInfo()
		if err != nil {
			return fmt.Errorf("failed to auto-detect cluster name: %w", err)
		}
		c.Cluster.Name = cluster.Name
		return nil
	}

	return fmt.Errorf("cannot auto-detect cluster name: client does not support GetClusterInfo")
}

// validateProxmoxConfig validates the Proxmox configuration.
func validateProxmoxConfig(proxmox *ProxmoxConfig) error {
	if proxmox.Host == "" {
		return fmt.Errorf("proxmox host is required")
	}

	// Allow empty username/password/token for local access
	if !strings.Contains(proxmox.Host, "localhost") && !strings.Contains(proxmox.Host, "127.0.0.1") {
		if proxmox.Username == "" && proxmox.Token == "" {
			return fmt.Errorf("either username/password or token is required for remote access")
		}
	}

	return nil
}

// validateBalancingConfig validates the balancing configuration.
func validateBalancingConfig(balancing *BalancingConfig) error {
	if err := validateBalancerType(balancing.BalancerType); err != nil {
		return err
	}

	if err := validateAggressiveness(balancing.Aggressiveness); err != nil {
		return err
	}

	if err := validateThresholds(&balancing.Thresholds); err != nil {
		return err
	}

	if err := validateWeights(&balancing.Weights); err != nil {
		return err
	}

	if err := validateLoadProfiles(&balancing.LoadProfiles); err != nil {
		return err
	}

	if err := validateCapacityConfig(&balancing.Capacity); err != nil {
		return err
	}

	return nil
}

// validateBalancerType validates the balancer type.
func validateBalancerType(balancerType string) error {
	if balancerType != "threshold" && balancerType != "advanced" {
		return fmt.Errorf("balancer_type must be 'threshold' or 'advanced'")
	}
	return nil
}

// validateAggressiveness validates the aggressiveness setting.
func validateAggressiveness(aggressiveness string) error {
	if aggressiveness != "low" &&
		aggressiveness != "medium" &&
		aggressiveness != "high" {
		return fmt.Errorf("aggressiveness must be 'low', 'medium', or 'high'")
	}
	return nil
}

// validateThresholds validates the threshold values.
func validateThresholds(thresholds *ResourceThresholds) error {
	if thresholds.CPU <= 0 || thresholds.CPU > 100 {
		return fmt.Errorf("CPU threshold must be between 1 and 100")
	}
	if thresholds.Memory <= 0 || thresholds.Memory > 100 {
		return fmt.Errorf("memory threshold must be between 1 and 100")
	}
	if thresholds.Storage <= 0 || thresholds.Storage > 100 {
		return fmt.Errorf("storage threshold must be between 1 and 100")
	}
	return nil
}

// validateWeights validates the weight values.
func validateWeights(weights *ResourceWeights) error {
	if weights.CPU < 0 {
		return fmt.Errorf("CPU weight cannot be negative")
	}
	if weights.Memory < 0 {
		return fmt.Errorf("memory weight cannot be negative")
	}
	if weights.Storage < 0 {
		return fmt.Errorf("storage weight cannot be negative")
	}
	return nil
}

// validateLoadProfiles validates the load profiles configuration.
func validateLoadProfiles(loadProfiles *LoadProfilesConfig) error {
	if loadProfiles.Enabled {
		if _, err := time.ParseDuration(loadProfiles.Window); err != nil {
			return fmt.Errorf("invalid load profiles window duration: %w", err)
		}
	}
	return nil
}

// validateCapacityConfig validates the capacity configuration.
func validateCapacityConfig(capacity *CapacityConfig) error {
	if capacity.Enabled {
		if _, err := time.ParseDuration(capacity.Forecast); err != nil {
			return fmt.Errorf("invalid capacity forecast duration: %w", err)
		}
	}
	return nil
}
