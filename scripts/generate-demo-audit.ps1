param([string]$DataDir = ".tmp/web-audit-demo", [int]$LargeEvents = 1000)
$ErrorActionPreference = "Stop"
go run ./cmd/audit-demo-data --data-dir $DataDir --large-events $LargeEvents
Write-Host "Start the console with: go run ./cmd/auditd --data-dir $DataDir"
