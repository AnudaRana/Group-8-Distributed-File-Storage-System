package main

import (
	"dfs-system/internal/transport"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

func main() {
	msg, _ := types.NewMessage(
		types.MsgHeartbeat,
		"client",
		map[string]interface{}{"status": "ping"},
	)
	utils.Log("CLIENT", "Sending heartbeat to 127.0.0.1:12345...")
	err := transport.Send("http://127.0.0.1:12345/message", msg)
	if err != nil {
		utils.Log("CLIENT", "Error connecting to node: %v", err)
		return
	}
	utils.Log("CLIENT", "Message successfully sent!")
}