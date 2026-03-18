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

func LoadConfig() *Config {
	return &Config{
		NodeID: getEnv("NODE_ID", "node1"),
		Host:   getEnv("HOST", "127.0.0.1"),
		Port:   getEnv("PORT", "12345"),
		Peers:  parsePeers(getEnv("PEERS", "")),
	}
}

func parsePeers(peersStr string) []string {
	if peersStr == "" {
		return []string{}
	}
	parts := strings.Split(peersStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}