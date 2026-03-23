package replication

import "sync"

var (
	fileStore    = make(map[string]FileData) // fileName -> latest file
	replicaMap   = make(map[string][]string) // fileName -> nodeIDs storing replicas
	nodeFileMap  = make(map[string][]string) // nodeID -> fileNames
	clusterNodes = make(map[string]string)   // nodeID -> nodeURL
	storeMutex   sync.RWMutex
)

func RegisterNode(nodeID, nodeURL string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	clusterNodes[nodeID] = nodeURL
}

func SaveFile(file FileData, nodeIDs []string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	fileStore[file.Name] = file
	replicaMap[file.Name] = uniqueStrings(nodeIDs)

	for _, nodeID := range nodeIDs {
		nodeFileMap[nodeID] = appendIfMissing(nodeFileMap[nodeID], file.Name)
	}
}

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

func RemoveNodeFromAllReplicaMaps(nodeID string) {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	for fileName, nodes := range replicaMap {
		replicaMap[fileName] = removeString(nodes, nodeID)
	}

	delete(nodeFileMap, nodeID)
}

func GetFile(fileName string) (FileData, bool) {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	file, ok := fileStore[fileName]
	return file, ok
}

func GetAllFiles() map[string]FileData {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	result := make(map[string]FileData)
	for k, v := range fileStore {
		result[k] = v
	}
	return result
}

func GetFilesOnNode(nodeID string) []string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	files := nodeFileMap[nodeID]
	result := make([]string, len(files))
	copy(result, files)
	return result
}

func GetReplicaNodes(fileName string) []string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	nodes := replicaMap[fileName]
	result := make([]string, len(nodes))
	copy(result, nodes)
	return result
}

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

func GetNodeURL(nodeID string) (string, bool) {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	url, ok := clusterNodes[nodeID]
	return url, ok
}

func GetAllRegisteredNodes() map[string]string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	result := make(map[string]string)
	for k, v := range clusterNodes {
		result[k] = v
	}
	return result
}

func appendIfMissing(slice []string, value string) []string {
	for _, item := range slice {
		if item == value {
			return slice
		}
	}
	return append(slice, value)
}

func removeString(slice []string, value string) []string {
	var result []string
	for _, item := range slice {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

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
