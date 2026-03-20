package fault

import (
	"sync"
	"testing"
	"time"

	"dfs-system/internal/types"
)

// Test 1: FaultManager starts without panicking
func TestFaultManagerStarts(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)

	fm := NewFaultManager(
		"node1",
		[]string{"127.0.0.1:8081"},
		[]*types.Node{node2},
	)

	// Should not panic
	fm.Start()
	time.Sleep(100 * time.Millisecond)
}

// Test 2: End-to-end — failure detected and callback fires
func TestFaultManagerDetectsFailure(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)

	fm := NewFaultManager(
		"node1",
		[]string{}, // no real peers needed for this test
		[]*types.Node{node2},
	)

	// Override timeout to be short for testing
	fm.Detector.timeout = 500 * time.Millisecond

	var mu sync.Mutex
	failedNode := ""

	fm.Detector.OnFailure = func(nodeID string) {
		mu.Lock()
		failedNode = nodeID
		mu.Unlock()
	}

	fm.Start()
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	result := failedNode
	mu.Unlock()

	if result != "node2" {
		t.Errorf("[TEST] Expected node2 to be detected as failed, but got: '%s'", result)
	}
}

// Test 3: GetStatuses returns all registered nodes
func TestFaultManagerGetStatuses(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)
	node3 := types.NewNode("node3", "127.0.0.1", 8082)

	fm := NewFaultManager(
		"node1",
		[]string{},
		[]*types.Node{node2, node3},
	)

	statuses := fm.Detector.GetStatuses()
	if len(statuses) != 2 {
		t.Errorf("[TEST] Expected 2 nodes registered, but got: %d", len(statuses))
	}
}

// Test 4: FaultManager correctly marks node as failed after timeout
func TestFaultManagerNodeMarkedFailed(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)

	fm := NewFaultManager(
		"node1",
		[]string{},
		[]*types.Node{node2},
	)

	fm.Detector.timeout = 500 * time.Millisecond
	fm.Start()

	time.Sleep(1500 * time.Millisecond)

	statuses := fm.Detector.GetStatuses()
	if statuses["node2"] != types.StatusFailed {
		t.Errorf("[TEST] Expected node2 to be failed, but got: %s", statuses["node2"])
	}
}

// Test 5: FaultManager — node stays alive if heartbeats are sent
func TestFaultManagerNodeStaysAliveWithHeartbeats(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)

	fm := NewFaultManager(
		"node1",
		[]string{},
		[]*types.Node{node2},
	)

	fm.Detector.timeout = 600 * time.Millisecond
	fm.Start()

	// Keep sending heartbeats for node2
	go func() {
		for i := 0; i < 6; i++ {
			time.Sleep(200 * time.Millisecond)
			fm.Detector.RecordHeartbeat("node2")
		}
	}()

	time.Sleep(1500 * time.Millisecond)

	statuses := fm.Detector.GetStatuses()
	if statuses["node2"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node2 to stay alive, but got: %s", statuses["node2"])
	}
}

// Test 6: FaultManager — multiple nodes, only silent one fails
func TestFaultManagerOnlyFailedNodeMarked(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)
	node3 := types.NewNode("node3", "127.0.0.1", 8082)

	fm := NewFaultManager(
		"node1",
		[]string{},
		[]*types.Node{node2, node3},
	)

	fm.Detector.timeout = 500 * time.Millisecond
	fm.Start()

	// node3 keeps sending heartbeats, node2 goes silent
	go func() {
		for i := 0; i < 8; i++ {
			time.Sleep(200 * time.Millisecond)
			fm.Detector.RecordHeartbeat("node3")
		}
	}()

	time.Sleep(1500 * time.Millisecond)

	statuses := fm.Detector.GetStatuses()

	if statuses["node2"] != types.StatusFailed {
		t.Errorf("[TEST] Expected node2 FAILED, but got: %s", statuses["node2"])
	}
	if statuses["node3"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node3 ALIVE, but got: %s", statuses["node3"])
	}
}

// Test 7: FaultManager — callback fires exactly once per failure
func TestFaultManagerCallbackFiresOnce(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)

	fm := NewFaultManager(
		"node1",
		[]string{},
		[]*types.Node{node2},
	)

	fm.Detector.timeout = 500 * time.Millisecond

	var mu sync.Mutex
	callCount := 0

	fm.Detector.OnFailure = func(nodeID string) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	fm.Start()

	// Wait long enough for multiple monitoring cycles
	time.Sleep(2000 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count != 1 {
		t.Errorf("[TEST] Expected OnFailure to fire exactly once, but it fired %d times", count)
	}
}

// Test 8: FaultManager — node recovery after rejoining
func TestFaultManagerNodeRecovery(t *testing.T) {
	node2 := types.NewNode("node2", "127.0.0.1", 8081)

	fm := NewFaultManager(
		"node1",
		[]string{},
		[]*types.Node{node2},
	)

	fm.Detector.timeout = 500 * time.Millisecond
	fm.Start()

	// Let node2 go offline
	time.Sleep(1200 * time.Millisecond)

	statuses := fm.Detector.GetStatuses()
	if statuses["node2"] != types.StatusFailed {
		t.Errorf("[TEST] Expected node2 FAILED before recovery, but got: %s", statuses["node2"])
	}

	// Simulate node2 coming back — send a heartbeat
	fm.Detector.RecordHeartbeat("node2")

	statuses = fm.Detector.GetStatuses()
	if statuses["node2"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node2 ALIVE after recovery, but got: %s", statuses["node2"])
	}
}