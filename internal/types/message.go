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

	MsgNewPeerJoined = "PEER_JOINED"
	MsgPromote       = "PROMOTE"

	MsgAck = "ACK"
)

type Message struct {
	Type    string                 `json:"type"`
	Sender  string                 `json:"sender"`
	Payload map[string]interface{} `json:"payload"`
	SentAt  float64                `json:"sent_at"`
}

// NewMessage constructs, maps, and serializes a high-level struct blueprint down into a raw byte buffer,
// actively branding the payload with its node generation timestamp.
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

// ParseMessage reverses a network-received byte stream back into an actively mapped generic JSON payload.
func ParseMessage(data []byte) (*Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return &m, err
}
