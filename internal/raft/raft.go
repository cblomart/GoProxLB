// Package raft implements distributed consensus for GoProxLB clustering.
package raft

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// RaftPeer represents a Raft peer with both ID and address.
type RaftPeer struct {
	NodeID  string
	Address string
}

// RaftNode represents a Raft node for leader election.
type RaftNode struct {
	raft       *raft.Raft
	nodeID     string
	address    string
	dataDir    string
	peers      []RaftPeer
	leaderChan chan bool
	shutdownCh chan struct{}
}

// NewRaftNode creates a new Raft node for leader election (backward compatibility).
func NewRaftNode(nodeID, address, dataDir string, peers []string) (*RaftNode, error) {
	// Convert string peers to RaftPeer format for backward compatibility
	var raftPeers []RaftPeer
	for _, peer := range peers {
		raftPeers = append(raftPeers, RaftPeer{
			NodeID:  peer, // Use address as ID (old behavior)
			Address: peer,
		})
	}
	return NewRaftNodeWithPeers(nodeID, address, dataDir, raftPeers)
}

// NewRaftNodeWithPeers creates a new Raft node with proper peer information.
func NewRaftNodeWithPeers(nodeID, address, dataDir string, peers []RaftPeer) (*RaftNode, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Create Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)
	config.SnapshotInterval = 30 * time.Second
	config.SnapshotThreshold = 1000
	config.HeartbeatTimeout = 1000 * time.Millisecond
	config.ElectionTimeout = 1000 * time.Millisecond
	config.CommitTimeout = 500 * time.Millisecond
	config.MaxAppendEntries = 64
	config.ShutdownOnRemove = false

	// Create transport
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	transport, err := raft.NewTCPTransport(address, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create log store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	// Create stable store
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "stable.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create stable store: %w", err)
	}

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(dataDir, 3, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create FSM (Finite State Machine)
	fsm := &LoadBalancerFSM{}

	// Create Raft instance
	r, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %w", err)
	}

	// Bootstrap the cluster if this is the first node
	fmt.Printf("DEBUG: Raft bootstrap - peers count: %d\n", len(peers))
	for i, peer := range peers {
		fmt.Printf("DEBUG: Raft bootstrap - peer %d: %s\n", i, peer)
	}

	if len(peers) == 0 {
		fmt.Printf("DEBUG: Bootstrapping single-node cluster\n")
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		r.BootstrapCluster(configuration)
	} else {
		fmt.Printf("DEBUG: Bootstrapping multi-node cluster with %d peers\n", len(peers))
		// Add this node to the cluster
		servers := []raft.Server{
			{
				ID:      config.LocalID,
				Address: transport.LocalAddr(),
			},
		}

		// Add other peers
		for _, peer := range peers {
			fmt.Printf("DEBUG: Adding peer to bootstrap: NodeID=%s, Address=%s\n", peer.NodeID, peer.Address)
			servers = append(servers, raft.Server{
				ID:      raft.ServerID(peer.NodeID),
				Address: raft.ServerAddress(peer.Address),
			})
		}

		fmt.Printf("DEBUG: Final server configuration has %d servers\n", len(servers))
		for i, server := range servers {
			fmt.Printf("DEBUG: Server %d: ID=%s, Address=%s\n", i, server.ID, server.Address)
		}

		configuration := raft.Configuration{Servers: servers}
		r.BootstrapCluster(configuration)
	}

	return &RaftNode{
		raft:       r,
		nodeID:     nodeID,
		address:    address,
		dataDir:    dataDir,
		peers:      peers,
		leaderChan: make(chan bool, 1),
		shutdownCh: make(chan struct{}),
	}, nil
}

// Start starts the Raft node and begins leader election monitoring.
func (r *RaftNode) Start() error {
	// Start monitoring leader changes
	go r.monitorLeaderChanges()

	// Wait for the Raft node to be ready
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("raft node failed to start within timeout")
		case <-ticker.C:
			if r.raft.State() == raft.Leader || r.raft.State() == raft.Follower {
				return nil
			}
		}
	}
}

// Stop stops the Raft node.
func (r *RaftNode) Stop() error {
	close(r.shutdownCh)

	// Wait for the Raft node to shut down with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start shutdown in a goroutine
	errChan := make(chan error, 1)
	go func() {
		future := r.raft.Shutdown()
		errChan <- future.Error()
	}()

	// Wait for shutdown to complete or timeout
	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("raft shutdown timeout")
	}
}

// IsLeader returns true if this node is the current leader.
func (r *RaftNode) IsLeader() bool {
	return r.raft.State() == raft.Leader
}

// GetLeader returns the current leader's address.
func (r *RaftNode) GetLeader() string {
	return string(r.raft.Leader())
}

// GetState returns the current Raft state.
func (r *RaftNode) GetState() raft.RaftState {
	return r.raft.State()
}

// GetPeers returns the list of peers.
func (r *RaftNode) GetPeers() []RaftPeer {
	return r.peers
}

// WaitForLeader waits for a leader to be elected.
func (r *RaftNode) WaitForLeader(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if r.raft.Leader() != "" {
				return nil
			}
		}
	}
}

// monitorLeaderChanges monitors for leader changes and notifies via channel.
func (r *RaftNode) monitorLeaderChanges() {
	var lastLeader string
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.shutdownCh:
			return
		case <-ticker.C:
			currentLeader := string(r.raft.Leader())
			if currentLeader != lastLeader {
				lastLeader = currentLeader
				isLeader := r.raft.State() == raft.Leader

				select {
				case r.leaderChan <- isLeader:
				default:
					// Channel is full, skip this update
				}
			}
		}
	}
}

// GetLeaderChan returns a channel that receives leader status changes.
func (r *RaftNode) GetLeaderChan() <-chan bool {
	return r.leaderChan
}

// LoadBalancerFSM implements the Raft FSM interface.
type LoadBalancerFSM struct {
	// This is a minimal FSM since we only need leader election
	// No actual state changes are needed for load balancer coordination
}

// Apply applies a log entry to the FSM.
func (f *LoadBalancerFSM) Apply(log *raft.Log) interface{} {
	// For load balancer coordination, we don't need to apply any logs
	// The leader election is handled by Raft itself
	return nil
}

// Snapshot creates a snapshot of the FSM.
func (f *LoadBalancerFSM) Snapshot() (raft.FSMSnapshot, error) {
	// Return a minimal snapshot since we don't have state to snapshot
	return &LoadBalancerSnapshot{}, nil
}

// Restore restores the FSM from a snapshot.
func (f *LoadBalancerFSM) Restore(rc io.ReadCloser) error {
	// No state to restore
	if rc != nil {
		return rc.Close()
	}
	return nil
}

// LoadBalancerSnapshot implements the FSMSnapshot interface.
type LoadBalancerSnapshot struct{}

// Persist persists the snapshot.
func (s *LoadBalancerSnapshot) Persist(sink raft.SnapshotSink) error {
	// No state to persist
	if sink != nil {
		return sink.Close()
	}
	return nil
}

// Release releases the snapshot.
func (s *LoadBalancerSnapshot) Release() {
	// Nothing to release
}
