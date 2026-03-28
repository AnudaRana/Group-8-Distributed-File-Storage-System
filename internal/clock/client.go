package clock

import (
    "encoding/json"
    "net/http"
    "time"
)

func RequestServerTime(url string) (int64, int64, int64, error) {
    T1 := time.Now().UnixNano()

    resp, err := http.Get(url)
    if err != nil {
        return 0, 0, 0, err
    }

    var data TimeResponse
    json.NewDecoder(resp.Body).Decode(&data)

    T3 := time.Now().UnixNano()

    return T1, data.ServerTime, T3, nil
}