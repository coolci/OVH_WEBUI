import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { toast } from "sonner";
import { useEffect, useState } from "react";

export interface LogEntry {
  id: string;
  timestamp: string;
  level: "INFO" | "WARNING" | "ERROR" | "DEBUG" | string;
  message: string;
  source: string;
}

export interface LogsQueryParams {
  /** 拉取条数，默认 200，最大 500 */
  limit?: number;
  level?: string;
  source?: string;
  /** desc=最新在前（默认） */
  order?: "asc" | "desc";
  /** 是否自动轮询 */
  autoRefresh?: boolean;
  /** 轮询间隔 ms，默认 12000 */
  refreshIntervalMs?: number;
  enabled?: boolean;
}

export interface LogsResult {
  logs: LogEntry[];
  total: number;
  returned: number;
  truncated: boolean;
}

function parseLogsResponse(data: unknown): LogsResult {
  if (Array.isArray(data)) {
    return {
      logs: data as LogEntry[],
      total: data.length,
      returned: data.length,
      truncated: false,
    };
  }
  if (data && typeof data === "object") {
    const o = data as Record<string, unknown>;
    const logs = (Array.isArray(o.logs) ? o.logs : []) as LogEntry[];
    const total = typeof o.total === "number" ? o.total : logs.length;
    const returned = typeof o.returned === "number" ? o.returned : logs.length;
    const truncated = Boolean(o.truncated ?? total > returned);
    return { logs, total, returned, truncated };
  }
  return { logs: [], total: 0, returned: 0, truncated: false };
}

/** 页面是否可见（隐藏标签页时停轮询，避免后台过载） */
function useDocumentVisible() {
  const [visible, setVisible] = useState(
    () => typeof document === "undefined" || document.visibilityState === "visible"
  );
  useEffect(() => {
    const onVis = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  }, []);
  return visible;
}

/**
 * 日志列表（限量 + 可选过滤 + 智能轮询）
 * - 默认 limit=200，避免一次塞满 1000 条
 * - 标签页不可见时停止 autoRefresh
 */
export function useLogs(opts: LogsQueryParams | boolean = {}) {
  // 兼容旧签名 useLogs(true) / useLogs(false)
  const params: LogsQueryParams =
    typeof opts === "boolean" ? { autoRefresh: opts } : opts || {};

  const limit = params.limit ?? 200;
  const level = params.level && params.level !== "all" ? params.level : undefined;
  const source = params.source?.trim() || undefined;
  const order = params.order ?? "desc";
  const autoRefresh = params.autoRefresh ?? false;
  const interval = params.refreshIntervalMs ?? 12_000;
  const enabled = params.enabled !== false;
  const visible = useDocumentVisible();

  return useQuery({
    queryKey: qk.logs.list({ limit, level: level || "", source: source || "", order }),
    queryFn: async (): Promise<LogsResult> => {
      const res = await api.get("/logs", {
        params: {
          limit,
          order,
          ...(level ? { level } : {}),
          ...(source ? { source } : {}),
        },
      });
      return parseLogsResponse(res.data);
    },
    enabled,
    // 自动刷新：仅在开启且页面可见时
    refetchInterval: autoRefresh && visible ? interval : false,
    // 列表稍短 stale，手动刷新更灵敏；轮询时不重复打爆
    staleTime: autoRefresh ? interval / 2 : 15_000,
    placeholderData: (prev) => prev,
  });
}

/** 轻量预览：仪表盘 / 顶栏用，固定小 limit */
export function useRecentLogs(limit = 15, autoRefresh = true) {
  return useLogs({
    limit,
    order: "desc",
    autoRefresh,
    refreshIntervalMs: 15_000,
  });
}

/** 清空日志 */
export function useClearLogs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => (await api.delete("/logs")).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["logs"] });
      toast.success("已清空日志");
    },
    onError: (e: any) =>
      toast.error(e.response?.data?.message || e.response?.data?.error || "清空失败"),
  });
}
