import { useState, useEffect, useCallback } from "react";
import { apiRequest } from "@/lib/http";

/**
 * 严格校验 /health 响应必须为 JSON 对象且包含合法健康状态字段，
 * 防止无后端/离线环境下 SPA Fallback 返回 index.html (200 text/html) 被误判为后端在线。
 */
export function isValidHealthPayload(
  payload: unknown,
  contentType?: string | null
): boolean {
  if (contentType != null && !contentType.toLowerCase().includes("application/json")) {
    return false;
  }
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    return false;
  }
  const status = (payload as Record<string, unknown>).status;
  return status === "ok" || status === "healthy";
}

export function useBackendConnection() {
  const [isConnected, setIsConnected] = useState(false);
  const [isChecking, setIsChecking] = useState(true);

  const checkConnection = useCallback(async () => {
    setIsChecking(true);
    try {
      // 1) 优先走配置的 /health（可能经 backendUrl），校验 JSON 结构防 SPA HTML 假阳性
      try {
        const data = await apiRequest<unknown>("/health", {}, { silent401: true });
        if (isValidHealthPayload(data)) {
          setIsConnected(true);
          return true;
        }
      } catch {
        /* fallthrough */
      }
      // 2) 回退：Vite 同源代理 /health，同时校验 Content-Type 与 JSON 结构
      const res = await fetch("/health", {
        method: "GET",
        headers: { Accept: "application/json" },
      });
      const contentType = res.headers.get("content-type") || "";
      if (res.ok && contentType.toLowerCase().includes("application/json")) {
        const body: unknown = await res.json();
        if (isValidHealthPayload(body, contentType)) {
          setIsConnected(true);
          return true;
        }
      }
      setIsConnected(false);
      return false;
    } catch {
      setIsConnected(false);
      return false;
    } finally {
      setIsChecking(false);
    }
  }, []);

  useEffect(() => {
    void checkConnection();
    const interval = setInterval(() => void checkConnection(), 30000);
    return () => clearInterval(interval);
  }, [checkConnection]);

  return { isConnected, isChecking, checkConnection };
}
