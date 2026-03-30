package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var projectRoot string

func TestMain(m *testing.M) {
	pwd, _ := os.Getwd()
	projectRoot = filepath.Join(pwd, "..", "..")

	binaryPath := filepath.Join(projectRoot, "test_node.exe")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "cmd/node/main.go")
	buildCmd.Dir = projectRoot
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("Failed to build test binary: %v\n", err)
		os.Exit(1)
	}

	exitCode := m.Run()

	os.Remove(binaryPath)
	os.RemoveAll(filepath.Join(projectRoot, "data", "node1"))
	os.RemoveAll(filepath.Join(projectRoot, "data", "node2"))
	os.RemoveAll(filepath.Join(projectRoot, "data", "node3"))

	os.Exit(exitCode)
}

func spawnNode(t *testing.T, id string, port string, peers string) *exec.Cmd {
	binaryPath := filepath.Join(projectRoot, "test_node.exe")
	cmd := exec.Command(binaryPath)
	cmd.Dir = projectRoot
	
	var filteredEnv []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "NODE_ID=") && !strings.HasPrefix(e, "PORT=") && !strings.HasPrefix(e, "HOST=") && !strings.HasPrefix(e, "PEERS=") {
			filteredEnv = append(filteredEnv, e)
		}
	}
	cmd.Env = append(filteredEnv,
		fmt.Sprintf("NODE_ID=%s", id),
		fmt.Sprintf("PORT=%s", port),
		"HOST=127.0.0.1",
		fmt.Sprintf("PEERS=%s", peers),
	)
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start node %s on port %s: %v", id, port, err)
	}

	time.Sleep(200 * time.Millisecond)

	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	return cmd
}

func pollForLeader(ports []string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, port := range ports {
			resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/api/consensus", port))
			if err != nil {
				continue
			}
			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				if state, ok := result["state"].(string); ok && state == "Leader" {
					resp.Body.Close()
					return port, nil
				}
			}
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", fmt.Errorf("leader election timed out after %v", timeout)
}

func TestClusterFormationAndElection(t *testing.T) {
	os.RemoveAll(filepath.Join(projectRoot, "data", "node1"))
	os.RemoveAll(filepath.Join(projectRoot, "data", "node2"))
	os.RemoveAll(filepath.Join(projectRoot, "data", "node3"))
	
	peers := "node1=127.0.0.1:9011,node2=127.0.0.1:9012,node3=127.0.0.1:9013"
	
	spawnNode(t, "node1", "9011", peers)
	spawnNode(t, "node2", "9012", peers)
	spawnNode(t, "node3", "9013", peers)
	
	leaderPort, err := pollForLeader([]string{"9011", "9012", "9013"}, 15*time.Second)
	if err != nil {
		t.Fatalf("Cluster failed to elect a Leader: %v", err)
	}
	t.Logf("Raft consensus achieved! Master is running on port: %s", leaderPort)
	
	time.Sleep(2 * time.Second)
	
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/api/status", leaderPort))
	if err != nil {
		t.Fatalf("Cluster API dead: %v", err)
	}
	defer resp.Body.Close()
	
	var status map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&status)
	
	if len(status) < 3 {
		t.Fatalf("Expected 3 registered nodes in /api/status, found %d", len(status))
	}
}

func TestFileUploadPropagation(t *testing.T) {
	os.RemoveAll(filepath.Join(projectRoot, "data", "node1"))
	os.RemoveAll(filepath.Join(projectRoot, "data", "node2"))
	os.RemoveAll(filepath.Join(projectRoot, "data", "node3"))
	
	peers := "node1=127.0.0.1:9021,node2=127.0.0.1:9022,node3=127.0.0.1:9023"
	
	spawnNode(t, "node1", "9021", peers)
	spawnNode(t, "node2", "9022", peers)
	spawnNode(t, "node3", "9023", peers)
	
	leaderPort, err := pollForLeader([]string{"9021", "9022", "9023"}, 15*time.Second)
	if err != nil {
		t.Fatalf("Cluster failed to elect a Leader: %v", err)
	}
	
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "integration_test.txt")
	io.Copy(part, strings.NewReader("hello distributed world!"))
	writer.Close()
	
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%s/api/files/upload", leaderPort), body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to successfully POST file to Leader cluster API")
	}
	resp.Body.Close()
	
	time.Sleep(3 * time.Second)
	
	filesResp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/api/files", leaderPort))
	if err != nil {
		t.Fatalf("Cluster API dead on read: %v", err)
	}
	defer filesResp.Body.Close()
	
	var filesList []map[string]interface{}
	json.NewDecoder(filesResp.Body).Decode(&filesList)
	
	found := false
	for _, f := range filesList {
		if fmt.Sprint(f["name"]) == "integration_test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Uploaded file utterly failed to persist into cluster filesystem list")
	}
	
	successCount := 0
	for _, id := range []string{"node1", "node2", "node3"} {
		path := filepath.Join(projectRoot, "data", id, "files", "integration_test.txt")
		if _, err := os.Stat(path); err == nil {
			successCount++
		}
	}
	
	if successCount < 3 {
		t.Fatalf("Data persistence error: expected physical disk copies on 3 nodes, found %d", successCount)
	}
}