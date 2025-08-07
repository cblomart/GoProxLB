package proxmox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cblomart/GoProxLB/internal/config"
)

// Helper function for encoding JSON in tests
func writeJSON(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// In tests, we can ignore the error or log it
		return
	}
}

// Mock server for testing
func setupMockServer() (*httptest.Server, *config.ProxmoxConfig) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock authentication check
		if r.URL.Path == "/api2/json/access/ticket" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": map[string]interface{}{
					"ticket":              "test-ticket",
					"CSRFPreventionToken": "test-csrf",
				},
			})
			return
		}

		// Mock cluster info
		if r.URL.Path == "/api2/json/cluster/status" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"name": "test-cluster",
						"type": "cluster",
					},
				},
			})
			return
		}

		// Mock nodes
		if r.URL.Path == "/api2/json/nodes" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"node":            "node1",
						"status":          "online",
						"cpu":             4,
						"level":           "",
						"maxcpu":          8,
						"maxmem":          8589934592,
						"mem":             4294967296,
						"ssl_fingerprint": "test-fingerprint",
						"type":            "node",
					},
					{
						"node":            "node2",
						"status":          "online",
						"cpu":             2,
						"level":           "",
						"maxcpu":          8,
						"maxmem":          8589934592,
						"mem":             2147483648,
						"ssl_fingerprint": "test-fingerprint",
						"type":            "node",
					},
				},
			})
			return
		}

		// Mock VMs for node1
		if r.URL.Path == "/api2/json/nodes/node1/qemu" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"vmid":   100,
						"name":   "test-vm-1",
						"status": "running",
						"cpu":    0.1,
						"mem":    1073741824,
						"maxmem": 2147483648,
						"tags":   "plb_affinity_web",
					},
					{
						"vmid":   101,
						"name":   "test-vm-2",
						"status": "running",
						"cpu":    0.2,
						"mem":    2147483648,
						"maxmem": 4294967296,
						"tags":   "plb_anti_affinity_ntp",
					},
				},
			})
			return
		}

		// Mock VMs for node2
		if r.URL.Path == "/api2/json/nodes/node2/qemu" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"vmid":   102,
						"name":   "test-vm-3",
						"status": "running",
						"cpu":    0.1,
						"mem":    1073741824,
						"maxmem": 2147483648,
						"tags":   "plb_affinity_web",
					},
				},
			})
			return
		}

		// Mock storage info
		if r.URL.Path == "/api2/json/nodes/node1/storage" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"storage": "local",
						"type":    "dir",
						"avail":   8589934592,
						"total":   10737418240,
						"used":    2147483648,
					},
				},
			})
			return
		}

		if r.URL.Path == "/api2/json/nodes/node2/storage" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{
					{
						"storage": "local",
						"type":    "dir",
						"avail":   10737418240,
						"total":   10737418240,
						"used":    0,
					},
				},
			})
			return
		}

		// Mock node status
		if r.URL.Path == "/api2/json/nodes/node1/status" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": map[string]interface{}{
					"cpu":     4,
					"maxcpu":  8,
					"mem":     4294967296,
					"maxmem":  8589934592,
					"loadavg": []float64{1.0, 1.0, 1.0},
				},
			})
			return
		}

		if r.URL.Path == "/api2/json/nodes/node2/status" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": map[string]interface{}{
					"cpu":     2,
					"maxcpu":  8,
					"mem":     2147483648,
					"maxmem":  8589934592,
					"loadavg": []float64{0.5, 0.5, 0.5},
				},
			})
			return
		}

		// Mock containers (empty for both nodes)
		if r.URL.Path == "/api2/json/nodes/node1/lxc" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{},
			})
			return
		}

		if r.URL.Path == "/api2/json/nodes/node2/lxc" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": []map[string]interface{}{},
			})
			return
		}

		// Mock migration endpoint
		if r.Method == "POST" && r.URL.Path == "/api2/json/nodes/node1/qemu/100/migrate" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			writeJSON(w, map[string]interface{}{
				"data": "UPID:node1:00000001:00000001:test-migration",
			})
			return
		}

		// Default response
		w.WriteHeader(http.StatusNotFound)
	}))

	cfg := &config.ProxmoxConfig{
		Host:     server.URL,
		Username: "test-user@pve",
		Password: "test-password",
		Insecure: true,
	}

	return server, cfg
}

