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
	mu sync.Mutex

	ID        string
	Host      string
	Port      string
	PeerURLs  []string 

	CurrentTerm int
	VotedFor    string
	State       State
	
	Log         []LogEntry
	CommitIndex int
	
	nextIndex   map[string]int
	matchIndex  map[string]int

	VotesReceived int

	lastHeartbeat  time.Time
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

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
		shutdownCh:  make(chan struct{}),
	}
	rand.Seed(time.Now().UnixNano())
	return r
}

/*
 * Function: Start
 * Description: Starts Raft background processes.
 * Why: Begins election timer and consensus participation.
 * Inputs: None
 * Outputs / Expected Outcome: Node starts participating in the cluster.
 */
func (r *Raft) Start() {
	r.resetElectionTimer()
	utils.Log(r.ID, "Raft consensus module started")
}

/*
 * Function: Stop
 * Description: Stops Raft safely.
 * Why: Prevents timers and goroutines from leaking after shutdown.
 * Inputs: None
 * Outputs / Expected Outcome: All timers and channels are closed cleanly.
 */
func (r *Raft) Stop() {
	r.mu.Lock()
	if r.electionTimer != nil {
		r.electionTimer.Stop()
	}
	if r.heartbeatTimer != nil {
		r.heartbeatTimer.Stop()
	}
	r.mu.Unlock()

	select {
	case <-r.shutdownCh:
	default:
		close(r.shutdownCh)
	}
}
