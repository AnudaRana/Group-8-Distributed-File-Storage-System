package utils

import (
	"fmt"
	"time"
)

// Log dynamically injects a node-level console standard output event containing
// strict timeline milliseconds and node origin indicators to prevent tracing collisions.
func Log(nodeID, format string, a ...interface{}) {
	timestamp := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("[%s] [%s] %s\n", timestamp, nodeID, msg)
}
