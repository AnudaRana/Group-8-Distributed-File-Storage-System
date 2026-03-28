package consensus

import (
	"testing"
	"dfs-system/internal/types"
)

func TestTrueRaftAppendEntries_Success(t *testing.T) {
	r := NewRaft("node1", "localhost", "8000", []string{"localhost:8001", "localhost:8002"})
	defer r.Stop()
	
	r.Log = append(r.Log, LogEntry{Term: 1, Op: "A"})
	r.Log = append(r.Log, LogEntry{Term: 1, Op: "B"})
	r.CurrentTerm = 2

	// Receive heartbeat appending C at index 2
	entriesRaw := []interface{}{
		map[string]interface{}{"Term": float64(2), "Op": "C"},
	}
	payload := map[string]interface{}{
		"term":         float64(2),
		"leaderUrl":    "localhost:8002",
		"prevLogIndex": float64(1),
		"prevLogTerm":  float64(1),
		"entries":      entriesRaw,
		"leaderCommit": float64(0),
	}
	msg := &types.Message{
		Type:    types.MsgLeaderHB,
		Sender:  "node2",
		Payload: payload,
	}

	r.HandleLeaderHeartbeat(msg)

	if len(r.Log) != 3 {
		t.Fatalf("Expected log length 3, got %d", len(r.Log))
	}
	if r.Log[2].Op != "C" {
		t.Errorf("Failed to append new entry")
	}
}

func TestTrueRaftAppendEntries_RejectionAndRetry(t *testing.T) {
	r := NewRaft("leader", "localhost", "8000", []string{"localhost:8001"})
	defer r.Stop()
	
	r.State = Leader
	r.CurrentTerm = 2
	r.Log = append(r.Log, LogEntry{Term: 1, Op: "A"})
	r.Log = append(r.Log, LogEntry{Term: 2, Op: "B"})
	
	r.nextIndex["localhost:8001"] = 2
	r.matchIndex["localhost:8001"] = 0

	replyPayload := map[string]interface{}{
		"term":       float64(2),
		"success":    false,
		"followerUrl": "localhost:8001",
		"matchIndex": float64(-1),
	}
	msg := &types.Message{
		Type:    "APPEND_REPLY",
		Sender:  "follower1",
		Payload: replyPayload,
	}

	r.HandleAppendReply(msg)

	if r.nextIndex["localhost:8001"] != 1 {
		t.Errorf("Expected nextIndex to decrement to 1, got %d", r.nextIndex["localhost:8001"])
	}
}

func TestTrueRaftAppendEntries_MajorityCommit(t *testing.T) {
	r := NewRaft("leader", "localhost", "8000", []string{"localhost:8001", "localhost:8002"})
	defer r.Stop()
	
	r.State = Leader
	r.CurrentTerm = 2
	r.CommitIndex = 0
	r.Log = append(r.Log, LogEntry{Term: 1, Op: "A"})
	r.Log = append(r.Log, LogEntry{Term: 2, Op: "B"}) 
	
	r.nextIndex["localhost:8001"] = 1
	r.matchIndex["localhost:8001"] = 0

	replyPayload := map[string]interface{}{
		"term":       float64(2),
		"success":    true,
		"followerUrl": "localhost:8001",
		"matchIndex": float64(1),
	}
	msg := &types.Message{
		Type:    "APPEND_REPLY",
		Sender:  "follower1",
		Payload: replyPayload,
	}

	r.HandleAppendReply(msg)

	if r.CommitIndex != 1 {
		t.Errorf("Expected CommitIndex to advance to 1, got %d", r.CommitIndex)
	}
}
