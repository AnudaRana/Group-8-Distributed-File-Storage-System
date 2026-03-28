# PowerShell script to run a single node on Windows

$env:NODE_ID = $env:NODE_ID, "node1" | Select-Object -First 1
$env:HOST = $env:HOST, "127.0.0.1" | Select-Object -First 1
$env:PORT = $env:PORT, "9001" | Select-Object -First 1
$env:PEERS = $env:PEERS, "" | Select-Object -First 1

Write-Host "Starting Node $($env:NODE_ID) on $($env:HOST):$($env:PORT)..." -ForegroundColor Green

& go run cmd/node/main.go
