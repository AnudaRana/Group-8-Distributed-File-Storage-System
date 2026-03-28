package utils

import (
	"fmt"
	"time"
)

// Log prints a formatted message with a timestamp and node ID.
func Log(nodeID, format string, a ...interface{}) {
	timestamp := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("[%s] [%s] %s\n", timestamp, nodeID, msg)
}