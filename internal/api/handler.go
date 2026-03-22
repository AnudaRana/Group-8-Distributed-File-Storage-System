package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"dfs-system/internal/config"
	"dfs-system/internal/fault"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

var cfg = config.LoadConfig()
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

	case "GOSSIP":
		if FM != nil {
			failedNode, ok := msg.Payload["failed_node"].(string)
			if ok && failedNode != "" {
				FM.Gossip.ReceiveGossip(msg.Sender, failedNode, FM.Detector)
			}
		}

	// Other members add their cases below — do not remove existing cases
	// case types.MsgReplicate:  → Member 2
	// case types.MsgSyncClock:  → Member 3
	// case types.MsgVoteReq:    → Member 4

	default:
		utils.Log(cfg.NodeID, "Unknown message type: %s", msg.Type)
	}

	w.WriteHeader(http.StatusOK)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	if FM == nil {
		http.Error(w, "FaultManager not initialized", 500)
		return
	}

	statuses := FM.Detector.GetStatuses()
	records := FM.Recovery.GetAllRecords()

	result := map[string]interface{}{}

	for nodeID, status := range statuses {
		entry := map[string]interface{}{
			"status": status,
		}
		if record, ok := records[nodeID]; ok {
			entry["state"] = record.State
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}