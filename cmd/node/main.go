package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"dfs-system/internal/api"
	"dfs-system/internal/clock"
	"dfs-system/internal/config"
	"dfs-system/internal/consensus"
	"dfs-system/internal/fault"
	"dfs-system/internal/replication"
	"dfs-system/internal/transport"
	"dfs-system/internal/types"
)

var (
	cfg        = config.LoadConfig()
	manager    = replication.NewManager()
	storageDir string

	logMu  sync.Mutex
	actLog []LogEntry

	standbyMu    sync.Mutex
	standbyNodes = []string{"node4", "node5"}

	spawnedForMu sync.Mutex
	spawnedFor   = make(map[string]bool)

	permanentlyRemovedMu sync.Mutex
	permanentlyRemoved   = make(map[string]bool)

	startTime = time.Now()
)

const bootstrapGracePeriod = 60 * time.Second

type LogEntry struct {
	Time    string `json:"time"`
	User    string `json:"user"`
	Action  string `json:"action"`
	File    string `json:"file,omitempty"`
	Details string `json:"details,omitempty"`
}

// addLog intercepts any file-system mutating action (upload/delete/rename) handled by the leader,
// appends it dynamically into a capped memory ring-buffer, and persists the raw JSON representation
// directly into `activity.log` on the local node disk for durability.
func addLog(user, action, file, details string) {
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		User:    user,
		Action:  action,
		File:    file,
		Details: details,
	}

	logMu.Lock()
	actLog = append(actLog, entry)
	if len(actLog) > 100 {
		actLog = actLog[1:]
	}
	logMu.Unlock()

	logPath := filepath.Join("data", cfg.NodeID, "activity.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		jsonData, _ := json.Marshal(entry)
		f.Write(append(jsonData, '\n'))
	}
}

// loadLogsFromDisk reads the physical `activity.log` file from the host's target data directory during bootstrap,
// recovering up to the last 100 historical file system actions into memory to instantly populate the React frontend.
func loadLogsFromDisk() {
	logPath := filepath.Join("data", cfg.NodeID, "activity.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	logMu.Lock()
	defer logMu.Unlock()

	fileActions := map[string]bool{"upload": true, "download": true, "delete": true, "rename": true}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil && fileActions[entry.Action] {
			actLog = append(actLog, entry)
		}
	}
	if len(actLog) > 100 {
		actLog = actLog[len(actLog)-100:]
	}
}

