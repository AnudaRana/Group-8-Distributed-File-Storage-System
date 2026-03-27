package consensus

import (
	"fmt"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

/*
 * Function: ProposeState
 * Description: Adds a new operation to the log.
 * Why: Only the leader can accept and replicate operations.
 * Inputs:
 *   - op (string): Operation to replicate.
 * Outputs / Expected Outcome: Returns true if leader accepts the proposal.
 */
func (r *Raft) ProposeState(op string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State != Leader {
		return false
	}

	r.Log = append(r.Log, LogEntry{Term: r.CurrentTerm, Op: op})
	
	utils.Log(r.ID, "Leader accepted proposed state: %s. Replicating via AppendEntries.", op)
	return true
}

/*
 * Function: startHeartbeats
 * Description: Continuously sends heartbeats to maintain leadership.
 * Why: Prevents followers from starting elections.
 * Inputs: None
 * Outputs / Expected Outcome: Heartbeats are sent periodically.
 */
func (r *Raft) startHeartbeats() {
	r.sendHeartbeats()
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
	}
	r.heartbeatTimer = time.AfterFunc(500*time.Millisecond, r.startHeartbeats)
}

/*
 * Function: sendHeartbeats
 * Description: Sends AppendEntries-style messages to all peers.
 * Why: Keeps followers in sync with the leader’s log.
 * Inputs: None
 * Outputs / Expected Outcome: Sends log updates to all peers.
 */
func (r *Raft) sendHeartbeats() {
	r.mu.Lock()
	if r.State != Leader {
		r.mu.Unlock()
		return
	}
	term := r.CurrentTerm
	commitIdx := r.CommitIndex
	logs := r.Log
	myUrl := fmt.Sprintf("%s:%s", r.Host, r.Port)
	r.mu.Unlock()

	for _, peerURL := range r.PeerURLs {
		r.mu.Lock()
		nextIdx := r.nextIndex[peerURL]
		r.mu.Unlock()

		prevLogIndex := nextIdx - 1
		prevLogTerm := -1
		if prevLogIndex >= 0 && prevLogIndex < len(logs) {
			prevLogTerm = logs[prevLogIndex].Term
		}

		var entries []LogEntry
		if nextIdx < len(logs) && nextIdx >= 0 {
			entries = logs[nextIdx:]
		}

		payload := map[string]interface{}{
			"term":         float64(term),
			"leaderUrl":    myUrl,
			"prevLogIndex": float64(prevLogIndex),
			"prevLogTerm":  float64(prevLogTerm),
			"entries":      entries, 
			"leaderCommit": float64(commitIdx),
		}
		
		msg, err := types.NewMessage(types.MsgLeaderHB, r.ID, payload)
		if err == nil {
			url := fmt.Sprintf("http://%s/message", peerURL)
			go transport.Send(url, msg)
		}
	}
}

/*
 * Function: HandleLeaderHeartbeat
 * Description: Processes incoming AppendEntries from the leader.
 * Why: Validates logs and updates follower state.
 * Inputs:
 *   - msg (*types.Message): Incoming heartbeat message.
 * Outputs / Expected Outcome: Updates logs and sends reply.
 */
