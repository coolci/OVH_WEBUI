#!/usr/bin/env python3
"""Generate frontend API code dictionaries from server Go sources.

Scans server/**/*.go (excluding *_test.go) for API error/status codes and
synchronizes:
  - web/src/i18n/api-codes.generated.json
  - web/src/i18n/locales/api-zh.generated.ts
  - web/src/i18n/locales/api-en.generated.ts
"""
from __future__ import annotations

import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
BACKEND_DIR = ROOT / "server"
JSON_OUT = ROOT / "web" / "src" / "i18n" / "api-codes.generated.json"
ZH_OUT = ROOT / "web" / "src" / "i18n" / "locales" / "api-zh.generated.ts"
EN_OUT = ROOT / "web" / "src" / "i18n" / "locales" / "api-en.generated.ts"
EN_TRANSLATED = ROOT / "web" / "src" / "i18n" / "locales" / "api-en.translated.ts"

CURATED_ZH: dict[str, str] = {
    "AUTH_RATE_LIMITED": "API 密钥连续错误次数过多，已暂时拒绝该来源的请求，请稍后再试",
    "CHAT_IDENTITY_MISSING": "OVH 未返回客户编号，未申请聊天令牌",
    "CHAT_INVALID_PRODUCT": "请选择独服或 VPS 客服",
    "CHAT_REQUEST_FAILED": "无法准备官方聊天认证，请检查 GET /me、POST /auth/token 权限及账户状态",
    "CHAT_TOKEN_INVALID": "OVH 未返回有效聊天令牌",
    "E1C4B0FD2": "该域名不在二级 DNS 列表中，无需删除",
    "E2BA0443E": "当前没有快照可回滚",
    "E308E982D": "这台 VPS 已经有一个重启在进行中,无需重复发送 —— 稍等它完成即可",
    "E3E1F5824": "当前没有快照，无法修改描述",
    "E4D9E7F86": "该域名已在二级 DNS 列表中，无需重复添加",
    "E5B2D91A4": "订阅不存在",
    "E704347D1": "这台服务器已经有一个重启在进行中,无需重复发送 —— 稍等它完成即可",
    "E7E8BB801": "该 IP 已在永久缓解中，无需重复开启",
    "E9E0B7F01": "该 IP 未开启永久缓解，无需关闭",
    "EA0CDDD4E": "已切回硬盘启动,并且服务器已有一个重启在进行中 —— 这次重启就会回到正常系统,约 3~5 分钟",
    "EF4E0B03F": "当前没有快照，无需删除",
    "EFD5D37BB": "回滚会丢弃快照之后的全部数据，请在请求里带 confirm:true 确认",
    "ENQUEUE_REJECTED": "任务未入队：存在重复任务、监控已暂停或队列已达上限",
    "ERASE_FALSE_MISSING_GID": "保留数据的磁盘组必须指定具体的磁盘组编号（diskGroupId > 0）",
    "ERASE_FALSE_ON_INSTALL_GROUP": "磁盘组 {{gid}} 同时带了存储配置和保留数据 —— 安装盘组必须擦除,保留数据只能用于非安装盘组",
    "ERR_ACCOUNT_NOT_FOUND": "账户不存在",
    "ERR_DB_SAVE": "状态已在运行中修改，但写入数据库失败，重启后可能恢复原状",
    "ERR_DELETE_ACCOUNT": "删除账户失败",
    "ERR_ENQUEUE_FAILED": "任务未能写入数据库，已自动撤回",
    "ERR_GET_ACCOUNT": "获取账户详情失败",
    "ERR_INVALID_INTERVAL": "重试间隔不在允许的秒数范围内",
    "ERR_INVALID_SUBSIDIARY": "未知的 OVH 子公司代码（ovhSubsidiary）",
    "ERR_LIST_ACCOUNTS": "获取账户列表失败",
    "ERR_QUEUE_NOT_FOUND": "任务不存在",
    "ERR_SET_DEFAULT": "设置默认账户失败",
    "EVPS_MULTI_SSH_KEY": "VPS 系统重建仅支持配置单个已在 OVH 登记的 SSH 密钥，请勿指定多个密钥",
    "EVPS_RAW_PUBKEY_NOT_ALLOWED": "VPS 系统重建要求填入已在 OVH 登记的密钥名称（如 id_rsa），不能直接填入公钥文本",
    "E_BATCH_AUTO_ORDER_FORBIDDEN": "全目录批量订阅禁止开启自动下单，请针对特定机型单独配置自动下单以防意外扣款",
    "E_INVALID_DOMAIN": "域名格式不正确",
    "E_NO_RETRACTION_RIGHT": "该订单无撤单权（企业/非欧盟账户或已放弃），无法申请撤单",
    "E_RETRACTION_EXPIRED": "该订单的 14 天撤单期已过期，无法申请撤单",
    "FS_UNSUPPORTED": "不支持的文件系统 {{fs}}",
    "HWRAID_BAD_SHAPE": "硬件 RAID 配置格式不正确",
    "HWRAID_LEVEL_UNSUPPORTED": "硬件 RAID 级别 {{level}} 不受支持，可选：0/1/5/6/10/50/60",
    "HWRAID_MISSING_LEVEL": "硬件 RAID 缺少 raidLevel",
    "INTERNAL_ERROR": "服务器内部处理错误",
    "INVALID_SERVICE_NAME": "服务名称格式不合法",
    "NO_OVH_ACCOUNT": "未配置 OVH 账户",
    "OVH_AUTH_INVALID": "OVH 账户 API 凭据无效或已过期，请核对账户密钥设置",
    "OVH_UPSTREAM_ERROR": "OVH 上游接口返回错误，请稍后重试",
    "PARTITION_BAD_SHAPE": "分区配置格式不正确",
    "PARTITION_FS_RAID_INCOMPAT": "分区 {{mp}}: {{reason}}",
    "PARTITION_MISSING_MOUNTPOINT": "分区缺少挂载点",
    "PARTITION_MULTI_FILL": "最多只能有一个分区把大小留空(占满剩余空间),当前有多个,请给其余分区指定大小",
    "PARTITION_NEGATIVE_SIZE": "分区 {{mp}} 的大小不能为负数",
    "PARTITION_RAID_UNSUPPORTED": "分区 {{mp}} 的软 RAID 级别 {{level}} 不受支持，可选：0/1/5/6/7/10",
    "PERMANENT_MITIGATION_REMOVED": "OVH 已停用 IP 永久缓解模式。自动 DDoS 防护无需手动开启，当前页面仅查询缓解状态。",
    "PRESERVE_REQUIRES_INSTALL_GROUP": "保留数据时必须为另一磁盘组明确配置安装分区，不能使用默认磁盘组或仅勾选保留数据",
    "QUEUE_ALREADY_ORDERED": "该任务已下单,不能再次启动。如需再买,请新建任务。",
    "RENEWAL_INVALID_REQUEST": "请填写有效的续费方式与周期。",
    "RENEWAL_MODE_UNAVAILABLE": "OVH 当前未允许该服务使用所选续费方式，请刷新后重试。",
    "REQUEST_TOO_LARGE": "请求体过大，请减少提交内容",
    "RESCUE_BOOT_CHANGED": "无法重新核对启动配置，未执行操作",
    "RESCUE_EMAIL_REQUIRED": "请填写完整邮箱，或明确选择使用账户联系邮箱",
    "RESCUE_INVALID_REQUEST": "救援请求格式不正确，未执行操作",
    "RETRACTION_ORDER_CHANGED": "请重新打开撤单窗口，确认订单号后再提交",
    "RETRACTION_SUBMITTED": "订单撤回申请已提交，影响整张订单的所有产品及数据（含备份）。退款按 OVH 规则处理，可能扣除已使用天数；请在 OVH 订单页查看结果",
    "RETRACTION_UNAVAILABLE": "无法确认此订单仍可撤回，请刷新并在 OVH 订单页核对",
    "STORAGE_DUPLICATE_GROUP": "同一磁盘组不能重复配置或同时安装与保留数据",
    "STORAGE_EMPTY": "自定义存储配置为空",
    "STORAGE_GROUP_EMPTY": "自定义存储配置里有磁盘组既没有分区也没有硬件 RAID",
    "STORAGE_GROUP_NOT_OBJECT": "自定义存储配置格式不正确，磁盘组应为对象",
    "STORAGE_NOT_ARRAY": "自定义存储配置格式不正确，应为数组",
    "SUPPORT_ACCOUNT_NOT_FOUND": "OVH 账户不存在",
    "SUPPORT_ACCOUNT_UNAVAILABLE": "OVH 账户无法使用，请检查账户凭据和区域设置",
    "SWAP_CANNOT_FILL": "swap 分区必须指定大小,不能留空占满磁盘(OVH 不允许)",
    "ZFS_HARDWARE_FETCH_FAILED": "无法获取服务器磁盘硬件规格，已中止 Proxmox ZFS 重装以防容量计算错误",
    "ZFS_RAID_LEVEL_UNSUPPORTED": "RAID 级别 {{level}} 不受支持，可选：0/1/5/6/7/10",
    "ZFS_SINGLE_DISK_RAID": "该服务器只有 {{count}} 块磁盘，无法使用 RAID{{level}}，请改用 RAID0",
    "ZFS_VZ_OVER_LIMIT": "/var/lib/vz 容量 {{vz}}MB 超出可用范围：本机 RAID{{level}} 下可用约 {{usable}}MB，扣除 /boot {{boot}}MB 和 swap {{swap}}MB 后，该值必须小于 {{max}}MB",
}

