# -*- coding: utf-8 -*-
"""Port ovh/web routes → OVH_WEBUI pages with AppLayout + react-router."""
from pathlib import Path
import re

SRC = Path(r"C:\Users\video\Desktop\OVH\ovh\web\src\routes")
DST = Path(r"C:\Users\video\Desktop\OVH\OVH_WEBUI\src\pages")
COMP = Path(r"C:\Users\video\Desktop\OVH\OVH_WEBUI\src\components")


def strip_router(text: str, route_path: str, component_name: str) -> str:
    text = text.replace('import { createFileRoute } from "@tanstack/react-router";\n', "")
    text = text.replace("import { createFileRoute } from '@tanstack/react-router';\n", "")
    # remove Route export block
    text = re.sub(
        rf"export const Route = createFileRoute\(\"{re.escape(route_path)}\"\)\(\{{[\s\S]*?\}}\);\s*",
        "",
        text,
    )
    return text


def wrap_page(body: str, title: str, component_name: str) -> str:
    """body should define `function ComponentName()` that returns JSX of content only."""
    return f'''import {{ AppLayout }} from "@/components/layout/AppLayout";
import {{ Helmet }} from "react-helmet-async";
{body}

const Page = () => (
  <>
    <Helmet>
      <title>{title} | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <{component_name} />
    </AppLayout>
  </>
);

export default Page;
'''


def port_file(name: str, route: str, component: str, title: str, out_name: str | None = None):
    raw = (SRC / name).read_text(encoding="utf-8")
    body = strip_router(raw, route, component)
    # ensure component is not exported as Route
    if f"function {component}" not in body and f"const {component}" not in body:
        print("WARN missing component", component, name)
    out = wrap_page(body, title, component)
    target = DST / (out_name or name)
    target.write_text(out, encoding="utf-8", newline="\n")
    print("ported", target.name, "bytes", len(out))


# Core control pages from ovh/web
port_file("server-control.tsx", "/server-control", "ServerControlPage", "服务器控制", "ServerControlPage.tsx")
port_file("vps-control.tsx", "/vps-control", "VpsControlPage", "VPS 控制", "VpsControlPage.tsx")

# settings multi-account
port_file("settings.tsx", "/settings", "SettingsPage", "系统设置", "SettingsPage.tsx")

# Also port remaining route pages that improve parity
for fname, route, comp, title, outn in [
    ("servers.tsx", "/servers", "ServersPage", "服务器列表", "ServersPage.tsx"),
    ("queue.tsx", "/queue", "QueuePage", "抢购队列", "QueuePage.tsx"),
    ("history.tsx", "/history", "HistoryPage", "购买历史", "HistoryPage.tsx"),
    ("monitor.tsx", "/monitor", "MonitorPage", "独服监控", "MonitorPage.tsx"),
    ("vps-monitor.tsx", "/vps-monitor", "VpsMonitorPage", "VPS 监控", "VpsMonitorPage.tsx"),
    ("logs.tsx", "/logs", "LogsPage", "系统日志", "LogsPage.tsx"),
    ("account.tsx", "/account", "AccountPage", "账户管理", "AccountPage.tsx"),
    ("index.tsx", "/", "DashboardPage", "仪表盘", "Index.tsx"),
]:
    srcp = SRC / fname
    if not srcp.exists():
        print("skip missing", fname)
        continue
    raw = srcp.read_text(encoding="utf-8")
    # index may use different component name
    m = re.search(r"component:\s*(\w+)", raw)
    if m:
        comp = m.group(1)
    if fname == "index.tsx":
        # find function name
        m2 = re.search(r"function\s+(\w+)\s*\(", raw)
        if m2:
            comp = m2.group(1)
    body = strip_router(raw, route, comp)
    # for index route path is /
    if fname == "index.tsx":
        body = strip_router(raw, "/", comp)
        body = re.sub(r"export const Route = createFileRoute\(\"/\"\)\(\{[\s\S]*?\}\);\s*", "", body)
    out = wrap_page(body, title, comp)
    (DST / outn).write_text(out, encoding="utf-8", newline="\n")
    print("ported", outn, "comp=", comp)

# Fix CommandPalette tanstack navigate
cp = COMP / "common" / "CommandPalette.tsx"
if cp.exists():
    t = cp.read_text(encoding="utf-8")
    t = t.replace(
        'import { useNavigate } from "@tanstack/react-router";',
        'import { useNavigate } from "react-router-dom";',
    )
    # tanstack navigate({to}) vs react-router navigate(path)
    t = t.replace("navigate({ to: ", "navigate(")
    # might leave extra }) - fix common pattern navigate({ to: "/x" }) -> navigate("/x")
    t = re.sub(r"navigate\(\{\s*to:\s*([^}]+)\s*\}\)", r"navigate(\1)", t)
    cp.write_text(t, encoding="utf-8", newline="\n")
    print("fixed CommandPalette")

print("ALL_DONE")
