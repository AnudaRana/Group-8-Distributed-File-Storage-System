package consensus

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

type State string

const (
	Follower  State = "Follower"
	Candidate State = "Candidate"
	Leader    State = "Leader"
)

type Raft struct {
	mu sync.Mutex

	ID        string
	Host      string
	Port      string
	PeerURLs  []string 

	CurrentTerm int
	VotedFor    string
	State       State

	VotesReceived int

	lastHeartbeat  time.Time
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	shutdownCh chan struct{}
}

/*
 * Function: NewRaft
 * Description: Initializes a new Raft node.
 * Date Written: 21/03/2026
 * Why: Sets up the node as a Follower with basic config before joining the cluster.
 * Inputs:
 *   - id (string): Unique identifier for this node.
 *   - host (string): Host address of the node.
 *   - port (string): Port on which the node listens.
 *   - peerURLs ([]string): List of peer addresses.
 * Outputs / Expected Outcome:
 *   - Returns a pointer to a new Raft instance initialized as a Follower.
 */

func NewRaft(id, host, port string, peerURLs []string) *Raft {
	r := &Raft{
		ID:         id,
		Host:       host,
		Port:       port,
		PeerURLs:   peerURLs,
		State:      Follower,
		shutdownCh: make(chan struct{}),
	}
	rand.Seed(time.Now().UnixNano())
	return r
}

/*
 * Function: Start
 * Description: Starts the Raft process.
 * Date Written: 21/03/2026
 * Why: Kicks off the election timer after the node is fully initialized.
 * Inputs: None
 * Outputs / Expected Outcome: Begins leader election cycle.
 */

func (r *Raft) Start() {
	r.resetElectionTimer()
	utils.Log(r.ID, "Raft consensus module started")
}

/*
 * Function: Stop
 * Description: Stops the Raft node.
 * Date Written: 21/03/2026
 * Why: Allows clean shutdown of all background processes.
 * Inputs: None
 * Outputs / Expected Outcome: Closes shutdown channel.
 */

func (r *Raft) Stop() {
	close(r.shutdownCh)
}

/*
 * Function: electionTimeout
 * Description: Generates a random election timeout.
 * Date Written: 21/03/2026
 * Why: Prevents multiple nodes from starting elections at the same time.
 * Inputs: None
 * Outputs / Expected Outcome: Random duration between 1.5s–3s.
 */

func (r *Raft) electionTimeout() time.Duration {
	return time.Duration(1500+rand.Intn(1500)) * time.Millisecond
}

/*
 * Function: resetElectionTimer
 * Description: Resets the election timer.
 * Date Written: 21/03/2026
 * Why: Called after heartbeat or vote to avoid unnecessary elections.
 * Inputs: None
 * Outputs / Expected Outcome: Starts a new timer that triggers startElection().
 */

func (r *Raft) resetElectionTimer() {
	if r.electionTimer != nil {
		r.electionTimer.Stop()
	}
	r.electionTimer = time.AfterFunc(r.electionTimeout(), r.startElection)
}

/*
 * Function: startElection
 * Description: Starts a new election.
 * Date Written: 21/03/2026
 * Why: Triggered when no leader is detected within timeout.
 * Inputs: None
 * Outputs / Expected Outcome: Node becomes Candidate, increments term, votes for itself, requests votes.
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
 * Date Written: 21/03/2026
 * Why: Needed to collect votes and become Leader.
 * Inputs:
 *   - peerURL (string): Network routing address of the target peer.
 *   - term (int): Candidate's current election term.
 * Outputs / Expected Outcome: Sends MsgVoteReq to peer.
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
 * Date Written: 21/03/2026
 * Why: Decides whether to grant vote based on term and voting rules.
 * Inputs:
 *   - msg (*types.Message): The network message payload containing the Candidate's term and callback URL.
 * Outputs / Expected Outcome: Updates term if needed, grants/rejects vote, sends reply.
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
 * Date Written: 21/03/2026
 * Why: Candidate needs responses to determine majority.
 * Inputs:
 *   - candidateUrl (string): Network routing address of the requesting Candidate.
 *   - term (int): Current term of the voting node.
 *   - grantVote (bool): True if the node is voting for the Candidate, False otherwise.
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
 * Description: Handles incoming vote replies.
 * Date Written: 21/03/2026
 * Why: Used to count votes and decide if this node becomes Leader.
 * Inputs:
 *   - msg (*types.Message): Contains term and whether vote was granted.
 * Outputs / Expected Outcome: Becomes Leader if majority is reached, otherwise continues election.
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
			if r.electionTimer != nil {
				r.electionTimer.Stop()
			}
			r.startHeartbeats()
		}
	}
}

/*
 * Function: startHeartbeats
 * Description: Starts the heartbeat loop.
 * Date Written: 21/03/2026
 * Why: Leader needs to continuously signal it is alive to all followers.
 * Inputs: None
 * Outputs / Expected Outcome: Sends heartbeats immediately and schedules them every 500ms.
 */

func (r *Raft) startHeartbeats() {
	r.sendHeartbeats()
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
	}
	r.heartbeatTimer = time.AfterFunc(500*time.Millisecond, r.startHeartbeats)
}

/*
 * Function: sendHeartbeats
 * Description: Sends heartbeat messages to all peers.
 * Date Written: 21/03/2026
 * Why: Keeps followers from starting new elections and maintains leadership.
 * Inputs: None
 * Outputs / Expected Outcome: Broadcasts MsgLeaderHB to all peers.
 */

func (r *Raft) sendHeartbeats() {
	r.mu.Lock()
	if r.State != Leader {
		r.mu.Unlock()
		return
	}
	term := r.CurrentTerm
	r.mu.Unlock()

	for _, peerURL := range r.PeerURLs {
		payload := map[string]interface{}{
			"term": float64(term),
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
 * Description: Handles incoming leader heartbeat.
 * Date Written: 21/03/2026
 * Why: Confirms leader is still active and prevents this node from starting an election.
 * Inputs:
 *   - msg (*types.Message): Contains leader's current term.
 * Outputs / Expected Outcome: Updates term if needed, stays as Follower, and resets election timer.
 */

func (r *Raft) HandleLeaderHeartbeat(msg *types.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	termFloat, ok := msg.Payload["term"].(float64)
	if !ok {
		return
	}
	term := int(termFloat)

	if term >= r.CurrentTerm {
		r.CurrentTerm = term
		r.State = Follower
		r.resetElectionTimer()
	}
}
