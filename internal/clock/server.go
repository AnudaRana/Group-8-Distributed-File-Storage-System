package clock

import (
	"encoding/json"
	"net/http"
	"time"
)

// TimeHandler is the HTTP handler that responds with the server's current
// Unix nanosecond timestamp.
func TimeHandler(w http.ResponseWriter, r *http.Request) {
	response := TimeResponse{
		ServerTime: time.Now().UnixNano(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
	}
}