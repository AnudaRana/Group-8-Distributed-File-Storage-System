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

// ReplicationIntegrator allows the fault package to call replication
// operations without creating an import cycle.
type ReplicationIntegrator interface {
	ReplicateFilesFromFailedNode(nodeID string) error
	SyncNodeFromCheckpoint(nodeID string) error
}

type RecoveryManager struct {
	mu                sync.Mutex
	replicationFactor int
	records           map[string]*RecoveryRecord
	onRecover         func(nodeID string)
	replication       ReplicationIntegrator
	isLeader          func() bool
}

// NewRecoveryManager initializes a controller designed to repair or re-replicate
// files when peers undergo catastrophic silent failures or abrupt crashes.
func NewRecoveryManager(replicationFactor int, rep ReplicationIntegrator, isLeader func() bool) *RecoveryManager {
	return &RecoveryManager{
		replicationFactor: replicationFactor,
		records:           make(map[string]*RecoveryRecord),
		replication:       rep,
		isLeader:          isLeader,
	}
}

// SetOnRecover attaches a generic lifecycle callback to fire the moment
// a previously dead node signals successful reconnection and recovery.
func (r *RecoveryManager) SetOnRecover(fn func(nodeID string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onRecover = fn
}

// OnNodeFailure logs a confirmed failure event and commands the replication
// layer to urgently redistribute and clone files held previously by the dead node.
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

	if r.isLeader != nil && r.isLeader() {
		r.triggerReReplication(nodeID)
	}
}

// OnNodeRejoin resets a failing node's operational state matrix and explicitly triggers
// a checkpoint state transfer to get its orphaned local disk completely up to speed.
func (r *RecoveryManager) OnNodeRejoin(nodeID string) {
	r.mu.Lock()

	record, exists := r.records[nodeID]

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

	if r.isLeader != nil && r.isLeader() {
		r.syncCheckpoint(nodeID)
	}

	if callback != nil {
		go callback(nodeID)
	}
}

// GetRecord exposes the deep downtime metadata history of one explicit node.
func (r *RecoveryManager) GetRecord(nodeID string) *RecoveryRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.records[nodeID]
}

// GetAllRecords safely dumps the entire cluster downtime timeline tracking map.
func (r *RecoveryManager) GetAllRecords() map[string]*RecoveryRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	copy := make(map[string]*RecoveryRecord)
	for k, v := range r.records {
		copy[k] = v
	}
	return copy
}

// triggerReReplication forks a background process to shift data off dead nodes.
func (r *RecoveryManager) triggerReReplication(nodeID string) {
	fmt.Printf("[RECOVERY] 🔄 Triggering re-replication for files lost on node %s\n", nodeID)
	if r.replication != nil {
		go func() {
			if err := r.replication.ReplicateFilesFromFailedNode(nodeID); err != nil {
				fmt.Printf("[RECOVERY] ⚠️  Re-replication error for node %s: %v\n", nodeID, err)
			} else {
				fmt.Printf("[RECOVERY] ✅ Re-replication complete for node %s\n", nodeID)
			}
		}()
	}
}

// syncCheckpoint initiates a full Raft and Log transfer from the master
// memory tree down to a newly restored physical node structure.
func (r *RecoveryManager) syncCheckpoint(nodeID string) {
	fmt.Printf("[RECOVERY] 📦 Syncing checkpoint to rejoined node %s\n", nodeID)
	if r.replication != nil {
		go func() {
			if err := r.replication.SyncNodeFromCheckpoint(nodeID); err != nil {
				fmt.Printf("[RECOVERY] ⚠️  Checkpoint sync error for node %s: %v\n", nodeID, err)
			} else {
				fmt.Printf("[RECOVERY] ✅ Checkpoint synced to node %s\n", nodeID)
			}
		}()
	}
}
