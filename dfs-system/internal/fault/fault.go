package fault

import (
	"fmt"
	"time"

	"dfs-system/internal/types"
)

type FaultManager struct {
	Detector  *Detector
	Heartbeat *HeartbeatSender
}

func NewFaultManager(selfID string, peers []string, peerNodes []*types.Node) *FaultManager {
	detector := NewDetector(6 * time.Second)

	for _, node := range peerNodes {
		detector.RegisterNode(node)
	}

	detector.OnFailure = func(nodeID string) {
		fmt.Printf("[MANAGER]   Node %s confirmed FAILED - recovery needed\n", nodeID)
	}

	hbSender := NewHeartbeatSender(selfID, peers, 2*time.Second)

	return &FaultManager{
		Detector:  detector,
		Heartbeat: hbSender,
	}
}

func (fm *FaultManager) Start() {
	fm.Detector.StartMonitoring()
	fm.Heartbeat.Start()
	fmt.Println("[MANAGER]   Started - heartbeat sender and detector running")
}