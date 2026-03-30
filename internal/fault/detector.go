package fault

import (
	"fmt"
	"sync"
	"time"

	"dfs-system/internal/types"
)

type Detector struct {
	mu           sync.Mutex
	nodes        map[string]*types.Node
	lastSeen     map[string]time.Time
	registeredAt map[string]time.Time
	timeout      time.Duration
	OnFailure    func(nodeID string, addr string)
	OnRejoin     func(nodeID string)
}

// NewDetector initializes a fault detector that tracks node heartbeats
// and declares failures if the specified timeout duration implies a silent drop.
func NewDetector(timeout time.Duration) *Detector {
	return &Detector{
		nodes:        make(map[string]*types.Node),
		lastSeen:     make(map[string]time.Time),
		registeredAt: make(map[string]time.Time),
		timeout:      timeout,
	}
}

// RegisterNode binds a new node metadata into the monitoring maps
// and resets its active timestamp to the current local time.
func (d *Detector) RegisterNode(node *types.Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[node.ID] = node
	d.lastSeen[node.ID] = time.Now()
	d.registeredAt[node.ID] = time.Now()
	fmt.Printf("[DETECTOR] Registered node %s at %s:%d\n", node.ID, node.Host, node.Port)
}

// UnregisterNode structurally entirely removes a node out of the health monitoring,
// used primarily when purging permanently banned or gracefully exited nodes.
func (d *Detector) UnregisterNode(nodeID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.nodes, nodeID)
	delete(d.lastSeen, nodeID)
	delete(d.registeredAt, nodeID)
}

// RecordHeartbeat refreshes the last-seen tracker for a specific node,
// potentially reviving a previously offline node if it resumes pinging.
func (d *Detector) RecordHeartbeat(nodeID string) {
	d.mu.Lock()

	d.lastSeen[nodeID] = time.Now()

	wasOffline := false
	if node, ok := d.nodes[nodeID]; ok {
		if node.Status == types.StatusFailed {
			wasOffline = true
			fmt.Printf("[DETECTOR] Node %s is back ONLINE\n", nodeID)
		}
		node.Status = types.StatusAlive
	}

	rejoinCallback := d.OnRejoin
	d.mu.Unlock()

	// Fire rejoin callback outside the lock
	if wasOffline && rejoinCallback != nil {
		rejoinCallback(nodeID)
	}
}

// StartMonitoring spawns a background routine that perpetually evaluates
// the temporal status of all active peers against the timeout grace period.
func (d *Detector) StartMonitoring() {
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			d.checkAll()
		}
	}()
}

// checkAll iterates through all active nodes, locking the map and scanning
// for any node whose last heartbeat exceeded the timeout, firing failure hooks.
func (d *Detector) checkAll() {
	d.mu.Lock()

	var justFailed []string
	var failedAddrs []string

	for id, node := range d.nodes {
		last, exists := d.lastSeen[id]
		if exists && time.Since(last) > d.timeout && node.Status == types.StatusAlive {
			// Skip failure marking during the bootstrap grace period (60s after registration)
			if time.Since(d.registeredAt[id]) < 60*time.Second {
				continue
			}

			node.Status = types.StatusFailed
			fmt.Printf("[DETECTOR] âŒ Node %s is now OFFLINE (no heartbeat for %v)\n",
				id, d.timeout)
			justFailed = append(justFailed, id)
			failedAddrs = append(failedAddrs, fmt.Sprintf("%s:%d", node.Host, node.Port))
		}
	}

	d.mu.Unlock()

	for i, id := range justFailed {
		if d.OnFailure != nil {
			d.OnFailure(id, failedAddrs[i])
		}
	}
}

// GetStatuses returns a thread-safe snapshot mapping all recorded node IDs
// to their current health (Alive or Failed).
func (d *Detector) GetStatuses() map[string]types.NodeStatus {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make(map[string]types.NodeStatus)
	for id, node := range d.nodes {
		result[id] = node.Status
	}
	return result
}