CURATED_EN: dict[str, str] = {
    "AUTH_RATE_LIMITED": "Too many failed API key attempts; requests from this origin are temporarily rate-limited. Please try again shortly.",
    "CHAT_IDENTITY_MISSING": "OVH returned no customer ID. No chat token was requested.",
    "CHAT_INVALID_PRODUCT": "Select dedicated server or VPS support.",
    "CHAT_REQUEST_FAILED": "Could not prepare chat authentication. Check GET /me and POST /auth/token access and the account status.",
    "CHAT_TOKEN_INVALID": "OVH returned no valid chat token. Retry or use the Help Centre.",
    "E1C4B0FD2": "This domain is not in the secondary DNS list; no deletion is needed.",
    "E2BA0443E": "No snapshot is currently available to roll back.",
    "E308E982D": "A reboot is already in progress on this VPS — no need to send another; just wait for it to finish",
    "E3E1F5824": "No snapshot exists; description cannot be updated.",
    "E4D9E7F86": "This domain is already in the secondary DNS list.",
    "E5B2D91A4": "Subscription not found.",
    "E704347D1": "A reboot is already in progress on this server — no need to send another; just wait for it to finish",
    "E7E8BB801": "Permanent mitigation is already enabled for this IP.",
    "E9E0B7F01": "Permanent mitigation is not enabled for this IP.",
    "EA0CDDD4E": "Switched back to disk boot, and a reboot is already in progress — that reboot will bring the normal system back in ~3-5 minutes",
    "EF4E0B03F": "No snapshot exists; no deletion is needed.",
    "EFD5D37BB": "Rolling back discards all data created after the snapshot. Add confirm:true to proceed.",
    "ENQUEUE_REJECTED": "Task was not enqueued: duplicate task, monitor paused/stopped, or queue limit reached.",
    "ERASE_FALSE_MISSING_GID": "Keeping data on a disk group requires an explicit diskGroupId (> 0).",
    "ERASE_FALSE_ON_INSTALL_GROUP": "Disk group {{gid}} carries both a storage configuration and keep-data — the install group must be erased; keeping data is only for non-install groups",
    "ERR_ACCOUNT_NOT_FOUND": "Account not found.",
    "ERR_DB_SAVE": "State updated in memory, but failed to persist to the database; changes may revert on restart.",
    "ERR_DELETE_ACCOUNT": "Failed to delete account.",
    "ERR_ENQUEUE_FAILED": "Failed to persist task to database; task has been rolled back.",
    "ERR_GET_ACCOUNT": "Failed to load account details.",
    "ERR_INVALID_INTERVAL": "Retry interval is outside the allowed range in seconds.",
    "ERR_INVALID_SUBSIDIARY": "Unknown OVH subsidiary code (ovhSubsidiary).",
    "ERR_LIST_ACCOUNTS": "Failed to list accounts.",
    "ERR_QUEUE_NOT_FOUND": "Queue task not found.",
    "ERR_SET_DEFAULT": "Failed to set default account.",
    "EVPS_MULTI_SSH_KEY": "VPS rebuild supports only a single registered OVH SSH key.",
    "EVPS_RAW_PUBKEY_NOT_ALLOWED": "VPS rebuild requires a registered OVH SSH key name (e.g. id_rsa), not a raw public key.",
    "E_BATCH_AUTO_ORDER_FORBIDDEN": "Batch-subscribing the full catalog cannot enable auto-ordering; configure auto-order per model to prevent accidental charges.",
    "E_INVALID_DOMAIN": "Invalid domain format.",
    "E_NO_RETRACTION_RIGHT": "This order is not eligible for withdrawal (business/non-EU account or waived right).",
    "E_RETRACTION_EXPIRED": "The 14-day withdrawal window for this order has expired.",
    "FS_UNSUPPORTED": "Unsupported filesystem {{fs}}",
    "HWRAID_BAD_SHAPE": "Malformed hardware RAID configuration",
    "HWRAID_LEVEL_UNSUPPORTED": "Hardware RAID level {{level}} is not supported; allowed: 0/1/5/6/10/50/60",
    "HWRAID_MISSING_LEVEL": "Hardware RAID is missing raidLevel",
    "INTERNAL_ERROR": "Internal server error.",
    "INVALID_SERVICE_NAME": "Invalid service name format.",
    "NO_OVH_ACCOUNT": "No OVH account is configured.",
    "OVH_AUTH_INVALID": "OVH API credentials for this account are invalid or expired.",
    "OVH_UPSTREAM_ERROR": "Upstream OVH API returned an error.",
    "PARTITION_BAD_SHAPE": "Malformed partition configuration",
    "PARTITION_FS_RAID_INCOMPAT": "Partition {{mp}}: {{reason}}",
    "PARTITION_MISSING_MOUNTPOINT": "Partition is missing a mount point",
    "PARTITION_MULTI_FILL": "At most one partition can fill the remaining space (size 0) — set explicit sizes for the others",
    "PARTITION_NEGATIVE_SIZE": "Partition {{mp}} size cannot be negative",
    "PARTITION_RAID_UNSUPPORTED": "Software RAID level {{level}} on partition {{mp}} is not supported; allowed: 0/1/5/6/7/10",
    "PERMANENT_MITIGATION_REMOVED": "OVH has retired permanent IP mitigation. Automatic DDoS protection requires no manual activation.",
    "PRESERVE_REQUIRES_INSTALL_GROUP": "Keeping data requires an explicit installation partition on another disk group.",
    "QUEUE_ALREADY_ORDERED": "This task has already been ordered and cannot be restarted. Create a new task to buy again.",
    "RENEWAL_INVALID_REQUEST": "Enter a valid renewal mode and period.",
    "RENEWAL_MODE_UNAVAILABLE": "OVH does not currently allow the selected renewal mode for this service. Refresh and retry.",
    "REQUEST_TOO_LARGE": "Request body is too large; please reduce the payload size.",
    "RESCUE_BOOT_CHANGED": "The boot configuration could not be confirmed against this window. No operation was performed.",
    "RESCUE_EMAIL_REQUIRED": "Enter a complete, valid email or explicitly select the account contact email.",
    "RESCUE_INVALID_REQUEST": "Invalid rescue request. No operation was performed.",
    "RETRACTION_ORDER_CHANGED": "Reopen the withdrawal window and confirm the order number before submitting.",
    "RETRACTION_SUBMITTED": "Order withdrawal was requested for every product and its data, including backups. Used service days may be deducted from the refund. Check the result in OVH.",
    "RETRACTION_UNAVAILABLE": "This order could not be confirmed as eligible for withdrawal. Nothing was submitted. Refresh and verify it in OVH.",
    "STORAGE_DUPLICATE_GROUP": "The same disk group cannot be configured twice or used for both installation and data preservation.",
    "STORAGE_EMPTY": "Custom storage configuration is empty",
    "STORAGE_GROUP_EMPTY": "A disk group in the custom storage configuration has neither partitions nor hardware RAID",
    "STORAGE_GROUP_NOT_OBJECT": "Invalid custom storage configuration: disk group must be an object",
    "STORAGE_NOT_ARRAY": "Invalid custom storage configuration: expected an array",
    "SUPPORT_ACCOUNT_NOT_FOUND": "OVH account not found.",
    "SUPPORT_ACCOUNT_UNAVAILABLE": "OVH account is unavailable; check credentials and region settings.",
    "SWAP_CANNOT_FILL": "The swap partition must have an explicit size; filling the disk with swap is not allowed by OVH",
    "ZFS_HARDWARE_FETCH_FAILED": "Failed to fetch server disk hardware specifications; Proxmox ZFS reinstallation was aborted to prevent capacity miscalculation.",
    "ZFS_RAID_LEVEL_UNSUPPORTED": "RAID level {{level}} is not supported; allowed: 0/1/5/6/7/10",
    "ZFS_SINGLE_DISK_RAID": "This server has only {{count}} disk(s); RAID{{level}} is impossible — use RAID0 instead",
    "ZFS_VZ_OVER_LIMIT": "/var/lib/vz size {{vz}}MB exceeds the limit: under RAID{{level}} this machine has about {{usable}}MB usable; after /boot {{boot}}MB and swap {{swap}}MB, the value must be below {{max}}MB",
}


