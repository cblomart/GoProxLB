package proxmox

import (
	"fmt"
	"testing"

	"github.com/cblomart/GoProxLB/internal/models"
)

// MockClient implements ClientInterface for testing
type MockClient struct {
	clusterInfo *models.Cluster
	nodes       []models.Node
	err         error
}

func (m *MockClient) GetClusterInfo() (*models.Cluster, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.clusterInfo, nil
}

func (m *MockClient) GetNodes() ([]models.Node, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes, nil
}

func (m *MockClient) MigrateVM(vmID int, sourceNode, targetNode string) error {
	return m.err
}

func (m *MockClient) GetNodeHistoricalData(nodeName string, timeframe string) ([]HistoricalMetric, error) {
	return nil, m.err
}

func (m *MockClient) GetVMHistoricalData(nodeName string, vmID int, vmType string, timeframe string) ([]HistoricalMetric, error) {
	return nil, m.err
}

func TestNewDiscoveryService(t *testing.T) {
	mockClient := &MockClient{}
	port := 7946

	service := NewDiscoveryService(mockClient, port)
	if service == nil {
		t.Fatal("Expected discovery service but got nil")
	}

	if service.client != mockClient {
		t.Errorf("Expected client %v, got %v", mockClient, service.client)
	}

	if service.port != port {
		t.Errorf("Expected port %d, got %d", port, service.port)
	}
}

func TestDiscoveryServiceDiscoverClusterNodes(t *testing.T) {
	tests := []struct {
		name        string
		clusterInfo *models.Cluster
		nodes       []models.Node
		err         error
		expectErr   bool
		expectCount int
	}{
		{
			name: "successful discovery",
			clusterInfo: &models.Cluster{
				Name: "test-cluster",
			},
			nodes: []models.Node{
				{Name: "pve-192.168.1.10", Status: "online"},
				{Name: "pve-192.168.1.11", Status: "online"},
				{Name: "pve-192.168.1.12", Status: "offline"},
			},
			err:         nil,
			expectErr:   false,
			expectCount: 3,
		},
		{
			name:        "client error",
			clusterInfo: nil,
			nodes:       nil,
			err:         fmt.Errorf("connection failed"),
			expectErr:   true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{
				clusterInfo: tt.clusterInfo,
				nodes:       tt.nodes,
				err:         tt.err,
			}

			service := NewDiscoveryService(mockClient, 7946)
			nodes, err := service.DiscoverClusterNodes()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(nodes) != tt.expectCount {
				t.Errorf("Expected %d nodes, got %d", tt.expectCount, len(nodes))
			}

			// Verify node properties
			for i, node := range nodes {
				if node.NodeID != tt.nodes[i].Name {
					t.Errorf("Expected NodeID %s, got %s", tt.nodes[i].Name, node.NodeID)
				}
				if node.Name != tt.nodes[i].Name {
					t.Errorf("Expected Name %s, got %s", tt.nodes[i].Name, node.Name)
				}
				if node.Status != tt.nodes[i].Status {
					t.Errorf("Expected Status %s, got %s", tt.nodes[i].Status, node.Status)
				}
				if node.Online != (tt.nodes[i].Status == "online") {
					t.Errorf("Expected Online %v, got %v", tt.nodes[i].Status == "online", node.Online)
				}
			}
		})
	}
}

func TestDiscoveryServiceGetRaftPeers(t *testing.T) {
	mockClient := &MockClient{
		clusterInfo: &models.Cluster{Name: "test-cluster"},
		nodes: []models.Node{
			{Name: "pve-192.168.1.10", Status: "online"},
			{Name: "pve-192.168.1.11", Status: "online"},
			{Name: "pve-192.168.1.12", Status: "offline"},
		},
	}

	service := NewDiscoveryService(mockClient, 7946)

	// Test getting peers for node1
	// Since we can't easily mock the service detection, we'll test the logic
	// by ensuring the function doesn't panic and handles the case properly
	peers, err := service.GetRaftPeers("pve-192.168.1.10")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// The actual number of peers will depend on network connectivity
	// We just verify the function works without errors
	t.Logf("Found %d peers for node pve-192.168.1.10", len(peers))
	
	// Verify no peers contain the current node
	for _, peer := range peers {
		if peer == "192.168.1.10:7946" {
			t.Errorf("Peer list should not include current node")
		}
	}
}

