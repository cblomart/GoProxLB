package proxmox

import "github.com/cblomart/GoProxLB/internal/models"

// ClientInterface defines the interface for Proxmox API operations
type ClientInterface interface {
	GetClusterInfo() (*models.Cluster, error)
	GetNodes() ([]models.Node, error)
	MigrateVM(vmID int, sourceNode, targetNode string) error
	GetNodeHistoricalData(nodeName string, timeframe string) ([]HistoricalMetric, error)
	GetVMHistoricalData(nodeName string, vmID int, vmType string, timeframe string) ([]HistoricalMetric, error)
}
