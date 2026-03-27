#!/bin/bash

export NODE_ID=${NODE_ID:-node1}
export HOST=${HOST:-127.0.0.1}
export PORT=${PORT:-9001}
export PEERS=${PEERS:-""}

echo "Starting Node $NODE_ID on $HOST:$PORT..."
go run cmd/node/main.go