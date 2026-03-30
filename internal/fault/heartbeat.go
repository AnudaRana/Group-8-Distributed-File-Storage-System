package fault

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
)

type HeartbeatSender struct {
	mu       sync.RWMutex
	selfID   string
	peers    []string
	interval time.Duration
	count    int64 // total heartbeats sent (atomic)
}

// NewHeartbeatSender spawns an asynchronous worker responsible for perpetually
// alerting the central cluster that this specific local node remains physically healthy.
func NewHeartbeatSender(selfID string, peers []string, interval time.Duration) *HeartbeatSender {
	return &HeartbeatSender{
		selfID:   selfID,
		peers:    peers,
		interval: interval,
	}
}

// AddPeer safely incorporates a new network endpoint into the heartbeat broadcast array.
func (h *HeartbeatSender) AddPeer(peer string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range h.peers {
		if p == peer {
			return
		}
	}
	h.peers = append(h.peers, peer)
}

// RemovePeer purges a specific endpoint string from the heartbeat broadcast array.
func (h *HeartbeatSender) RemovePeer(peer string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	newPeers := make([]string, 0)
	for _, p := range h.peers {
		if p != peer {
			newPeers = append(newPeers, p)
		}
	}
	h.peers = newPeers
}

// GetCount returns the total number of heartbeats sent since startup.
func (h *HeartbeatSender) GetCount() int64 {
	return atomic.LoadInt64(&h.count)
}

// Start spawns the non-terminating background heartbeat pulse routine.
func (h *HeartbeatSender) Start() {
	go func() {
		for {
			time.Sleep(h.interval)
			h.sendToAll()
		}
	}()
}

// sendToAll fires a concurrent broadcast packet hitting all configured peer endpoints
// iteratively to assert dominance and operational health.
func (h *HeartbeatSender) sendToAll() {
	msgBytes, err := types.NewMessage(types.MsgHeartbeat, h.selfID, nil)
	if err != nil {
		fmt.Printf("[HEARTBEAT] Failed to build heartbeat message: %v\n", err)
		return
	}

	h.mu.RLock()
	peersCopy := make([]string, len(h.peers))
	copy(peersCopy, h.peers)
	h.mu.RUnlock()

	for _, peer := range peersCopy {
		url := "http://" + peer + "/message"
		err := transport.Send(url, msgBytes)
		if err != nil {
			fmt.Printf("[HEARTBEAT] Connection failed to reach %s: %v\n", peer, err)
		} else {
			atomic.AddInt64(&h.count, 1)
		}
	}
}
