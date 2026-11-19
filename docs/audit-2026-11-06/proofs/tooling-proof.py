"""Offline evidence. Never import deployment modules or contact their hosts."""
import ast
import csv
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

root = Path(__file__).resolve().parents[3]


def tree(file):
    return ast.parse((root / file).read_text(encoding='utf-8-sig'))


# TL-01: Legacy scratch/ SSH deploy scripts removed and no Python script under scripts/ or tools/ contains literal SSH passwords.
assert not (root / 'scratch').exists(), 'scratch/ directory must be removed'
secrets = []
for base in [root / 'scripts', root / 'tools']:
    for py_file in base.rglob('*.py'):
        if py_file.name.endswith('_test.py') or py_file.name.startswith('test_'):
            continue
        for node in ast.walk(ast.parse(py_file.read_text(encoding='utf-8-sig'))):
            if isinstance(node, ast.keyword) and node.arg == 'password' and isinstance(node.value, ast.Constant) and node.value.value:
                secrets.append({'file': str(py_file.relative_to(root)), 'line': node.lineno, 'kind': 'literal SSH password'})
            if isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id == 'SERVER_PASS' for t in node.targets):
                if isinstance(node.value, ast.Constant) and node.value.value:
                    secrets.append({'file': str(py_file.relative_to(root)), 'line': node.lineno, 'kind': 'literal SSH password'})
assert len(secrets) == 0, secrets
print('REGRESSION TL-01: scratch/ removed and no SSH password literals remain')

# TL-02: Linux deployment health fragment fails on timeout (works on Linux CI and Windows Git Bash).
git_bash = Path('C:/Program Files/Git/bin/bash.exe')
bash = git_bash if git_bash.exists() else Path(shutil.which('bash') or '/bin/bash')
assert bash.exists(), 'Bash required for isolated shell fragment proof'
linux = (root / 'scripts/linux-oneclick-deploy.sh').read_text(encoding='utf-8')
frag_start = linux.index('backend_healthy=0')
fragment = linux[frag_start:linux.index('API_KEY="$(grep', frag_start)]
assert 'backend_healthy=1' in fragment and 'exit 1' in fragment, 'health fragment extraction failed'
for success in [True, False]:
    mock = 'sleep(){ :; }; grn(){ :; }; docker(){ return ' + ('0' if success else '1') + '; };\n'
    result = subprocess.run([str(bash), '--noprofile', '--norc'], input=mock + fragment, text=True, capture_output=True)
    assert result.returncode == (0 if success else 1)
print('REGRESSION TL-02: Linux deployment health fragment succeeds when healthy and fails on timeout')

# TL-03: complete navigate conversion has balanced syntax.
port = tree('scripts/_port_pages.py')
assignments = [n for n in ast.walk(port) if isinstance(n, ast.Assign) and
               any(isinstance(t, ast.Name) and t.id == 't' for t in n.targets) and
               'navigate' in ast.unparse(n.value)]
env = {'t': 'navigate({ to: "/queue" })', 're': re}
for n in sorted(assignments, key=lambda n: n.lineno):
    exec(compile(ast.Module(body=[n], type_ignores=[]), '<isolated-transform>', 'exec'), env)
assert re.fullmatch(r'navigate\("/queue"\s*\)', env['t'])
print('REGRESSION TL-03: complete navigate conversion has balanced syntax')

# TL-04: route scanner reads all actual routes matching the ledger.
r = subprocess.run(['node', 'tools/audit-api-routes.cjs'], cwd=root, text=True, capture_output=True)
ledger_routes = len((root / 'docs/archive/progress/ledger.csv').read_text(encoding='utf-8').strip().splitlines()) - 1
assert r.returncode == 0 and f'Found {ledger_routes} backend routes' in r.stdout, (r.returncode, r.stdout[:200], r.stderr[:200])
print(f'REGRESSION TL-04: scanner reads all {ledger_routes} actual routes')

# TL-05: rebuild bypasses online exit, builds before replacing scoped binary.
start = (root / 'scripts/start-backend.ps1').read_text(encoding='utf-8-sig')
assert 'if ((Test-BackendUp) -and -not $Rebuild)' in start
assert 'Where-Object { $_.Path -eq $Exe } | Stop-Process' in start
assert start.index('& go build -o $BuildExe') < start.index('Move-Item -LiteralPath $BuildExe')
print('REGRESSION TL-05: rebuild bypasses online exit, builds before replacing scoped binary')

# TL-06: auth/server errors fail; capability absence is SKIP.
soft = next(n for n in ast.walk(tree('scripts/full_functional_test.py')) if isinstance(n, ast.FunctionDef) and n.name == 'ok_soft')
soft.returns = None
for arg in soft.args.args:
    arg.annotation = None
env = {}
exec(compile(ast.Module(body=[soft], type_ignores=[]), '<isolated-validator>', 'exec'), env)
assert env['ok_soft'](500, {'error': 'upstream unavailable'})[0] is False
assert env['ok_soft'](401, {'error': 'unauthorized'})[0] is False
assert env['ok_soft'](200, {})[0] is True
assert env['ok_soft'](404, {})[1].startswith('skip:')
for status in [400, 403, 429, 502]:
    assert env['ok_soft'](status, {'error': 'fixture'})[0] is False
print('REGRESSION TL-06: auth/server errors fail; capability absence is SKIP')

# TL-07: nonexistent method fails verification (temporary ledger only).
with (root / 'docs/prd-v2/progress/ledger.csv').open(encoding='utf-8-sig', newline='') as f:
    rows = list(csv.reader(f))
row = rows[1].copy()
row[3], row[6] = 'NOT_A_METHOD', 'done'
with tempfile.TemporaryDirectory() as directory:
    ledger = Path(directory) / 'ledger.csv'
    with ledger.open('w', encoding='utf-8', newline='') as f:
        csv.writer(f).writerows([rows[0], row, *rows[2:]])
    result = subprocess.run(['go', 'run', './tools/status', '--ledger', str(ledger), '--verify'],
                            cwd=root, capture_output=True, text=True)
    assert result.returncode != 0 and 'Invalid or duplicate registry entry' in result.stderr
print('REGRESSION TL-07: nonexistent method fails verification (temporary ledger only)')
