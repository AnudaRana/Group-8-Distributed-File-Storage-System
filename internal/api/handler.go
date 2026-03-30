package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"dfs-system/internal/clock"
	"dfs-system/internal/config"
	"dfs-system/internal/consensus"
	"dfs-system/internal/fault"
	"dfs-system/internal/replication"
	"dfs-system/internal/types"
	"dfs-system/internal/utils"
)

var cfg = config.LoadConfig()

var FM *fault.FaultManager
var Consensus *consensus.Raft
var ClockSyncer *clock.Syncer

// MessageHandler intercepts incoming peer-to-peer network messages and routes them to the correct internal subsystem.
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
		utils.Log(cfg.NodeID, "💗 Heartbeat recorded from %s", msg.Sender)

	case types.MsgSyncClock:
		if ClockSyncer != nil {
			utils.Log(cfg.NodeID, "🕐 Clock sync request from %s — offset: %dns",
				msg.Sender, ClockSyncer.Offset())
		}

	case "GOSSIP":
		if FM != nil {
			if failedNode, ok := msg.Payload["failed_node"].(string); ok && failedNode != "" {
				utils.Log(cfg.NodeID, "📢 Received GOSSIP from %s — node %s is OFFLINE", msg.Sender, failedNode)
				FM.Gossip.ReceiveGossip(msg.Sender, failedNode, FM.Detector)
			}
		}

	case types.MsgVoteReq:
		if Consensus != nil {
			Consensus.HandleVoteRequest(msg)
		}
	case types.MsgVoteReply:
		if Consensus != nil {
			Consensus.HandleVoteReply(msg)
		}
	case types.MsgLeaderHB:
		if Consensus != nil {
			Consensus.HandleLeaderHeartbeat(msg)
		}
	case "APPEND_REPLY":
		if Consensus != nil {
			Consensus.HandleAppendReply(msg)
		}

	case types.MsgNewPeerJoined:
		nodeID, ok1 := msg.Payload["node_id"].(string)
		host, ok2 := msg.Payload["host"].(string)
		portFloat, ok3 := msg.Payload["port"].(float64)
		if !ok1 || !ok2 || !ok3 {
			utils.Log(cfg.NodeID, "⚠️  Malformed PEER_JOINED payload — skipping")
			break
		}
		port := int(portFloat)

		newNode := types.NewNode(nodeID, host, port)
		nodeURL := fmt.Sprintf("http://%s:%d", host, port)
		replication.RegisterNode(nodeID, nodeURL)

		if Consensus != nil {
			Consensus.Lock()
			activeCount := 1
			for _, p := range Consensus.PeerURLs {
				if p != "" {
					activeCount++
				}
			}
			Consensus.Unlock()

			if activeCount < 3 {
				if FM != nil {
					FM.AddPeer(newNode)
				}
				Consensus.AddPeer(fmt.Sprintf("%s:%s", host, strconv.Itoa(port)))
				utils.Log(cfg.NodeID, "📢 Added new ACTIVE peer %s (%s:%d) from broadcast", nodeID, host, port)
			} else {
				utils.Log(cfg.NodeID, "📢 Registered node %s (%s:%d) as STANDBY from broadcast", nodeID, host, port)
			}
		}

	default:
		utils.Log(cfg.NodeID, "Unknown message type: %s", msg.Type)
	}

	w.WriteHeader(http.StatusOK)
}

// StatusHandler provides a JSON snapshot of the local fault manager and historical node up/downtimes.
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
