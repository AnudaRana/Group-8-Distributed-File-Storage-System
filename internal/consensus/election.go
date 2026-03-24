package consensus

import (
	"fmt"
	"math/rand"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

/*
 * Function: electionTimeout
 * Description: Generates random election timeout.
 * Why: Prevents split vote situations.
 * Inputs: None
 * Outputs / Expected Outcome: Returns randomized duration.
 */
func (r *Raft) electionTimeout() time.Duration {
	return time.Duration(1500+rand.Intn(1500)) * time.Millisecond
}

/*
 * Function: resetElectionTimer
 * Description: Restarts election timer.
 * Why: Keeps follower from starting elections when leader is alive.
 * Inputs: None
 * Outputs / Expected Outcome: Timer is reset.
 */
func (r *Raft) resetElectionTimer() {
	if r.electionTimer != nil {
		r.electionTimer.Stop()
	}
	r.electionTimer = time.AfterFunc(r.electionTimeout(), r.startElection)
}

/*
 * Function: startElection
 * Description: Starts election process.
 * Why: Promotes node to candidate and requests votes.
 * Inputs: None
 * Outputs / Expected Outcome: Sends vote requests to peers.
 */
func (r *Raft) startElection() {
	r.mu.Lock()
	if r.State == Leader {
		r.mu.Unlock()
		return
	}

	r.State = Candidate
	r.CurrentTerm++
	r.VotedFor = r.ID
	r.VotesReceived = 1
	term := r.CurrentTerm
	r.mu.Unlock()

	utils.Log(r.ID, "Starting election for term %d", term)

	for _, peer := range r.PeerURLs {
		go r.sendVoteRequest(peer, term)
	}

	r.mu.Lock()
	r.resetElectionTimer()
	r.mu.Unlock()
}

/*
 * Function: sendVoteRequest
 * Description: Sends vote request to a peer.
 * Why: Requests election votes from other nodes.
 * Inputs:
 *   - peerURL (string): Target node.
 *   - term (int): Current term.
 * Outputs / Expected Outcome: Sends MsgVoteReq.
 */
func (r *Raft) sendVoteRequest(peerURL string, term int) {
	payload := map[string]interface{}{
		"term":         float64(term),
		"candidateUrl": fmt.Sprintf("%s:%s", r.Host, r.Port),
	}
	msg, err := types.NewMessage(types.MsgVoteReq, r.ID, payload)
	if err != nil {
		return
	}
	url := fmt.Sprintf("http://%s/message", peerURL)
	transport.Send(url, msg)
}

/*
 * Function: HandleVoteRequest
 * Description: Handles incoming vote requests.
 * Why: Decides whether to grant or deny a vote.
 * Inputs:
 *   - msg (*types.Message): Vote request message.
 * Outputs / Expected Outcome: Sends vote reply.
 */
func (r *Raft) HandleVoteRequest(msg *types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	termFloat, ok1 := msg.Payload["term"].(float64)
	candidateUrl, ok2 := msg.Payload["candidateUrl"].(string)
	if !ok1 || !ok2 {
		return
	}
	term := int(termFloat)

	if term > r.CurrentTerm {
		r.CurrentTerm = term
		r.State = Follower
		r.VotedFor = ""
	}

	grantVote := false
	if term == r.CurrentTerm && (r.VotedFor == "" || r.VotedFor == msg.Sender) {
		grantVote = true
		r.VotedFor = msg.Sender
		r.lastHeartbeat = time.Now()
		r.resetElectionTimer()
	}

	utils.Log(r.ID, "Received VoteReq from %s for term %d. Grant vote: %v", msg.Sender, term, grantVote)

	go r.sendVoteReply(candidateUrl, r.CurrentTerm, grantVote)
}

/*
 * Function: sendVoteReply
 * Description: Sends vote reply to candidate.
 * Why: Allows election to be decided by majority.
 * Inputs:
 *   - candidateUrl (string): Candidate address.
 *   - term (int): Current term.
 *   - grantVote (bool): Vote decision.
 * Outputs / Expected Outcome: Sends MsgVoteReply.
 */
func (r *Raft) sendVoteReply(candidateUrl string, term int, grantVote bool) {
	payload := map[string]interface{}{
		"term":        float64(term),
		"voteGranted": grantVote,
	}
	replyMsg, err := types.NewMessage(types.MsgVoteReply, r.ID, payload)
	if err != nil {
		return
	}

	url := fmt.Sprintf("http://%s/message", candidateUrl)
	transport.Send(url, replyMsg)
}

/*
 * Function: HandleVoteReply
 * Description: Processes vote replies.
 * Why: Determines if node becomes leader.
 * Inputs:
 *   - msg (*types.Message): Vote reply message.
 * Outputs / Expected Outcome: Promotes to leader if majority achieved.
 */
func (r *Raft) HandleVoteReply(msg *types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	termFloat, ok1 := msg.Payload["term"].(float64)
	grantVote, ok2 := msg.Payload["voteGranted"].(bool)
	if !ok1 || !ok2 {
		return
	}
	term := int(termFloat)

	if term > r.CurrentTerm {
		r.CurrentTerm = term
		r.State = Follower
		r.VotedFor = ""
		r.resetElectionTimer()
		return
	}

	if r.State != Candidate || term != r.CurrentTerm {
		return
	}

	if grantVote {
		r.VotesReceived++
		totalNodes := len(r.PeerURLs) + 1
		if r.VotesReceived > totalNodes/2 {
			utils.Log(r.ID, "Won election for term %d. Becoming LEADER.", r.CurrentTerm)
			r.State = Leader
			
			for _, peerURL := range r.PeerURLs {
				r.nextIndex[peerURL] = len(r.Log)
				r.matchIndex[peerURL] = -1
			}
			
			if r.electionTimer != nil {
				r.electionTimer.Stop()
			}
			go r.startHeartbeats()
		}
	}
}
