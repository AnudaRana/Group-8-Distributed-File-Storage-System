@echo off
:: run_node.bat - Runs a single node (no supervisor loop).
:: Killing this terminal window = node is truly dead.
:: The Raft leader will spawn a standby replacement automatically.
::
:: Usage: run_node.bat <node_id> <port> <host> <peers>
:: Example: run_node.bat node1 8001 127.0.0.1 "node2=127.0.0.1:8002,node3=127.0.0.1:8003"

set NODE_ID=%1
set PORT=%2
set HOST=%3
set PEERS=%~4

title %NODE_ID% [%HOST%:%PORT%]

echo ==================================================
echo  Starting %NODE_ID% on %HOST%:%PORT%
echo  Peers: %PEERS%
echo ==================================================

node.exe -id %NODE_ID% -port %PORT% -host %HOST% -peers "%PEERS%"

echo.
echo [%NODE_ID%] Process exited. This terminal will close.
timeout /t 3 /nobreak >nul
