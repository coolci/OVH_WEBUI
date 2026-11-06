param([switch]$Race)

$ErrorActionPreference = 'Stop'
$auditRepo = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../../..')).Path
$auditBackend = Join-Path $auditRepo 'backend'
$auditEvidence = Join-Path $PSScriptRoot '../evidence/order-monitor-current.log'
$auditOverlayPath = Join-Path ([System.IO.Path]::GetTempPath()) ('ovh-order-monitor-audit-' + [guid]::NewGuid().ToString('N') + '.json')
$auditReplacement = @{}
if ($Race) {
    $auditReplacement[(Join-Path $auditBackend 'internal/price/zz_price_cache_race_audit_test.go')] = (Join-Path $PSScriptRoot 'price_cache_race_audit_test.go.txt')
}
[System.IO.File]::WriteAllText($auditOverlayPath, (@{ Replace = $auditReplacement } | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))
Push-Location $auditBackend
try {
    # All confirmed cases now use production regression tests.
    # Tests use httptest/mock transport, fixture accounts, and temporary SQLite databases.
    & go test -overlay $auditOverlayPath ./internal/purchase ./internal/telegram ./internal/handlers ./internal/monitor -run 'Test(PollerErrors|UncertainReconcileFailure|PrewarmMissing|PrewarmRejected|PrewarmComplete|PrewarmWithout|CustomDotted|DottedPlan|ReconcileRequires|WatchAllKeeps|EmptyConfigurationDeletion|ExplicitEmptyConfigurationPause|MultiAccount)'  -count=1 -v 2>&1 | Tee-Object -FilePath $auditEvidence
    if ($LASTEXITCODE -ne 0) { throw 'Order/monitor isolated reproduction failed. Inspect evidence/order-monitor-current.log.' }
    if ($Race) {
        # Optional only: needs CGO + a supported C compiler. A race report is an
        # expected nonzero result; never claim it passed based on the ordinary run.
        & go test -race -overlay $auditOverlayPath ./internal/price -run TestAuditConcurrentPriceKey -count=1 -v 2>&1 | Tee-Object -FilePath (Join-Path $PSScriptRoot '../evidence/order-monitor-race.log')
        if ($LASTEXITCODE -ne 0) { throw 'Race proof exited nonzero; inspect the output to distinguish detected race from unsupported environment.' }
    }
} finally {
    Pop-Location
    Remove-Item -LiteralPath $auditOverlayPath -Force
}
