package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"dfs-system/internal/api"
	"dfs-system/internal/clock"
	"dfs-system/internal/config"
	"dfs-system/internal/fault"
	"dfs-system/internal/types"
)

func main() {
	cfg := config.LoadConfig()

	// Build peer node objects from config
	peerNodes := buildPeerNodes(cfg.Peers)

	// Start fault manager (heartbeat + detection)
	fm := fault.NewFaultManager(cfg.NodeID, cfg.Peers, peerNodes)
	fm.Start()

	// Give handler access to fault manager
	api.FM = fm

	// Initialize clock syncer if peers are configured
	if len(cfg.Peers) > 0 && cfg.Peers[0] != "" {
		peerURL := fmt.Sprintf("http://%s", cfg.Peers[0])
		api.ClockSyncer = clock.NewSyncer(peerURL)

		// Perform initial sync
		if _, err := api.ClockSyncer.Synchronise(); err != nil {
			log.Printf("[NODE %s] Initial clock sync failed: %v", cfg.NodeID, err)
		} else {
			log.Printf("[NODE %s] Clock synchronized with %s", cfg.NodeID, peerURL)
		}

		// Start background sync loop
		stopCh := make(chan struct{})
		defer close(stopCh)
		api.ClockSyncer.RunLoop(30*time.Second, stopCh)
	}

	// Start HTTP server
	http.HandleFunc("/message", api.MessageHandler)
	http.HandleFunc("/time", clock.TimeHandler)

	addr := cfg.Host + ":" + cfg.Port
	fmt.Printf("[NODE %s] Starting on %s\n", cfg.NodeID, addr)
	
	log.Printf("⚙️  [ARCH_CONFIG] Time Synchronization Initialized | Trade-off selected: Periodic polling (30s interval) to minimize network overhead while proactively tracking offset drift.")

	// log.Fatal keeps the program alive — it only exits if the server crashes
	log.Fatal(http.ListenAndServe(addr, nil))
}

func buildPeerNodes(peers []string) []*types.Node {
	var nodes []*types.Node
	for i, peer := range peers {
		parts := strings.Split(peer, ":")
		if len(parts) != 2 {
			continue
		}
		host := parts[0]
		port := 8080
		fmt.Sscanf(parts[1], "%d", &port)

		node := types.NewNode(
			fmt.Sprintf("node%d", i+2),
			host,
			port,
		)
		nodes = append(nodes, node)
	}
	return nodes
}