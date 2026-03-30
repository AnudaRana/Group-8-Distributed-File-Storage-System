package clock

import (
	"encoding/json"
	"net/http"
	"time"
)

// TimeHandler answers incoming NTP-style sync requests by immediately 
// capturing and returning the native server's current nanosecond timestamp.
func TimeHandler(w http.ResponseWriter, r *http.Request) {
	response := TimeResponse{
		ServerTime: time.Now().UnixNano(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
	}
}
