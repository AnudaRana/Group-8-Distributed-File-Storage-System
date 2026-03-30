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
	history   map[string]map[string]bool
	maxRounds int
}

// NewGossipManager prepares an engine that diffuses failure and state events 
// via UDP-style epidemiological peer-to-peer rumor spreading.
func NewGossipManager(selfID string, fanout int) *GossipManager {
	return &GossipManager{
		selfID:    selfID,
		peers:     make(map[string]string),
		fanout:    fanout,
		rumours:   make(map[string]int),
		history:   make(map[string]map[string]bool),
		maxRounds: 3,
	}
}

// RegisterPeer locally caches a peer's identity and network address 
// to be utilized as a potential target within the gossip fanout pattern.
func (g *GossipManager) RegisterPeer(nodeID, address string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peers[nodeID] = address
}

// RemovePeer physically truncates a dead peer out of the routing map 
// so it is no longer inadvertently targeted during rumor propagation bursts.
func (g *GossipManager) RemovePeer(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.peers, nodeID)
}

// SpreadFailure initiates the cascading viral fanout of a node death, 
// informing randomly selected peers until maxRounds threshold is exhausted.
func (g *GossipManager) SpreadFailure(failedNodeID string) {
	g.mu.Lock()
	if g.rumours[failedNodeID] >= g.maxRounds {
		g.mu.Unlock()
		return
	}

	g.rumours[failedNodeID]++
	round := g.rumours[failedNodeID]

	if g.history[failedNodeID] == nil {
		g.history[failedNodeID] = make(map[string]bool)
	}

	targets := g.pickRandomPeers(g.fanout, failedNodeID)
	g.mu.Unlock()

	if len(targets) == 0 {
		return
	}

	fmt.Printf("[GOSSIP] Round %d — spreading news that %s is OFFLINE\n", round, failedNodeID)

	for id, addr := range targets {
		g.mu.Lock()
		g.history[failedNodeID][id] = true
		g.mu.Unlock()

		g.sendGossip(id, addr, failedNodeID)
	}
}

// ReceiveGossip processes an incoming rumor from a peer about a failed node,
// triggering local detector callbacks and re-fanning out if limits permit.
func (g *GossipManager) ReceiveGossip(fromNode, failedNodeID string, detector *Detector) {
	detector.mu.Lock()
	node, exists := detector.nodes[failedNodeID]
	isNewToUs := false
	if exists && node.Status == types.StatusAlive {
		node.Status = types.StatusFailed
		isNewToUs = true
	}
	detector.mu.Unlock()

	if isNewToUs {
		fmt.Printf("[GOSSIP] Confirmed %s is FAILED via gossip from %s\n", failedNodeID, fromNode)
		
		// Re-lock to safely read the node's host/port for the callback
		detector.mu.Lock()
		nodeAddr := ""
		if n, ok := detector.nodes[failedNodeID]; ok {
			nodeAddr = fmt.Sprintf("%s:%d", n.Host, n.Port)
		}
		detector.mu.Unlock()

		if detector.OnFailure != nil {
			go detector.OnFailure(failedNodeID, nodeAddr)
		}
	}

	g.mu.Lock()
	if g.history[failedNodeID] == nil {
		g.history[failedNodeID] = make(map[string]bool)
	}
	g.history[failedNodeID][fromNode] = true

	count := g.rumours[failedNodeID]
	g.mu.Unlock()

	if count < g.maxRounds {
		go g.SpreadFailure(failedNodeID)
	}
}

// pickRandomPeers non-deterministically shuffles the active peer list 
// and isolates n targets ensuring history bounds and self-reflection are avoided.
func (g *GossipManager) pickRandomPeers(n int, failedNodeID string) map[string]string {
	var candidates []string
	for id := range g.peers {

		if id != g.selfID && id != failedNodeID && !g.history[failedNodeID][id] {
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

// sendGossip handles the explicit low-level HTTP transmission of a rumor packet.
func (g *GossipManager) sendGossip(peerID, addr, failedNodeID string) {
	payload := map[string]interface{}{
		"failed_node": failedNodeID,
	}
	msgBytes, _ := types.NewMessage(msgGossip, g.selfID, payload)
	url := "http://" + addr + "/message"

	err := transport.Send(url, msgBytes)
	if err == nil {
		fmt.Printf("[GOSSIP] ✅ Told %s that %s is OFFLINE\n", peerID, failedNodeID)
	}
}

// GetPeerCount returns the number of registered peers. Used in tests.
func (g *GossipManager) GetPeerCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.peers)
}

// GetRumourCount returns how many times a failure has been gossiped. Used in tests.
func (g *GossipManager) GetRumourCount(nodeID string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rumours[nodeID]
}

// GetAllRumourCounts returns the full gossip rumour map (nodeID -> rounds spread).
func (g *GossipManager) GetAllRumourCounts() map[string]int {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := make(map[string]int)
	for k, v := range g.rumours {
		result[k] = v
	}
	return result
}
