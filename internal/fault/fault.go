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
}

func NewFaultManager(selfID string, peers []string, peerNodes []*types.Node) *FaultManager {
	detector := NewDetector(6 * time.Second)

	for _, node := range peerNodes {
		detector.RegisterNode(node)
	}

	gossip := NewGossipManager(selfID, 2)

	// Register peers with gossip manager
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

	fm := &FaultManager{
		Detector:  detector,
		Heartbeat: hbSender,
		Gossip:    gossip,
	}

	// When a node fails → gossip to peers automatically
	detector.OnFailure = func(nodeID string) {
		fmt.Printf("[FAULT MANAGER] 🚨 Node %s confirmed FAILED — spreading via gossip\n", nodeID)
		go fm.Gossip.SpreadFailure(nodeID)
		// Phase 4: recovery.OnNodeFailure(nodeID) goes here
	}

	return fm
}

func (fm *FaultManager) Start() {
	fm.Detector.StartMonitoring()
	fm.Heartbeat.Start()
	fmt.Println("[FAULT MANAGER] Started — heartbeat sender + detector running")
}
