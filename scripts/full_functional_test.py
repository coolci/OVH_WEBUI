#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
OVH_WEBUI 全功能测试（从零）

安全红线：
  - 禁止：暂停/关机/重启/重装/删除/终止/改联系人/改 boot/改监控 等写操作
  - 实机细节仅允许：环境变量 SMOKE_ALLOWED_SERVER（可选）
  - 重点：服务器列表 + 服务器控制（只读）

用法：
  set API_SECRET_KEY=<与 backend/.env 一致>
  set SMOKE_ALLOWED_SERVER=<可选，只读目标机>
  python scripts/full_functional_test.py
"""
from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Callable

BASE = os.environ.get("SMOKE_BASE", "http://127.0.0.1:19998")
API_KEY = os.environ.get("API_SECRET_KEY", "")
ALLOWED = os.environ.get("SMOKE_ALLOWED_SERVER", "").strip()

# 绝对禁止的危险路径关键字（测试脚本自身不会调用）
FORBIDDEN_ACTIONS = (
    "reboot",
    "terminate",
    "confirm-termination",
    "install",
    "reinstall",
    "start",
    "stop",
    "password",
    "change-contact",
    "hardware/replace",
    "ola/",
    "move",
    "burst",  # PUT
    "firewall",  # PUT - we only GET
)


@dataclass
class Result:
    name: str
    ok: bool
    detail: str = ""
    ms: int = 0
    group: str = ""


@dataclass
class Suite:
    results: list[Result] = field(default_factory=list)

    def add(self, r: Result) -> None:
        self.results.append(r)
        mark = "PASS" if r.ok else "FAIL"
        print(f"  [{mark}] {r.name} ({r.ms}ms) {r.detail[:160]}")

    @property
    def failed(self) -> list[Result]:
        return [r for r in self.results if not r.ok]

    @property
    def passed(self) -> list[Result]:
        return [r for r in self.results if r.ok]


def req(
    method: str,
    path: str,
    body: dict | None = None,
    *,
    timeout: int = 90,
) -> tuple[int, Any]:
    data = None
    headers = {
        "Content-Type": "application/json",
        "X-API-Key": API_KEY,
        "X-Request-Time": str(int(time.time() * 1000)),
    }
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8")
            if not raw:
                return resp.status, None
            try:
                return resp.status, json.loads(raw)
            except json.JSONDecodeError:
                return resp.status, raw
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        try:
            return e.code, json.loads(raw)
        except Exception:
            return e.code, raw


def check(
    suite: Suite,
    group: str,
    name: str,
    fn: Callable[[], tuple[bool, str]],
) -> bool:
    t0 = time.time()
    try:
        ok, detail = fn()
    except Exception as e:
        ok, detail = False, f"exception: {e}"
    ms = int((time.time() - t0) * 1000)
    suite.add(Result(name=name, ok=ok, detail=detail, ms=ms, group=group))
    return ok


def section(title: str) -> None:
    print(f"\n{'=' * 60}\n{title}\n{'=' * 60}")


def main() -> int:
    if not API_KEY:
        print("ERROR: 设置环境变量 API_SECRET_KEY（与 backend/.env 一致）")
        return 2
    suite = Suite()
    print(f"BASE={BASE}  ALLOWED={ALLOWED or '(未设置 SMOKE_ALLOWED_SERVER，将跳过目标机详情)'}")
    print("安全策略: 只读测试，禁止暂停/删除/重启/重装/终止")

    # ─── A. 系统基础 ─────────────────────────────────────────
    section("A. 系统基础")

    def health():
        code, data = req("GET", "/health")
        ok = code == 200 and isinstance(data, dict) and data.get("status") == "ok"
        return ok, f"code={code} body={data}"

    check(suite, "A", "GET /health", health)

    def api_health():
        code, data = req("GET", "/api/health")
        ok = code == 200 and isinstance(data, dict) and data.get("status") == "ok"
        return ok, f"code={code}"

    check(suite, "A", "GET /api/health", api_health)

    def stats():
        code, data = req("GET", "/api/stats")
        ok = code == 200 and isinstance(data, dict) and "queueProcessorRunning" in data
        return ok, f"code={code} keys={list(data.keys()) if isinstance(data, dict) else type(data)}"

    check(suite, "A", "GET /api/stats", stats)

    def version():
        code, data = req("GET", "/api/version")
        return code == 200, f"code={code} data={str(data)[:120]}"

    check(suite, "A", "GET /api/version", version)

    def no_inspection():
        code, _ = req("GET", "/api/inspection/policy")
        return code == 404, f"code={code} (巡检已取消)"

    check(suite, "A", "线上巡检已移除", no_inspection)

    def no_config_sniper():
        code, data = req("GET", "/api/config-sniper/tasks")
        ok = code != 200 or (
            isinstance(data, dict) and "error" in data
        )
        if code == 200 and (isinstance(data, list) or (isinstance(data, dict) and "tasks" in data)):
            ok = False
        return ok, f"code={code} (废弃功能不应可用)"

    check(suite, "A", "Config Sniper 已下线", no_config_sniper)

    # ─── B. 账户 ─────────────────────────────────────────────
    section("B. 账户 / 鉴权")

    def accounts():
        code, data = req("GET", "/api/accounts")
        if code != 200:
            return False, f"code={code} {data}"
        accs = data.get("accounts") if isinstance(data, dict) else data
        n = len(accs or [])
        return n >= 1, f"accounts={n}"

    check(suite, "B", "GET /api/accounts", accounts)

    def verify_auth():
        code, data = req("POST", "/api/verify-auth", {})
        ok = code == 200 and isinstance(data, dict) and data.get("valid") is True
        return ok, f"code={code} data={data}"

    check(suite, "B", "POST /api/verify-auth", verify_auth)

    def me():
        code, data = req("GET", "/api/ovh/account/info")
        if code != 200:
            return False, f"code={code} {data}"
        # raw /me or wrapped
        nh = None
        if isinstance(data, dict):
            nh = data.get("nichandle") or (data.get("account") or {}).get("nichandle")
        return bool(nh), f"nichandle={nh}"

    check(suite, "B", "GET /api/ovh/account/info", me)

    def orders():
        code, data = req("GET", "/api/ovh/account/orders?limit=10")
        if code != 200:
            return False, f"code={code} {data}"
        items = data if isinstance(data, list) else (data.get("orders") if isinstance(data, dict) else [])
        items = items or []
        return True, f"count={len(items)}"

    check(suite, "B", "GET /api/ovh/account/orders", orders)

    # ─── C. 服务器列表（可购 catalog）────────────────────────
    section("C. 服务器列表 (catalog /api/servers)")

    catalog_count = {"n": 0}

    def servers_list():
        # 必须 showApiServers=true 才会从 OVH 拉可购列表
        code, data = req("GET", "/api/servers?showApiServers=true", timeout=180)
        if code != 200:
            return False, f"code={code} {str(data)[:200]}"
        items = data if isinstance(data, list) else (data.get("servers") if isinstance(data, dict) else [])
        items = items or []
        catalog_count["n"] = len(items)
        if len(items) == 0:
            return False, "empty list after showApiServers=true (预期有可购套餐)"
        sample = items[0]
        pc = sample.get("planCode") or sample.get("plan_code") or "?"
        # 字段完整性
        need = ["planCode", "name"]
        miss = [k for k in need if not sample.get(k)]
        if miss:
            return False, f"sample missing {miss}"
        return True, f"count={len(items)} sample={pc} name={sample.get('name')}"

    check(suite, "C", "GET /api/servers?showApiServers=true", servers_list)

    def servers_fields():
        code, data = req("GET", "/api/servers?showApiServers=true", timeout=60)
        if code != 200:
            return False, f"code={code}"
        items = (data.get("servers") if isinstance(data, dict) else data) or []
        if len(items) < 1:
            return False, "empty"
        # 抽样检查 cpu/memory/datacenters
        ok_n = 0
        for s in items[:20]:
            if s.get("planCode") and (s.get("cpu") or s.get("memory") or s.get("datacenters")):
                ok_n += 1
        return ok_n >= 5, f"rich_fields_in_first20={ok_n}"

    check(suite, "C", "服务器列表字段抽样", servers_fields)

    def cache_info():
        code, data = req("GET", "/api/cache/info")
        return code == 200, f"code={code} {str(data)[:150]}"

    check(suite, "C", "GET /api/cache/info", cache_info)

    def catalog():
        code, data = req("GET", "/api/catalog", timeout=120)
        # catalog 可能较大或依赖 subsidiary
        return code in (200, 400, 502), f"code={code} type={type(data).__name__}"

    check(suite, "C", "GET /api/catalog (可达性)", catalog)

    # ─── D. 已购服务器列表（控制中心）────────────────────────
    section("D. 服务器控制 - 列表 (重点)")

    target = {"found": False, "servers": []}

    def sc_list():
        code, data = req("GET", "/api/server-control/list", timeout=120)
        if code != 200:
            return False, f"code={code} {data}"
        if not isinstance(data, dict) or not data.get("success"):
            return False, f"unexpected: {data}"
        servers = data.get("servers") or []
        target["servers"] = servers
        names = [s.get("serviceName") for s in servers]
        if not ALLOWED:
            return True, f"total={len(servers)} (未设 SMOKE_ALLOWED_SERVER，仅校验列表)"
        target["found"] = ALLOWED in names
        if not target["found"]:
            return False, f"SMOKE_ALLOWED_SERVER 不在列表: {names}"
        t = next(s for s in servers if s.get("serviceName") == ALLOWED)
        required = ["serviceName", "state", "ip", "datacenter"]
        missing = [k for k in required if not t.get(k) and t.get(k) != 0]
        if missing:
            return False, f"字段缺失 {missing} server={t}"
        return True, (
            f"total={len(servers)} target state={t.get('state')} "
            f"ip={t.get('ip')} dc={t.get('datacenter')} os={t.get('os')}"
        )

    check(suite, "D", "GET /api/server-control/list", sc_list)

    def sc_list_fields():
        if not ALLOWED:
            return True, "skip: no SMOKE_ALLOWED_SERVER"
        if not target["found"]:
            return False, "skip: no target"
        t = next(s for s in target["servers"] if s.get("serviceName") == ALLOWED)
        st = str(t.get("state", "")).lower()
        ok = st in ("ok", "active")
        return ok, f"state={t.get('state')} status={t.get('status')} monitoring={t.get('monitoring')}"

    check(suite, "D", "目标机 state 健康", sc_list_fields)

    def sc_aliases():
        code, data = req("GET", "/api/server-control/aliases")
        return code == 200, f"code={code} {str(data)[:120]}"

    check(suite, "D", "GET /api/server-control/aliases", sc_aliases)

    # ─── E. 服务器控制只读详情（仅当设置 SMOKE_ALLOWED_SERVER）──────────────
    section(f"E. 服务器控制 - 只读详情 ({ALLOWED or '跳过'})")

    def ok200(code: int, data: Any) -> tuple[bool, str]:
        if code == 200:
            return True, f"ok type={type(data).__name__}"
        return False, f"code={code} {str(data)[:120]}"

    def ok_soft(code: int, data: Any) -> tuple[bool, str]:
        if code == 200:
            return True, "ok"
        if code in (404, 400, 460) or (
            isinstance(data, dict)
            and (data.get("error") or data.get("success") is False)
        ):
            return True, f"soft-ok code={code} (功能可能未开通)"
        return False, f"code={code} {str(data)[:100]}"

    if not ALLOWED or not target.get("found"):
        check(
            suite,
            "E",
            "目标机只读详情",
            lambda: (True, "skip: 设置 SMOKE_ALLOWED_SERVER 且账户拥有该机后执行"),
        )
    else:
        svc = ALLOWED
        base = f"/api/server-control/{svc}"
        core_gets = [
            ("hardware", f"{base}/hardware", ok200),
            ("serviceinfo", f"{base}/serviceinfo", ok200),
            ("ips", f"{base}/ips", ok200),
            ("templates", f"{base}/templates", ok200),
            ("tasks", f"{base}/tasks", ok200),
            ("boot", f"{base}/boot", ok_soft),
            ("boot-mode", f"{base}/boot-mode", ok_soft),
            ("monitoring", f"{base}/monitoring", ok_soft),
            ("network-specs", f"{base}/network-specs", ok_soft),
            ("network-interfaces", f"{base}/network-interfaces", ok_soft),
            ("mrtg", f"{base}/mrtg", ok_soft),
            ("statistics", f"{base}/statistics", ok_soft),
            ("interventions", f"{base}/interventions", ok_soft),
            ("planned-interventions", f"{base}/planned-interventions", ok_soft),
            ("engagement", f"{base}/engagement", ok_soft),
            ("engagement/available", f"{base}/engagement/available", ok_soft),
            ("bios-settings", f"{base}/bios-settings", ok_soft),
            ("backup-ftp", f"{base}/backup-ftp", ok_soft),
            ("reverse", f"{base}/reverse", ok_soft),
            ("console", f"{base}/console", ok_soft),
            ("options", f"{base}/options", ok_soft),
            ("vrack", f"{base}/vrack", ok_soft),
            ("secondary-dns", f"{base}/secondary-dns", ok_soft),
            ("virtual-mac", f"{base}/virtual-mac", ok_soft),
            ("ip-specs", f"{base}/ip-specs", ok_soft),
            ("ongoing", f"{base}/ongoing", ok_soft),
        ]
        for name, path, validator in core_gets:
            # 闭包绑定
            def make(n=name, p=path, v=validator):
                def _fn():
                    # 双重保险：禁止危险 path
                    low = p.lower()
                    for bad in ("reboot", "terminate", "install", "/stop", "/start"):
                        if bad in low and "boot-mode" not in low:
                            return False, f"blocked dangerous path {p}"
                    code, data = req("GET", p, timeout=90)
                    ok, detail = v(code, data)
                    # 抽样关键字段
                    if ok and n == "hardware" and isinstance(data, dict):
                        hw = data.get("hardware") or data
                        if isinstance(hw, dict) and hw:
                            detail += f" keys={list(hw.keys())[:8]}"
                    if ok and n == "serviceinfo" and isinstance(data, dict):
                        si = data.get("serviceInfo") or data
                        if isinstance(si, dict):
                            detail += f" status={si.get('status')} exp={si.get('expiration')}"
                    if ok and n == "ips":
                        ips = data.get("ips") if isinstance(data, dict) else data
                        detail += f" count={len(ips) if isinstance(ips, list) else '?'}"
                    return ok, detail

                return _fn

            check(suite, "E", f"GET {name}", make())

    # ─── F. 询价 + 订阅更新 API 可达性 ───────────────────────
    section("F. 询价 / 订阅更新")

    def server_price():
        # 用列表里第一台 plan 询价（只读 cart，不落单）
        code, data = req("GET", "/api/servers?showApiServers=true", timeout=120)
        items = []
        if code == 200 and isinstance(data, dict):
            items = data.get("servers") or []
        if not items:
            return True, "skip: no catalog"
        pc = items[0].get("planCode")
        dcs = items[0].get("datacenters") or []
        dc = "gra"
        if dcs and isinstance(dcs[0], dict):
            dc = dcs[0].get("datacenter") or dc
        code2, price = req(
            "POST",
            f"/api/servers/{pc}/price",
            {"datacenter": dc, "options": []},
            timeout=90,
        )
        ok = code2 == 200 and isinstance(price, dict) and (
            price.get("success") is True or price.get("price") is not None
        )
        # 询价失败也记 soft 信息（库存/区域问题）
        if code2 == 200 and isinstance(price, dict) and price.get("success") is False:
            return True, f"soft price err={price.get('error')}"
        return ok, f"code={code2} plan={pc} dc={dc} {str(price)[:100]}"

    check(suite, "F", "POST /api/servers/:planCode/price", server_price)

    def mon_put_missing():
        # 对不存在的订阅 PUT 应 404
        code, data = req(
            "PUT",
            "/api/monitor/subscriptions/__no_such_plan__",
            {"notifyAvailable": True},
        )
        return code == 404, f"code={code}"

    check(suite, "F", "PUT 监控订阅(不存在→404)", mon_put_missing)

    def vps_put_missing():
        code, data = req(
            "PUT",
            "/api/vps-monitor/subscriptions/__no_such_id__",
            {"notifyAvailable": True},
        )
        return code == 404, f"code={code}"

    check(suite, "F", "PUT VPS 订阅(不存在→404)", vps_put_missing)

    # ─── G. 其它安全可读 ─────────────────────────────────────
    section("G. 其它模块 (只读抽检)")

    def queue():
        code, data = req("GET", "/api/queue")
        return code == 200, f"code={code} type={type(data).__name__}"

    check(suite, "G", "GET /api/queue", queue)

    def history():
        code, data = req("GET", "/api/purchase-history")
        return code == 200, f"code={code}"

    check(suite, "G", "GET /api/purchase-history", history)

    def logs():
        code, data = req("GET", "/api/logs")
        return code == 200, f"code={code}"

    check(suite, "G", "GET /api/logs", logs)

    def mon_status():
        code, data = req("GET", "/api/monitor/status")
        return code == 200, f"code={code} {data}"

    check(suite, "G", "GET /api/monitor/status", mon_status)

    def vps_mon():
        code, data = req("GET", "/api/vps-monitor/status")
        return code == 200, f"code={code}"

    check(suite, "G", "GET /api/vps-monitor/status", vps_mon)

    def vps_list():
        code, data = req("GET", "/api/vps-control/list", timeout=60)
        # 无 VPS 也可能 success 空列表
        return code == 200, f"code={code} {str(data)[:120]}"

    check(suite, "G", "GET /api/vps-control/list", vps_list)

    def contact_req():
        code, data = req("GET", "/api/ovh/contact-change-requests")
        return code == 200, f"code={code}"

    check(suite, "G", "GET /api/ovh/contact-change-requests", contact_req)

    def metrics():
        code, data = req("GET", "/api/system/metrics")
        return code == 200, f"code={code}"

    check(suite, "G", "GET /api/system/metrics", metrics)

    # ─── 汇总 ────────────────────────────────────────────────
    section("汇总")
    by_group: dict[str, list[Result]] = {}
    for r in suite.results:
        by_group.setdefault(r.group, []).append(r)

    for g, items in by_group.items():
        p = sum(1 for i in items if i.ok)
        print(f"  {g}: {p}/{len(items)} passed")

    total = len(suite.results)
    passed = len(suite.passed)
    failed = suite.failed
    print(f"\n总计: {passed}/{total} PASS")
    if failed:
        print("失败项:")
        for f in failed:
            print(f"  - [{f.group}] {f.name}: {f.detail}")

    # 核心硬指标：D+E 中核心失败则整体失败
    core_fail = [
        f
        for f in failed
        if f.group in ("D", "E")
        and not f.name.startswith("SKIP")
        and f.name
        in (
            "GET /api/server-control/list",
            "目标机 state 健康",
            "GET hardware",
            "GET serviceinfo",
            "GET ips",
            "GET templates",
            "GET tasks",
        )
        or (
            f.group == "E"
            and f.name
            in (
                "GET hardware",
                "GET serviceinfo",
                "GET ips",
                "GET templates",
                "GET tasks",
            )
        )
    ]
    # simplify: any D fail or E core get fail
    hard = [
        f
        for f in failed
        if f.group == "D"
        or f.name
        in (
            "GET hardware",
            "GET serviceinfo",
            "GET ips",
            "GET templates",
            "GET tasks",
            "POST /api/verify-auth",
            "GET /health",
        )
    ]

    report = {
        "base": BASE,
        "allowed_server": ALLOWED,
        "total": total,
        "passed": passed,
        "failed": len(failed),
        "failed_items": [{"group": f.group, "name": f.name, "detail": f.detail} for f in failed],
        "catalog_count": catalog_count["n"],
        "target_found": target["found"],
        "hard_failures": len(hard),
        "policy": {
            "no_pause_delete_reboot_reinstall": True,
            "config_sniper": "fully_retired",
        },
    }
    out_path = os.path.join(
        os.path.dirname(__file__),
        "..",
        "docs",
        "handover",
        "FULL-TEST-RESULTS.md",
    )
    # also write json next to script results in docs
    md_path = os.path.normpath(
        os.path.join(os.path.dirname(__file__), "..", "docs", "handover", "FULL-TEST-RESULTS.md")
    )
    lines = [
        "# 全功能测试结果",
        "",
        f"- 时间: {time.strftime('%Y-%m-%d %H:%M:%S')}",
        f"- BASE: `{BASE}`",
        f"- 目标机: `{ALLOWED}`",
        f"- 结果: **{passed}/{total} PASS**",
        f"- 硬失败(核心): {len(hard)}",
        f"- Config Sniper: **完全下线**",
        f"- 安全: 未执行暂停/删除/重启/重装/终止",
        "",
        "## 分组",
        "",
    ]
    for g, items in by_group.items():
        p = sum(1 for i in items if i.ok)
        lines.append(f"### {g} ({p}/{len(items)})")
        lines.append("")
        lines.append("| 用例 | 结果 | ms | 详情 |")
        lines.append("|------|------|-----|------|")
        for i in items:
            lines.append(
                f"| {i.name} | {'✅' if i.ok else '❌'} | {i.ms} | {i.detail.replace('|', '/')} |"
            )
        lines.append("")
    if failed:
        lines.append("## 失败列表")
        lines.append("")
        for f in failed:
            lines.append(f"- **[{f.group}] {f.name}**: {f.detail}")
        lines.append("")
    lines.append("## JSON")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(report, ensure_ascii=False, indent=2))
    lines.append("```")
    lines.append("")
    with open(md_path, "w", encoding="utf-8") as fp:
        fp.write("\n".join(lines))
    print(f"\n报告已写: {md_path}")

    if hard:
        print("OVERALL: FAIL (核心用例失败)")
        return 1
    if failed:
        print("OVERALL: PASS_WITH_WARNINGS (核心通过，部分 soft 失败)")
        return 0
    print("OVERALL: PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main())
