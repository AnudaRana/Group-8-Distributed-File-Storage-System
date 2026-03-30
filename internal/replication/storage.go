package replication

import "sync"

var (
	fileStore    = make(map[string]FileData) // fileName -> latest file
	replicaMap   = make(map[string][]string) // fileName -> nodeIDs storing replicas
	nodeFileMap  = make(map[string][]string) // nodeID -> fileNames
	clusterNodes = make(map[string]string)   // nodeID -> nodeURL
	storeMutex   sync.RWMutex
)

// RegisterNode adds a new node's URL to the cluster map.
func RegisterNode(nodeID, nodeURL string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	clusterNodes[nodeID] = nodeURL
}

// SaveFile stores a file and maps it to specific node replicas.
func SaveFile(file FileData, nodeIDs []string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	fileStore[file.Name] = file
	replicaMap[file.Name] = uniqueStrings(nodeIDs)

	for _, nodeID := range nodeIDs {
		nodeFileMap[nodeID] = appendIfMissing(nodeFileMap[nodeID], file.Name)
	}
}

// SaveReplicaOnNode records that a specific file exists on a specific node.
func SaveReplicaOnNode(file FileData, nodeID string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	current, exists := fileStore[file.Name]
	if !exists || file.Version >= current.Version {
		fileStore[file.Name] = file
	}

	replicaMap[file.Name] = appendIfMissing(replicaMap[file.Name], nodeID)
	nodeFileMap[nodeID] = appendIfMissing(nodeFileMap[nodeID], file.Name)
}

// RemoveNodeFromAllReplicaMaps purges a node from all file-tracking systems.
func RemoveNodeFromAllReplicaMaps(nodeID string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	for fileName, nodes := range replicaMap {
		replicaMap[fileName] = removeString(nodes, nodeID)
	}

	delete(nodeFileMap, nodeID)
	delete(clusterNodes, nodeID)
}

// GetFile retrieves a specific file's metadata from storage.
func GetFile(fileName string) (FileData, bool) {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	file, ok := fileStore[fileName]
	return file, ok
}

// GetAllFiles returns a complete map of all files actively stored.
func GetAllFiles() map[string]FileData {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	result := make(map[string]FileData)
	for k, v := range fileStore {
		result[k] = v
	}
	return result
}

// GetFilesOnNode returns a list of filenames stored on a specific node.
func GetFilesOnNode(nodeID string) []string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	files := nodeFileMap[nodeID]
	result := make([]string, len(files))
	copy(result, files)
	return result
}

// GetReplicaNodes returns a list of node IDs that hold copies of a specific file.
func GetReplicaNodes(fileName string) []string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	nodes := replicaMap[fileName]
	result := make([]string, len(nodes))
	copy(result, nodes)
	return result
}

// UpdateReplicaNodes forcefully overwrites the list of nodes hosting a specific file.
func UpdateReplicaNodes(fileName string, nodeIDs []string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	replicaMap[fileName] = uniqueStrings(nodeIDs)

	// clean existing references for this file
	for nodeID, files := range nodeFileMap {
		nodeFileMap[nodeID] = removeString(files, fileName)
	}

	// add fresh references
	for _, nodeID := range nodeIDs {
		nodeFileMap[nodeID] = appendIfMissing(nodeFileMap[nodeID], fileName)
	}
}

// GetActiveNodesExcluding returns all registered node IDs except the excluded one.
func GetActiveNodesExcluding(excludeNodeID string) []string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	var nodes []string
	for nodeID := range clusterNodes {
		if nodeID != excludeNodeID {
			nodes = append(nodes, nodeID)
		}
	}
	return nodes
}

// GetNodeURL retrieves the network URL for a given node ID.
func GetNodeURL(nodeID string) (string, bool) {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	url, ok := clusterNodes[nodeID]
	return url, ok
}

// GetAllRegisteredNodes retrieves a complete map of all nodes and their network URLs.
func GetAllRegisteredNodes() map[string]string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	result := make(map[string]string)
	for k, v := range clusterNodes {
		result[k] = v
	}
	return result
}

// appendIfMissing securely adds a string to a slice only if it does not already exist.
func appendIfMissing(slice []string, value string) []string {
	for _, item := range slice {
		if item == value {
			return slice
		}
	}
	return append(slice, value)
}

// removeString removes all instances of a specific string from a slice.
func removeString(slice []string, value string) []string {
	var result []string
	for _, item := range slice {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

// uniqueStrings deduplicates a string slice and returns only unique values.
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range input {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// DeleteFile fully purges a file and all its network metadata from storage.
func DeleteFile(fileName string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	delete(fileStore, fileName)
	delete(replicaMap, fileName)

	// clean existing references for this file
	for nodeID, files := range nodeFileMap {
		nodeFileMap[nodeID] = removeString(files, fileName)
	}
}

// RenameFile updates all storage metadata records to replace an old filename with a new one.
func RenameFile(oldName, newName string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	if file, ok := fileStore[oldName]; ok {
		file.Name = newName
		fileStore[newName] = file
		delete(fileStore, oldName)
	}

	if nodes, ok := replicaMap[oldName]; ok {
		replicaMap[newName] = nodes
		delete(replicaMap, oldName)
	}

	for nodeID, files := range nodeFileMap {
		for i, name := range files {
			if name == oldName {
				files[i] = newName
			}
		}
		nodeFileMap[nodeID] = files
	}
}