func main() {
	var peersStr string
	flag.StringVar(&cfg.NodeID, "id", cfg.NodeID, "node ID")
	flag.StringVar(&cfg.Port, "port", cfg.Port, "HTTP port")
	flag.StringVar(&cfg.Host, "host", cfg.Host, "host address")
	flag.StringVar(&peersStr, "peers", "", "comma-separated peer list (node=host:port)")
	flag.Parse()

	if peersStr != "" {
		cfg.Peers = config.ParsePeers(peersStr)
	}

	storageDir = filepath.Join("data", cfg.NodeID, "files")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		log.Fatalf("cannot create storage dir: %v", err)
	}

	loadLogsFromDisk()

	addr := cfg.Host + ":" + cfg.Port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	assignedPort := listener.Addr().(*net.TCPAddr).Port
	cfg.Port = strconv.Itoa(assignedPort)
	actualAddr := cfg.Host + ":" + cfg.Port

	nodeURL := fmt.Sprintf("http://%s", actualAddr)
	replication.RegisterNode(cfg.NodeID, nodeURL)

	peerNodes := buildPeerNodes(cfg.Peers)
	for _, node := range peerNodes {
		peerURL := fmt.Sprintf("http://%s:%d", node.Host, node.Port)
		replication.RegisterNode(node.ID, peerURL)
	}

	var rawAddresses []string
	for _, node := range peerNodes {
		rawAddresses = append(rawAddresses, fmt.Sprintf("%s:%d", node.Host, node.Port))
	}

	isLeader := func() bool {
		return api.Consensus != nil && api.Consensus.IsLeader()
	}

	fm := fault.NewFaultManager(cfg.NodeID, rawAddresses, peerNodes, manager, isLeader)

	fm.OnNodeFailureCallback = func(deadNodeID string, deadAddr string) {

		if time.Since(startTime) < bootstrapGracePeriod {
			fmt.Printf("[ORCHESTRATOR] ⚠️  Node %s timed out during bootstrap — NOT purging yet.\n", deadNodeID)
			return
		}

		fmt.Printf("[ORCHESTRATOR] 🗑️  Purging dead node %s from membership...\n", deadNodeID)

		permanentlyRemovedMu.Lock()
		permanentlyRemoved[deadNodeID] = true
		permanentlyRemovedMu.Unlock()

		replication.RemoveNodeFromAllReplicaMaps(deadNodeID)

		if api.Consensus != nil && deadAddr != "" {
			api.Consensus.RemovePeer(deadAddr)
		}

		if api.FM != nil && deadAddr != "" {
			api.FM.RemovePeer(deadNodeID, deadAddr)
		}
	}

	fm.Start()

	api.FM = fm

	var peerURLs []string
	for _, node := range peerNodes {
		peerURLs = append(peerURLs, fmt.Sprintf("%s:%d", node.Host, node.Port))
	}
	raftNode := consensus.NewRaft(cfg.NodeID, cfg.Host, cfg.Port, peerURLs)
	if cfg.NodeID == "node1" || cfg.NodeID == "node2" || cfg.NodeID == "node3" {
		raftNode.IsActive = true
	}
	raftNode.Start()
	api.Consensus = raftNode

	manager.StartSelfHealing(func() bool {
		return api.Consensus != nil && api.Consensus.IsLeader()
	})

	go func() {
		for {
			time.Sleep(3 * time.Second)

			if api.Consensus == nil || !api.Consensus.IsLeader() {
				continue
			}

			api.Consensus.Lock()
			activeCount := 0
			var activePeers []string
			for _, p := range api.Consensus.PeerURLs {
				if p != "" {
					activeCount++
					activePeers = append(activePeers, p)
				}
			}
			if api.Consensus.IsActive {
				activeCount++
			}
			api.Consensus.Unlock()

			if activeCount >= 3 {
				continue
			}

			fmt.Printf("[ORCHESTRATOR] ⚠️ Leader detected cluster size dropped to %d. Initiating standby replacement...\n", activeCount)

			standbyMu.Lock()
			if len(standbyNodes) == 0 {
				fmt.Printf("[ORCHESTRATOR] âŒ Cannot spawn replacement: No nodes left in standby pool!\n")
				standbyMu.Unlock()
				time.Sleep(10 * time.Second) // Prevent log spam when depleted
				continue
			}
			newNodeID := standbyNodes[0]
			standbyNodes = standbyNodes[1:]
			standbyMu.Unlock()

			port := "8004"
			if newNodeID == "node5" {
				port = "8005"
			}

			survivorPeers := []string{fmt.Sprintf("%s=%s:%s", cfg.NodeID, cfg.Host, cfg.Port)}
			freshNodes := replication.GetAllRegisteredNodes()
			for _, p := range activePeers {
				pID := ""
				for id, url := range freshNodes {
					if strings.TrimPrefix(url, "http://") == p {
						pID = id
						break
					}
				}
				if pID != "" {
					survivorPeers = append(survivorPeers, fmt.Sprintf("%s=%s", pID, p))
				}
			}
			peerArg := strings.Join(survivorPeers, ",")

			exe, _ := os.Executable()
			if exe == "" {
				exe = "node.exe"
			}

			fmt.Printf("[ORCHESTRATOR] 🚀 Popping NEW terminal for %s on port %s!\n", newNodeID, port)

			// Simple, native Windows start command without nested cmd /k string chaos
			cmdArgs := []string{"/c", "start", fmt.Sprintf("Node %s", newNodeID), "cmd", "/c", "echo Starting Standby Node & " + exe + " -id " + newNodeID + " -port " + port + " -host 127.0.0.1 -peers " + peerArg + " & pause"}

			cmd := exec.Command("cmd.exe", cmdArgs...)

			if err := cmd.Run(); err != nil {
				fmt.Printf("[ORCHESTRATOR] ❌ Failed to execute popup: %v\n", err)
				standbyMu.Lock()
				standbyNodes = append([]string{newNodeID}, standbyNodes...)
				standbyMu.Unlock()
			} else {
				fmt.Printf("[ORCHESTRATOR] ✅ Terminal spawned for %s! Waiting for it to join the cluster...\n", newNodeID)
				time.Sleep(5 * time.Second)

				go func(nodeID string, nodePort string) {
					time.Sleep(2 * time.Second)

					nodeAddr := fmt.Sprintf("127.0.0.1:%s", nodePort)

					var activePeers []string
					api.Consensus.Lock()
					for _, p := range api.Consensus.PeerURLs {
						if p != "" {
							activePeers = append(activePeers, p)
						}
					}
					if api.Consensus.IsActive {
						activePeers = append(activePeers, fmt.Sprintf("%s:%s", cfg.Host, cfg.Port))
					}
					api.Consensus.Unlock()

					promoteURL := fmt.Sprintf("http://%s/api/promote", nodeAddr)
					payload := map[string]interface{}{"peers": activePeers}
					jsonData, _ := json.Marshal(payload)

					resp, err := http.Post(promoteURL, "application/json", bytes.NewBuffer(jsonData))
					if err != nil {
						fmt.Printf("[ORCHESTRATOR] ❌ Failed to promote %s: %v\n", nodeID, err)
						return
					}
					resp.Body.Close()
					fmt.Printf("[ORCHESTRATOR] ✅ Node %s promoted to ACTIVE\n", nodeID)

					api.Consensus.AddPeer(nodeAddr)
					if api.FM != nil {
						portInt, _ := strconv.Atoi(nodePort)
						api.FM.AddPeer(types.NewNode(nodeID, "127.0.0.1", portInt))
					}
				}(newNodeID, port)
			}
		}
	}()

	selfAddr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	for _, pn := range peerNodes {
		peerAddr := fmt.Sprintf("%s:%d", pn.Host, pn.Port)
		if peerAddr == selfAddr {
			continue
		}
		peerURL := fmt.Sprintf("http://%s", peerAddr)
		api.ClockSyncer = clock.NewSyncer(peerURL)
		if _, err := api.ClockSyncer.Synchronise(); err != nil {
			log.Printf("[NODE %s] Initial clock sync failed: %v", cfg.NodeID, err)
		}
		stopCh := make(chan struct{})
		api.ClockSyncer.RunLoop(30*time.Second, stopCh)
		break
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/message", api.MessageHandler)
	mux.HandleFunc("/time", clock.TimeHandler)
	mux.HandleFunc("/status", api.StatusHandler)
	mux.HandleFunc("/replicate", replicateHandler)
	mux.HandleFunc("/checkpoint", checkpointHandler)
	mux.HandleFunc("/health", healthHandler)

	mux.HandleFunc("/api/status", corsMiddleware(apiStatusHandler))
	mux.HandleFunc("/api/fault-stats", corsMiddleware(apiFaultStatsHandler))
	mux.HandleFunc("/api/consensus", corsMiddleware(apiConsensusHandler))
	mux.HandleFunc("/api/clock", corsMiddleware(apiClockHandler))
	mux.HandleFunc("/api/replication", corsMiddleware(apiReplicationHandler))
	mux.HandleFunc("/api/files", corsMiddleware(apiFilesHandler))
	mux.HandleFunc("/api/files/upload", corsMiddleware(gracefulMiddleware(apiUploadHandler)))
	mux.HandleFunc("/api/files/", corsMiddleware(gracefulMiddleware(apiFileActionHandler)))
	mux.HandleFunc("/api/logs", corsMiddleware(apiLogsHandler))
	mux.HandleFunc("/api/promote", corsMiddleware(apiPromoteHandler))
	mux.HandleFunc("/api/join", corsMiddleware(apiJoinHandler))
	mux.HandleFunc("/api/nodes", corsMiddleware(apiNodesHandler))
	mux.HandleFunc("/api/simulate/kill", corsMiddleware(apiSimulateKillHandler))
	mux.HandleFunc("/api/simulate/revive", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		apiSimulateReviveHandler(w, r, peerNodes)
	}))
	mux.HandleFunc("/internal/delete", internalDeleteHandler)
	mux.HandleFunc("/internal/rename", internalRenameHandler)

	fmt.Printf("[NODE %s] Starting on http://%s\n", cfg.NodeID, actualAddr)

	if len(peerNodes) > 0 {
		go discoverAndJoin(peerNodes)
	}

	log.Fatal(http.Serve(listener, mux))
}