def parse_ts_map(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}
    txt = path.read_text(encoding="utf-8")
    out: dict[str, str] = {}
    for m in re.finditer(r'^\s*"?([A-Z0-9_]+)"?\s*:\s*"((?:[^"\\]|\\.)*)"', txt, re.M):
        out[m.group(1)] = bytes(m.group(2), "utf-8").decode("unicode_escape") if "\\" in m.group(2) else m.group(2)
    return out


def scan_backend_codes() -> dict[str, str]:
    found: dict[str, str] = {}
    for p in sorted(BACKEND_DIR.rglob("*.go")):
        if p.name.endswith("_test.go"):
            continue
        txt = p.read_text(encoding="utf-8", errors="ignore")
        for m in re.finditer(r'"(?:error|message)"\s*:\s*"((?:[^"\\]|\\.)*)"[^{}\n]{0,200}?"code"\s*:\s*"([A-Z0-9_]+)"', txt, re.S):
            msg, code = m.group(1), m.group(2)
            if msg and not found.get(code):
                found[code] = msg
        for m in re.finditer(r'"code"\s*:\s*"([A-Z0-9_]+)"[^{}\n]{0,200}?"(?:error|message)"\s*:\s*"((?:[^"\\]|\\.)*)"', txt, re.S):
            code, msg = m.group(1), m.group(2)
            if msg and not found.get(code):
                found[code] = msg
        for m in re.finditer(r'httperr\.\w+\([^,\n]+,\s*"((?:[^"\\]|\\.)*)"\s*,\s*"([A-Z0-9_]+)"', txt):
            msg, code = m.group(1), m.group(2)
            if msg and not found.get(code):
                found[code] = msg
        for m in re.finditer(r'serr\(\s*"([A-Z0-9_]+)"\s*,[^,\n]+,\s*"((?:[^"\\]|\\.)*)"', txt):
            code, msg = m.group(1), m.group(2)
            if msg and not found.get(code):
                found[code] = msg
        for m in re.finditer(r'"code"\s*:\s*"([A-Z0-9_]+)"', txt):
            found.setdefault(m.group(1), "")
        for m in re.finditer(r'httperr\.\w+\([^,\n]+,[^,\n]+,\s*"([A-Z0-9_]+)"', txt):
            found.setdefault(m.group(1), "")
        for m in re.finditer(r'serr\(\s*"([A-Z0-9_]+)"', txt):
            found.setdefault(m.group(1), "")
        for m in re.finditer(r'Code\s*:\s*"([A-Z0-9_]+)"', txt):
            found.setdefault(m.group(1), "")
    return found


