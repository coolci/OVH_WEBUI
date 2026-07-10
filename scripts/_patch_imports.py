from pathlib import Path

root = Path(r"C:\Users\video\Desktop\OVH\OVH_WEBUI\src")

for p in (root / "hooks" / "ovh").rglob("*.ts"):
    t = p.read_text(encoding="utf-8")
    n = t.replace('from "@/lib/api"', 'from "@/lib/http"')
    n = n.replace("from '@/lib/api'", "from '@/lib/http'")
    if n != t:
        p.write_text(n, encoding="utf-8", newline="\n")
        print("hook", p.name)

dirs = [
    root / "components" / "server-control",
    root / "components" / "common",
    root / "components" / "vps-control",
]
for d in dirs:
    for p in d.rglob("*.tsx"):
        t = p.read_text(encoding="utf-8")
        if "@/lib/api" not in t and "'@/lib/api'" not in t:
            continue
        n = t.replace('from "@/lib/api"', 'from "@/lib/http"')
        n = n.replace("from '@/lib/api'", "from '@/lib/http'")
        p.write_text(n, encoding="utf-8", newline="\n")
        print("comp", p.relative_to(root))

# hook re-exports
hooks = root / "hooks"
ovh = hooks / "ovh"
for p in ovh.glob("use-*.ts"):
    name = p.name
    out = hooks / name
    content = f'/** re-export from ovh-web port */\nexport * from "./ovh/{name[:-3]}";\n'
    out.write_text(content, encoding="utf-8", newline="\n")
    print("export", name)

print("DONE")
