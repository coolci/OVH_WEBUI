$ErrorActionPreference = 'Stop'
$repoPath = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../../..')).Path
Push-Location (Join-Path $repoPath 'backend')
try {
 go test ./internal/server -run 'Test(Run|Failed|Pairing|AccountUpdate|Deleted|VPSMutators|ConcurrentVPS|LegacyCredentials)' -count=1 -v
 if ($LASTEXITCODE -ne 0) { throw 'Regression failed' }
} finally { Pop-Location }
