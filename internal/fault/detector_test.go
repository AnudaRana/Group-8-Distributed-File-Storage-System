package fault

import (
	"testing"
	"time"

	"dfs-system/internal/types"
)

// Test 1: Node starts as alive when registered
func TestNodeRegisteredAsAlive(t *testing.T) {
	detector := NewDetector(6 * time.Second)

	node := types.NewNode("node2", "127.0.0.1", 8081)
	detector.RegisterNode(node)

	statuses := detector.GetStatuses()
	if statuses["node2"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node2 to be alive, but got: %s", statuses["node2"])
	}
}

// Test 2: Heartbeat keeps node alive
func TestHeartbeatKeepsNodeAlive(t *testing.T) {
	detector := NewDetector(1 * time.Second)

	node := types.NewNode("node2", "127.0.0.1", 8081)
	detector.RegisterNode(node)

	// Send heartbeats for 2 seconds (faster than timeout)
	go func() {
		for i := 0; i < 4; i++ {
			time.Sleep(400 * time.Millisecond)
			detector.RecordHeartbeat("node2")
		}
	}()

	time.Sleep(2 * time.Second)

	statuses := detector.GetStatuses()
	if statuses["node2"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node2 to stay alive, but got: %s", statuses["node2"])
	}
}

// Test 3: Node marked failed after timeout (THE CORE TEST)
func TestNodeMarkedFailedAfterTimeout(t *testing.T) {
	// Short timeout for fast testing
	detector := NewDetector(500 * time.Millisecond)

	node := types.NewNode("node2", "127.0.0.1", 8081)
	detector.RegisterNode(node)
	detector.StartMonitoring()

	// Do NOT send any heartbeats — simulate crash

	// Wait longer than timeout
	time.Sleep(1500 * time.Millisecond)

	statuses := detector.GetStatuses()
	if statuses["node2"] != types.StatusFailed {
		t.Errorf("[TEST] Expected node2 to be failed, but got: %s", statuses["node2"])
	}
}

// Test 4: OnFailure callback is triggered
func TestOnFailureCallbackFired(t *testing.T) {
	detector := NewDetector(500 * time.Millisecond)

	node := types.NewNode("node2", "127.0.0.1", 8081)
	detector.RegisterNode(node)

	callbackFired := false
	detector.OnFailure = func(nodeID string) {
		if nodeID == "node2" {
			callbackFired = true
		}
	}

	detector.StartMonitoring()
	time.Sleep(1500 * time.Millisecond)

	if !callbackFired {
		t.Error("[TEST] Expected OnFailure callback to fire for node2, but it did not")
	}
}

// Test 5: Node comes back online after sending heartbeat again
func TestNodeRecoveryAfterHeartbeat(t *testing.T) {
	detector := NewDetector(500 * time.Millisecond)

	node := types.NewNode("node2", "127.0.0.1", 8081)
	detector.RegisterNode(node)
	detector.StartMonitoring()

	// Let it go offline
	time.Sleep(1200 * time.Millisecond)

	statuses := detector.GetStatuses()
	if statuses["node2"] != types.StatusFailed {
		t.Errorf("[TEST] Expected node2 to be failed first, but got: %s", statuses["node2"])
	}

	// Now simulate it coming back
	detector.RecordHeartbeat("node2")

	statuses = detector.GetStatuses()
	if statuses["node2"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node2 to be alive after heartbeat, but got: %s", statuses["node2"])
	}
}

// Test 6: Multiple nodes — only the silent one fails
func TestOnlyOfflineNodeFails(t *testing.T) {
	detector := NewDetector(500 * time.Millisecond)

	node2 := types.NewNode("node2", "127.0.0.1", 8081)
	node3 := types.NewNode("node3", "127.0.0.1", 8082)
	detector.RegisterNode(node2)
	detector.RegisterNode(node3)
	detector.StartMonitoring()

	// node3 keeps sending heartbeats, node2 goes silent
	go func() {
		for i := 0; i < 5; i++ {
			time.Sleep(300 * time.Millisecond)
			detector.RecordHeartbeat("node3")
		}
	}()

	time.Sleep(1500 * time.Millisecond)

	statuses := detector.GetStatuses()

	if statuses["node2"] != types.StatusFailed {
		t.Errorf("[TEST] Expected node2 FAILED, but got: %s", statuses["node2"])
	}
	if statuses["node3"] != types.StatusAlive {
		t.Errorf("[TEST] Expected node3 ALIVE, but got: %s", statuses["node3"])
	}
}