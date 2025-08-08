package app

import (
	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/models"
	"github.com/cblomart/GoProxLB/internal/proxmox"
)

// BalancerInterface defines the interface for load balancer operations.
type BalancerInterface interface {
	Run(force bool) ([]models.BalancingResult, error)
	GetClusterStatus() (*models.ClusterStatus, error)
}

// ClientInterface defines the interface for Proxmox API operations.
type ClientInterface interface {
	GetClusterInfo() (*models.Cluster, error)
	GetNodes() ([]models.Node, error)
	MigrateVM(vmID int, sourceNode, targetNode string) error
	GetNodeHistoricalData(nodeName string, timeframe string) ([]proxmox.HistoricalMetric, error)
	GetVMHistoricalData(nodeName string, vmID int, vmType string, timeframe string) ([]proxmox.HistoricalMetric, error)
}

// ConfigLoaderInterface defines the interface for configuration loading.
type ConfigLoaderInterface interface {
	Load(configPath string) (*config.Config, error)
}