// discoverAndJoin provides a self-healing bootstrap sequence which pings seed nodes provided via CLI parameters,
// seamlessly integrating the current instance into a live cluster by receiving peer lists and syncing up replication topologies.
func discoverAndJoin(peerNodes []*types.Node) {
	time.Sleep(1 * time.Second)
	for _, peer := range peerNodes {
		peerUrl := fmt.Sprintf("http://%s:%d/api/join", peer.Host, peer.Port)

		payload := map[string]string{
			"node_id": cfg.NodeID,
			"host":    cfg.Host,
			"port":    cfg.Port,
		}
		jsonData, _ := json.Marshal(payload)

		resp, err := http.Post(peerUrl, "application/json", bytes.NewBuffer(jsonData))
		if err == nil && resp.StatusCode == 200 {
			var result struct {
				Peers map[string]string `json:"peers"`
			}
			json.NewDecoder(resp.Body).Decode(&result)

			for id, url := range result.Peers {
				if id == cfg.NodeID {
					continue
				}

				permanentlyRemovedMu.Lock()
				isRemoved := permanentlyRemoved[id]
				permanentlyRemovedMu.Unlock()
				if isRemoved {
					fmt.Printf("[DISCOVERY] Skipping permanently removed node %s\n", id)
					continue
				}

				replication.RegisterNode(id, url)

				addr := strings.TrimPrefix(url, "http://")
				addrParts := strings.Split(addr, ":")
				if len(addrParts) == 2 {
					p, _ := strconv.Atoi(addrParts[1])
					api.FM.AddPeer(types.NewNode(id, addrParts[0], p))
					api.Consensus.AddPeer(addr)
				}
			}
			fmt.Printf("[DISCOVERY] Joined cluster via %s! Synced %d peers.\n", peer.ID, len(result.Peers))
			return
		}
	}
	fmt.Printf("[DISCOVERY] Could not reach any seed nodes. Proceeding as independent cluster.\n")
}

