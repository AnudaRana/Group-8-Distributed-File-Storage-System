package fault

import (
	"fmt"
	"sync"
	"time"

	"dfs-system/internal/types"
)

type Detector struct {
	mu        sync.Mutex
	nodes     map[string]*types.Node
	lastSeen  map[string]time.Time
	timeout   time.Duration
	OnFailure func(nodeID string)
	OnRejoin  func(nodeID string)
}

func NewDetector(timeout time.Duration) *Detector {
	return &Detector{
		nodes:    make(map[string]*types.Node),
		lastSeen: make(map[string]time.Time),
		timeout:  timeout,
	}
}

func (d *Detector) RegisterNode(node *types.Node) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[node.ID] = node
	d.lastSeen[node.ID] = time.Now()
	fmt.Printf("[DETECTOR] Registered node %s at %s:%d\n", node.ID, node.Host, node.Port)
}

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

func (d *Detector) StartMonitoring() {
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			d.checkAll()
		}
	}()
}

func (d *Detector) checkAll() {
	d.mu.Lock()

	var justFailed []string

	for id, node := range d.nodes {
		last, exists := d.lastSeen[id]
		if !exists {
			continue
		}
		if time.Since(last) > d.timeout && node.Status == types.StatusAlive {
			node.Status = types.StatusFailed
			fmt.Printf("[DETECTOR] ❌ Node %s is now OFFLINE (no heartbeat for %v)\n",
				id, d.timeout)
			justFailed = append(justFailed, id)
		}
	}

	d.mu.Unlock()

	for _, id := range justFailed {
		if d.OnFailure != nil {
			d.OnFailure(id)
		}
	}
}

func (d *Detector) GetStatuses() map[string]types.NodeStatus {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make(map[string]types.NodeStatus)
	for id, node := range d.nodes {
		result[id] = node.Status
	}
	return result
}