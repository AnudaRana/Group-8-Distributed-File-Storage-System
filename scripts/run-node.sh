#!/bin/bash

# Start a single node with default settings
export NODE_ID=${NODE_ID:-node1}
export PORT=${PORT:-12345}
export PEERS=${PEERS:-""}

echo "Starting Node $NODE_ID on port $PORT..."
go run cmd/node/main.go
