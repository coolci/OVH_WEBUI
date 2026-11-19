import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** 校验外部跳转链接必须为 https: 协议，拦截 javascript:/data:/http: 等不安全协议 */
export function safeHttpsUrl(raw?: string | null): string | undefined {
  if (!raw || typeof raw !== "string") return undefined;
  try {
    const u = new URL(raw.trim());
    return u.protocol === "https:" ? u.toString() : undefined;
  } catch {
    return undefined;
  }
}

export function openSafeHttpsUrl(raw?: string | null): boolean {
  const safe = safeHttpsUrl(raw);
  if (!safe || typeof window === "undefined") return false;
  window.open(safe, "_blank", "noopener,noreferrer");
  return true;
}

