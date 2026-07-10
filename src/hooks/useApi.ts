import { useState, useEffect, useCallback } from "react";
import api from "@/lib/api";
import type {
  Stats,
  QueueItem,
  LogEntry,
  HistoryEntry,
  MonitorStatus,
  Subscription,
  VpsSubscription,
  ServerPlan,
  ManagedServer,
  ContactChangeRequest,
  AccountInfo,
} from "@/lib/types";

// 通用 API Hook
export function useApiQuery<T>(queryFn: () => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const refetch = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await queryFn();
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setIsLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    void refetch();
  }, [refetch]);

  return { data, isLoading, error, refetch };
}

export function useStats() {
  return useApiQuery<Stats>(() => api.getStats(), []);
}

export function useServers() {
  return useApiQuery<ServerPlan[]>(() => api.getServers(), []);
}

export function useQueue() {
  return useApiQuery<QueueItem[]>(() => api.getQueue(), []);
}

export function usePurchaseHistory() {
  return useApiQuery<HistoryEntry[]>(() => api.getPurchaseHistory(), []);
}

export function useLogs(limit?: number) {
  return useApiQuery<LogEntry[]>(() => api.getLogs(limit), [limit]);
}

export function useSubscriptions() {
  return useApiQuery<Subscription[]>(() => api.getSubscriptions(), []);
}

export function useMonitorStatus() {
  return useApiQuery<MonitorStatus>(() => api.getMonitorStatus(), []);
}

export function useManualCheckDedicated() {
  const [data, setData] = useState<unknown>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const check = useCallback(async (planCode: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await api.manualCheckDedicated(planCode);
      setData(result);
      return result;
    } catch (err) {
      const e = err instanceof Error ? err : new Error(String(err));
      setError(e);
      throw e;
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { data, isLoading, error, check };
}

export function useVpsSubscriptions() {
  return useApiQuery<VpsSubscription[]>(() => api.getVpsSubscriptions(), []);
}

export function useVpsMonitorStatus() {
  return useApiQuery<MonitorStatus>(() => api.getVpsMonitorStatus(), []);
}

export function useMyServers() {
  return useApiQuery<{ success: boolean; servers: ManagedServer[]; total: number }>(
    () => api.getMyServers(),
    []
  );
}

export function useOvhAccount() {
  return useApiQuery<{ success: boolean; account?: AccountInfo; error?: string }>(
    () => api.getOvhAccountInfo(),
    []
  );
}

export function useOvhBalance() {
  return useApiQuery<{ success: boolean; balance?: { value: number; currencyCode: string }; data?: unknown }>(
    () => api.getOvhBalance() as Promise<{ success: boolean; balance?: { value: number; currencyCode: string }; data?: unknown }>,
    []
  );
}

export function useOvhOrders(limit?: number) {
  return useApiQuery<{ success: boolean; orders: unknown[] }>(
    () => api.getOvhOrders(limit),
    [limit]
  );
}

export function useOvhBills(limit?: number) {
  return useApiQuery<{ success: boolean; bills: unknown[] }>(
    () => api.getOvhBills(limit),
    [limit]
  );
}

export function useOvhEmails(limit?: number) {
  return useApiQuery<{ success: boolean; emails: unknown[] }>(
    () => api.getOvhEmails(limit),
    [limit]
  );
}

export function useOvhRefunds(limit?: number) {
  return useApiQuery<{ success: boolean; refunds: unknown[] }>(
    () => api.getOvhRefunds(limit),
    [limit]
  );
}

export function useContactChangeRequests() {
  return useApiQuery<{ success: boolean; requests: ContactChangeRequest[] }>(
    () => api.getContactChangeRequests(),
    []
  );
}

export function useOvhCreditBalance() {
  return useApiQuery<{ success: boolean; data?: unknown[] }>(
    () => api.getOvhCreditBalance(),
    []
  );
}

export function useOvhSubAccounts() {
  return useApiQuery<{ success: boolean; data?: unknown[] }>(
    () => api.getOvhSubAccounts(),
    []
  );
}

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
