package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"dfs-system/internal/transport"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

var allNodes = map[string]string{
	"node1": "127.0.0.1:9001", // PRIMARY
	"node2": "127.0.0.1:9002", // BACKUP
	"node3": "127.0.0.1:9003", // BACKUP
}

var primaryNode = "node1"
var backupOrder = []string{"node2", "node3"}

func main() {
	fmt.Println("=== Distributed File Storage Client ===")
	fmt.Println("Consistency Model: Strong Consistency (Passive Replication)")
	fmt.Println()

	// Step 1 — check cluster status
	fmt.Println("Step 1: Checking cluster status...")
	statuses := getClusterStatus()
	printStatuses(statuses)
	fmt.Println()

	// Step 2 — get the correct node to use (primary first)
	fmt.Println("Step 2: Determining which node to use...")
	targetNode := getTargetNode(statuses)

	if targetNode == "" {
		fmt.Println("❌ No nodes available — cannot serve request")
		return
	}
	fmt.Printf("✅ Using node: %s (%s)\n\n", getNodeID(targetNode), targetNode)

	// Step 3 — request file from correct node only
	fmt.Println("Step 3: Requesting file 'photo.jpg'...")
	requestFile("photo.jpg", targetNode)
	fmt.Println()

	// Step 4 — heartbeat to all alive nodes
	fmt.Println("Step 4: Sending heartbeats...")
	sendHeartbeats(statuses)
}

func getClusterStatus() map[string]string {
	statuses := map[string]string{
		"node1": "unknown",
		"node2": "unknown",
		"node3": "unknown",
	}

	for nodeID, addr := range allNodes {
		resp, err := http.Get("http://" + addr + "/status")
		if err != nil {
			statuses[nodeID] = "failed"
			continue
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)

		var peerStatuses map[string]map[string]interface{}
		if err := json.Unmarshal(body, &peerStatuses); err == nil {
			for peerID, info := range peerStatuses {
				if s, ok := info["status"].(string); ok {
					statuses[peerID] = s
				}
			}
		}

		statuses[nodeID] = "alive"
		break
	}

	return statuses
}

func getTargetNode(statuses map[string]string) string {
	// Always try primary first for strong consistency
	if statuses[primaryNode] == "alive" {
		fmt.Printf("  ✅ Primary (%s) is alive — using primary\n", primaryNode)
		return allNodes[primaryNode]
	}

	// Primary is dead — failover to backup
	fmt.Printf("  ❌ Primary (%s) is FAILED\n", primaryNode)
	fmt.Println("  🔄 Failing over to backup...")

	for _, backupID := range backupOrder {
		if statuses[backupID] == "alive" {
			fmt.Printf("  ✅ Failing over to backup: %s\n", backupID)
			return allNodes[backupID]
		}
		fmt.Printf("  ❌ Backup %s also FAILED\n", backupID)
	}

	return ""
}

func requestFile(filename, addr string) {
	msg, _ := types.NewMessage(
		types.MsgHeartbeat,
		"client",
		map[string]interface{}{
			"action":   "read",
			"filename": filename,
		},
	)

	err := transport.Send("http://"+addr+"/message", msg)
	if err != nil {
		fmt.Printf("  ❌ Failed to reach %s: %v\n", addr, err)
		return
	}
	fmt.Printf("  ✅ File '%s' served from %s\n", filename, addr)
}

func sendHeartbeats(statuses map[string]string) {
	for nodeID, status := range statuses {
		if status != "alive" {
			continue
		}
		addr := allNodes[nodeID]
		msg, _ := types.NewMessage(
			types.MsgHeartbeat,
			"client",
			map[string]interface{}{"status": "ping"},
		)
		err := transport.Send("http://"+addr+"/message", msg)
		if err != nil {
			utils.Log("CLIENT", "⚠️  Could not reach %s", addr)
		} else {
			utils.Log("CLIENT", "✅ Heartbeat sent to %s", addr)
		}
	}
}

func printStatuses(statuses map[string]string) {
	for nodeID, status := range statuses {
		role := "BACKUP"
		if nodeID == primaryNode {
			role = "PRIMARY"
		}
		if status == "alive" {
			fmt.Printf("  ✅ %s (%s) — ALIVE\n", nodeID, role)
		} else {
			fmt.Printf("  ❌ %s (%s) — FAILED\n", nodeID, role)
		}
	}
}

func getNodeID(addr string) string {
	for nodeID, a := range allNodes {
		if a == addr {
			return nodeID
		}
	}
	return addr
}