package transport

import "fmt"

func PeerURL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d", host, port)
}