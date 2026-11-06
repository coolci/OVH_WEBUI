$ErrorActionPreference = 'Stop'
$repoPath = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../../..')).Path
Push-Location (Join-Path $repoPath 'backend')
try {
 go test ./internal/handlers -run 'Test(ServerSupport|SnapshotConfirmed|ReinstallPreservation|PreservationRejects|NormalizeStorage)' -count=1 -v
 if ($LASTEXITCODE -ne 0) { throw 'Regression failed' }
} finally { Pop-Location }
