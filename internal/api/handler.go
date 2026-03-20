package api

import (
	"io/ioutil"
	"net/http"

	"dfs-system/internal/config"
	"dfs-system/internal/fault"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

var cfg = config.LoadConfig()

// FaultManager is set from main.go after startup
var FM *fault.FaultManager

func MessageHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	msg, err := types.ParseMessage(body)
	if err != nil {
		http.Error(w, "Invalid message", 400)
		return
	}

	utils.Log(cfg.NodeID, "Received [%s] from %s", msg.Type, msg.Sender)

	switch msg.Type {

	case types.MsgHeartbeat:
		if FM != nil {
			FM.Detector.RecordHeartbeat(msg.Sender)
		}
		utils.Log(cfg.NodeID, "💓 Heartbeat recorded from %s", msg.Sender)

	// Other members will add their cases here:
	// case types.MsgReplicate:  → Member 2
	// case types.MsgSyncClock:  → Member 3
	// case types.MsgVoteReq:    → Member 4

	default:
		utils.Log(cfg.NodeID, "Unknown message type: %s", msg.Type)
	}

	w.WriteHeader(http.StatusOK)
}