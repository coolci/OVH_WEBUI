#!/usr/bin/env python3
"""
OVH_WEBUI 烟测脚本（与 ADR：巡检 / Config Sniper 已下线对齐）

- 健康检查
- 确认已废弃接口返回 404
- 列出账户；仅当设置了 OVH_* 环境变量时才自动创建
- 有账户时 verify-auth / account info
- 可选：SMOKE_ALLOWED_SERVER 存在时做 list + hardware 只读

用法（勿把真实密钥写进仓库）:
  set API_SECRET_KEY=<与 backend/.env 一致>
  set OVH_APP_KEY=...
  set OVH_APP_SECRET=...
  set OVH_CONSUMER_KEY=...
  set SMOKE_ALLOWED_SERVER=nsXXXX.ip-....net   # 可选
  python scripts/smoke_test.py
"""
from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

BASE = os.environ.get("SMOKE_BASE", "http://127.0.0.1:19998")
API_KEY = os.environ.get("API_SECRET_KEY", "")
ALLOWED = os.environ.get("SMOKE_ALLOWED_SERVER", "").strip()

APP_KEY = os.environ.get("OVH_APP_KEY", "").strip()
APP_SECRET = os.environ.get("OVH_APP_SECRET", "").strip()
CONSUMER_KEY = os.environ.get("OVH_CONSUMER_KEY", "").strip()


class Fail(Exception):
    pass


def req(method: str, path: str, body: dict | None = None) -> dict | list | None:
    if not API_KEY:
        raise Fail("请设置环境变量 API_SECRET_KEY（与 backend/.env 一致）")
    data = None
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY,
        "X-Request-Time": str(int(__import__("time").time() * 1000)),
    }
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r, timeout=120) as resp:
            raw = resp.read().decode("utf-8")
            if not raw:
                return None
            return json.loads(raw)
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        raise Fail(f"{method} {path} -> HTTP {e.code}: {raw}") from e
    except Exception as e:
        raise Fail(f"{method} {path} -> {e}") from e


def req_status(method: str, path: str) -> int:
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY or "x",
        "X-Request-Time": str(int(__import__("time").time() * 1000)),
    }
    r = urllib.request.Request(BASE + path, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r, timeout=30) as resp:
            return int(resp.status)
    except urllib.error.HTTPError as e:
        return int(e.code)


def ok(msg: str) -> None:
    print(f"[PASS] {msg}")


def step(title: str) -> None:
    print(f"\n=== {title} ===")


