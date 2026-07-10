#Requires -Version 5.1
# Stable Windows backend: fully detached (survives terminal close).
#   .\scripts\start-backend.ps1
#   .\scripts\start-backend.ps1 -Rebuild
#   .\scripts\start-backend.ps1 -Stop
param(
  [switch]$Rebuild,
  [switch]$Stop
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Backend = Join-Path $Root "backend"
$Exe = Join-Path $Backend "ovh-webui.exe"

function Test-BackendUp {
  try {
    $r = Invoke-WebRequest -Uri "http://127.0.0.1:19998/health" -UseBasicParsing -TimeoutSec 2
    return ($r.StatusCode -eq 200)
  } catch {
    return $false
  }
}

if ($Stop) {
  Get-Process ovh-webui -ErrorAction SilentlyContinue | Stop-Process -Force
  Write-Host "backend stopped"
  exit 0
}

if (Test-BackendUp) {
  Write-Host "backend already online: http://127.0.0.1:19998/health"
  Get-Process ovh-webui -ErrorAction SilentlyContinue | Format-Table Id, ProcessName, StartTime -AutoSize
  exit 0
}

$env:Path = "C:\Program Files\Go\bin;" + $env:Path
New-Item -ItemType Directory -Path (Join-Path $Backend "data\logs") -Force | Out-Null
New-Item -ItemType Directory -Path (Join-Path $Backend "data\cache") -Force | Out-Null

if (-not (Test-Path -LiteralPath (Join-Path $Backend ".env"))) {
  Write-Host "WARN: no backend/.env - run .\scripts\init-first-run.ps1 first" -ForegroundColor Yellow
}

if ($Rebuild -or -not (Test-Path -LiteralPath $Exe)) {
  Write-Host "building ovh-webui.exe ..."
  Push-Location $Backend
  try {
    & go build -o ovh-webui.exe .
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
  } finally {
    Pop-Location
  }
}

Get-Process ovh-webui -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 400

# Fully detached: UseShellExecute=true so process is not killed when this shell exits.
# Binary loads backend/.env via godotenv from WorkingDirectory.
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $Exe
$psi.WorkingDirectory = $Backend
$psi.UseShellExecute = $true
$psi.WindowStyle = [System.Diagnostics.ProcessWindowStyle]::Hidden
$proc = [System.Diagnostics.Process]::Start($psi)
if (-not $proc) { throw "failed to start process" }
Write-Host "started pid=$($proc.Id) (detached) cwd=$Backend"

$ok = $false
for ($i = 0; $i -lt 30; $i++) {
  Start-Sleep -Milliseconds 500
  if (Test-BackendUp) {
    $ok = $true
    break
  }
}

if ($ok) {
  Write-Host "ONLINE http://127.0.0.1:19998/health" -ForegroundColor Green
  exit 0
}

Write-Host "FAILED health check - is port 19998 free? is .env valid?" -ForegroundColor Red
exit 1
