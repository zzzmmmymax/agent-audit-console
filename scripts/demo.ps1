$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$demoRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('agent-audit-demo-' + [guid]::NewGuid().ToString('N'))
$workspace = Join-Path $demoRoot 'workspace'
$dataDir = Join-Path $demoRoot 'audit-data'
$auditBin = Join-Path $demoRoot 'audit.exe'
New-Item -ItemType Directory -Path $workspace -Force | Out-Null

Push-Location $repoRoot
try { go build -o $auditBin ./cmd/audit } finally { Pop-Location }
Push-Location $workspace
try {
  git init -q
  "module example.com/agent-audit-demo`n`ngo 1.26.0`n" | Set-Content -Encoding utf8 go.mod
  "package demo`n`nfunc Value() int { return 1 }`n" | Set-Content -Encoding utf8 value.go
  "package demo`n`nimport `"testing`"`n`nfunc TestValue(t *testing.T) { if Value() != 2 { t.Fatal(Value()) } }`n" | Set-Content -Encoding utf8 value_test.go
  $runOutput = & $auditBin --data-dir $dataDir run -C $workspace -- powershell -NoProfile -Command "Set-Content -Encoding utf8 value.go 'package demo`n`nfunc Value() int { return 2 }'; go test ./..."
  $runOutput | Write-Output
  $runId = ([regex]::Match(($runOutput -join "`n"), 'run (run_[^:]+):')).Groups[1].Value
  if (-not $runId) { throw 'Unable to parse run ID' }
  & $auditBin --data-dir $dataDir show $runId
  & $auditBin --data-dir $dataDir verify $runId
  & $auditBin --data-dir $dataDir restore (Join-Path $workspace 'value.go')
  & $auditBin --data-dir $dataDir verify $runId
  Write-Output "Demo completed in $demoRoot"
} finally { Pop-Location }
