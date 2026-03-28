# -----------------------------
# Distributed File Storage System
# Makefile for CLI testing
# -----------------------------

# Go commands
GO=go
NODE=cmd/node/main.go
CLIENT=cmd/client/main.go

# Run a single node
run-node:
	$(GO) run $(NODE)

# Run client
run-client:
	$(GO) run $(CLIENT)

# Run 3-node cluster locally
run-cluster:
# Node 1
	cmd /C "start /B cmd /C \"set NODE_ID=node1 && set PORT=8000 && set PEERS=localhost:8001,localhost:8002 && go run $(NODE)\""
# Node 2
	cmd /C "start /B cmd /C \"set NODE_ID=node2 && set PORT=8001 && set PEERS=localhost:8000,localhost:8002 && go run $(NODE)\""
# Node 3
	cmd /C "start /B cmd /C \"set NODE_ID=node3 && set PORT=8002 && set PEERS=localhost:8000,localhost:8001 && go run $(NODE)\""
	@echo "3-node cluster started (Windows)"

# Clean binaries (if any)
clean:
	del /Q *.exe


build:
	go build -buildvcs=false ./...

test:
	go test -buildvcs=false ./internal/fault/... -v

run-node:
	go run -buildvcs=false cmd/node/main.go
