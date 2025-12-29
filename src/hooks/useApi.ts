import { useState, useEffect, useCallback } from 'react';
import api from '@/lib/api';

// 通用API Hook
export function useApiQuery<T>(
  queryFn: () => Promise<T>,
  deps: any[] = []
) {
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
  }, deps);

  useEffect(() => {
    refetch();
  }, [refetch]);

  return { data, isLoading, error, refetch };
}

// 统计数据Hook
export function useStats() {
  return useApiQuery(() => api.getStats(), []);
}

// 服务器列表Hook
export function useServers() {
  return useApiQuery(() => api.getServers(), []);
}

// 队列Hook
export function useQueue() {
  return useApiQuery(() => api.getQueue(), []);
}

// 购买历史Hook
export function usePurchaseHistory() {
  return useApiQuery(() => api.getPurchaseHistory(), []);
}

// 日志Hook
export function useLogs(limit?: number) {
  return useApiQuery(() => api.getLogs(limit), [limit]);
}

// 订阅列表Hook（专机监控）
export function useSubscriptions() {
  return useApiQuery(() => api.getSubscriptions(), []);
}

// 监控状态Hook
export function useMonitorStatus() {
  return useApiQuery(() => api.getMonitorStatus(), []);
}

// VPS订阅Hook
export function useVpsSubscriptions() {
  return useApiQuery(() => api.getVpsSubscriptions(), []);
}

// VPS监控状态Hook
export function useVpsMonitorStatus() {
  return useApiQuery(() => api.getVpsMonitorStatus(), []);
}

// 配置绑定狙击任务Hook
export function useConfigSniperTasks() {
  return useApiQuery(() => api.getConfigSniperTasks(), []);
}

// 已购服务器Hook
export function useMyServers() {
  return useApiQuery(() => api.getMyServers(), []);
}

// OVH账户信息Hook
export function useOvhAccount() {
  return useApiQuery(() => api.getOvhAccountInfo(), []);
}

// OVH余额Hook
export function useOvhBalance() {
  return useApiQuery(() => api.getOvhBalance(), []);
}

// OVH订单Hook
export function useOvhOrders(limit?: number) {
  return useApiQuery(() => api.getOvhOrders(limit), [limit]);
}

// OVH账单Hook
export function useOvhBills(limit?: number) {
  return useApiQuery(() => api.getOvhBills(limit), [limit]);
}

// OVH邮件历史Hook
export function useOvhEmails(limit?: number) {
  return useApiQuery(() => api.getOvhEmails(limit), [limit]);
}

// OVH退款记录Hook
export function useOvhRefunds(limit?: number) {
  return useApiQuery(() => api.getOvhRefunds(limit), [limit]);
}

// 联系人变更请求Hook
export function useContactChangeRequests() {
  return useApiQuery(() => api.getContactChangeRequests(), []);
}

// 后端连接状态Hook
export function useBackendConnection() {
  const [isConnected, setIsConnected] = useState(false);
  const [isChecking, setIsChecking] = useState(true);

  const checkConnection = useCallback(async () => {
    setIsChecking(true);
    try {
      await api.health();
      setIsConnected(true);
    } catch {
      setIsConnected(false);
    } finally {
      setIsChecking(false);
    }
  }, []);

  useEffect(() => {
    checkConnection();
    // 每30秒检查一次连接状态
    const interval = setInterval(checkConnection, 30000);
    return () => clearInterval(interval);
  }, [checkConnection]);

  return { isConnected, isChecking, checkConnection };
}