func (r *Raft) HandleLeaderHeartbeat(msg *types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	termFloat, ok := msg.Payload["term"].(float64)
	if !ok {
		return
	}
	term := int(termFloat)

	leaderUrl := msg.Payload["leaderUrl"].(string)

	replyPayload := map[string]interface{}{
		"term":       float64(r.CurrentTerm),
		"success":    false,
		"followerUrl": fmt.Sprintf("%s:%s", r.Host, r.Port),
		"matchIndex": float64(-1),
	}

	if term < r.CurrentTerm {
		go r.sendAppendReply(leaderUrl, replyPayload)
		return
	}

	if term > r.CurrentTerm {
		r.CurrentTerm = term
		r.VotedFor = ""
	}
	r.State = Follower
	r.resetElectionTimer()
	replyPayload["term"] = float64(r.CurrentTerm) 

	prevLogIndex := int(msg.Payload["prevLogIndex"].(float64))
	prevLogTerm := int(msg.Payload["prevLogTerm"].(float64))

	if prevLogIndex >= 0 {
		if prevLogIndex >= len(r.Log) || r.Log[prevLogIndex].Term != prevLogTerm {
			go r.sendAppendReply(leaderUrl, replyPayload)
			return
		}
	}

	var newEntries []LogEntry
	if logsRaw, hasLogs := msg.Payload["entries"]; hasLogs {
		if rawArray, isArr := logsRaw.([]interface{}); isArr {
			for _, entry := range rawArray {
				mapEntry := entry.(map[string]interface{})
				newEntries = append(newEntries, LogEntry{
					Term: int(mapEntry["Term"].(float64)),
					Op:   mapEntry["Op"].(string),
				})
			}
		}
	}

	if prevLogIndex >= 0 {
		r.Log = r.Log[:prevLogIndex+1]
	} else {
		r.Log = make([]LogEntry, 0)
	}
	r.Log = append(r.Log, newEntries...)

	leaderCommit := int(msg.Payload["leaderCommit"].(float64))
	if leaderCommit > r.CommitIndex {
		lastNewEntry := len(r.Log) - 1
		if leaderCommit < lastNewEntry {
			r.CommitIndex = leaderCommit
		} else {
			r.CommitIndex = lastNewEntry
		}
		utils.Log(r.ID, "Follower updated CommitIndex to %d", r.CommitIndex)
	}

	replyPayload["success"] = true
	replyPayload["matchIndex"] = float64(len(r.Log) - 1)
	go r.sendAppendReply(leaderUrl, replyPayload)
}

/*
 * Function: sendAppendReply
 * Description: Sends append response back to the leader.
 * Why: Lets the leader track replication success.
 * Inputs:
 *   - leaderUrl (string): Leader address.
 *   - payload (map): Reply data.
 * Outputs / Expected Outcome: Sends APPEND_REPLY message.
 */
func (r *Raft) sendAppendReply(leaderUrl string, payload map[string]interface{}) {
	replyMsg, err := types.NewMessage("APPEND_REPLY", r.ID, payload)
	if err != nil {
		return
	}
	url := fmt.Sprintf("http://%s/message", leaderUrl)
	transport.Send(url, replyMsg)
}

/*
 * Function: HandleAppendReply
 * Description: Processes follower replies to replication.
 * Why: Updates nextIndex and advances commit index on majority.
 * Inputs:
 *   - msg (*types.Message): Reply message.
 * Outputs / Expected Outcome: Adjusts replication state and commits logs.
 */
func (r *Raft) HandleAppendReply(msg *types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.State != Leader {
		return
	}

	term := int(msg.Payload["term"].(float64))
	if term > r.CurrentTerm {
		r.CurrentTerm = term
		r.State = Follower
		r.VotedFor = ""
		r.resetElectionTimer()
		return
	}

	success := msg.Payload["success"].(bool)
	followerUrl := msg.Payload["followerUrl"].(string)

	if success {
		matchIdx := int(msg.Payload["matchIndex"].(float64))
		r.matchIndex[followerUrl] = matchIdx
		r.nextIndex[followerUrl] = matchIdx + 1

		totalNodes := len(r.PeerURLs) + 1
		for n := len(r.Log) - 1; n > r.CommitIndex; n-- {
			if r.Log[n].Term == r.CurrentTerm {
				count := 1 
				for _, mIdx := range r.matchIndex {
					if mIdx >= n {
						count++
					}
				}
				if count > totalNodes/2 {
					r.CommitIndex = n
					utils.Log(r.ID, "Leader successfully COMMITTED log entry %d (majority agreement)", n)
					break
				}
			}
		}
	} else {
		r.nextIndex[followerUrl]--
		if r.nextIndex[followerUrl] < 0 {
			r.nextIndex[followerUrl] = 0
		}
	}
}
