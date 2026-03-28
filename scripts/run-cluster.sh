#!/bin/bash

echo "Starting Node 1 (Port 9001)..."
NODE_ID=node1 HOST=127.0.0.1 PORT=9001 PEERS=127.0.0.1:9002,127.0.0.1:9003 go run cmd/node/main.go &
PID1=$!

sleep 1

echo "Starting Node 2 (Port 9002)..."
NODE_ID=node2 HOST=127.0.0.1 PORT=9002 PEERS=127.0.0.1:9001,127.0.0.1:9003 go run cmd/node/main.go &
PID2=$!

sleep 1

echo "Starting Node 3 (Port 9003)..."
NODE_ID=node3 HOST=127.0.0.1 PORT=9003 PEERS=127.0.0.1:9001,127.0.0.1:9002 go run cmd/node/main.go &
PID3=$!

echo ""
echo "✅ 3-node cluster started!"
echo "PIDs: $PID1 $PID2 $PID3"
echo ""
echo "To test fault detection:"
echo "  Kill node2 with: kill $PID2"
echo "  Watch node1 and node3 log that node2 is OFFLINE after 6 seconds"
echo ""
echo "Press Ctrl+C to stop all nodes"

trap "kill $PID1 $PID2 $PID3 2>/dev/null" EXIT
wait