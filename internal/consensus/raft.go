package consensus

import (
	"math/rand"
	"sync"
	"time"

	"dfs-system/internal/utils"
)

type State string

const (
	Follower  State = "Follower"
	Candidate State = "Candidate"
	Leader    State = "Leader"
)

/*
 * Struct: LogEntry
 * Description: Represents a single operation in the Raft log.
 * Why: Ensures consistent replication across nodes.
 */
type LogEntry struct {
	Term int
	Op   string
}

/*
 * Struct: Raft
 * Description: Core state machine for leader election and log replication.
 * Why: Manages all consensus logic for the node.
 */
type Raft struct {
	sync.Mutex

	ID       string
	Host     string
	Port     string
	PeerURLs []string

	CurrentTerm int
	VotedFor    string
	State       State

	Log         []LogEntry
	CommitIndex int

	nextIndex  map[string]int
	matchIndex map[string]int

	VotesReceived int

	lastHeartbeat  time.Time
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	IsActive bool

	shutdownCh chan struct{}
}

/*
 * Function: NewRaft
 * Description: Creates a new Raft node.
 * Why: Initializes the node in follower state with empty logs.
 * Inputs:
 *   - id (string): Unique node identifier.
 *   - host (string): Node host address.
 *   - port (string): Node port.
 *   - peerURLs ([]string): List of peer addresses.
 * Outputs / Expected Outcome: Returns initialized Raft instance.
 */
func NewRaft(id, host, port string, peerURLs []string) *Raft {
	r := &Raft{
		ID:          id,
		Host:        host,
		Port:        port,
		PeerURLs:    peerURLs,
		State:       Follower,
		Log:         make([]LogEntry, 0),
		CommitIndex: -1,
		nextIndex:   make(map[string]int),
		matchIndex:  make(map[string]int),
		IsActive:    false, // Default to standby
		shutdownCh:  make(chan struct{}),
	}
	rand.Seed(time.Now().UnixNano())
	return r
}

func (r *Raft) StepDown() {
	r.State = Follower
	utils.Log(r.ID, "Stepping down to Follower state.")
}

/*
 * Function: SimulateDeath
 * Description: Stops Raft heartbeats and steps down from leadership.
 * Why: When a node is "killed" via the simulation endpoint, it must go silent
 *      so that followers' election timers fire and a new leader is elected.
 *      Without this, a dead-simulated leader keeps sending heartbeats and
 *      followers never start an election.
 * Inputs: None
 * Outputs / Expected Outcome: Heartbeat timer stops; state becomes Follower.
 */
func (r *Raft) SimulateDeath() {
	r.Lock()
	defer r.Unlock()
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
		r.heartbeatTimer = nil
	}
	if r.electionTimer != nil {
		r.electionTimer.Stop()
		r.electionTimer = nil
	}
	r.State = Follower
	utils.Log(r.ID, "⚰️  Node simulated dead — Raft heartbeats stopped, stepped down from leadership.")
}

/*
 * Function: SimulateRevive
 * Description: Restarts the election timer so the node re-joins consensus.
 * Why: When a node is revived via the simulation endpoint, it must start
 *      participating in elections again to become a candidate/follower.
 * Inputs: None
 * Outputs / Expected Outcome: Election timer restarts; node is Follower.
 */
func (r *Raft) SimulateRevive() {
	r.Lock()
	defer r.Unlock()
	r.State = Follower
	r.VotedFor = ""
	r.resetElectionTimer()
	utils.Log(r.ID, "💚 Node revived — election timer restarted.")
}

/*
 * Function: SimulateReviveAsBackup
 * Description: Revives node as a backup/standby node.
 * Why: When a failed node is revived by user, it should come back as backup
 *      not as an active node. Backup nodes don't participate in elections.
 * Inputs: None
 * Outputs / Expected Outcome: Node is Follower, IsActive=false, no election timer.
 */
func (r *Raft) SimulateReviveAsBackup() {
	r.Lock()
	defer r.Unlock()
	r.State = Follower
	r.VotedFor = ""
	r.IsActive = false // Ensure it stays as backup
	// Don't start election timer - backups don't participate in elections
	if r.electionTimer != nil {
		r.electionTimer.Stop()
		r.electionTimer = nil
	}
	// Keep heartbeat timer nil - backups don't send heartbeats
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
		r.heartbeatTimer = nil
	}
	utils.Log(r.ID, "💚 Node revived as BACKUP — will not participate in elections")
}

/*
 * Function: Start
 * Description: Starts Raft background processes.
 * Why: Begins election timer and consensus participation.
 * Inputs: None
 * Outputs / Expected Outcome: Node starts participating in the cluster.
 */
func (r *Raft) Start() {
	r.Lock()
	defer r.Unlock()
	if r.IsActive {
		r.resetElectionTimer()
		utils.Log(r.ID, "Raft consensus module started (ACTIVE)")
	} else {
		utils.Log(r.ID, "Raft consensus module started (STANDBY)")
	}
}

/*
 * Function: Stop
 * Description: Stops Raft safely.
 * Why: Prevents timers and goroutines from leaking after shutdown.
 * Inputs: None
 * Outputs / Expected Outcome: All timers and channels are closed cleanly.
 */
func (r *Raft) Stop() {
	r.Lock()
	if r.electionTimer != nil {
		r.electionTimer.Stop()
	}
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
	}
	r.Unlock()

	select {
	case <-r.shutdownCh:
	default:
		close(r.shutdownCh)
	}
}

// IsLeader safely checks if the node is currently the Raft Leader
func (r *Raft) IsLeader() bool {
	r.Lock()
	defer r.Unlock()
	return r.State == Leader
}

// GetLeader returns the URL of the current leader (or empty if unknown)
func (r *Raft) GetLeader() string {
	r.Lock()
	defer r.Unlock()

	if r.State == Leader {
		return r.Host + ":" + r.Port
	}

	// If we voted for someone in the current term and they are the leader
	// Currently we only track VotedFor, but in a full implementation we'd track the actual leader.
	// We'll approximate: we don't know the exact leader URL natively from State,
	// so if we aren't leader, we'll return empty string and let the frontend poll.
	return ""
}

// AddPeer registers a new peer into the Raft cluster dynamically
func (r *Raft) AddPeer(peerURL string) {
	r.Lock()
	defer r.Unlock()

	// Check if already exists or if it's our own address
	selfURL := r.Host + ":" + r.Port
	if peerURL == selfURL {
		return
	}

	for _, p := range r.PeerURLs {
		if p == peerURL {
			return
		}
	}
	r.PeerURLs = append(r.PeerURLs, peerURL)
}

// RemovePeer evicts a dead node from the Raft cluster membership
func (r *Raft) RemovePeer(peerURL string) {
	r.Lock()
	defer r.Unlock()

	newPeers := make([]string, 0)
	for _, p := range r.PeerURLs {
		if p != peerURL {
			newPeers = append(newPeers, p)
		}
	}
	r.PeerURLs = newPeers

	// Also clean up indexing metadata
	delete(r.nextIndex, peerURL)
	delete(r.matchIndex, peerURL)
}
