@echo off
echo ============================================================
echo  Distributed File Storage System - Startup
echo ============================================================

:: Change to project root
cd /d "%~dp0.."

:: Build the binary first (fast startup, correct args passing)
echo [BUILD] Compiling node binary...
go build -o node.exe ./cmd/node
if errorlevel 1 (
    echo [ERROR] Build failed! Fix compilation errors first.
    pause
    exit /b 1
)
echo [BUILD] node.exe ready.
echo.

:: Start 3 active nodes directly (no supervisor loop - system handles standby replacement)
echo [START] Launching Node 1 (port 8001)...
start "Node 1 [node1:8001]" cmd /k "node.exe -id node1 -port 8001 -host 127.0.0.1 -peers node2=127.0.0.1:8002,node3=127.0.0.1:8003"

echo [START] Launching Node 2 (port 8002)...
start "Node 2 [node2:8002]" cmd /k "node.exe -id node2 -port 8002 -host 127.0.0.1 -peers node1=127.0.0.1:8001,node3=127.0.0.1:8003"

echo [START] Launching Node 3 (port 8003)...
start "Node 3 [node3:8003]" cmd /k "node.exe -id node3 -port 8003 -host 127.0.0.1 -peers node1=127.0.0.1:8001,node2=127.0.0.1:8002"

echo.
echo [INFO] 3 active nodes started. node4 and node5 are standby (spawned automatically on failure).
echo.

:: Start React frontend
echo [START] Launching React Frontend...
cd Frontend
start "React Frontend" cmd /k "npm run dev"
cd ..

echo.
echo [INFO] Waiting 5 seconds for nodes to elect a leader...
timeout /t 5 /nobreak >nul
start http://localhost:5173
echo [DONE] System is running!
