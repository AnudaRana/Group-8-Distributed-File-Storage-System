package consensus

import (
	"testing"
	"time"
	"dfs-system/internal/types"
)

func TestRaftInitialState(t *testing.T) {
	r := NewRaft("node1", "localhost", "8000", []string{"localhost:8001"})
	defer r.Stop()
	if r.State != Follower {
		t.Errorf("Expected Follower, got %v", r.State)
	}
	if r.CurrentTerm != 0 {
		t.Errorf("Expected term 0, got %d", r.CurrentTerm)
	}
}

func TestRaftHandleVoteRequest_GrantsVote(t *testing.T) {
	r := NewRaft("node1", "localhost", "8000", []string{"localhost:8001"})
	defer r.Stop()
	r.CurrentTerm = 1
	r.VotedFor = ""

	payload := map[string]interface{}{
		"term": float64(2), 
		"candidateUrl": "localhost:8001",
	}
	msg := &types.Message{
		Type:    types.MsgVoteReq,
		Sender:  "node2",
		Payload: payload,
	}

	r.HandleVoteRequest(msg)

	if r.CurrentTerm != 2 {
		t.Errorf("Expected term 2, got %d", r.CurrentTerm)
	}
	if r.VotedFor != "node2" {
		t.Errorf("Expected VotedFor node2, got %s", r.VotedFor)
	}
}

func TestRaftHandleVoteReply_BecomesLeader(t *testing.T) {
	r := NewRaft("node1", "localhost", "8000", []string{"localhost:8001", "localhost:8002"})
	defer r.Stop()
	r.State = Candidate
	r.CurrentTerm = 1
	r.VotesReceived = 1 

	payload := map[string]interface{}{
		"term": float64(1),
		"voteGranted": true,
	}
	msg := &types.Message{
		Type:    types.MsgVoteReply,
		Sender:  "node2",
		Payload: payload,
	}

	r.HandleVoteReply(msg)

	if r.State != Leader {
		t.Errorf("Expected Leader, got %v", r.State)
	}
}

func TestRaftSplitVote_TriggersNewElection(t *testing.T) {
	r := NewRaft("node1", "localhost", "8000", []string{"localhost:8001"})
	defer r.Stop()
	
	r.State = Candidate
	r.CurrentTerm = 1
	r.VotesReceived = 1 

	payload := map[string]interface{}{
		"term":        float64(1),
		"voteGranted": false,
	}
	msg := &types.Message{
		Type:    types.MsgVoteReply,
		Sender:  "node2",
		Payload: payload,
	}

	r.HandleVoteReply(msg)

	if r.State == Leader {
		t.Errorf("Expected Candidate, got Leader despite rejection")
	}
	
	r.mu.Lock()
	r.resetElectionTimer()
	r.mu.Unlock()
	time.Sleep(3500 * time.Millisecond) 
	
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.CurrentTerm < 2 {
		t.Errorf("Expected node to retry election with term >= 2, got %d", r.CurrentTerm)
	}
}

func TestRaftRejectOutdatedHeartbeat(t *testing.T) {
	r := NewRaft("node1", "localhost", "8000", []string{})
	defer r.Stop()
	r.CurrentTerm = 5

	payload := map[string]interface{}{
		"term": float64(3),
		"leaderUrl": "localhost:8002",
	}
	msg := &types.Message{
		Type:    types.MsgLeaderHB,
		Sender:  "node2",
		Payload: payload,
	}

	r.HandleLeaderHeartbeat(msg)

	if r.CurrentTerm == 3 {
		t.Errorf("Node dangerously downgraded its term to match a dead leader")
	}
}
