#Requires -Version 5.1
<#
.SYNOPSIS
  OVH_WEBUI first-run / re-init

.PARAMETER Fresh
  Wipe backend/data (backed up with timestamp)

.PARAMETER ForceEnv
  Regenerate backend/.env even if it exists
#>
param(
  [switch]$Fresh,
  [switch]$ForceEnv
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Backend = Join-Path $Root "backend"
$EnvExample = Join-Path $Backend ".env.example"
$EnvFile = Join-Path $Backend ".env"
$DataDir = Join-Path $Backend "data"
$RootEnvExample = Join-Path $Root ".env.example"
$RootEnv = Join-Path $Root ".env"

function New-ApiSecretKey {
  $bytes = New-Object byte[] 32
  [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
  return (-join ($bytes | ForEach-Object { $_.ToString("x2") }))
}

function Write-Step([string]$msg) {
  Write-Host ""
  Write-Host "==> $msg" -ForegroundColor Cyan
}

Write-Host "OVH_WEBUI init" -ForegroundColor Green
Write-Host "Root: $Root"

if (-not (Test-Path $EnvExample)) {
  throw "Missing $EnvExample"
}

Write-Step "backend/.env"
$apiKey = $null

if ((Test-Path $EnvFile) -and (-not $ForceEnv)) {
  Write-Host "  keep existing backend/.env (use -ForceEnv to regenerate)"
  foreach ($line in Get-Content $EnvFile) {
    if ($line -match '^\s*API_SECRET_KEY\s*=\s*(.+)\s*$') {
      $apiKey = $Matches[1].Trim().Trim('"').Trim("'")
      break
    }
  }
} else {
  if (Test-Path $EnvFile) {
    $bak = "$EnvFile.bak-" + (Get-Date -Format "yyyyMMdd-HHmmss")
    Copy-Item $EnvFile $bak -Force
    Write-Host "  backed up -> $bak"
  }
  $apiKey = New-ApiSecretKey
  $lines = Get-Content $EnvExample
  $out = New-Object System.Collections.Generic.List[string]
  foreach ($line in $lines) {
    if ($line -match 'INSPECTION_ALLOWLIST|ALLOW_FULL_INSPECTION') {
      continue
    }
    if ($line -match '^\s*API_SECRET_KEY\s*=') {
      $out.Add("API_SECRET_KEY=$apiKey")
    } else {
      $out.Add($line)
    }
  }
  $out | Set-Content -Path $EnvFile -Encoding UTF8
  Write-Host "  wrote backend/.env with random API_SECRET_KEY"
}

if (-not $apiKey) {
  $apiKey = "(see backend/.env API_SECRET_KEY)"
}

Write-Step "root .env (docker optional)"
if ((Test-Path $RootEnvExample) -and ((-not (Test-Path $RootEnv)) -or $ForceEnv)) {
  if ((Test-Path $RootEnv) -and $ForceEnv) {
    Copy-Item $RootEnv ($RootEnv + ".bak-" + (Get-Date -Format "yyyyMMdd-HHmmss")) -Force
  }
  $rootKey = $apiKey
  if ($rootKey -like "(see*") {
    $rootKey = New-ApiSecretKey
  }
  $rlines = Get-Content $RootEnvExample
  $rout = New-Object System.Collections.Generic.List[string]
  foreach ($line in $rlines) {
    if ($line -match '^\s*API_SECRET_KEY\s*=') {
      $rout.Add("API_SECRET_KEY=$rootKey")
    } else {
      $rout.Add($line)
    }
  }
  $rout | Set-Content -Path $RootEnv -Encoding UTF8
  Write-Host "  wrote root .env"
} elseif (Test-Path $RootEnv) {
  Write-Host "  keep existing root .env"
} else {
  Write-Host "  skip root .env"
}

Write-Step "backend/data"
if ($Fresh) {
  if (Test-Path $DataDir) {
    $bakDir = Join-Path $Backend ("data.bak-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
    Move-Item $DataDir $bakDir -Force
    Write-Host "  moved old data -> $bakDir"
  }
  New-Item -ItemType Directory -Path (Join-Path $DataDir "logs") -Force | Out-Null
  New-Item -ItemType Directory -Path (Join-Path $DataDir "cache") -Force | Out-Null
  Write-Host "  empty data/ ready (first-use mode)"
} else {
  if (-not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Path (Join-Path $DataDir "logs") -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $DataDir "cache") -Force | Out-Null
    Write-Host "  created data/"
  } else {
    Write-Host "  keep existing data/ (pass -Fresh to wipe)"
  }
}

Write-Step "security checklist"
$giPath = Join-Path $Root ".gitignore"
if (Test-Path $giPath) {
  $giText = Get-Content $giPath -Raw
  foreach ($pat in @("backend/.env", ".env", "backend/data/")) {
    if ($giText.Contains($pat)) {
      Write-Host "  [ok] gitignore has $pat" -ForegroundColor DarkGreen
    } else {
      Write-Host "  [!!] gitignore missing $pat" -ForegroundColor Yellow
    }
  }
}
Write-Host "  Never commit: backend/.env, backend/data/, OVH keys, Telegram tokens"

Write-Host ""
Write-Host "----------------------------------------" -ForegroundColor Green
Write-Host " Next steps" -ForegroundColor Green
Write-Host "----------------------------------------" -ForegroundColor Green
Write-Host "1. Backend:  cd backend ; go run ."
Write-Host "2. Frontend: npm install ; npm run dev"
Write-Host "3. Open http://127.0.0.1:8080 and login with:"
Write-Host "   $apiKey" -ForegroundColor Yellow
Write-Host "4. Add first OVH account in UI (keys stay in local SQLite only)"
Write-Host "5. Optional smoke:"
Write-Host '   $env:API_SECRET_KEY="<key above>"'
Write-Host '   $env:OVH_APP_KEY="..." ; $env:OVH_APP_SECRET="..." ; $env:OVH_CONSUMER_KEY="..."'
Write-Host "   python scripts/smoke_test.py"
Write-Host ""
Write-Host "Done."
