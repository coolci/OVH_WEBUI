# -*- coding: utf-8 -*-
from pathlib import Path
import re

pages = Path(r"C:\Users\video\Desktop\OVH\OVH_WEBUI\src\pages")


def rewrite_import(inner: str) -> str:
    parts = [x.strip() for x in inner.split(",") if x.strip() and x.strip() != "createFileRoute"]
    if not parts:
        return ""
    return f'import {{ {", ".join(parts)} }} from "react-router-dom";\n'


for p in pages.glob("*.tsx"):
    t = p.read_text(encoding="utf-8")
    orig = t
    t = re.sub(
        r'import\s*\{([^}]*)\}\s*from\s*["\']@tanstack/react-router["\'];\s*',
        lambda m: rewrite_import(m.group(1)),
        t,
    )
    t = re.sub(r"export const Route = createFileRoute\([^\)]*\)\(\{[\s\S]*?\}\);\s*", "", t)
    if t != orig:
        p.write_text(t, encoding="utf-8", newline="\n")
        print("fixed", p.name)

print("done")
