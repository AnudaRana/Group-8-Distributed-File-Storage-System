package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)


var httpClient = &http.Client{Timeout: 5 * time.Second}

func Send(url string, data []byte) error {
	_, err := httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	return err
}

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