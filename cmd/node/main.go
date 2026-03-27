package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"dfs-system/internal/replication"
)

var (
	nodeID  string
	port    string
	nodeURL string
	manager = replication.NewManager()

	localStorage = make(map[string]replication.FileData)
)

func main() {
	flag.StringVar(&nodeID, "id", "node1", "node ID")
	flag.StringVar(&port, "port", "8001", "port to run node on")
	flag.Parse()

	nodeURL = "http://localhost:" + port
	replication.RegisterNode(nodeID, nodeURL)

	// Register known nodes here for testing/demo
	replication.RegisterNode("node1", "http://localhost:8001")
	replication.RegisterNode("node2", "http://localhost:8002")
	replication.RegisterNode("node3", "http://localhost:8003")

	http.HandleFunc("/write", writeHandler)
	http.HandleFunc("/read", readHandler)
	http.HandleFunc("/replicate", replicateHandler)
	http.HandleFunc("/checkpoint", checkpointHandler)
	http.HandleFunc("/health", healthHandler)

	log.Printf("[%s] running on %s\n", nodeID, nodeURL)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func writeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Build nodes using the explicit ID=HOST:PORT mapping
	peerNodes := buildPeerNodes(cfg.Peers)

	// 2. We extract just the raw network addresses for the Heartbeat Sender
	// so it doesn't try to dial "node2=127.0.0.1:9002"
	var rawAddresses []string
	for _, node := range peerNodes {
		rawAddresses = append(rawAddresses, fmt.Sprintf("%s:%d", node.Host, node.Port))
	}

	fm := fault.NewFaultManager(cfg.NodeID, rawAddresses, peerNodes)
	fm.Start()

	api.FM = fm

	http.HandleFunc("/message", api.MessageHandler)
	http.HandleFunc("/status", api.StatusHandler)

	addr := cfg.Host + ":" + cfg.Port
	fmt.Printf("[NODE %s] Starting on %s\n", cfg.NodeID, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func buildPeerNodes(peers []string) []*types.Node {
	var nodes []*types.Node
	for _, peerStr := range peers {
		// Expects format: "node2=127.0.0.1:9002"
		parts := strings.Split(peerStr, "=")
		if len(parts) != 2 {
			continue
		}
		id := parts[0]
		addr := parts[1]

		addrParts := strings.Split(addr, ":")
		if len(addrParts) != 2 {
			continue
		}

		var port int
		fmt.Sscanf(addrParts[1], "%d", &port)

		node := types.NewNode(id, addrParts[0], port)
		nodes = append(nodes, node)
	}
	return nodes
}
	var file replication.FileData
	if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// for demo: replicate to all 3 nodes
	targetNodes := []string{"node1", "node2", "node3"}

	if err := manager.ReplicateFile(file, targetNodes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// keep local copy too
	latest, _ := replication.GetFile(file.Name)
	localStorage[file.Name] = latest

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "write successful",
		"file":    latest,
	})
}

func readHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "file name is required", http.StatusBadRequest)
		return
	}

	// try local node copy first
	if file, ok := localStorage[name]; ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(file)
		return
	}

	// fallback to global metadata store
	file, ok := replication.GetFile(name)
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(file)
}

func replicateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var file replication.FileData
	if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	localStorage[file.Name] = file
	replication.SaveReplicaOnNode(file, nodeID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("replicated successfully"))
}

func checkpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var snapshot replication.NodeSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		http.Error(w, "invalid checkpoint body", http.StatusBadRequest)
		return
	}

	for fileName, file := range snapshot.Files {
		localStorage[fileName] = file
		replication.SaveReplicaOnNode(file, nodeID)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("checkpoint synced"))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("%s is healthy", nodeID)))
}

// Optional helper if later you want to pass comma-separated targets
func parseTargets(targets string) []string {
	if strings.TrimSpace(targets) == "" {
		return nil
	}
	parts := strings.Split(targets, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
