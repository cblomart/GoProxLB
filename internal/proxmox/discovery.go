package proxmox

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	nodeStatusOnline = "online"
)

// ClusterNode represents a node in the Proxmox cluster.
type ClusterNode struct {
	NodeID      string `json:"nodeid"`
	Name        string `json:"name"`
	IP          string `json:"ip"`
	Status      string `json:"status"`
	Online      bool   `json:"online"`
	HasGoProxLB bool   `json:"has_goproxlb"`
}

// DiscoveryService handles Proxmox cluster node discovery.
type DiscoveryService struct {
	client ClientInterface
	port   int
}

// NewDiscoveryService creates a new discovery service.
func NewDiscoveryService(client ClientInterface, port int) *DiscoveryService {
	return &DiscoveryService{
		client: client,
		port:   port,
	}
}

// DiscoverClusterNodes discovers all nodes in the Proxmox cluster.
func (d *DiscoveryService) DiscoverClusterNodes() ([]ClusterNode, error) {
	// Get nodes from Proxmox
	proxmoxNodes, err := d.client.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	var nodes []ClusterNode

	// Process each node in the cluster
	for i := range proxmoxNodes {
		node := &proxmoxNodes[i]
		clusterNode := ClusterNode{
			NodeID: node.Name, // Use Name as NodeID since that's what we have
			Name:   node.Name,
			Status: node.Status,
			Online: node.Status == nodeStatusOnline,
		}

		// Try to determine the IP address
		if ip := d.extractIPFromNodeID(node.Name); ip != "" {
			clusterNode.IP = ip
		} else {
			// Fallback: try to resolve the node name
			if resolvedIP := d.resolveNodeName(node.Name); resolvedIP != "" {
				clusterNode.IP = resolvedIP
			}
		}

		// Check if this node has GoProxLB running
		clusterNode.HasGoProxLB = d.checkGoProxLBService(clusterNode.IP)

		nodes = append(nodes, clusterNode)
	}

	return nodes, nil
}

// RaftPeer represents a Raft peer with both ID and address.
type RaftPeer struct {
	NodeID  string
	Address string
}

// GetRaftPeers returns the list of peers for Raft configuration.
func (d *DiscoveryService) GetRaftPeers(currentNodeID string) ([]RaftPeer, error) {
	nodes, err := d.DiscoverClusterNodes()
	if err != nil {
		return nil, err
	}

	var peers []RaftPeer

	for _, node := range nodes {
		// Skip the current node
		if node.NodeID == currentNodeID {
			continue
		}

		// Include all online nodes with valid IPs (assume they will run GoProxLB)
		if node.Online && node.IP != "" {
			peer := RaftPeer{
				NodeID:  node.NodeID,
				Address: fmt.Sprintf("%s:%d", node.IP, d.port),
			}
			peers = append(peers, peer)
		}
	}

	return peers, nil
}

// GetCurrentNodeID determines the current node ID from the Proxmox client.
func (d *DiscoveryService) GetCurrentNodeID() (string, error) {
	// Get the current node from the Proxmox client
	// This assumes the client is connected to the local node
	proxmoxNodes, err := d.client.GetNodes()
	if err != nil {
		return "", fmt.Errorf("failed to get nodes: %w", err)
	}

	// Get the local hostname to match against node names
	hostname, err := d.getLocalHostname()
	if err != nil {
		return "", fmt.Errorf("failed to get local hostname: %w", err)
	}

	// Find the node that matches our hostname
	for i := range proxmoxNodes {
		node := &proxmoxNodes[i]
		if node.Status == nodeStatusOnline && d.hostnameMatches(node.Name, hostname) {
			return node.Name, nil
		}
	}

	// Fallback: if hostname matching fails, try the first online node
	// This maintains backward compatibility but logs a warning
	for i := range proxmoxNodes {
		node := &proxmoxNodes[i]
		if node.Status == nodeStatusOnline {
			fmt.Printf("Warning: Could not match hostname '%s' to any node, using first online node '%s'\n", hostname, node.Name)
			return node.Name, nil
		}
	}

	return "", fmt.Errorf("no online nodes found in cluster")
}

