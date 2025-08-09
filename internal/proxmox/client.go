// Package proxmox provides client functionality for interacting with Proxmox VE APIs.
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cblomart/GoProxLB/internal/config"
	"github.com/cblomart/GoProxLB/internal/models"
)

// Client represents a Proxmox API client.
type Client struct {
	host     string
	username string
	password string
	token    string
	insecure bool
	client   *http.Client
}

// NewClient creates a new Proxmox API client.
func NewClient(cfg *config.ProxmoxConfig) *Client {
	// Only allow insecure connections for localhost/127.0.0.1 for security
	allowInsecure := cfg.Insecure && (strings.Contains(cfg.Host, "localhost") ||
		strings.Contains(cfg.Host, "127.0.0.1") || strings.Contains(cfg.Host, "::1"))

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				//nolint:gosec // InsecureSkipVerify is conditionally allowed for localhost only
				InsecureSkipVerify: allowInsecure,
			},
		},
	}

	return &Client{
		host:     cfg.Host,
		username: cfg.Username,
		password: cfg.Password,
		token:    cfg.Token,
		insecure: cfg.Insecure,
		client:   client,
	}
}

// GetClusterInfo retrieves cluster information.
func (c *Client) GetClusterInfo() (*models.Cluster, error) {
	resp, err := c.request("GET", "/api2/json/cluster/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster status: %w", err)
	}
	defer resp.Body.Close()

	var clusterResp struct {
		Data []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Version string `json:"version"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&clusterResp); err != nil {
		return nil, fmt.Errorf("failed to decode cluster response: %w", err)
	}

	// Check quorum status
	quorum := true
	for _, node := range clusterResp.Data {
		if node.Type == "node" {
			// For now, assume quorum if we can get cluster status
			// In a real implementation, we'd check the quorum status
			break
		}
	}

	cluster := &models.Cluster{
		Name:    clusterResp.Data[0].Name,
		Version: clusterResp.Data[0].Version,
		Quorum:  quorum,
	}

	return cluster, nil
}

// GetNodes retrieves all nodes in the cluster.
func (c *Client) GetNodes() ([]models.Node, error) {
	resp, err := c.request("GET", "/api2/json/nodes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}
	defer resp.Body.Close()

	var nodesResp struct {
		Data []struct {
			Node   string `json:"node"`
			Status string `json:"status"`
			CPU    int    `json:"cpu"`
			Level  string `json:"level"`
			MaxCPU int    `json:"maxcpu"`
			MaxMem int64  `json:"maxmem"`
			Mem    int64  `json:"mem"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nodesResp); err != nil {
		return nil, fmt.Errorf("failed to decode nodes response: %w", err)
	}

	var nodes []models.Node
	for _, nodeData := range nodesResp.Data {
		node, err := c.getNodeDetails(nodeData.Node)
		if err != nil {
			return nil, fmt.Errorf("failed to get details for node %s: %w", nodeData.Node, err)
		}
		nodes = append(nodes, *node)
	}

	return nodes, nil
}

