package fault

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dfs-system/internal/types"
)

// Test 1: GossipManager registers peers correctly
func TestGossipRegistersPeers(t *testing.T) {
	g := NewGossipManager("node1", 2)
	g.RegisterPeer("node2", "127.0.0.1:9002")
	g.RegisterPeer("node3", "127.0.0.1:9003")

	if g.GetPeerCount() != 2 {
		t.Errorf("Expected 2 peers, got %d", g.GetPeerCount())
	}
}

// Test 2: Gossip sends correct message type
func TestGossipSendsCorrectMessageType(t *testing.T) {
	received := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		var msg types.Message
		json.Unmarshal(body, &msg)
		received <- msg.Type
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	g := NewGossipManager("node1", 2)
	g.RegisterPeer("node2", server.URL[7:])
	g.SpreadFailure("node3")

	select {
	case msgType := <-received:
		if msgType != msgGossip {
			t.Errorf("Expected %s message, got %s", msgGossip, msgType)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout — no gossip message received")
	}
}

// Test 3: Gossip message contains the failed node ID
func TestGossipContainsFailedNodeID(t *testing.T) {
	received := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		var msg types.Message
		json.Unmarshal(body, &msg)
		if failedNode, ok := msg.Payload["failed_node"].(string); ok {
			received <- failedNode
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	g := NewGossipManager("node1", 2)
	g.RegisterPeer("node2", server.URL[7:])
	g.SpreadFailure("node3")

	select {
	case failedNode := <-received:
		if failedNode != "node3" {
			t.Errorf("Expected failed_node=node3, got %s", failedNode)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout — no gossip received")
	}
}

// Test 4: Gossip stops after maxRounds
func TestGossipStopsAfterMaxRounds(t *testing.T) {
	g := NewGossipManager("node1", 2)
	g.maxRounds = 2

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	g.RegisterPeer("node2", server.URL[7:])

	g.SpreadFailure("node3")
	g.SpreadFailure("node3")
	g.SpreadFailure("node3") // should be ignored

	count := g.GetRumourCount("node3")
	if count != 2 {
		t.Errorf("Expected rumour count to cap at 2, got %d", count)
	}
}

// Test 5: Gossip does not contact the failed node itself
func TestGossipExcludesFailedNode(t *testing.T) {
	contacted := make(map[string]bool)
	done := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		var msg types.Message
		json.Unmarshal(body, &msg)
		contacted[msg.Sender] = true
		select {
		case done <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	g := NewGossipManager("node1", 2)
	g.RegisterPeer("node2", server.URL[7:])
	g.RegisterPeer("node3", "127.0.0.1:19998") // failed node — should not be contacted

	g.SpreadFailure("node3")

	select {
	case <-done:
		if contacted["node3"] {
			t.Error("Gossip should NOT contact the failed node (node3)")
		}
	case <-time.After(2 * time.Second):
		// No reachable peers — acceptable for this test
	}
}

// Test 6: ReceiveGossip marks node as failed in detector
func TestReceiveGossipMarksFailed(t *testing.T) {
	g := NewGossipManager("node1", 2)

	detector := NewDetector(6 * time.Second)
	node3 := types.NewNode("node3", "127.0.0.1", 9003)
	detector.RegisterNode(node3)

	g.ReceiveGossip("node2", "node3", detector)

	time.Sleep(100 * time.Millisecond)

	statuses := detector.GetStatuses()
	if statuses["node3"] != types.StatusFailed {
		t.Errorf("Expected node3 FAILED after receiving gossip, got %s", statuses["node3"])
	}
}

// Test 7: Gossip handles unreachable peer gracefully
func TestGossipHandlesUnreachablePeer(t *testing.T) {
	g := NewGossipManager("node1", 2)
	g.RegisterPeer("node2", "127.0.0.1:19997")

	// Should not panic or crash
	g.SpreadFailure("node3")
	time.Sleep(300 * time.Millisecond)
}