package clock

import (
    "encoding/json"
    "net/http"
    "time"
)

func TimeHandler(w http.ResponseWriter, r *http.Request) {
    response := TimeResponse{
        ServerTime: time.Now().UnixNano(),
    }
    json.NewEncoder(w).Encode(response)
}