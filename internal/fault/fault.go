package fault

import (
	"fmt"
	"strings"
	"time"

	"dfs-system/internal/types"
)

type FaultManager struct {
	Detector  *Detector
	Heartbeat *HeartbeatSender
	Gossip    *GossipManager
	Recovery  *RecoveryManager
}

func NewFaultManager(selfID string, peers []string, peerNodes []*types.Node) *FaultManager {
	detector := NewDetector(6 * time.Second)

	for _, node := range peerNodes {
		detector.RegisterNode(node)
	}

	gossip := NewGossipManager(selfID, 2)

	for i, addr := range peers {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			peerID := fmt.Sprintf("node%d", i+2)
			if i < len(peerNodes) {
				peerID = peerNodes[i].ID
			}
			gossip.RegisterPeer(peerID, addr)
		}
	}

	hbSender := NewHeartbeatSender(selfID, peers, 2*time.Second)
	recovery := NewRecoveryManager(3) // replication factor of 3

	fm := &FaultManager{
		Detector:  detector,
		Heartbeat: hbSender,
		Gossip:    gossip,
		Recovery:  recovery,
	}

	// When a node fails → gossip + trigger recovery
	detector.OnFailure = func(nodeID string) {
		fmt.Printf("[FAULT MANAGER] 🚨 Node %s confirmed FAILED\n", nodeID)
		go fm.Gossip.SpreadFailure(nodeID)
		fm.Recovery.OnNodeFailure(nodeID)
	}

	// When a node comes back → trigger rejoin recovery
	detector.OnRejoin = func(nodeID string) {
		fmt.Printf("[FAULT MANAGER] 💚 Node %s rejoined the cluster\n", nodeID)
		fm.Recovery.OnNodeRejoin(nodeID)
	}

	return fm
}

func (fm *FaultManager) Start() {
	fm.Detector.StartMonitoring()
	fm.Heartbeat.Start()
	fmt.Println("[FAULT MANAGER] Started — heartbeat sender + detector running")
}