#!/bin/bash

# Simplifies starting a specific node
# Usage: ./scripts/node.sh 1
# Usage: ./scripts/node.sh 2
# Usage: ./scripts/node.sh 3

NODE_NUM=$1

case $NODE_NUM in
  1)
    export NODE_ID=node1
    export HOST=127.0.0.1
    export PORT=9001
    export PEERS=127.0.0.1:9002,127.0.0.1:9003
    ;;
  2)
    export NODE_ID=node2
    export HOST=127.0.0.1
    export PORT=9002
    export PEERS=127.0.0.1:9001,127.0.0.1:9003
    ;;
  3)
    export NODE_ID=node3
    export HOST=127.0.0.1
    export PORT=9003
    export PEERS=127.0.0.1:9001,127.0.0.1:9002
    ;;
  *)
    echo "Usage: ./scripts/node.sh [1|2|3]"
    exit 1
    ;;
esac

echo "Starting Node $NODE_ID on port $PORT..."
go run -buildvcs=false cmd/node/main.go
