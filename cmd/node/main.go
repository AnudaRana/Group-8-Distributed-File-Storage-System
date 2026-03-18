package main

import (
	"fmt"
	"log"
	"net/http"

	"dfs-system/internal/api"
	"dfs-system/internal/config"
	"dfs-system/internal/utils"
)

func main() {
	cfg := config.LoadConfig()
	http.HandleFunc("/message", api.MessageHandler)
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	utils.Log(cfg.NodeID, "Node successfully started and listening on %s:%s", cfg.Host, cfg.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}