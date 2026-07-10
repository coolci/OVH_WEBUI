#Requires -Version 5.1
# 本地开发：稳定启动后端 + 前台 Vite
$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$env:Path = "C:\Program Files\Go\bin;C:\Program Files\nodejs;" + $env:Path

Write-Host "==> Backend"
& (Join-Path $PSScriptRoot "start-backend.ps1")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> Frontend http://127.0.0.1:8080"
Set-Location $Root
if (-not (Test-Path "node_modules")) {
  cmd /c "npm install"
}
$envFile = Join-Path $Root "backend\.env"
if (Test-Path $envFile) {
  Write-Host "API Key: 见 backend/.env 的 API_SECRET_KEY"
} else {
  Write-Host "提示: 先运行 .\scripts\init-first-run.ps1"
}
cmd /c "npx vite --host 0.0.0.0 --port 8080"
