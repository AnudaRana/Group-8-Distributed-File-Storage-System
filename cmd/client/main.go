package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	file := map[string]any{
		"name":    "notes.txt",
		"content": "Hello from distributed file storage client",
	}

	jsonData, err := json.Marshal(file)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Post("http://localhost:8001/write", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Write Response:", string(body))

	readResp, err := http.Get("http://localhost:8001/read?name=notes.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer readResp.Body.Close()

	readBody, _ := io.ReadAll(readResp.Body)
	fmt.Println("Read Response:", string(readBody))
}
