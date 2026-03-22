package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"dfs-system/internal/api"
	"dfs-system/internal/config"
	"dfs-system/internal/fault"
	"dfs-system/internal/types"
)

func main() {
	cfg := config.LoadConfig()

	peerNodes := buildPeerNodes(cfg.Peers)

	fm := fault.NewFaultManager(cfg.NodeID, cfg.Peers, peerNodes)
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