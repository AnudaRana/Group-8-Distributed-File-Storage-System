# PowerShell script to run a 3-node cluster on Windows

$ErrorActionPreference = "Continue"

Write-Host "Starting Node 1 (Port 9001)..." -ForegroundColor Green
$env:NODE_ID = "node1"
$env:HOST = "127.0.0.1"
$env:PORT = "9001"
$env:PEERS = "127.0.0.1:9002,127.0.0.1:9003"
$proc1 = Start-Process -FilePath "go" -ArgumentList "run", "cmd/node/main.go" -NoNewWindow -PassThru

Start-Sleep -Seconds 1

Write-Host "Starting Node 2 (Port 9002)..." -ForegroundColor Green
$env:NODE_ID = "node2"
$env:HOST = "127.0.0.1"
$env:PORT = "9002"
$env:PEERS = "127.0.0.1:9001,127.0.0.1:9003"
$proc2 = Start-Process -FilePath "go" -ArgumentList "run", "cmd/node/main.go" -NoNewWindow -PassThru

Start-Sleep -Seconds 1

Write-Host "Starting Node 3 (Port 9003)..." -ForegroundColor Green
$env:NODE_ID = "node3"
$env:HOST = "127.0.0.1"
$env:PORT = "9003"
$env:PEERS = "127.0.0.1:9001,127.0.0.1:9002"
$proc3 = Start-Process -FilePath "go" -ArgumentList "run", "cmd/node/main.go" -NoNewWindow -PassThru

Write-Host ""
Write-Host "3-node cluster started!" -ForegroundColor Cyan
Write-Host "PIDs: $($proc1.Id) $($proc2.Id) $($proc3.Id)"
Write-Host ""
Write-Host "To test fault detection:" -ForegroundColor Yellow
Write-Host "  Kill node2 with: Stop-Process -Id $($proc2.Id)"
Write-Host "  Watch node1 and node3 log that node2 is OFFLINE after 6 seconds"
Write-Host ""
Write-Host "To test clock synchronization:" -ForegroundColor Yellow
Write-Host "  curl http://127.0.0.1:9001/time"
Write-Host "  curl http://127.0.0.1:9002/time"
Write-Host "  curl http://127.0.0.1:9003/time"
Write-Host ""
Write-Host "Press Ctrl+C to stop all nodes"

# Wait for Ctrl+C
try {
    while ($true) {
        Start-Sleep -Seconds 1
    }
} finally {
    Write-Host "`nStopping all nodes..." -ForegroundColor Red
    Stop-Process -Id $proc1.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $proc2.Id -ErrorAction SilentlyContinue
    Stop-Process -Id $proc3.Id -ErrorAction SilentlyContinue
    Write-Host "All nodes stopped." -ForegroundColor Green
}
