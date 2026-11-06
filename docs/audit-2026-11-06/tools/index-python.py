"""Extract Python functions and executable script entrypoints without importing them."""
import ast
import json
from pathlib import Path
import re
import subprocess

files = subprocess.check_output(['git', 'ls-files'], text=True).splitlines()
entries = []
for file in files:
    path = Path(file)
    if path.suffix not in ('.py', '.ps1', '.sh'): continue
    source = path.read_text(encoding='utf-8-sig')
    lines = source.splitlines()
    entries.append(dict(file=file, name='<script entrypoint>', line=1, end=len(lines), kind='script'))
    if path.suffix == '.py':
        for n in ast.walk(ast.parse(source)):
            if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
                entries.append(dict(file=file, name=getattr(n,'name',f'<lambda@{n.lineno}>'), line=n.lineno, end=n.end_lineno, kind='Python AST'))
    else:
        pattern = r'^\s*(?:function\s+([\w-]+)|([\w-]+)\s*\(\s*\)\s*\{)'
        for i, line in enumerate(lines, 1):
            m = re.match(pattern, line, re.I)
            if m:
                entries.append(dict(file=file, name=m.group(1) or m.group(2), line=i, end=i, kind='shell/PowerShell definition (lexical)'))
print(json.dumps(entries))