def main() -> None:
    existing_json: dict[str, str] = {}
    if JSON_OUT.exists():
        existing_json = json.loads(JSON_OUT.read_text(encoding="utf-8"))
    existing_zh = parse_ts_map(ZH_OUT)
    existing_en_translated = parse_ts_map(EN_TRANSLATED)
    existing_en_gen = parse_ts_map(EN_OUT)
    go_codes = scan_backend_codes()

    all_codes = sorted(
        set(existing_json)
        | set(existing_zh)
        | set(existing_en_translated)
        | set(existing_en_gen)
        | set(go_codes)
        | set(CURATED_ZH)
        | set(CURATED_EN)
    )

    zh_final: dict[str, str] = {}
    en_final: dict[str, str] = {}
    for code in all_codes:
        zh_msg = (
            existing_zh.get(code)
            or existing_json.get(code)
            or CURATED_ZH.get(code)
            or go_codes.get(code)
            or f"接口错误 ({code})"
        )
        en_msg = (
            existing_en_translated.get(code)
            or existing_en_gen.get(code)
            or CURATED_EN.get(code)
            or go_codes.get(code)
            or f"API error ({code})"
        )
        zh_final[code] = zh_msg
        en_final[code] = en_msg

    JSON_OUT.write_text(
        json.dumps(zh_final, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
        newline="\n",
    )

    zh_lines = [
        "// 由 tools/gen-api-codes.py 生成,勿手改;重跑脚本会覆盖",
        "export const apiZh = {",
    ]
    for code, msg in zh_final.items():
        zh_lines.append(f"  {json.dumps(code)}: {json.dumps(msg, ensure_ascii=False)},")
    zh_lines.append("} as const;")
    if existing_en_translated:
        compat_keys = [k for k in all_codes if k in existing_en_translated]
        union_expr = " | ".join(json.dumps(k) for k in compat_keys)
        zh_lines.append(f"export type ApiCode = {union_expr};")
    else:
        zh_lines.append("export type ApiCode = keyof typeof apiZh;")
    zh_lines.append("export type GeneratedApiCode = keyof typeof apiZh;")
    ZH_OUT.write_text("\n".join(zh_lines) + "\n", encoding="utf-8", newline="\n")

    en_lines = [
        "// Generated by tools/gen-api-codes.py — do not edit manually.",
        'import type { GeneratedApiCode } from "./api-zh.generated";',
        "",
        "export const apiEnGenerated: Record<GeneratedApiCode, string> = {",
    ]
    for code, msg in en_final.items():
        en_lines.append(f"  {json.dumps(code)}: {json.dumps(msg, ensure_ascii=False)},")
    en_lines.append("};")
    EN_OUT.write_text("\n".join(en_lines) + "\n", encoding="utf-8", newline="\n")

    print(
        f"Synchronized {len(all_codes)} API codes -> "
        f"{JSON_OUT.relative_to(ROOT)}, {ZH_OUT.relative_to(ROOT)}, {EN_OUT.relative_to(ROOT)}"
    )


if __name__ == "__main__":
    main()