func TestNewClient(t *testing.T) {
	cfg := &config.ProxmoxConfig{
		Host:     "https://test-host:8006",
		Username: "test-user@pve",
		Password: "test-password",
		Insecure: true,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.host != cfg.Host {
		t.Errorf("Expected host %s, got %s", cfg.Host, client.host)
	}

	if client.username != cfg.Username {
		t.Errorf("Expected username %s, got %s", cfg.Username, client.username)
	}
}

func TestNewClientWithToken(t *testing.T) {
	cfg := &config.ProxmoxConfig{
		Host:     "https://test-host:8006",
		Token:    "test-user@pve!test-token=test-secret",
		Insecure: true,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.token != cfg.Token {
		t.Errorf("Expected token %s, got %s", cfg.Token, client.token)
	}
}

func TestNewClientLocalAccess(t *testing.T) {
	cfg := &config.ProxmoxConfig{
		Host:     "http://localhost:8006",
		Insecure: true,
	}

	client := NewClient(cfg)
	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.host != cfg.Host {
		t.Errorf("Expected host %s, got %s", cfg.Host, client.host)
	}
}

func TestGetClusterInfo(t *testing.T) {
	server, cfg := setupMockServer()
	defer server.Close()

	client := NewClient(cfg)
	info, err := client.GetClusterInfo()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if info.Name != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got %s", info.Name)
	}
}

func TestGetNodes(t *testing.T) {
	server, cfg := setupMockServer()
	defer server.Close()

	client := NewClient(cfg)
	nodes, err := client.GetNodes()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	// Check first node
	node1 := nodes[0]
	if node1.Name != "node1" {
		t.Errorf("Expected node name 'node1', got %s", node1.Name)
	}
	if node1.Status != "online" {
		t.Errorf("Expected status 'online', got %s", node1.Status)
	}
	if node1.CPU.Cores != 0 {
		t.Errorf("Expected 0 CPU cores (not implemented in mock), got %d", node1.CPU.Cores)
	}
	if node1.CPU.Usage != 400.0 {
		t.Errorf("Expected 400%% CPU usage (4 cores out of 8), got %.1f", node1.CPU.Usage)
	}

	// Check VMs
	if len(node1.VMs) != 2 {
		t.Errorf("Expected 2 VMs on node1, got %d", len(node1.VMs))
	}

	vm1 := node1.VMs[0]
	if vm1.ID != 100 {
		t.Errorf("Expected VM ID 100, got %d", vm1.ID)
	}
	if vm1.Name != "test-vm-1" {
		t.Errorf("Expected VM name 'test-vm-1', got %s", vm1.Name)
	}
	if vm1.Status != "running" {
		t.Errorf("Expected VM status 'running', got %s", vm1.Status)
	}
}

func TestGetNodesWithMaintenance(t *testing.T) {
	server, cfg := setupMockServer()
	defer server.Close()

	client := NewClient(cfg)
	nodes, err := client.GetNodes()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// All nodes should be online in our mock
	for _, node := range nodes {
		if node.Status != "online" {
			t.Errorf("Expected node %s to be online, got %s", node.Name, node.Status)
		}
	}
}

func TestMigrateVM(t *testing.T) {
	server, cfg := setupMockServer()
	defer server.Close()

	client := NewClient(cfg)
	err := client.MigrateVM(100, "node1", "node2")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestMigrateVMError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]interface{}{
			"errors": map[string]interface{}{
				"migration": "VM is locked",
			},
		})
	}))
	defer server.Close()

	cfg := &config.ProxmoxConfig{
		Host:     server.URL,
		Username: "test-user@pve",
		Password: "test-password",
		Insecure: true,
	}

	client := NewClient(cfg)
	err := client.MigrateVM(100, "node1", "node2")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestRequestWithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for authentication header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("Expected Authorization header")
		}
		w.WriteHeader(http.StatusOK)
		writeJSON(w, map[string]interface{}{
			"data": "test",
		})
	}))
	defer server.Close()

	cfg := &config.ProxmoxConfig{
		Host:     server.URL,
		Username: "test-user@pve",
		Password: "test-password",
		Insecure: true,
	}

	client := NewClient(cfg)
	_, err := client.request("GET", "/test", nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestRequestWithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for token header
		auth := r.Header.Get("Authorization")
		if auth != "PVEAPIToken=test-user@pve!test-token=test-secret" {
			t.Errorf("Expected token header, got %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		writeJSON(w, map[string]interface{}{
			"data": "test",
		})
	}))
	defer server.Close()

	cfg := &config.ProxmoxConfig{
		Host:     server.URL,
		Token:    "test-user@pve!test-token=test-secret",
		Insecure: true,
	}

	client := NewClient(cfg)
	_, err := client.request("GET", "/test", nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestRequestLocalAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Local access should work without auth headers
		w.WriteHeader(http.StatusOK)
		writeJSON(w, map[string]interface{}{
			"data": "test",
		})
	}))
	defer server.Close()

	cfg := &config.ProxmoxConfig{
		Host:     server.URL,
		Insecure: true,
	}

	client := NewClient(cfg)
	_, err := client.request("GET", "/test", nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.ProxmoxConfig{
		Host:     server.URL,
		Username: "test-user@pve",
		Password: "test-password",
		Insecure: true,
	}

	client := NewClient(cfg)
	_, err := client.GetNodes()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}
