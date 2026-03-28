package api

import (
	"io/ioutil"
	"net/http"

	"dfs-system/internal/clock"
	"dfs-system/internal/config"
	"dfs-system/internal/fault"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

var cfg = config.LoadConfig()


var FM *fault.FaultManager


var ClockSyncer *clock.Syncer

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

	case types.MsgSyncClock:
		if ClockSyncer != nil {
			utils.Log(cfg.NodeID, "🕐 Clock sync request from %s — offset: %dns",
				msg.Sender, ClockSyncer.Offset())
		}

	// case types.MsgReplicate: → Member 2
	// case types.MsgVoteReq:   → Member 4
	// case types.MsgVoteReply: → Member 4
	// case types.MsgLeaderHB:  → Member 4

	default:
		utils.Log(cfg.NodeID, "Unknown message type: %s", msg.Type)
	}

	w.WriteHeader(http.StatusOK)
}