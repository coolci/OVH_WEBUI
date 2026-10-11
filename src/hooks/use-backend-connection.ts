import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";

export function useBackendConnection() {
  const [isConnected, setIsConnected] = useState(false);
  const [isChecking, setIsChecking] = useState(true);

  const checkConnection = useCallback(async () => {
    setIsChecking(true);
    try {
      // 1) 优先走配置的 api.health（可能经 backendUrl）
      try {
        await api.health();
        setIsConnected(true);
        return true;
      } catch {
        /* fallthrough */
      }
      // 2) 回退：Vite 同源代理 /health（避免 backendUrl 填错导致误报离线）
      const res = await fetch("/health", { method: "GET" });
      if (res.ok) {
        setIsConnected(true);
        return true;
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
