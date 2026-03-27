package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	fmt.Println("=== Consistency Test ===")

	file := map[string]any{
		"name":    "report.txt",
		"content": "Version 2 content",
	}

	jsonData, _ := json.Marshal(file)

	resp, err := http.Post("http://localhost:8001/write", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Write error:", err)
		return
	}
	resp.Body.Close()

	nodes := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	for _, node := range nodes {
		readResp, err := http.Get(node + "/read?name=report.txt")
		if err != nil {
			fmt.Println("Read error from", node, ":", err)
			continue
		}

		body, _ := io.ReadAll(readResp.Body)
		readResp.Body.Close()

		fmt.Println(node, "->", string(body))
	}
}
