#!/bin/bash

# Start 3 nodes in background using different ports and NODE_IDs
echo "Starting Node 1 (Port 12345)..."
export NODE_ID=node1
export PORT=12345
export PEERS=127.0.0.1:12346,127.0.0.1:12347
go run cmd/node/main.go &

echo "Starting Node 2 (Port 12346)..."
export NODE_ID=node2
export PORT=12346
export PEERS=127.0.0.1:12345,127.0.0.1:12347
go run cmd/node/main.go &

echo "Starting Node 3 (Port 12347)..."
export NODE_ID=node3
export PORT=12347
export PEERS=127.0.0.1:12345,127.0.0.1:12346
go run cmd/node/main.go &

echo "3-node cluster started!"