def main() -> int:
    failures = 0

    def run(name, fn):
        nonlocal failures
        try:
            fn()
            ok(name)
        except Exception as e:
            failures += 1
            print(f"[FAIL] {name}: {e}")

    if not API_KEY:
        print("ERROR: 未设置 API_SECRET_KEY。请先运行 scripts/init-first-run.ps1 并 export 密钥。")
        return 2

    step("1. Health")

    def health():
        h = req("GET", "/health")
        assert isinstance(h, dict) and h.get("status") == "ok", h

    run("GET /health", health)

    step("2. Retired features must be 404")

    def retired():
        code_insp = req_status("GET", "/api/inspection/policy")
        code_snip = req_status("GET", "/api/config-sniper/tasks")
        assert code_insp == 404, f"inspection expected 404 got {code_insp}"
        assert code_snip == 404, f"config-sniper expected 404 got {code_snip}"
        print(f"  inspection={code_insp} config-sniper={code_snip}")

    run("inspection/config-sniper gone", retired)

    step("3. Accounts")

    def ensure_account():
        listed = req("GET", "/api/accounts")
        accs = listed.get("accounts") if isinstance(listed, dict) else listed
        accs = accs or []
        if accs:
            print(f"  existing accounts: {len(accs)}")
            return
        if not (APP_KEY and APP_SECRET and CONSUMER_KEY):
            print("  无账户且未设置 OVH_APP_* 环境变量，跳过创建")
            return
        created = req(
            "POST",
            "/api/accounts",
            {
                "name": "smoke-test",
                "zone": os.environ.get("OVH_ZONE", "IE"),
                "endpoint": os.environ.get("OVH_ENDPOINT", "ovh-eu"),
                "appKey": APP_KEY,
                "appSecret": APP_SECRET,
                "consumerKey": CONSUMER_KEY,
                "setDefault": True,
            },
        )
        print(f"  created: {json.dumps(created, ensure_ascii=False)[:200]}")

    run("create/list accounts", ensure_account)

    step("4. Verify auth /me")

    def verify():
        listed = req("GET", "/api/accounts")
        accs = (listed.get("accounts") if isinstance(listed, dict) else listed) or []
        if not accs:
            print("  skip: no accounts")
            return
        v = req("POST", "/api/verify-auth", {})
        assert v.get("valid") is True, v

    run("POST /api/verify-auth", verify)

    step("5. Account info")

    def me():
        listed = req("GET", "/api/accounts")
        accs = (listed.get("accounts") if isinstance(listed, dict) else listed) or []
        if not accs:
            print("  skip: no accounts")
            return
        info = req("GET", "/api/ovh/account/info")
        assert info.get("success") is True or "nichandle" in str(info), info
        print(f"  info keys: {list(info.keys()) if isinstance(info, dict) else type(info)}")

    run("GET /api/ovh/account/info", me)

    step("6. List dedicated servers")
    target = {"found": False}

    def list_servers():
        listed = req("GET", "/api/accounts")
        accs = (listed.get("accounts") if isinstance(listed, dict) else listed) or []
        if not accs:
            print("  skip: no accounts")
            return
        data = req("GET", "/api/server-control/list")
        assert data.get("success") is True, data
        servers = data.get("servers") or []
        names = [s.get("serviceName") for s in servers]
        print(f"  servers({len(names)}): {names}")
        if ALLOWED and ALLOWED in names:
            target["found"] = True
            srv = next(s for s in servers if s.get("serviceName") == ALLOWED)
            print(f"  target state={srv.get('state')} ip={srv.get('ip')} dc={srv.get('datacenter')}")
        elif ALLOWED:
            print(f"  WARN: SMOKE_ALLOWED_SERVER={ALLOWED} not in account")

    run("GET /api/server-control/list", list_servers)

    step("7. Catalog servers")

    def catalog():
        listed = req("GET", "/api/accounts")
        accs = (listed.get("accounts") if isinstance(listed, dict) else listed) or []
        if not accs:
            print("  skip: no accounts")
            return
        data = req("GET", "/api/servers?showApiServers=true")
        servers = data.get("servers") if isinstance(data, dict) else data
        assert isinstance(servers, list) and len(servers) > 0, data
        sample = servers[0]
        print(
            f"  count={len(servers)} sample={sample.get('planCode')} "
            f"storage={sample.get('storage')}"
        )

    run("GET /api/servers", catalog)

    step("8. Read-only hardware (optional allowlist)")

    def hardware():
        if not ALLOWED:
            print("  skip: set SMOKE_ALLOWED_SERVER to probe hardware")
            return
        if not target["found"]:
            print("  skip: target not in list")
            return
        hw = req("GET", f"/api/server-control/{ALLOWED}/hardware")
        assert hw is not None
        assert isinstance(hw, dict) and ("hardware" in hw or hw.get("success") is not False), hw
        print(f"  hardware keys={list(hw.keys()) if isinstance(hw, dict) else type(hw)}")

    run("GET hardware of allowlisted server", hardware)

    step("9. Stats")

    def stats():
        s = req("GET", "/api/stats")
        assert isinstance(s, dict)
        print(f"  stats: {json.dumps(s, ensure_ascii=False)[:300]}")

    run("GET /api/stats", stats)

    step("10. Logs limited")

    def logs():
        s = req("GET", "/api/logs?limit=5&order=desc")
        assert isinstance(s, dict) and "logs" in s
        print(f"  returned={s.get('returned')} total={s.get('total')} truncated={s.get('truncated')}")

    run("GET /api/logs?limit=5", logs)

    print("\n" + "=" * 40)
    if failures:
        print(f"SMOKE FAILED: {failures} check(s)")
        return 1
    print("SMOKE PASSED")
    return 0


if __name__ == "__main__":
    sys.exit(main())
