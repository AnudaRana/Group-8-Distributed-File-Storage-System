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

func NewManager() *Manager {
	return &Manager{}
}

// Initial write / update replication
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

// Called by Member 1 when a node fails
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

// Called by Member 1 when a node rejoins
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

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
