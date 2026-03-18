package types

import "sync"

type NodeStatus string
type NodeRole string

const (
	StatusAlive  NodeStatus = "alive"
	StatusFailed NodeStatus = "failed"

	RoleLeader   NodeRole = "leader"
	RoleFollower NodeRole = "follower"
)

type Node struct {
	ID          string
	Host        string
	Port        int
	Status      NodeStatus
	Role        NodeRole
	ClockOffset float64
	Files       map[string]*FileEntry
	Mu          sync.RWMutex // always lock before reading/writing Files
}

func NewNode(id, host string, port int) *Node {
	return &Node{
		ID:     id,
		Host:   host,
		Port:   port,
		Status: StatusAlive,
		Role:   RoleFollower,
		Files:  make(map[string]*FileEntry),
	}
}