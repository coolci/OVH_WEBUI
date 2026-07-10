#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""分析缓存服务器列表中 storage / options 对 NVMe/SSD/SATA(SA) 的表达与匹配准确性"""
from __future__ import annotations

import collections
import json
import os
import re
import urllib.request

BASE = os.environ.get("SMOKE_BASE", "http://127.0.0.1:19998")
KEY = os.environ.get("API_SECRET_KEY", "")


def get(path: str):
    import time

    if not KEY:
        raise SystemExit("请设置环境变量 API_SECRET_KEY")
    req = urllib.request.Request(
        BASE + path,
        headers={
            "X-API-Key": KEY,
            "X-Request-Time": str(int(time.time() * 1000)),
        },
    )
    with urllib.request.urlopen(req, timeout=120) as r:
        return json.loads(r.read().decode())


# 与 Go StandardizeConfig 简化对齐：小写 + 去尾缀介质标记
RE_STOR_SFX = re.compile(r"-(sas|sa|ssd|nvme)$")


def standardize(config: str) -> str:
    if not config:
        return ""
    n = config.strip().lower()
    # 简化版：只做介质尾缀剥离（报告重点）
    n2 = RE_STOR_SFX.sub("", n)
    return n2


def media_from_code(code: str) -> str:
    c = (code or "").lower()
    if not c or c == "n/a":
        return "na"
    if "hybrid" in c or "混合" in c:
        return "hybrid"
    # 顺序：更具体的先
    if "nvme" in c:
        return "nvme"
    if "ssd" in c:
        return "ssd"
    # OVH 用 sa 表示 SATA HDD（不是 SAS；SAS 常写作 sas）
    if re.search(r"(^|[^a-z])sa([^a-z]|$)|sata", c) or c.endswith("sa") or "-sa-" in c or c.endswith("-sa"):
        return "sata_sa"
    if "sas" in c:
        return "sas"
    if "hdd" in c:
        return "hdd"
    return "unknown"


def media_from_display(storage: str) -> str:
    s = storage or ""
    low = s.lower()
    if s in ("", "N/A", None):
        return "na"
    if "混合" in s or "hybrid" in low:
        return "hybrid"
    if "nvme" in low:
        return "nvme"
    if "ssd" in low:
        return "ssd"
    # 展示层 ToUpper("sa") => "SA"
    if re.search(r"\bsa\b", low) or " sata" in low:
        return "sata_sa"
    if "sas" in low:
        return "sas"
    if "hdd" in low:
        return "hdd"
    return "unknown"


def extract_storage_options(s: dict) -> list[str]:
    out = []
    for key in ("defaultOptions", "availableOptions"):
        for o in s.get(key) or []:
            if isinstance(o, dict):
                val = o.get("value") or o.get("label") or ""
                fam = (o.get("family") or "").lower()
            else:
                val = str(o)
                fam = ""
            vlow = val.lower()
            if fam in ("storage", "system-storage", "disk", "drive") or any(
                x in vlow for x in ("softraid", "raid", "nvme", "ssd", "hdd", "disk", "hybrid")
            ):
                if val and val not in out:
                    out.append(val)
    return out


