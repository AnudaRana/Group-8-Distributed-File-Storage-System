package fault

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
)

const msgGossip = "GOSSIP"

type GossipManager struct {
	mu        sync.Mutex
	selfID    string
	peers     map[string]string
	fanout    int
	rumours   map[string]int
	maxRounds int
}

func NewGossipManager(selfID string, fanout int) *GossipManager {
	return &GossipManager{
		selfID:    selfID,
		peers:     make(map[string]string),
		fanout:    fanout,
		rumours:   make(map[string]int),
		maxRounds: 3,
	}
}

func (g *GossipManager) RegisterPeer(nodeID, address string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peers[nodeID] = address
	fmt.Printf("[GOSSIP] Registered peer %s at %s\n", nodeID, address)
}

func (g *GossipManager) SpreadFailure(failedNodeID string) {
	g.mu.Lock()

	if g.rumours[failedNodeID] >= g.maxRounds {
		g.mu.Unlock()
		fmt.Printf("[GOSSIP] Already spread failure of %s enough times, stopping\n", failedNodeID)
		return
	}

	g.rumours[failedNodeID]++
	round := g.rumours[failedNodeID]
	targets := g.pickRandomPeers(g.fanout, failedNodeID)
	g.mu.Unlock()

	fmt.Printf("[GOSSIP] Round %d — spreading news that %s is OFFLINE to %d peers\n",
		round, failedNodeID, len(targets))

	for peerID, addr := range targets {
		g.sendGossip(peerID, addr, failedNodeID)
	}
}

func (g *GossipManager) ReceiveGossip(fromNode, failedNodeID string, detector *Detector) {
	// Fix 1 — ignore gossip about ourselves
	if failedNodeID == g.selfID {
		fmt.Printf("[GOSSIP] Ignoring gossip about ourselves from %s\n", fromNode)
		return
	}

	// Fix 2 — ignore stale gossip about nodes already back online
	if detector != nil {
		detector.mu.Lock()
		node, exists := detector.nodes[failedNodeID]
		if exists && node.Status == types.StatusAlive {
			detector.mu.Unlock()
			fmt.Printf("[GOSSIP] Ignoring stale gossip — %s is already ONLINE\n", failedNodeID)
			return
		}
		detector.mu.Unlock()
	}

	fmt.Printf("[GOSSIP] Received from %s: node %s is OFFLINE\n", fromNode, failedNodeID)

	if detector != nil {
		detector.mu.Lock()
		if node, ok := detector.nodes[failedNodeID]; ok {
			if node.Status == types.StatusAlive {
				node.Status = types.StatusFailed
				fmt.Printf("[GOSSIP] Marked %s as FAILED based on gossip from %s\n",
					failedNodeID, fromNode)
			}
		}
		detector.mu.Unlock()
	}

	go g.SpreadFailure(failedNodeID)
}

func (g *GossipManager) GetRumourCount(nodeID string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rumours[nodeID]
}

func (g *GossipManager) GetPeerCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.peers)
}

func (g *GossipManager) pickRandomPeers(n int, excludeID string) map[string]string {
	var candidates []string
	for id := range g.peers {
		if id != g.selfID && id != excludeID {
			candidates = append(candidates, id)
		}
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	if n > len(candidates) {
		n = len(candidates)
	}

	result := make(map[string]string)
	for _, id := range candidates[:n] {
		result[id] = g.peers[id]
	}
	return result
}

func (g *GossipManager) sendGossip(peerID, addr, failedNodeID string) {
	payload := map[string]interface{}{
		"failed_node": failedNodeID,
	}

	msgBytes, err := types.NewMessage(msgGossip, g.selfID, payload)
	if err != nil {
		fmt.Printf("[GOSSIP] Failed to build gossip message: %v\n", err)
		return
	}

	url := "http://" + addr + "/message"
	err = transport.Send(url, msgBytes)
	if err != nil {
		fmt.Printf("[GOSSIP] ⚠️  Could not reach %s (%s): %v\n", peerID, addr, err)
	} else {
		fmt.Printf("[GOSSIP] ✅ Told %s that %s is OFFLINE\n", peerID, failedNodeID)
	}
}