package main

import (
	"dfs-system/internal/transport"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

func main() {
	nodes := []string{
		"127.0.0.1:9001",
		"127.0.0.1:9002",
		"127.0.0.1:9003",
	}

	for _, node := range nodes {
		msg, _ := types.NewMessage(
			types.MsgHeartbeat,
			"client",
			map[string]interface{}{"status": "ping"},
		)

		utils.Log("CLIENT", "Sending heartbeat to %s...", node)
		err := transport.Send("http://"+node+"/message", msg)
		if err != nil {
			utils.Log("CLIENT", "Error connecting to %s: %v", node, err)
		} else {
			utils.Log("CLIENT", "✅ Message successfully sent to %s!", node)
		}
	}
}