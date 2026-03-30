package replication

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const ReplicationFactor = 3

type Manager struct{}

// NewManager initializes and returns a new replica manager instance.
func NewManager() *Manager {
	return &Manager{}
}

// ReplicateFile replicates a new or updated file across its preferred storage nodes.
func (m *Manager) ReplicateFile(file FileData, preferredNodeIDs []string) error {
	file.Timestamp = time.Now().Unix()

	existing, exists := GetFile(file.Name)
	if exists {
		file.Version = existing.Version + 1
	} else {
		file.Version = 1
	}

	targetNodes := uniqueStrings(preferredNodeIDs)
	if len(targetNodes) > ReplicationFactor {
		targetNodes = targetNodes[:ReplicationFactor]
	}

	for _, nodeID := range targetNodes {
		if err := sendFileToNode(nodeID, file); err != nil {
			return err
		}
	}

	SaveFile(file, targetNodes)
	return nil
}

// ReplicateFilesFromFailedNode automatically reassigns and replicates files when a node drops offline.
func (m *Manager) ReplicateFilesFromFailedNode(nodeID string) error {
	files := GetFilesOnNode(nodeID)
	activeNodes := GetActiveNodesExcluding(nodeID)

	// remove failed node from maps first
	RemoveNodeFromAllReplicaMaps(nodeID)

	for _, fileName := range files {
		file, ok := GetFile(fileName)
		if !ok {
			continue
		}

		currentReplicas := GetReplicaNodes(fileName)

		for _, candidate := range activeNodes {
			if len(currentReplicas) >= ReplicationFactor {
				break
			}
			if contains(currentReplicas, candidate) {
				continue
			}

			if err := sendFileToNode(candidate, file); err == nil {
				currentReplicas = append(currentReplicas, candidate)
			}
		}

		UpdateReplicaNodes(fileName, currentReplicas)
	}

	return nil
}

// SyncNodeFromCheckpoint rapidly updates a newly joined node by sending it the current active cluster state.
func (m *Manager) SyncNodeFromCheckpoint(nodeID string) error {
	url, ok := GetNodeURL(nodeID)
	if !ok {
		return fmt.Errorf("node URL not found for %s", nodeID)
	}

	allFiles := GetAllFiles()

	// First send full checkpoint
	snapshot := NodeSnapshot{
		NodeID: nodeID,
		Files:  allFiles,
	}

	jsonData, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	resp, err := http.Post(url+"/checkpoint", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checkpoint sync failed with status %d", resp.StatusCode)
	}

	// Then decide which files should include the rejoining node
	for fileName, file := range allFiles {
		replicas := GetReplicaNodes(fileName)

		if !contains(replicas, nodeID) && len(replicas) < ReplicationFactor {
			if err := sendFileToNode(nodeID, file); err == nil {
				replicas = append(replicas, nodeID)
			}
		}

		// remove extra replica if > 3
		if len(replicas) > ReplicationFactor {
			var trimmed []string
			for _, replicaNode := range replicas {
				if len(trimmed) >= ReplicationFactor {
					break
				}
				trimmed = append(trimmed, replicaNode)
			}
			replicas = trimmed
		}

		UpdateReplicaNodes(fileName, replicas)
	}

	return nil
}

// sendFileToNode natively transfers the given file packet directly to a specific target offline or online node API.
func sendFileToNode(nodeID string, file FileData) error {
	url, ok := GetNodeURL(nodeID)
	if !ok {
		return fmt.Errorf("node URL not found for %s", nodeID)
	}

	jsonData, err := json.Marshal(file)
	if err != nil {
		return err
	}

	resp, err := http.Post(url+"/replicate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("replication to node %s failed with status %d", nodeID, resp.StatusCode)
	}

	return nil
}

// contains performs a quick check if a specific string is present within a given array.
func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// DeleteFileReplicas signals all storage nodes holding a specific file to immediately delete their local copies.
func (m *Manager) DeleteFileReplicas(fileName string) error {
	replicas := GetReplicaNodes(fileName)
	for _, nodeID := range replicas {
		url, ok := GetNodeURL(nodeID)
		if !ok {
			continue
		}
		req, _ := http.NewRequest(http.MethodDelete, url+"/internal/delete?name="+fileName, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	DeleteFile(fileName)
	return nil
}

// RenameFileReplicas signals all storage nodes to synchronously rename their physical local files.
func (m *Manager) RenameFileReplicas(oldName, newName string) error {
	replicas := GetReplicaNodes(oldName)
	for _, nodeID := range replicas {
		url, ok := GetNodeURL(nodeID)
		if !ok {
			continue
		}
		req, _ := http.NewRequest(http.MethodPatch, url+"/internal/rename?old="+oldName+"&new="+newName, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
	RenameFile(oldName, newName)
	return nil
}

// StartSelfHealing is a continuous background process run by the leader to repair any missing or corrupted cluster data.
func (m *Manager) StartSelfHealing(isLeader func() bool) {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			if !isLeader() {
				continue
			}
			allFiles := GetAllFiles()
			activeNodes := GetActiveNodesExcluding("") // all active

			for fileName, file := range allFiles {
				replicas := GetReplicaNodes(fileName)
				
				// Aggressively fix inconsistency: every active node should have every file.
				if len(replicas) < len(activeNodes) {
					// Collect all available nodes that don't have the file yet
					var candidates []string
					for _, nodeID := range activeNodes {
						if !contains(replicas, nodeID) {
							candidates = append(candidates, nodeID)
						}
					}

					// Proactively push to candidates
					for _, nodeID := range candidates {
						if err := sendFileToNode(nodeID, file); err == nil {
							replicas = append(replicas, nodeID)
							fmt.Printf("[SELF-HEALING] Proactively pushed missing replica of %s to %s for total consistency\n", fileName, nodeID)
						}
					}
					UpdateReplicaNodes(fileName, replicas)
				}
			}
			fmt.Printf("[SELF-HEALING] 🔄 5s cluster sync complete. Cluster state is healthy.\n")
		}
	}()
}
