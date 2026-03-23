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