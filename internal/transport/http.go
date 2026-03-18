package transport

import (
	"bytes"
	"net/http"
)

func Send(url string, data []byte) error {
	_, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	return err
}