func TestDiscoveryServiceGetCurrentNodeID(t *testing.T) {
	mockClient := &MockClient{
		clusterInfo: &models.Cluster{Name: "test-cluster"},
		nodes: []models.Node{
			{Name: "pve-192.168.1.10", Status: "online"},
			{Name: "pve-192.168.1.11", Status: "offline"},
		},
	}

	service := NewDiscoveryService(mockClient, 7946)

	nodeID, err := service.GetCurrentNodeID()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should return the first online node
	if nodeID != "pve-192.168.1.10" {
		t.Errorf("Expected node ID pve-192.168.1.10, got %s", nodeID)
	}
}

func TestDiscoveryServiceGetCurrentNodeIDNoOnlineNodes(t *testing.T) {
	mockClient := &MockClient{
		clusterInfo: &models.Cluster{Name: "test-cluster"},
		nodes: []models.Node{
			{Name: "pve-192.168.1.10", Status: "offline"},
			{Name: "pve-192.168.1.11", Status: "offline"},
		},
	}

	service := NewDiscoveryService(mockClient, 7946)

	_, err := service.GetCurrentNodeID()
	if err == nil {
		t.Errorf("Expected error for no online nodes but got none")
	}
}

func TestDiscoveryServiceExtractIPFromNodeID(t *testing.T) {
	service := NewDiscoveryService(&MockClient{}, 7946)

	tests := []struct {
		nodeID string
		expect string
	}{
		{"pve-192.168.1.10", "192.168.1.10"},
		{"node-10.0.0.5", "10.0.0.5"},
		{"simple-node", ""},
		{"pve-2001:db8::1", "2001:db8::1"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.nodeID, func(t *testing.T) {
			result := service.extractIPFromNodeID(tt.nodeID)
			if result != tt.expect {
				t.Errorf("Expected %s, got %s", tt.expect, result)
			}
		})
	}
}

func TestDiscoveryServiceResolveNodeName(t *testing.T) {
	service := NewDiscoveryService(&MockClient{}, 7946)

	// Test with localhost (should resolve)
	ip := service.resolveNodeName("localhost")
	if ip == "" {
		t.Logf("Could not resolve localhost, this might be normal in some environments")
	}

	// Test with invalid hostname
	ip = service.resolveNodeName("invalid-hostname-that-should-not-exist")
	if ip != "" {
		t.Errorf("Expected empty string for invalid hostname, got %s", ip)
	}
}

func TestDiscoveryServiceCheckGoProxLBService(t *testing.T) {
	service := NewDiscoveryService(&MockClient{}, 7946)

	// Test with empty IP
	hasService := service.checkGoProxLBService("")
	if hasService {
		t.Errorf("Expected false for empty IP, got true")
	}

	// Test with localhost (should fail to connect to port 7946)
	hasService = service.checkGoProxLBService("127.0.0.1")
	if hasService {
		t.Logf("GoProxLB service detected on localhost:7946, this might be normal if service is running")
	}
}

func TestDiscoveryServiceGetNodeAddress(t *testing.T) {
	mockClient := &MockClient{
		clusterInfo: &models.Cluster{Name: "test-cluster"},
		nodes: []models.Node{
			{Name: "pve-192.168.1.10", Status: "online"},
		},
	}

	service := NewDiscoveryService(mockClient, 7946)

	// Test getting address for existing node
	address, err := service.GetNodeAddress("pve-192.168.1.10")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expected := "192.168.1.10:7946"
	if address != expected {
		t.Errorf("Expected address %s, got %s", expected, address)
	}

	// Test getting address for non-existent node
	_, err = service.GetNodeAddress("non-existent-node")
	if err == nil {
		t.Errorf("Expected error for non-existent node but got none")
	}
}

func TestDiscoveryServiceValidateClusterTopology(t *testing.T) {
	tests := []struct {
		name       string
		nodes      []models.Node
		expectErr  bool
		expectWarn bool
	}{
		{
			name: "no nodes error",
			nodes: []models.Node{},
			expectErr: true,
			expectWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockClient{
				clusterInfo: &models.Cluster{Name: "test-cluster"},
				nodes:       tt.nodes,
			}

			service := NewDiscoveryService(mockClient, 7946)

			err := service.ValidateClusterTopology()

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