// extractIPFromNodeID tries to extract IP from node ID.
func (d *DiscoveryService) extractIPFromNodeID(nodeID string) string {
	// Common patterns for node IDs that include IPs
	// Example: "pve-192.168.1.10" or "node-10.0.0.5"

	// Try to find IP pattern in node ID
	parts := strings.Split(nodeID, "-")
	for _, part := range parts {
		if net.ParseIP(part) != nil {
			return part
		}
	}

	return ""
}

// resolveNodeName tries to resolve a node name to an IP address.
func (d *DiscoveryService) resolveNodeName(nodeName string) string {
	// Try to resolve the node name to an IP
	ips, err := net.LookupIP(nodeName)
	if err != nil || len(ips) == 0 {
		return ""
	}

	// Return the first IPv4 address
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String()
		}
	}

	// Fallback to IPv6 if no IPv4 found
	if len(ips) > 0 {
		return ips[0].String()
	}

	return ""
}

// checkGoProxLBService checks if GoProxLB is running on a node.
func (d *DiscoveryService) checkGoProxLBService(nodeIP string) bool {
	if nodeIP == "" {
		return false
	}

	// Try to connect to the Raft port on the node
	address := fmt.Sprintf("%s:%d", nodeIP, d.port)

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close() //nolint:errcheck // connection cleanup, error not actionable

	return true
}

// GetNodeAddress returns the full address for a node.
func (d *DiscoveryService) GetNodeAddress(nodeID string) (string, error) {
	nodes, err := d.DiscoverClusterNodes()
	if err != nil {
		return "", err
	}

	for _, node := range nodes {
		if node.NodeID == nodeID && node.IP != "" {
			return fmt.Sprintf("%s:%d", node.IP, d.port), nil
		}
	}

	return "", fmt.Errorf("node %s not found or no IP available", nodeID)
}

// getLocalHostname gets the local hostname for node identification.
func (d *DiscoveryService) getLocalHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	// Remove domain suffix if present (e.g., "proxmox01.lab" -> "proxmox01")
	if idx := strings.Index(hostname, "."); idx >= 0 {
		hostname = hostname[:idx]
	}

	return hostname, nil
}

// hostnameMatches checks if a node name matches the local hostname.
func (d *DiscoveryService) hostnameMatches(nodeName, hostname string) bool {
	// Direct match
	if nodeName == hostname {
		return true
	}

	// Strip domain from node name and try again
	nodeBaseName := nodeName
	if idx := strings.Index(nodeName, "."); idx >= 0 {
		nodeBaseName = nodeName[:idx]
	}

	return nodeBaseName == hostname
}

// ValidateClusterTopology validates the cluster topology for Raft.
func (d *DiscoveryService) ValidateClusterTopology() error {
	nodes, err := d.DiscoverClusterNodes()
	if err != nil {
		return err
	}

	// Count nodes with GoProxLB
	goproxlbNodes := 0
	for _, node := range nodes {
		if node.HasGoProxLB {
			goproxlbNodes++
		}
	}

	// Warn if no other nodes have GoProxLB
	if goproxlbNodes == 0 {
		return fmt.Errorf("no other nodes in the cluster appear to have GoProxLB running")
	}

	// Warn if only one node has GoProxLB (no redundancy)
	if goproxlbNodes == 1 {
		fmt.Println("⚠️  Warning: Only one node has GoProxLB running - no redundancy")
	}

	// Warn if even number of nodes (split-brain risk)
	if goproxlbNodes%2 == 0 {
		fmt.Printf("⚠️  Warning: Even number of GoProxLB nodes (%d) - consider adding one more for optimal quorum\n", goproxlbNodes)
	}

	return nil
}
