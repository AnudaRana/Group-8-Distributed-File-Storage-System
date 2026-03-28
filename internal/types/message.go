package types

import (
	"encoding/json"
	"time"
)

const (
	MsgHeartbeat = "HB"
	MsgReplicate = "REPLICATE"
	MsgSyncClock = "SYNC"

	MsgVoteReq   = "VOTE_REQ"
	MsgVoteReply = "VOTE_REPLY"
	MsgLeaderHB  = "LEADER_HB"

	MsgAck = "ACK"
)

type Message struct {
	Type    string                 `json:"type"`
	Sender  string                 `json:"sender"`
	Payload map[string]interface{} `json:"payload"`
	SentAt  float64                `json:"sent_at"`
}

func NewMessage(msgType, senderID string, payload map[string]interface{}) ([]byte, error) {
	if payload == nil {
		payload = make(map[string]interface{})
	}
	msg := Message{
		Type:    msgType,
		Sender:  senderID,
		Payload: payload,
		SentAt:  float64(time.Now().UnixNano()) / 1e9,
	}
	return json.Marshal(msg)
}

func ParseMessage(data []byte) (*Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return &m, err
}