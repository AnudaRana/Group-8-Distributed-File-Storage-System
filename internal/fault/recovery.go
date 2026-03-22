package fault

import (
	"fmt"
	"sync"
	"time"
)

type RecoveryState string

const (
	RecoveryPending  RecoveryState = "pending"
	RecoveryComplete RecoveryState = "complete"
)

type RecoveryRecord struct {
	NodeID      string
	FailedAt    time.Time
	RecoveredAt time.Time
	State       RecoveryState
}

type RecoveryManager struct {
	mu                sync.Mutex
	replicationFactor int
	records           map[string]*RecoveryRecord
	onRecover         func(nodeID string)
}

func NewRecoveryManager(replicationFactor int) *RecoveryManager {
	return &RecoveryManager{
		replicationFactor: replicationFactor,
		records:           make(map[string]*RecoveryRecord),
	}
}

func (r *RecoveryManager) SetOnRecover(fn func(nodeID string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onRecover = fn
}

func (r *RecoveryManager) OnNodeFailure(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Always create fresh record on failure
	// This resets state so next rejoin fires correctly
	r.records[nodeID] = &RecoveryRecord{
		NodeID:   nodeID,
		FailedAt: time.Now(),
		State:    RecoveryPending,
	}

	fmt.Printf("[RECOVERY] 📋 Recorded failure of node %s at %s\n",
		nodeID, r.records[nodeID].FailedAt.Format("15:04:05"))
	fmt.Printf("[RECOVERY] Replication factor is %d — re-replication will be triggered\n",
		r.replicationFactor)

	r.triggerReReplication(nodeID)
}

func (r *RecoveryManager) OnNodeRejoin(nodeID string) {
	r.mu.Lock()

	record, exists := r.records[nodeID]

	// Fix — if already complete, don't sync again (prevents double sync)
	if exists && record.State == RecoveryComplete {
		r.mu.Unlock()
		fmt.Printf("[RECOVERY] Node %s already recovered — ignoring duplicate rejoin\n", nodeID)
		return
	}

	if !exists {
		r.records[nodeID] = &RecoveryRecord{
			NodeID:      nodeID,
			RecoveredAt: time.Now(),
			State:       RecoveryComplete,
		}
		r.mu.Unlock()
		fmt.Printf("[RECOVERY] Node %s rejoined (no prior failure record)\n", nodeID)
		return
	}

	record.RecoveredAt = time.Now()
	record.State = RecoveryComplete
	downtime := record.RecoveredAt.Sub(record.FailedAt)
	callback := r.onRecover

	r.mu.Unlock()

	fmt.Printf("[RECOVERY] ✅ Node %s is back online (was down for %s)\n",
		nodeID, downtime.Round(time.Second))

	r.syncCheckpoint(nodeID)

	if callback != nil {
		go callback(nodeID)
	}
}

func (r *RecoveryManager) GetRecord(nodeID string) *RecoveryRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.records[nodeID]
}

func (r *RecoveryManager) GetAllRecords() map[string]*RecoveryRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	copy := make(map[string]*RecoveryRecord)
	for k, v := range r.records {
		copy[k] = v
	}
	return copy
}

func (r *RecoveryManager) triggerReReplication(nodeID string) {
	fmt.Printf("[RECOVERY] 🔄 Triggering re-replication for files lost on node %s\n", nodeID)
	// Integration point with Member 2
	// replicationManager.ReplicateFilesFromFailedNode(nodeID)
}

func (r *RecoveryManager) syncCheckpoint(nodeID string) {
	fmt.Printf("[RECOVERY] 📦 Syncing checkpoint to rejoined node %s\n", nodeID)
	// Integration point with Member 2
	// replicationManager.SyncNodeFromCheckpoint(nodeID)
}