package fault

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
)

// Local constant — avoids modifying shared types/message.go
const msgGossip = "GOSSIP"

type GossipManager struct {
	mu        sync.Mutex
	selfID    string
	peers     map[string]string // nodeID → address (e.g. "127.0.0.1:9002")
	fanout    int               // how many peers to gossip to each round
	rumours   map[string]int    // nodeID → how many times we've gossiped about it
	maxRounds int               // stop gossiping after this many rounds
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

// Register a peer so gossip knows who to tell
func (g *GossipManager) RegisterPeer(nodeID, address string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.peers[nodeID] = address
	fmt.Printf("[GOSSIP] Registered peer %s at %s\n", nodeID, address)
}

// Called when this node detects a failure — spread the news
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

// Called when we receive gossip FROM another node about a failure
func (g *GossipManager) ReceiveGossip(fromNode, failedNodeID string, detector *Detector) {
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

	// Forward the gossip further (epidemic spreading)
	go g.SpreadFailure(failedNodeID)
}

// GetRumourCount returns how many times we gossiped about a node (for testing)
func (g *GossipManager) GetRumourCount(nodeID string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rumours[nodeID]
}

// GetPeerCount returns number of registered peers (for testing)
func (g *GossipManager) GetPeerCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.peers)
}

// ---- Private helpers ----

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