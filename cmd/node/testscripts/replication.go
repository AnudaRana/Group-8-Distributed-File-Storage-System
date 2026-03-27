package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	fmt.Println("=== Replication Test ===")

	nodes := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	for _, node := range nodes {
		resp, err := http.Get(node + "/read?name=notes.txt")
		if err != nil {
			fmt.Println("Error connecting to", node, ":", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		fmt.Println(node, "->", string(body))
	}
}
