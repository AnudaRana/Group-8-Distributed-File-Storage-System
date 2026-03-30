package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)


var httpClient = &http.Client{Timeout: 5 * time.Second}

// Send drops a raw byte array payload aggressively into a target cross-cluster node via an HTTP POST request.
func Send(url string, data []byte) error {
	_, err := httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return err
}

// GetJSON performs an explicit blocking HTTP GET fetch, seamlessly unpacking the returned
// raw JSON payload packet structurally into a local memory-mapped object interface.
func GetJSON(url string, dst interface{}) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode response from %s: %w", url, err)
	}
	return nil
}
