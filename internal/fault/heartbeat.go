package fault

import (
	"fmt"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
)

type HeartbeatSender struct {
	selfID   string
	peers    []string
	interval time.Duration
}

func NewHeartbeatSender(selfID string, peers []string, interval time.Duration) *HeartbeatSender {
	return &HeartbeatSender{
		selfID:   selfID,
		peers:    peers,
		interval: interval,
	}
}

func (h *HeartbeatSender) Start() {
	go func() {
		for {
			time.Sleep(h.interval)
			h.sendToAll()
		}
	}()
}

func (h *HeartbeatSender) sendToAll() {
	msgBytes, err := types.NewMessage(types.MsgHeartbeat, h.selfID, nil)
	if err != nil {
		fmt.Printf("[HEARTBEAT] Failed to build heartbeat message: %v\n", err)
		return
	}

	for _, peer := range h.peers {
		url := "http://" + peer + "/message"
		err := transport.Send(url, msgBytes)
		if err != nil {
			// Don't panic — peer might be down, detector will catch it
			fmt.Printf("[HEARTBEAT] Connection failed to reach %s: %v\n", peer, err)
		} else {
			fmt.Printf("[HEARTBEAT] Successfully sent heartbeat to %s\n", peer)
		}
	}
}