def main():
    data = get("/api/servers?showApiServers=true")
    servers = data.get("servers") if isinstance(data, dict) else data
    print(f"缓存服务器条目: {len(servers)}")
    print(f"cacheInfo: {json.dumps(data.get('cacheInfo'), ensure_ascii=False) if isinstance(data, dict) else 'n/a'}")
    print()

    # 1) 展示字段 storage 分类
    disp_counts = collections.Counter()
    raw_storage_values = collections.Counter()
    for s in servers:
        st = s.get("storage") or "N/A"
        raw_storage_values[st] += 1
        disp_counts[media_from_display(st)] += 1

    print("=== 1. 列表展示字段 storage 的介质分类 ===")
    for k, v in disp_counts.most_common():
        print(f"  {k:12} {v:4d}")
    print()

    print("=== 2. storage 字段原始取值 Top 25 ===")
    for st, n in raw_storage_values.most_common(25):
        print(f"  [{n:3d}] {st}")
    print()

    # 3) 选项级介质（default + available）
    opt_media = collections.Counter()
    opt_samples = collections.defaultdict(list)
    mismatch_display_vs_default = []
    standardize_collision = []  # 不同介质标准化后相同

    for s in servers:
        plan = s.get("planCode")
        storage_disp = s.get("storage") or "N/A"
        disp_m = media_from_display(storage_disp)
        opts = extract_storage_options(s)
        default_opts = []
        for o in s.get("defaultOptions") or []:
            if isinstance(o, dict):
                default_opts.append(o.get("value") or "")
            else:
                default_opts.append(str(o))

        # default storage option media
        def_stor = [x for x in default_opts if any(t in x.lower() for t in ("softraid", "raid", "nvme", "ssd", "hdd", "disk", "hybrid", "sa"))]
        def_m = media_from_code(def_stor[0]) if def_stor else "na"

        if disp_m not in ("na", "unknown") and def_m not in ("na", "unknown") and disp_m != def_m:
            if len(mismatch_display_vs_default) < 30:
                mismatch_display_vs_default.append(
                    {
                        "plan": plan,
                        "display": storage_disp,
                        "display_media": disp_m,
                        "default_opt": def_stor[0] if def_stor else None,
                        "default_media": def_m,
                    }
                )

        # all storage options medias + standardize collisions
        medias_for_plan = set()
        std_map = {}  # std -> set(media)
        for opt in opts:
            m = media_from_code(opt)
            opt_media[m] += 1
            medias_for_plan.add(m)
            if len(opt_samples[m]) < 8:
                opt_samples[m].append(opt)
            std = standardize(opt)
            std_map.setdefault(std, set()).add(m)

        for std, medias in std_map.items():
            # 若标准化后同一字符串对应多种介质 → 危险
            real = medias - {"na", "unknown"}
            if len(real) >= 2:
                standardize_collision.append(
                    {"plan": plan, "std": std, "medias": sorted(real), "opts": [o for o in opts if standardize(o) == std][:6]}
                )

    print("=== 3. 选项 value 中出现的介质（default+available 累加计数）===")
    for k, v in opt_media.most_common():
        print(f"  {k:12} {v:4d}")
    print()
    print("=== 4. 各介质选项样例（原始 addon code）===")
    for m in ("nvme", "ssd", "sata_sa", "sas", "hdd", "hybrid", "unknown"):
        if m not in opt_samples:
            continue
        print(f"  [{m}]")
        for o in opt_samples[m][:6]:
            print(f"    {o}  → standardize≈ {standardize(o)!r}")
    print()

    print("=== 5. 展示 storage 与 defaultOptions 存储介质不一致 ===")
    print(f"  数量: {len(mismatch_display_vs_default)}")
    for it in mismatch_display_vs_default[:15]:
        print(
            f"  {it['plan']}: display={it['display']!r}({it['display_media']}) "
            f"default={it['default_opt']!r}({it['default_media']})"
        )
    print()

    print("=== 6. StandardizeConfig 剥离介质后缀后的冲突（同 std 多介质）===")
    # 去重
    uniq = {}
    for c in standardize_collision:
        key = (c["std"], tuple(c["medias"]))
        if key not in uniq:
            uniq[key] = c
    print(f"  冲突模式数: {len(uniq)}")
    for c in list(uniq.values())[:20]:
        print(f"  std={c['std']!r} medias={c['medias']}")
        for o in c["opts"]:
            print(f"    - {o}")
    print()

    # 7) 解析 regex 覆盖：storRe 是否识别 sa/nvme/ssd
    stor_re = re.compile(r"(?i)(raid|softraid)-(\d+)x(\d+)(ssd|hdd|nvme|sa)")
    hybrid_re = re.compile(
        r"(?i)hybridsoftraid-(\d+)x(\d+)(sa|ssd|hdd)-(\d+)x(\d+)(nvme|ssd|hdd)"
    )
    parse_ok = collections.Counter()
    unparsed = []
    for s in servers:
        for o in s.get("defaultOptions") or []:
            val = o.get("value") if isinstance(o, dict) else str(o)
            if not val:
                continue
            vlow = val.lower()
            if not any(t in vlow for t in ("softraid", "raid", "hybrid", "nvme", "ssd")):
                continue
            if hybrid_re.search(val):
                parse_ok["hybrid_ok"] += 1
            elif stor_re.search(val):
                m = stor_re.search(val)
                parse_ok[f"stor_ok_{m.group(4).lower()}"] += 1
            else:
                parse_ok["unparsed"] += 1
                if len(unparsed) < 20:
                    unparsed.append(val)

    print("=== 7. 默认存储 addon 解析覆盖（catalog.go storRe/hybridRe）===")
    for k, v in parse_ok.most_common():
        print(f"  {k}: {v}")
    if unparsed:
        print("  未解析样例:")
        for u in unparsed:
            print(f"    {u}")
    print()

    # 8) SA 是否被正确显示为 SA 而非 SATA 文字
    sa_display = [st for st in raw_storage_values if re.search(r"\bSA\b", st) or st.lower().endswith(" sa")]
    sata_word = [st for st in raw_storage_values if "sata" in st.lower()]
    print("=== 8. 命名：SA vs SATA ===")
    print(f"  展示含 'SA'（OVH 介质码）的 storage 值种类: {len(sa_display)}")
    for st in sa_display[:10]:
        print(f"    {st} (n={raw_storage_values[st]})")
    print(f"  展示含 'SATA' 字样的 storage 值种类: {len(sata_word)}")
    for st in sata_word[:10]:
        print(f"    {st}")
    print()

    # 9) 结论统计
    print("=== 结论摘要 ===")
    print(
        f"  展示字段覆盖: NVMe={disp_counts['nvme']}, SSD={disp_counts['ssd']}, "
        f"SA/SATA={disp_counts['sata_sa']}, SAS={disp_counts['sas']}, HDD={disp_counts['hdd']}, "
        f"hybrid={disp_counts['hybrid']}, N/A={disp_counts['na']}, unknown={disp_counts['unknown']}"
    )
    print(f"  display vs default 介质不一致条数: {len(mismatch_display_vs_default)}")
    print(f"  Standardize 去介质后缀后的多介质冲突模式: {len(uniq)}")
    print(f"  默认 addon 未解析: {parse_ok.get('unparsed', 0)}")


if __name__ == "__main__":
    main()
