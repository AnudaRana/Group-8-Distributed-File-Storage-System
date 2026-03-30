package config

import (
	"os"
	"strings"
)

type Config struct {
	NodeID string
	Host   string
	Port   string
	Peers  []string
}

// LoadConfig pulls operative properties from the underlying OS environment, supplying static 
// default fallbacks to ensure independent nodes can spin up consistently without explicit CLI flags.
func LoadConfig() *Config {
    return &Config{
        NodeID: getEnv("NODE_ID", "node1"),
        Host:   getEnv("HOST", "127.0.0.1"),
        Port:   getEnv("PORT", "9001"),        // â† change from 12345 to 9001
        Peers:  ParsePeers(getEnv("PEERS", "")),
    }
}
// ParsePeers accepts a raw comma-separated list of peer connections natively provided from 
// startup arguments and parses them rigidly into a string slice representing explicit cluster peers.
func ParsePeers(peersStr string) []string {
	if peersStr == "" {
		return []string{}
	}
	parts := strings.Split(peersStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// getEnv checks the operating system for a dynamically injected environment variable, returning
// a structured fallback if the target key is totally unbound.
func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
