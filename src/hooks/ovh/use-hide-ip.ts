import { useCallback, useEffect, useState } from "react";

const KEY = "ovh_sniper_hide_ip";
const EVT = "ovh-sniper-hide-ip";

/** 存储被禁用(隐私模式等)时读写会抛异常:读不到当作关闭,写不进去只影响刷新后的保留 */
function readHidden(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(KEY) === "1";
  } catch {
    return false;
  }
}

/** 存不进 localStorage 时,同页其它组件靠这个内存值同步 */
let memoryHidden: boolean | null = null;

/**
 * 隐私模式：是否打码 IP 与 MAC 地址。
 * 持久化在 localStorage，跨页面共享同一开关，刷新保留。
 */
export function useHideIp() {
  const [hidden, setHidden] = useState<boolean>(() => memoryHidden ?? readHidden());

  // 副作用不能放在 setState 的更新函数里:更新函数必须是纯函数,
  // StrictMode 下会被调用两次(写两次存储、广播两次事件)。
  const toggle = useCallback(() => {
    const next = !(memoryHidden ?? readHidden());
    memoryHidden = next;
    try {
      window.localStorage.setItem(KEY, next ? "1" : "0");
    } catch {
      /* 存不进去:本次会话内仍然生效 */
    }
    setHidden(next);
    // 让同页其它组件同步
    window.dispatchEvent(new Event(EVT));
  }, []);

  // 跨组件 / 跨标签页同步
  useEffect(() => {
    const sync = () => setHidden(memoryHidden ?? readHidden());
    const onStorage = (e: StorageEvent) => {
      if (e.key !== null && e.key !== KEY) return;
      memoryHidden = null;
      sync();
    };
    window.addEventListener(EVT, sync);
    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener(EVT, sync);
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  return { hidden, toggle };
}

/** 把 IP / MAC / 反向 DNS 主机名等敏感字符串打码（保留长度感）。开关关闭时原样返回。 */
export function maskSensitive(value: string, hidden: boolean): string {
  if (!hidden || !value) return value;
  if (typeof value !== "string") value = String(value);
  const str = value.trim();

  // 1. IPv4 单 IP、带 CIDR 或端口：1.2.3.4, 1.2.3.4/24, 1.2.3.4:8080
  const ipv4Match = str.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})((\/\d+)?(:\d+)?)$/);
  if (ipv4Match) {
    return `${ipv4Match[1]}.***.***.***${ipv4Match[5] || ""}`;
  }

  // 2. MAC 地址：00:1a:2b:3c:4d:5e
  const macMatch = str.match(/^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$/);
  if (macMatch) {
    return str.slice(0, 8) + ":**:**:**";
  }

  // 3. IPv6 地址：2001:41d0:1:2345::1 或 2001:41d0:1:2345::/64
  if (str.includes(":") && /^[0-9a-fA-F:]+(\/\d+)?$/.test(str)) {
    const parts = str.split("/");
    const segs = parts[0].split(":");
    const masked = segs.slice(0, 2).join(":") + ":****:****::*" + (parts[1] ? "/" + parts[1] : "");
    return masked;
  }

  // 4~7 是"文本里夹带"的形式,可能同时出现好几种(如 "ns123.ip-1-2-3.eu (1.2.3.4)"),
  // 必须依次全部打码,不能命中第一种就返回 —— 否则括号里的真实 IP 原样露出来。
  let out = str;
  let changed = false;
  const apply = (re: RegExp, fn: (m: string, ...g: string[]) => string) => {
    if (re.test(out)) {
      re.lastIndex = 0;
      out = out.replace(re, fn);
      changed = true;
    }
  };

  // 4. OVH 反向 DNS 主机名：含 2~4 段 IP 数字（如 ns392029.ip-37-187-28.eu / ip-54-38-222-10.eu）
  if (/ip[-.]\d+/i.test(out)) {
    apply(/ns\d+/gi, (m) => m.slice(0, 2) + "*".repeat(m.length - 2));
    apply(/ip((?:[-.]\d{1,3}){2,4})/gi, (m) => m.replace(/\d+/g, "***"));
  }

  // 5. VPS 主机名 / 默认 serviceName：vps-123456.vps.ovh.net, vps-123456
  apply(/^vps-([a-zA-Z0-9]+)/i, (_m, id) => "vps-" + "*".repeat(Math.min(id.length, 8)));

  // 6. 独立服务器默认服务名：ns123456...
  apply(/^ns(\d+)/i, (_m, digits) => "ns" + "*".repeat(digits.length));

  // 7. 文本中内嵌的 IPv4 地址
  apply(/\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b/g, (_m, p1) => `${p1}.***.***.***`);

  // 8. 文本中内嵌的 IPv6 地址(至少含一个 "::" 或 7 个冒号的十六进制段)
  apply(/\b[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{0,4}){2,7}\b/g, (m) =>
    m.includes("::") || (m.match(/:/g) || []).length >= 7 ? m.split(":").slice(0, 2).join(":") + ":****:****::*" : m
  );

  return changed ? out : value;
}

