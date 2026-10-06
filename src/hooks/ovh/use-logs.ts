import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { toast } from "sonner";
import { useEffect, useState } from "react";
import i18n from "@/i18n";
import { apiMessage } from "@/lib/api-error";

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

function parseLogsResponse(data: unknown, params: LogsQueryParams): LogsResult {
  let list: LogEntry[] = [];
  let total = 0;
  let truncated = false;

  if (Array.isArray(data)) {
    list = [...data];
    total = list.length;
  } else if (data && typeof data === "object") {
    const o = data as Record<string, unknown>;
    list = (Array.isArray(o.logs) ? [...o.logs] : []) as LogEntry[];
    total = typeof o.total === "number" ? o.total : list.length;
    truncated = Boolean(o.truncated ?? total > list.length);
  }

  // 客户端过滤与排序（最新日志排在前面）
  if (params.level && params.level !== "all") {
    const lvl = params.level.toUpperCase();
    list = list.filter((l) => (l.level || "").toUpperCase() === lvl);
  }
  if (params.source) {
    const src = params.source.toLowerCase();
    list = list.filter((l) => (l.source || "").toLowerCase().includes(src));
  }
  if (params.order === "desc") {
    list.reverse();
  }
  if (params.limit && params.limit > 0 && list.length > params.limit) {
    truncated = true;
    list = list.slice(0, params.limit);
  }

  return {
    logs: list,
    total,
    returned: list.length,
    truncated,
  };
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
 * - 默认 limit=200，最新日志在前
 * - 兼容对象参数与布尔值参数
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
    queryKey: ["logs", "list", { limit, level: level || "", source: source || "", order }],
    queryFn: async (): Promise<LogsResult> => {
      const res = await api.get("/logs", {
        params: {
          limit,
          order,
          ...(level ? { level } : {}),
          ...(source ? { source } : {}),
        },
      });
      return parseLogsResponse(res.data, { limit, level, source, order });
    },
    enabled,
    refetchInterval: autoRefresh && visible ? interval : false,
    staleTime: autoRefresh ? interval / 2 : 15_000,
    placeholderData: (prev) => prev,
  });
}

/** 轻量预览：仪表盘 / 顶栏用，固定小 limit（最新日志在前） */
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
      toast.success(i18n.t("hooksMsg.logs.cleared"));
    },
    onError: (e: any) => toast.error(apiMessage(e)),
  });
}