// corsMiddleware forcefully intercepts incoming HTTP requests from the React dashboard,
// injecting critical permissve CORS headers allowing cross-origin UI interaction.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// userID extracts the proprietary 'X-User-ID' header forwarded by the dashboard,
// defaulting safely to "anonymous" if no identification context exists.
func userID(r *http.Request) string {
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return uid
	}
	return "anonymous"
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func notLeaderError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	json.NewEncoder(w).Encode(map[string]string{
		"error":  "not_leader",
		"leader": api.Consensus.GetLeader(),
	})
}

func buildPeerNodes(peers []string) []*types.Node {
	var nodes []*types.Node
	for _, peerStr := range peers {
		parts := strings.Split(peerStr, "=")
		if len(parts) != 2 {
			continue
		}
		id := strings.TrimSpace(parts[0])
		if id == cfg.NodeID {
			continue // Prevent the node from treating itself as an external structural peer
		}

		addr := strings.TrimSpace(parts[1])

		addrParts := strings.Split(addr, ":")
		if len(addrParts) != 2 {
			continue
		}

		var port int
		fmt.Sscanf(addrParts[1], "%d", &port)
		nodes = append(nodes, types.NewNode(id, addrParts[0], port))
	}
	return nodes
}

// replicateHandler catches incoming Raft follower replication pushes, directly streaming
// base64 file payloads locally onto the follower's replica disk.
func replicateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var file replication.FileData
	if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	replication.SaveReplicaOnNode(file, cfg.NodeID)
	saveToDisk(file)
	w.WriteHeader(http.StatusOK)
}

func checkpointHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var snapshot replication.NodeSnapshot
	if err := json.NewDecoder(r.Body).Decode(&snapshot); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Enforce total identity: if the leader doesn't have it, we shouldn't either.
	snapshotFiles := make(map[string]bool)
	for _, file := range snapshot.Files {
		snapshotFiles[file.Name] = true
		replication.SaveReplicaOnNode(file, cfg.NodeID)
		saveToDisk(file)
	}

	diskFiles, _ := os.ReadDir(storageDir)
	for _, f := range diskFiles {
		if !f.IsDir() && !snapshotFiles[f.Name()] {
			log.Printf("[CHECKPOINT] 🗑️  Purging orphaned file %s from local disk", f.Name())
			os.Remove(filepath.Join(storageDir, f.Name()))
			replication.DeleteFile(f.Name())
		}
	}
	w.WriteHeader(http.StatusOK)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(cfg.NodeID + " OK"))
}

// saveToDisk drops a FileData object directly onto the operating system filesystem,
// seamlessly unpacking any Base64 encoding enforced by the HTTP network layer over the wire.
func saveToDisk(file replication.FileData) {
	data, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		data = []byte(file.Content)
	}
	path := filepath.Join(storageDir, file.Name)
	_ = os.WriteFile(path, data, 0644)
}

func loadAllFromDisk() []replication.FileData {
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return nil
	}
	var files []replication.FileData
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		path := filepath.Join(storageDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		files = append(files, replication.FileData{
			Name:      e.Name(),
			Content:   base64.StdEncoding.EncodeToString(raw),
			Timestamp: info.ModTime().Unix(),
		})
	}
	return files
}

func gracefulMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
}

func apiStatusHandler(w http.ResponseWriter, r *http.Request) {
	if api.FM == nil {
		jsonOK(w, map[string]interface{}{})
		return
	}
	statuses := api.FM.Detector.GetStatuses()
	records := api.FM.Recovery.GetAllRecords()

	result := map[string]interface{}{}
	result[cfg.NodeID] = map[string]interface{}{"status": "alive", "self": true}

	for nodeID, status := range statuses {
		entry := map[string]interface{}{"status": string(status)}
		if record, ok := records[nodeID]; ok {
			entry["state"] = string(record.State)
			if !record.FailedAt.IsZero() {
				entry["failed_at"] = record.FailedAt.Format("15:04:05")
			}
			if !record.RecoveredAt.IsZero() {
				entry["recovered_at"] = record.RecoveredAt.Format("15:04:05")
				entry["downtime"] = record.RecoveredAt.Sub(record.FailedAt).Round(time.Second).String()
			}
		}
		result[nodeID] = entry
	}
	jsonOK(w, result)
}

func apiFaultStatsHandler(w http.ResponseWriter, r *http.Request) {
	if api.FM == nil {
		jsonOK(w, map[string]interface{}{})
		return
	}
	statuses := api.FM.Detector.GetStatuses()
	heartbeatsSent := api.FM.Heartbeat.GetCount()
	gossipRounds := api.FM.Gossip.GetAllRumourCounts()

	nodeInfo := map[string]interface{}{}
	nodeInfo[cfg.NodeID] = map[string]interface{}{"status": "alive", "self": true}
	for id, st := range statuses {
		nodeInfo[id] = map[string]interface{}{"status": string(st)}
	}

	jsonOK(w, map[string]interface{}{
		"self_id":         cfg.NodeID,
		"nodes":           nodeInfo,
		"heartbeats_sent": heartbeatsSent,
		"gossip_rounds":   gossipRounds,
	})
}

