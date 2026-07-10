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
  // IPv4：保留首段
  const ipv4 = value.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
  if (ipv4) return `${ipv4[1]}.***.***.***`;
  // MAC：保留前 6 位（厂商段）
  const mac = value.match(/^([0-9a-fA-F]{2}[:\-]){5}[0-9a-fA-F]{2}$/);
  if (mac) return value.slice(0, 8) + ":**:**:**:**";
  // OVH 反向 DNS 主机名：ns123.ip-54-38-222.eu / 8.ip-54-38-222.eu / ip-54-38-222.eu
  // 同时把 dash 形式和 dot 形式的四段 IP 都打码
  if (/ip[-.]\d{1,3}[-.]\d{1,3}[-.]\d{1,3}[-.]\d{1,3}/i.test(value)) {
    return value
      .replace(/ip-\d{1,3}-\d{1,3}-\d{1,3}-\d{1,3}/gi, "ip-***-***-***-***")
      .replace(/ip\.\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/gi, "ip.***.***.***.***");
  }
  // 其它：星号占满
  return "*".repeat(Math.min(value.length, 16));
}
