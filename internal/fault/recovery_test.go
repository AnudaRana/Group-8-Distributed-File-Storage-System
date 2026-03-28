package fault

import (
	"testing"
	"time"

	"dfs-system/internal/types"
)

// test helper — creates a node for testing
func newTestNode(id, host string, port int) *types.Node {
	return &types.Node{
		ID:     id,
		Host:   host,
		Port:   port,
		Status: types.StatusAlive,
	}
}

// Test 1: Failure record is created when node fails
func TestRecoveryRecordsFailure(t *testing.T) {
	rm := NewRecoveryManager(3)
	rm.OnNodeFailure("node2")

	record := rm.GetRecord("node2")
	if record == nil {
		t.Fatal("Expected a recovery record for node2, got nil")
	}
	if record.State != RecoveryPending {
		t.Errorf("Expected state PENDING, got %s", record.State)
	}
	if record.NodeID != "node2" {
		t.Errorf("Expected nodeID node2, got %s", record.NodeID)
	}
}

// Test 2: Recovery record updated when node rejoins
func TestRecoveryRecordsRejoin(t *testing.T) {
	rm := NewRecoveryManager(3)

	rm.OnNodeFailure("node2")
	time.Sleep(100 * time.Millisecond)
	rm.OnNodeRejoin("node2")

	record := rm.GetRecord("node2")
	if record == nil {
		t.Fatal("Expected a recovery record for node2, got nil")
	}
	if record.State != RecoveryComplete {
		t.Errorf("Expected state COMPLETE, got %s", record.State)
	}
}

// Test 3: Downtime is tracked correctly
func TestRecoveryTracksDowntime(t *testing.T) {
	rm := NewRecoveryManager(3)

	rm.OnNodeFailure("node2")
	time.Sleep(300 * time.Millisecond)
	rm.OnNodeRejoin("node2")

	record := rm.GetRecord("node2")
	if record == nil {
		t.Fatal("Expected recovery record, got nil")
	}

	downtime := record.RecoveredAt.Sub(record.FailedAt)
	if downtime < 300*time.Millisecond {
		t.Errorf("Expected downtime >= 300ms, got %s", downtime)
	}
}

// Test 4: OnRecover callback fires when node rejoins
func TestRecoveryCallbackFires(t *testing.T) {
	rm := NewRecoveryManager(3)

	recovered := ""
	rm.SetOnRecover(func(nodeID string) {
		recovered = nodeID
	})

	rm.OnNodeFailure("node2")
	rm.OnNodeRejoin("node2")

	// Give callback goroutine time to fire
	time.Sleep(100 * time.Millisecond)

	if recovered != "node2" {
		t.Errorf("Expected recovery callback for node2, got '%s'", recovered)
	}
}

// Test 5: Multiple nodes tracked independently
func TestRecoveryTracksMultipleNodes(t *testing.T) {
	rm := NewRecoveryManager(3)

	rm.OnNodeFailure("node2")
	rm.OnNodeFailure("node3")

	rm.OnNodeRejoin("node2") // only node2 comes back

	records := rm.GetAllRecords()

	if len(records) != 2 {
		t.Errorf("Expected 2 records, got %d", len(records))
	}
	if records["node2"].State != RecoveryComplete {
		t.Errorf("Expected node2 COMPLETE, got %s", records["node2"].State)
	}
	if records["node3"].State != RecoveryPending {
		t.Errorf("Expected node3 PENDING, got %s", records["node3"].State)
	}
}

// Test 6: Node rejoins with no prior failure record
func TestRecoveryHandlesUnexpectedRejoin(t *testing.T) {
	rm := NewRecoveryManager(3)

	rm.OnNodeRejoin("node2")

	record := rm.GetRecord("node2")
	if record == nil {
		t.Fatal("Expected a record even for unexpected rejoin")
	}
	if record.State != RecoveryComplete {
		t.Errorf("Expected state COMPLETE, got %s", record.State)
	}
}

// Test 7: End-to-end — detector triggers recovery automatically
func TestRecoveryIntegratesWithDetector(t *testing.T) {
	rm := NewRecoveryManager(3)
	detector := NewDetector(500 * time.Millisecond)

	// Wire detector to recovery
	detector.OnFailure = func(nodeID string) {
		rm.OnNodeFailure(nodeID)
	}
	detector.OnRejoin = func(nodeID string) {
		rm.OnNodeRejoin(nodeID)
	}

	detector.RegisterNode(newTestNode("node2", "127.0.0.1", 9002))
	detector.StartMonitoring()

	// Let node2 go offline
	time.Sleep(1200 * time.Millisecond)

	record := rm.GetRecord("node2")
	if record == nil || record.State != RecoveryPending {
		t.Errorf("Expected node2 PENDING after timeout, got %v", record)
	}

	// Simulate node2 coming back
	detector.RecordHeartbeat("node2")
	time.Sleep(200 * time.Millisecond)

	record = rm.GetRecord("node2")
	if record.State != RecoveryComplete {
		t.Errorf("Expected node2 COMPLETE after rejoin, got %s", record.State)
	}
}