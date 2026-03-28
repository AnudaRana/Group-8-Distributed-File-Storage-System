package fault

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dfs-system/internal/types"
)

// Test 1: Heartbeat sends correct message type
func TestHeartbeatSendsCorrectMessageType(t *testing.T) {
	received := make(chan string, 1)

	// Spin up a fake peer server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		var msg types.Message
		json.Unmarshal(body, &msg)
		received <- msg.Type
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Strip the "http://" so HeartbeatSender gets just host:port
	addr := server.URL[7:]

	sender := NewHeartbeatSender("node1", []string{addr}, 200*time.Millisecond)
	sender.Start()

	select {
	case msgType := <-received:
		if msgType != types.MsgHeartbeat {
			t.Errorf("[TEST] Expected message type HB, but got: %s", msgType)
		}
	case <-time.After(1 * time.Second):
		t.Error("[TEST] Timeout: no heartbeat received by peer")
	}
}

// Test 2: Heartbeat sends correct sender ID
func TestHeartbeatSendsCorrectSenderID(t *testing.T) {
	received := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		var msg types.Message
		json.Unmarshal(body, &msg)
		received <- msg.Sender
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := server.URL[7:]
	sender := NewHeartbeatSender("node1", []string{addr}, 200*time.Millisecond)
	sender.Start()

	select {
	case senderID := <-received:
		if senderID != "node1" {
			t.Errorf("[TEST] Expected sender 'node1', but got: '%s'", senderID)
		}
	case <-time.After(1 * time.Second):
		t.Error("[TEST] Timeout: no heartbeat received")
	}
}

// Test 3: Heartbeat doesn't crash when peer is unreachable
func TestHeartbeatHandlesUnreachablePeer(t *testing.T) {
	// Point to a port nothing is listening on
	sender := NewHeartbeatSender("node1", []string{"127.0.0.1:19999"}, 200*time.Millisecond)
	sender.Start()

	// If it panics or crashes, this test will fail — otherwise pass
	time.Sleep(600 * time.Millisecond)
}

// Test 4: Multiple heartbeats are sent over time
func TestMultipleHeartbeatsSent(t *testing.T) {
	count := 0
	done := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count >= 3 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := server.URL[7:]
	sender := NewHeartbeatSender("node1", []string{addr}, 100*time.Millisecond)
	sender.Start()

	select {
	case <-done:
		// Got 3+ heartbeats — pass
	case <-time.After(2 * time.Second):
		t.Errorf("[TEST] Expected 3 or more heartbeats, but only got: %d", count)
	}
}