func apiConsensusHandler(w http.ResponseWriter, r *http.Request) {
	if api.Consensus == nil {
		jsonOK(w, map[string]interface{}{"error": "consensus not initialized"})
		return
	}
	c := api.Consensus
	c.Lock()
	defer c.Unlock()
	jsonOK(w, map[string]interface{}{
		"node_id":    c.ID,
		"state":      string(c.State),
		"term":       c.CurrentTerm,
		"voted_for":  c.VotedFor,
		"log_length": len(c.Log),
		"commit_idx": c.CommitIndex,
		"peers":      c.PeerURLs,
	})
}

func apiClockHandler(w http.ResponseWriter, r *http.Request) {
	if api.ClockSyncer == nil {
		jsonOK(w, map[string]interface{}{
			"node_id":      cfg.NodeID,
			"offset_ns":    0,
			"offset_ms":    0.0,
			"synced":       false,
			"skew_history": []interface{}{},
		})
		return
	}
	history := api.ClockSyncer.SkewHistory()
	skewOut := make([]map[string]interface{}, 0, len(history))
	for _, s := range history {
		skewOut = append(skewOut, map[string]interface{}{
			"timestamp_ns": s.TimestampNs,
			"skew_ns":      s.SkewNs,
		})
	}
	jsonOK(w, map[string]interface{}{
		"node_id":      cfg.NodeID,
		"offset_ns":    api.ClockSyncer.Offset(),
		"offset_ms":    float64(api.ClockSyncer.Offset()) / 1e6,
		"synced":       true,
		"skew_history": skewOut,
	})
}

func apiJoinHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NodeID string `json:"node_id"`
		Host   string `json:"host"`
		Port   string `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	permanentlyRemovedMu.Lock()
	isRemoved := permanentlyRemoved[body.NodeID]
	permanentlyRemovedMu.Unlock()
	if isRemoved {
		fmt.Printf("[JOIN] Rejected join from permanently removed node %s\n", body.NodeID)
		http.Error(w, "node permanently removed from cluster", http.StatusForbidden)
		return
	}

	nodeURL := fmt.Sprintf("http://%s:%s", body.Host, body.Port)
	replication.RegisterNode(body.NodeID, nodeURL)

	portInt, _ := strconv.Atoi(body.Port)
	newNode := types.NewNode(body.NodeID, body.Host, portInt)

	api.Consensus.Lock()
	activeCount := 1 // self
	for _, p := range api.Consensus.PeerURLs {
		if p != "" {
			activeCount++
		}
	}
	api.Consensus.Unlock()

	if activeCount < 3 {
		if api.FM != nil {
			api.FM.AddPeer(newNode)
		}
		if api.Consensus != nil {
			api.Consensus.AddPeer(body.Host + ":" + body.Port)
		}
		fmt.Printf("[DISCOVERY] Registered active node %s at %s\n", body.NodeID, nodeURL)
	} else {
		fmt.Printf("[DISCOVERY] Registered standby node %s at %s\n", body.NodeID, nodeURL)
	}

	go broadcastJoin(newNode)

	nodes := replication.GetAllRegisteredNodes()
	permanentlyRemovedMu.Lock()
	for id := range permanentlyRemoved {
		delete(nodes, id)
	}
	permanentlyRemovedMu.Unlock()

	jsonOK(w, map[string]interface{}{
		"message": "joined",
		"leader":  api.Consensus.GetLeader(),
		"peers":   nodes,
	})
}

func apiPromoteHandler(w http.ResponseWriter, r *http.Request) {
	if api.Consensus == nil {
		http.Error(w, "consensus not init", 500)
		return
	}

	var body struct {
		Peers []string `json:"peers"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	api.Consensus.Lock()
	if !api.Consensus.IsActive {
		api.Consensus.IsActive = true
		if len(body.Peers) > 0 {
			api.Consensus.PeerURLs = body.Peers
		}
		api.Consensus.Unlock()
		api.Consensus.Start()
		fmt.Printf("[CONSENSUS] Node %s promoted to ACTIVE with peers %v\n", cfg.NodeID, body.Peers)
		w.WriteHeader(http.StatusOK)
		return
	}
	api.Consensus.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func apiSimulateKillHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	fmt.Printf("[SIMULATION] ☠️  Node %s is being KILLED (simulated failure)\n", cfg.NodeID)

	go func() {
		peers := replication.GetAllRegisteredNodes()
		for id, url := range peers {
			if id == cfg.NodeID {
				continue
			}
			payload := map[string]interface{}{
				"failed_node": cfg.NodeID,
			}
			msg, _ := types.NewMessage("GOSSIP", cfg.NodeID, payload)
			transport.Send(url+"/message", msg)
		}
	}()

	if api.Consensus != nil {
		api.Consensus.SimulateDeath()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "killed",
		"node":   cfg.NodeID,
	})
}

