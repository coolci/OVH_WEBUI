"""Offline evidence. Never import deployment modules or contact their hosts."""
import ast
import csv
import json
from pathlib import Path
import re
import subprocess
import tempfile

root = Path(__file__).resolve().parents[3]
def tree(file):
    return ast.parse((root / file).read_text(encoding='utf-8-sig'))

# Record presence and source locations only. Never print credential values.
secrets = []
for file in ['scratch/check_remote.py', 'scratch/check_server_compose.py', 'scratch/package_and_deploy.py']:
    for node in ast.walk(tree(file)):
        if isinstance(node, ast.keyword) and node.arg == 'password' and isinstance(node.value, ast.Constant) and node.value.value:
            secrets.append({'file': file, 'line': node.lineno, 'kind': 'literal SSH password'})
        if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'SERVER_PASS' for t in node.targets):
            assert isinstance(node.value, ast.Constant) and node.value.value
            secrets.append({'file': file, 'line': node.lineno, 'kind': 'literal SSH password'})
assert len(secrets) == 0
print('REGRESSION TL-01: no SSH password literals remain')

bash = Path('C:/Program Files/Git/bin/bash.exe')
assert bash.exists(), 'Git Bash required for isolated shell fragment proof'
deploy = tree('scratch/package_and_deploy.py')
health = next(n.value.value for n in ast.walk(deploy) if isinstance(n, ast.Assign)
              and any(isinstance(t, ast.Name) and t.id == 'verify_script' for t in n.targets))
# Only the health fragment is executed; shell functions replace all external commands.
for status, expected in [('healthy', 0), ('unhealthy', 1), ('missing', 1)]:
    mock = 'sleep(){ :; }; docker(){ if [ "$1" = inspect ]; then ' + ('return 1' if status == 'missing' else 'echo '+status) + '; else :; fi; };\n'
    r = subprocess.run([str(bash), '--noprofile', '--norc'], input=mock+health, text=True, capture_output=True)
    assert r.returncode == expected, (status, r.stdout, r.stderr)
print('REGRESSION TL-02: healthy succeeds; unhealthy/missing containers fail (mock only)')
linux = (root/'scripts/linux-oneclick-deploy.sh').read_text(encoding='utf-8')
fragment = linux[linux.index('backend_healthy=0'):linux.index('API_KEY="$(grep')]
for success in [True,False]:
    mock = 'sleep(){ :; }; grn(){ :; }; docker(){ return '+('0' if success else '1')+'; };\n'
    result = subprocess.run([str(bash),'--noprofile','--norc'],input=mock+fragment,text=True,capture_output=True)
    assert result.returncode == (0 if success else 1)
print('REGRESSION TL-02: Linux deployment health fragment also fails on timeout')


port = tree('scripts/_port_pages.py')
assignments = [n for n in ast.walk(port) if isinstance(n, ast.Assign) and n.lineno in (105,107)]
# Select the two real navigate transformations, avoiding all file writes.
assignments = [n for n in ast.walk(port) if isinstance(n, ast.Assign) and
               any(isinstance(t, ast.Name) and t.id == 't' for t in n.targets) and
               'navigate' in ast.unparse(n.value)]
env = {'t': 'navigate({ to: "/queue" })', 're': re}
for n in sorted(assignments, key=lambda n:n.lineno):
    exec(compile(ast.Module(body=[n], type_ignores=[]), '<isolated-transform>', 'exec'), env)
assert re.fullmatch(r'navigate\("/queue"\s*\)',env['t'])
print('REGRESSION TL-03: complete navigate conversion has balanced syntax')

r = subprocess.run(['node', 'scratch/audit_api_routes.cjs'], cwd=root, text=True, capture_output=True)
assert r.returncode == 0 and 'Found 256 backend routes' in r.stdout
print('REGRESSION TL-04: scanner reads all 256 actual routes')

start = (root/'scripts/start-backend.ps1').read_text(encoding='utf-8-sig')
assert 'if ((Test-BackendUp) -and -not $Rebuild)' in start
assert 'Where-Object { $_.Path -eq $Exe } | Stop-Process' in start
assert start.index('& go build -o $BuildExe') < start.index('Move-Item -LiteralPath $BuildExe')
print('REGRESSION TL-05: rebuild bypasses online exit, builds before replacing scoped binary')

soft = next(n for n in ast.walk(tree('scripts/full_functional_test.py')) if isinstance(n, ast.FunctionDef) and n.name == 'ok_soft')
soft.returns = None
for arg in soft.args.args: arg.annotation = None
env = {}
exec(compile(ast.Module(body=[soft], type_ignores=[]), '<isolated-validator>', 'exec'), env)
assert env['ok_soft'](500, {'error': 'upstream unavailable'})[0] is False
assert env['ok_soft'](401, {'error': 'unauthorized'})[0] is False
assert env['ok_soft'](200, {})[0] is True
assert env['ok_soft'](404, {})[1].startswith('skip:')
for status in [400,403,429,502]: assert env['ok_soft'](status, {'error':'fixture'})[0] is False
print('REGRESSION TL-06: auth/server errors fail; capability absence is SKIP')

with (root/'docs/prd-v2/progress/ledger.csv').open(encoding='utf-8-sig', newline='') as f:
    rows = list(csv.reader(f))
row = rows[1].copy()
row[3], row[6] = 'NOT_A_METHOD', 'done'
with tempfile.TemporaryDirectory() as directory:
    ledger = Path(directory)/'ledger.csv'
    with ledger.open('w', encoding='utf-8', newline='') as f:
        csv.writer(f).writerows([rows[0], row, *rows[2:]])
    result = subprocess.run(['go', 'run', './tools/status', '--ledger', str(ledger), '--verify'],
                            cwd=root, capture_output=True, text=True)
    assert result.returncode != 0 and 'Invalid or duplicate registry entry' in result.stderr
print('REGRESSION TL-07: nonexistent method fails verification (temporary ledger only)')