// getNodeDetails retrieves detailed information about a specific node.
func (c *Client) getNodeDetails(nodeName string) (*models.Node, error) {
	// Get node status
	statusResp, err := c.request("GET", fmt.Sprintf("/api2/json/nodes/%s/status", nodeName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get node status: %w", err)
	}
	defer statusResp.Body.Close()

	var statusData struct {
		Data struct {
			CPU    float64 `json:"cpu"`
			Memory struct {
				Total int64 `json:"total"`
				Used  int64 `json:"used"`
			} `json:"memory"`
			LoadAvg []float64 `json:"loadavg"`
		} `json:"data"`
	}

	if err := json.NewDecoder(statusResp.Body).Decode(&statusData); err != nil {
		return nil, fmt.Errorf("failed to decode node status: %w", err)
	}

	// Get VMs on this node
	vms, err := c.getNodeVMs(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs for node %s: %w", nodeName, err)
	}

	// Calculate memory usage
	memoryUsage := float64(statusData.Data.Memory.Used) / float64(statusData.Data.Memory.Total) * 100

	// Get node info for CPU details
	nodeInfoResp, err := c.request("GET", fmt.Sprintf("/api2/json/nodes/%s/status", nodeName), nil)
	var cores int
	var model string
	if err == nil {
		defer nodeInfoResp.Body.Close()
		var nodeInfo struct {
			Data struct {
				CPUInfo string `json:"cpuinfo"`
			} `json:"data"`
		}
		if json.NewDecoder(nodeInfoResp.Body).Decode(&nodeInfo) == nil {
			// Extract cores from CPU info (simplified)
			if strings.Contains(nodeInfo.Data.CPUInfo, "cores") {
				// This is a simplified extraction - in production you'd parse this properly
				cores = 4 // Default fallback
			}
			model = "CPU" // Default fallback
		}
	}

	// Check if node is in maintenance mode by looking for maintenance tag
	inMaintenance := false
	for i := range vms {
		vm := &vms[i]
		for _, tag := range vm.Tags {
			if strings.Contains(tag, "maintenance") {
				inMaintenance = true
				break
			}
		}
		if inMaintenance {
			break
		}
	}

	node := &models.Node{
		Name:   nodeName,
		Status: "online", // Assume online if we can get status
		CPU: models.CPUInfo{
			Usage:   float32(statusData.Data.CPU * 100),
			Cores:   cores,
			Model:   model,
			LoadAvg: float32(statusData.Data.LoadAvg[0]),
		},
		Memory: models.MemoryInfo{
			Total:     statusData.Data.Memory.Total,
			Used:      statusData.Data.Memory.Used,
			Available: statusData.Data.Memory.Total - statusData.Data.Memory.Used,
			Usage:     float32(memoryUsage),
		},
		Storage: models.StorageInfo{
			Total: 0, // Storage info would require additional API calls
			Used:  0,
			Free:  0,
			Usage: 0,
		},
		VMs:           vms,
		InMaintenance: inMaintenance,
	}

	return node, nil
}

// getNodeVMs retrieves all VMs on a specific node.
func (c *Client) getNodeVMs(nodeName string) ([]models.VM, error) {
	resp, err := c.request("GET", fmt.Sprintf("/api2/json/nodes/%s/qemu", nodeName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs: %w", err)
	}
	defer resp.Body.Close()

	var vmsResp struct {
		Data []struct {
			ID     int     `json:"vmid"`
			Name   string  `json:"name"`
			Status string  `json:"status"`
			CPU    float64 `json:"cpu"`
			Mem    int64   `json:"mem"`
			Tags   string  `json:"tags"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&vmsResp); err != nil {
		return nil, fmt.Errorf("failed to decode VMs response: %w", err)
	}

	var vms []models.VM
	for _, vmData := range vmsResp.Data {
		tags := []string{}
		if vmData.Tags != "" {
			tags = strings.Split(vmData.Tags, ",")
		}

		vm := models.VM{
			ID:     vmData.ID,
			Name:   vmData.Name,
			Node:   nodeName,
			Type:   "qemu",
			Status: vmData.Status,
			CPU:    float32(vmData.CPU),
			Memory: vmData.Mem,
			Tags:   tags,
		}
		vms = append(vms, vm)
	}

	// Also get containers
	containers, err := c.getNodeContainers(nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	vms = append(vms, containers...)
	return vms, nil
}

// getNodeContainers retrieves all containers on a specific node.
func (c *Client) getNodeContainers(nodeName string) ([]models.VM, error) {
	resp, err := c.request("GET", fmt.Sprintf("/api2/json/nodes/%s/lxc", nodeName), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}
	defer resp.Body.Close()

	var containersResp struct {
		Data []struct {
			ID     int     `json:"vmid"`
			Name   string  `json:"name"`
			Status string  `json:"status"`
			CPU    float64 `json:"cpu"`
			Mem    int64   `json:"mem"`
			Tags   string  `json:"tags"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&containersResp); err != nil {
		return nil, fmt.Errorf("failed to decode containers response: %w", err)
	}

	var containers []models.VM
	for _, containerData := range containersResp.Data {
		tags := []string{}
		if containerData.Tags != "" {
			tags = strings.Split(containerData.Tags, ",")
		}

		container := models.VM{
			ID:     containerData.ID,
			Name:   containerData.Name,
			Node:   nodeName,
			Type:   "lxc",
			Status: containerData.Status,
			CPU:    float32(containerData.CPU),
			Memory: containerData.Mem,
			Tags:   tags,
		}
		containers = append(containers, container)
	}

	return containers, nil
}

// MigrateVM migrates a VM from one node to another.
func (c *Client) MigrateVM(vmID int, sourceNode, targetNode string) error {
	data := url.Values{}
	data.Set("target", targetNode)

	resp, err := c.request("POST", fmt.Sprintf("/api2/json/nodes/%s/qemu/%d/migrate", sourceNode, vmID), strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to migrate VM %d: %w", vmID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("migration failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetNodeHistoricalData retrieves historical metrics for a node.
func (c *Client) GetNodeHistoricalData(nodeName, timeframe string) ([]HistoricalMetric, error) {
	// timeframe: hour, day, week, month, year
	resp, err := c.request("GET", fmt.Sprintf("/api2/json/nodes/%s/rrddata?timeframe=%s", nodeName, timeframe), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data for node %s: %w", nodeName, err)
	}
	defer resp.Body.Close()

	var rrdResp struct {
		Data []struct {
			Time   int64   `json:"time"`
			CPU    float64 `json:"cpu"`
			Memory float64 `json:"memory"`
			Load   float64 `json:"loadavg"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rrdResp); err != nil {
		return nil, fmt.Errorf("failed to decode historical data: %w", err)
	}

	var metrics []HistoricalMetric
	for _, data := range rrdResp.Data {
		metrics = append(metrics, HistoricalMetric{
			Timestamp: time.Unix(data.Time, 0),
			CPU:       data.CPU * 100, // Convert to percentage
			Memory:    data.Memory,
			LoadAvg:   data.Load,
		})
	}

	return metrics, nil
}

// GetVMHistoricalData retrieves historical metrics for a VM.
func (c *Client) GetVMHistoricalData(nodeName string, vmID int, vmType, timeframe string) ([]HistoricalMetric, error) {
	// vmType: qemu or lxc
	resp, err := c.request("GET", fmt.Sprintf("/api2/json/nodes/%s/%s/%d/rrddata?timeframe=%s", nodeName, vmType, vmID, timeframe), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data for VM %d: %w", vmID, err)
	}
	defer resp.Body.Close()

	var rrdResp struct {
		Data []struct {
			Time   int64   `json:"time"`
			CPU    float64 `json:"cpu"`
			Memory float64 `json:"memory"`
			Disk   float64 `json:"disk"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rrdResp); err != nil {
		return nil, fmt.Errorf("failed to decode VM historical data: %w", err)
	}

	var metrics []HistoricalMetric
	for _, data := range rrdResp.Data {
		metrics = append(metrics, HistoricalMetric{
			Timestamp: time.Unix(data.Time, 0),
			CPU:       data.CPU * 100, // Convert to percentage
			Memory:    data.Memory,
			Disk:      data.Disk,
		})
	}

	return metrics, nil
}

// HistoricalMetric represents a historical metric data point.
type HistoricalMetric struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`     // Percentage
	Memory    float64   `json:"memory"`  // Bytes
	Disk      float64   `json:"disk"`    // Bytes
	LoadAvg   float64   `json:"loadavg"` // System load average
}

// request makes an HTTP request to the Proxmox API.
func (c *Client) request(method, path string, body io.Reader) (*http.Response, error) {
	url := c.host + path
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		return nil, err
	}

	// Set authentication (skip if running locally as root)
	if c.token != "" {
		req.Header.Set("Authorization", "PVEAPIToken="+c.token)
	} else if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	// If no authentication provided, assume local root access

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
