package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
proxmox:
  host: "https://test-host:8006"
  username: "test-user"
  password: "test-pass"
  insecure: true

cluster:
  name: "test-cluster"
  maintenance_nodes: ["node1", "node2"]

balancing:
  enabled: true
  interval: "10m"
  thresholds:
    cpu: 75
    memory: 80
    storage: 85
  weights:
    cpu: 1.0
    memory: 1.0
    storage: 0.5

logging:
  level: "debug"
  format: "text"
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(configContent); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Load config
	config, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test Proxmox config
	if config.Proxmox.Host != "https://test-host:8006" {
		t.Errorf("Expected host 'https://test-host:8006', got '%s'", config.Proxmox.Host)
	}
	if config.Proxmox.Username != "test-user" {
		t.Errorf("Expected username 'test-user', got '%s'", config.Proxmox.Username)
	}
	if config.Proxmox.Password != "test-pass" {
		t.Errorf("Expected password 'test-pass', got '%s'", config.Proxmox.Password)
	}
	if !config.Proxmox.Insecure {
		t.Error("Expected insecure to be true")
	}

	// Test cluster config
	if config.Cluster.Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got '%s'", config.Cluster.Name)
	}
	if len(config.Cluster.MaintenanceNodes) != 2 {
		t.Errorf("Expected 2 maintenance nodes, got %d", len(config.Cluster.MaintenanceNodes))
	}

	// Test balancing config - balancing is always enabled when running
	if config.Balancing.Interval != "10m" {
		t.Errorf("Expected interval '10m', got '%s'", config.Balancing.Interval)
	}
	if config.Balancing.Thresholds.CPU != 75 {
		t.Errorf("Expected CPU threshold 75, got %d", config.Balancing.Thresholds.CPU)
	}

	// Test logging config
	if config.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.Logging.Level)
	}
	if config.Logging.Format != "text" {
		t.Errorf("Expected log format 'text', got '%s'", config.Logging.Format)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Create a minimal config file
	configContent := `
proxmox:
  host: "https://test-host:8006"
  username: "test-user"
  password: "test-pass"

cluster:
  name: "test-cluster"
`

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(configContent); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Load config
	config, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test defaults
	if !config.Proxmox.Insecure {
		t.Error("Expected insecure to be true by default for localhost")
	}
	// Balancing is always enabled when running
	if config.Balancing.Interval != "5m" {
		t.Errorf("Expected default interval '5m', got '%s'", config.Balancing.Interval)
	}
	if config.Balancing.Thresholds.CPU != 80 {
		t.Errorf("Expected default CPU threshold 80, got %d", config.Balancing.Thresholds.CPU)
	}
	if config.Logging.Level != "info" {
		t.Errorf("Expected default log level 'info', got '%s'", config.Logging.Level)
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Host:     "https://test-host:8006",
					Username: "test-user",
					Password: "test-pass",
				},
				Cluster: ClusterConfig{
					Name: "test-cluster",
				},
				Balancing: BalancingConfig{
					BalancerType:   "advanced",
					Aggressiveness: "low",
					Thresholds: ResourceThresholds{
						CPU:     80,
						Memory:  85,
						Storage: 90,
					},
					Weights: ResourceWeights{
						CPU:     1.0,
						Memory:  1.0,
						Storage: 0.5,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Username: "test-user",
					Password: "test-pass",
				},
				Cluster: ClusterConfig{
					Name: "test-cluster",
				},
			},
			wantErr: true,
		},
		{
			name: "missing authentication",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Host: "https://test-host:8006",
				},
				Cluster: ClusterConfig{
					Name: "test-cluster",
				},
			},
			wantErr: true,
		},
		{
			name: "missing cluster name",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Host:     "https://test-host:8006",
					Username: "test-user",
					Password: "test-pass",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid CPU threshold",
			config: &Config{
				Proxmox: ProxmoxConfig{
					Host:     "https://test-host:8006",
					Username: "test-user",
					Password: "test-pass",
				},
				Cluster: ClusterConfig{
					Name: "test-cluster",
				},
				Balancing: BalancingConfig{
					Thresholds: ResourceThresholds{
						CPU: 150, // Invalid
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetInterval(t *testing.T) {
	config := &Config{
		Balancing: BalancingConfig{
			Interval: "5m",
		},
	}

	interval, err := config.GetInterval()
	if err != nil {
		t.Fatalf("Failed to get interval: %v", err)
	}

	if interval != 5*60*time.Second {
		t.Errorf("Expected interval 5m, got %v", interval)
	}
}
