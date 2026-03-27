# -----------------------------
# Distributed File Storage System
# Makefile for CLI testing
# -----------------------------

# Go commands
GO=go
NODE=cmd/node/main.go
CLIENT=cmd/client/main.go

# Build the node binary
build-node:
	$(GO) build -o node.exe -buildvcs=false $(NODE)

# Run a single node (uses built binary)
run-node: build-node
	./node.exe

# Run client
run-client:
	$(GO) run $(CLIENT)

# Run 3-node cluster locally (Windows)
run-cluster: build-node
# Node 1
	start /B cmd /C "set NODE_ID=node1&& set PORT=8000&& set PEERS=localhost:8001,localhost:8002&& node.exe"
# Node 2
	start /B cmd /C "set NODE_ID=node2&& set PORT=8001&& set PEERS=localhost:8000,localhost:8002&& node.exe"
# Node 3
	start /B cmd /C "set NODE_ID=node3&& set PORT=8002&& set PEERS=localhost:8000,localhost:8001&& node.exe"
	@echo "3-node cluster started with node.exe (Windows)"

# Kill all dangling node processes
kill-all:
	@taskkill /F /IM node.exe /T 2>nul || exit 0
	@echo "All node processes terminated."

# Clean binaries (if any)
clean:
	del /Q *.exe

build:
	go build -buildvcs=false ./...

test:
	go test -buildvcs=false ./internal/fault/... -v
