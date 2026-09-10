import { useCallback, useEffect, useState } from "react";

const KEY = "ovh_sniper_hide_ip";

/**
 * 隐私模式：是否打码 IP 与 MAC 地址。
 * 持久化在 localStorage，跨页面共享同一开关，刷新保留。
 */
export function useHideIp() {
  const [hidden, setHidden] = useState<boolean>(() => {
    if (typeof window === "undefined") return false;
    return window.localStorage.getItem(KEY) === "1";
  });

  const toggle = useCallback(() => {
    setHidden((prev) => {
      const next = !prev;
      window.localStorage.setItem(KEY, next ? "1" : "0");
      // 触发 storage 事件，让其它组件同步
      window.dispatchEvent(new Event("ovh-sniper-hide-ip"));
      return next;
    });
  }, []);

  // 跨组件同步
  useEffect(() => {
    const handler = () => setHidden(window.localStorage.getItem(KEY) === "1");
    window.addEventListener("ovh-sniper-hide-ip", handler);
    window.addEventListener("storage", handler);
    return () => {
      window.removeEventListener("ovh-sniper-hide-ip", handler);
      window.removeEventListener("storage", handler);
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

  // 4. OVH 反向 DNS 主机名：含 2~4 段 IP 数字（如 ns392029.ip-37-187-28.eu / ip-54-38-222-10.eu）
  if (/ip[-.]\d+/i.test(str)) {
    return str
      .replace(/ns\d+/gi, (m) => m.slice(0, 2) + "*".repeat(m.length - 2))
      .replace(/ip((?:[-.]\d{1,3}){2,4})/gi, (m) => m.replace(/\d+/g, "***"));
  }

  // 5. VPS 主机名 / 默认 serviceName：vps-123456.vps.ovh.net, vps-123456
  if (/^vps-[\w.-]+/i.test(str)) {
    return str.replace(/^vps-([a-zA-Z0-9]+)/i, (_, id) => "vps-" + "*".repeat(Math.min(id.length, 8)));
  }

  // 6. 独立服务器默认服务名：ns123456...
  if (/^ns\d+[\w.-]*/i.test(str)) {
    return str.replace(/^ns(\d+)/i, (_, digits) => "ns" + "*".repeat(digits.length));
  }

  // 7. 文本中内嵌的 IPv4 地址
  if (/\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b/.test(str)) {
    return str.replace(/\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b/g, (_m, p1) => `${p1}.***.***.***`);
  }

  return value;
}