func apiSimulateReviveHandler(w http.ResponseWriter, r *http.Request, peerNodes []*types.Node) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	fmt.Printf("[SIMULATION] 💚 Node %s is being REVIVED as BACKUP\n", cfg.NodeID)

	if api.Consensus != nil {
		api.Consensus.SimulateReviveAsBackup()
	}

	go func() {
		time.Sleep(1 * time.Second)
		discoverAndJoin(peerNodes)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "revived",
		"node":   cfg.NodeID,
		"role":   "backup",
	})
}

func broadcastJoin(newNode *types.Node) {
	peers := replication.GetAllRegisteredNodes()
	payload := map[string]interface{}{
		"node_id": newNode.ID,
		"host":    newNode.Host,
		"port":    float64(newNode.Port),
	}
	msg, _ := types.NewMessage(types.MsgNewPeerJoined, cfg.NodeID, payload)

	for id, url := range peers {
		if id == newNode.ID || id == cfg.NodeID {
			continue
		}
		transport.Send(url+"/message", msg)
	}
}

func apiNodesHandler(w http.ResponseWriter, r *http.Request) {
	nodes := replication.GetAllRegisteredNodes()

	permanentlyRemovedMu.Lock()
	for id := range permanentlyRemoved {
		delete(nodes, id)
	}
	permanentlyRemovedMu.Unlock()

	var arr []string
	for _, url := range nodes {
		arr = append(arr, strings.TrimPrefix(url, "http://"))
	}
	jsonOK(w, arr)
}

func apiReplicationHandler(w http.ResponseWriter, r *http.Request) {
	allFiles := replication.GetAllFiles()
	result := make([]map[string]interface{}, 0, len(allFiles))
	for name, file := range allFiles {
		replicas := replication.GetReplicaNodes(name)
		result = append(result, map[string]interface{}{
			"name":      name,
			"version":   file.Version,
			"replicas":  replicas,
			"timestamp": file.Timestamp,
		})
	}
	jsonOK(w, result)
}

func apiFilesHandler(w http.ResponseWriter, r *http.Request) {
	diskFiles := loadAllFromDisk()
	type FileInfo struct {
		Name      string `json:"name"`
		Size      int    `json:"size"`
		Timestamp int64  `json:"timestamp"`
		ModTime   string `json:"mod_time"`
	}
	out := make([]FileInfo, 0, len(diskFiles))
	for _, f := range diskFiles {
		raw, _ := base64.StdEncoding.DecodeString(f.Content)
		out = append(out, FileInfo{
			Name:      f.Name,
			Size:      len(raw),
			Timestamp: f.Timestamp,
			ModTime:   time.Unix(f.Timestamp, 0).Format("2006-01-02 15:04:05"),
		})
	}
	jsonOK(w, out)
}

func apiUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if !api.Consensus.IsLeader() {
		notLeaderError(w)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	path := filepath.Join(storageDir, header.Filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		http.Error(w, "write error", http.StatusInternalServerError)
		return
	}

	fd := replication.FileData{
		Name:      header.Filename,
		Content:   base64.StdEncoding.EncodeToString(data),
		Timestamp: time.Now().Unix(),
	}
	allNodeIDs := getAllNodeIDs()
	if err := manager.ReplicateFile(fd, allNodeIDs); err != nil {
		log.Printf("[UPLOAD] replication error: %v", err)
	}

	uid := userID(r)
	addLog(uid, "upload", header.Filename, fmt.Sprintf("size=%d bytes", len(data)))

	jsonOK(w, map[string]interface{}{
		"message":   "uploaded",
		"name":      header.Filename,
		"size":      len(data),
		"timestamp": fd.Timestamp,
	})
}

func apiFileActionHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/files/")
	parts := strings.SplitN(rest, "/", 2)
	name := parts[0]
	action := ""
	if len(parts) == 2 {
		action = parts[1]
	}

	switch {
	case action == "download" && r.Method == http.MethodGet:
		apiDownloadFile(w, r, name)
	case action == "rename" && (r.Method == http.MethodPatch || r.Method == http.MethodPost):
		apiRenameFile(w, r, name)
	case action == "" && r.Method == http.MethodDelete:
		apiDeleteFile(w, r, name)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func apiDownloadFile(w http.ResponseWriter, r *http.Request, name string) {
	path := filepath.Join(storageDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(data)
	addLog(userID(r), "download", name, "")
}

func apiDeleteFile(w http.ResponseWriter, r *http.Request, name string) {
	if !api.Consensus.IsLeader() {
		notLeaderError(w)
		return
	}
	path := filepath.Join(storageDir, name)
	if err := os.Remove(path); err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	manager.DeleteFileReplicas(name)
	addLog(userID(r), "delete", name, "")
	jsonOK(w, map[string]string{"message": "deleted"})
}

func apiRenameFile(w http.ResponseWriter, r *http.Request, oldName string) {
	if !api.Consensus.IsLeader() {
		notLeaderError(w)
		return
	}
	var body struct {
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewName == "" {
		http.Error(w, "provide newName in JSON body", http.StatusBadRequest)
		return
	}
	oldPath := filepath.Join(storageDir, oldName)
	newPath := filepath.Join(storageDir, body.NewName)
	if err := os.Rename(oldPath, newPath); err != nil {
		http.Error(w, "rename failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	manager.RenameFileReplicas(oldName, body.NewName)
	addLog(userID(r), "rename", oldName, "➔ "+body.NewName)
	jsonOK(w, map[string]string{"message": "renamed", "name": body.NewName})
}

func apiLogsHandler(w http.ResponseWriter, r *http.Request) {
	userFilter := r.URL.Query().Get("user")

	logMu.Lock()
	var out []LogEntry
	for _, entry := range actLog {
		if userFilter != "" && entry.User != userFilter {
			continue
		}
		out = append(out, entry)
	}
	logMu.Unlock()
	jsonOK(w, out)
}

func getAllNodeIDs() []string {
	nodes := replication.GetAllRegisteredNodes()

	permanentlyRemovedMu.Lock()
	for id := range permanentlyRemoved {
		delete(nodes, id)
	}
	permanentlyRemovedMu.Unlock()

	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	return ids
}

func internalDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		return
	}
	name := r.URL.Query().Get("name")
	path := filepath.Join(storageDir, name)
	os.Remove(path)
	replication.DeleteFile(name)
	w.WriteHeader(http.StatusOK)
}

func internalRenameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		return
	}
	oldName := r.URL.Query().Get("old")
	newName := r.URL.Query().Get("new")
	oldPath := filepath.Join(storageDir, oldName)
	newPath := filepath.Join(storageDir, newName)
	os.Rename(oldPath, newPath)
	replication.RenameFile(oldName, newName)
	w.WriteHeader(http.StatusOK)
}
