$ErrorActionPreference='Stop'
$auditRepo=(Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '../../..')).Path
$auditOverlay=Join-Path ([IO.Path]::GetTempPath()) ('ovh-audit-routes-'+[guid]::NewGuid().ToString('N')+'.json')
$auditMap=@{Replace=@{(Join-Path $auditRepo 'backend/internal/router/audit_routes_test.go')=(Join-Path $PSScriptRoot 'routes_test.go')}}
[IO.File]::WriteAllText($auditOverlay,($auditMap|ConvertTo-Json -Depth 4),[Text.UTF8Encoding]::new($false))
$env:AUDIT_ROUTE_OUTPUT=Join-Path $auditRepo 'docs/audit-2026-11-06/evidence/routes.json'
Push-Location (Join-Path $auditRepo 'backend')
try {
 go test -overlay $auditOverlay ./internal/router -run '^TestAuditRouteInventory$' -count=1 -v
 if($LASTEXITCODE -ne 0){throw 'Route registry audit failed'}
} finally {
 Pop-Location
 Remove-Item -LiteralPath $auditOverlay -Force
 Remove-Item Env:AUDIT_ROUTE_OUTPUT
}
