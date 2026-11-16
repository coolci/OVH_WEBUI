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

function Test-WeakApiSecretKey([string]$Key) {
  if ([string]::IsNullOrWhiteSpace($Key) -or $Key.Trim().Length -lt 24) {
    return $true
  }
  $lower = $Key.Trim().ToLowerInvariant()
  $weakExact = @("123456", "12345678", "1234567890", "password", "admin", "secret", "changeme", "default", "ovh-webui-dev-key")
  if ($weakExact -contains $lower) {
    return $true
  }
  foreach ($prefix in @("change-me", "your-api-key", "replace-me")) {
    if ($lower.StartsWith($prefix)) {
      return $true
    }
  }
  return $false
}

function Assert-EnvApiKeyWritten([string]$Path, [string]$ExpectedKey) {
  if (Test-WeakApiSecretKey $ExpectedKey) {
    throw "Refusing weak or empty API_SECRET_KEY for $Path"
  }
  $writtenLines = Get-Content $Path -Encoding UTF8
  $expectedLine = "API_SECRET_KEY=$ExpectedKey"
  if (-not ($writtenLines -contains $expectedLine)) {
    throw "Failed to verify API_SECRET_KEY written to $Path"
  }
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
  foreach ($line in Get-Content $EnvFile -Encoding UTF8) {
    if ($line -match '^\s*API_SECRET_KEY\s*=\s*(.*)\s*$') {
      $apiKey = $Matches[1].Trim().Trim('"').Trim("'")
      break
    }
  }
  if (Test-WeakApiSecretKey $apiKey) {
    throw "Existing $EnvFile has empty or weak API_SECRET_KEY; re-run with -ForceEnv to regenerate."
  }
} else {
  if (Test-Path $EnvFile) {
    $bak = "$EnvFile.bak-" + (Get-Date -Format "yyyyMMdd-HHmmss")
    Copy-Item $EnvFile $bak -Force
    Write-Host "  backed up -> $bak"
  }
  $apiKey = New-ApiSecretKey
  $lines = Get-Content $EnvExample -Encoding UTF8
  $out = New-Object System.Collections.Generic.List[string]
  $replaced = $false
  foreach ($line in $lines) {
    if ($line -match 'INSPECTION_ALLOWLIST|ALLOW_FULL_INSPECTION') {
      continue
    }
    if ($line -match '^API_SECRET_KEY=.*' -or $line -match '^\s*API_SECRET_KEY\s*=') {
      if (-not $replaced) {
        $out.Add("API_SECRET_KEY=$apiKey")
        $replaced = $true
      }
    } else {
      $out.Add($line)
    }
  }
  if (-not $replaced) {
    $out.Add("API_SECRET_KEY=$apiKey")
  }
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllLines($EnvFile, $out, $utf8NoBom)
  Assert-EnvApiKeyWritten -Path $EnvFile -ExpectedKey $apiKey
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
  $rlines = Get-Content $RootEnvExample -Encoding UTF8
  $rout = New-Object System.Collections.Generic.List[string]
  $rReplaced = $false
  foreach ($line in $rlines) {
    if ($line -match '^API_SECRET_KEY=.*' -or $line -match '^\s*API_SECRET_KEY\s*=') {
      if (-not $rReplaced) {
        $rout.Add("API_SECRET_KEY=$rootKey")
        $rReplaced = $true
      }
    } else {
      $rout.Add($line)
    }
  }
  if (-not $rReplaced) {
    $rout.Add("API_SECRET_KEY=$rootKey")
  }
  $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
  [System.IO.File]::WriteAllLines($RootEnv, $rout, $utf8NoBom)
  Assert-EnvApiKeyWritten -Path $RootEnv -ExpectedKey $rootKey
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
  $giText = Get-Content $giPath -Raw -Encoding UTF8
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
