package main

import (
	"fmt"

	"dfs-system/internal/replication"
)

func main() {
	fmt.Println("=== Recovery Test ===")

	replication.RegisterNode("node1", "http://localhost:8001")
	replication.RegisterNode("node2", "http://localhost:8002")
	replication.RegisterNode("node3", "http://localhost:8003")

	manager := replication.NewManager()

	err := manager.SyncNodeFromCheckpoint("node2")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Checkpoint sync triggered successfully for node2")
}
