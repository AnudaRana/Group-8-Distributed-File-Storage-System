package fault

import (
	"fmt"
	"time"

	"dfs-system/internal/types"
)

type FaultManager struct {
	Detector              *Detector
	Heartbeat             *HeartbeatSender
	Gossip                *GossipManager
	Recovery              *RecoveryManager
	OnNodeFailureCallback func(nodeID string, addr string)
}

// NewFaultManager agglomerates the Detector, Gossip, Heartbeat, and Recovery models
// into a singular facade controlling the lifecycle and robustness operations of the cluster.
func NewFaultManager(selfID string, peers []string, peerNodes []*types.Node, rep ReplicationIntegrator, isLeader func() bool) *FaultManager {
	detector := NewDetector(6 * time.Second)
	gossip := NewGossipManager(selfID, 2)

	for _, node := range peerNodes {
		detector.RegisterNode(node)
		addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
		gossip.RegisterPeer(node.ID, addr)
	}

	hbSender := NewHeartbeatSender(selfID, peers, 2*time.Second)
	recovery := NewRecoveryManager(3, rep, isLeader)

	fm := &FaultManager{
		Detector:  detector,
		Heartbeat: hbSender,
		Gossip:    gossip,
		Recovery:  recovery,
	}

	// When a node fails â†’ gossip + trigger recovery + call orchestrator callback
	detector.OnFailure = func(nodeID string, addr string) {
		fmt.Printf("[FAULT MANAGER] 🚨 Node %s (%s) confirmed FAILED\n", nodeID, addr)
		go fm.Gossip.SpreadFailure(nodeID)
		fm.Recovery.OnNodeFailure(nodeID)
		if fm.OnNodeFailureCallback != nil {
			go fm.OnNodeFailureCallback(nodeID, addr)
		}
	}

	// When a node comes back â†’ trigger rejoin recovery
	detector.OnRejoin = func(nodeID string) {
		fmt.Printf("[FAULT MANAGER] 💚 Node %s rejoined the cluster\n", nodeID)
		fm.Recovery.OnNodeRejoin(nodeID)
	}

	return fm
}

// Start ignites the internal timers corresponding to physical fault detection 
// boundaries and routine heartbeat emitters inside the orchestrated system.
func (fm *FaultManager) Start() {
	fm.Detector.StartMonitoring()
	fm.Heartbeat.Start()
	fmt.Println("[MANAGER]   Started - heartbeat sender and detector running")
}

// AddPeer registers a new peer into all fault management subsystems.
func (fm *FaultManager) AddPeer(node *types.Node) {
	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	fm.Detector.RegisterNode(node)
	fm.Gossip.RegisterPeer(node.ID, addr)
	fm.Heartbeat.AddPeer(addr)
}

// RemovePeer fully evicts a dead node from ALL fault management subsystems.
// nodeID = logical name e.g. "node3"
// addr   = "host:port"  e.g. "127.0.0.1:8003"
// Both are needed because the detector/gossip key on ID, heartbeat sender keys on addr.
func (fm *FaultManager) RemovePeer(nodeID string, addr string) {
	fm.Detector.UnregisterNode(nodeID)
	fm.Gossip.RemovePeer(nodeID)
	fm.Heartbeat.RemovePeer(addr)
}
