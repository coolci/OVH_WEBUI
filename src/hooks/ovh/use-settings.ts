import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { toast } from "sonner";

export interface SettingsConfig {
  appKey?: string;
  appSecret?: string;
  consumerKey?: string;
  endpoint?: string;
  zone?: string;
  iam?: string;
  tgToken?: string;
  tgChatId?: string;
  /** 自定义通知地址：补货/下单结果由本程序 POST 到这里（出站 HTTP，不是 Telegram 入站） */
  notifyWebhookUrl?: string;
  /** 全局自动扣款总开关：默认关闭，只有开启后且任务指定 autoPay 时才会自动扣款 */
  autoPayEnabled?: boolean;
  /** 新建抢购任务的默认重试间隔（秒）。网页弹窗 / TG /buy / 一键下单按钮都用它 */
  defaultRetryInterval?: number;
  /** 监控触发的自动下单用的重试间隔（秒）。货刚出现那一刻窗口很窄，默认比普通任务激进 */
  quickOrderRetryInterval?: number;
}

/** 重试间隔的合法区间与默认值，与后端 types.ClampRetryInterval 一致 */
export const RETRY_INTERVAL = {
  min: 1,
  max: 86400,
  defaultTask: 60,
  defaultQuick: 2,
} as const;

export interface TelegramPollerStatus {
  running?: boolean;
  configured?: boolean;
  botUsername?: string;
  lastError?: string;
  lastUpdateAt?: string;
  offset?: number;
}

/** 读取后端 config */
export function useSettings() {
  return useQuery({
    queryKey: qk.settings.config(),
    queryFn: async () => (await api.get<SettingsConfig>("/settings")).data,
  });
}

/** 保存 config（仅本地后端配置：Token/ChatID 等；不含 Telegram setWebhook） */
export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: SettingsConfig) => {
      return (await api.post("/settings", payload)).data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.settings.config() });
      qc.invalidateQueries({ queryKey: ["telegram", "verify"] });
      qc.invalidateQueries({ queryKey: qk.settings.telegramPoller() });
      toast.success("设置已保存");
    },
    onError: (e: any) =>
      toast.error(e.response?.data?.message || e.response?.data?.error || "保存失败"),
  });
}

/** Telegram 轮询入站状态（走已有 /telegram/verify，避免独立 status 接口 404） */
export function useTelegramPollerStatus(enabled = true) {
  return useQuery({
    queryKey: qk.settings.telegramPoller(),
    queryFn: async (): Promise<TelegramPollerStatus> => {
      try {
        const res = await api.get<{
          ok?: boolean;
          reason?: string;
          polling?: TelegramPollerStatus;
        }>("/telegram/verify");
        const p = { ...(res.data?.polling || {}) };
        if (p.running == null && res.data?.ok) {
          p.running = true;
          p.configured = true;
        }
        if (p.configured == null) p.configured = !!res.data?.ok;
        if (!p.running && res.data?.ok === false && res.data.reason) {
          p.lastError = res.data.reason;
        }
        return p;
      } catch {
        return { configured: false, lastError: "暂时无法读取（请确认后端已启动）" };
      }
    },
    enabled,
    refetchInterval: 8000,
    retry: false,
  });
}

/** 缓存信息 */
export function useCacheInfo() {
  return useQuery({
    queryKey: qk.settings.cacheInfo(),
    queryFn: async () => (await api.get("/cache/info")).data,
  });
}



/** 清除缓存 */
export function useClearCache() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (type: "all" | "memory" | "sqlite") =>
      (await api.post("/cache/clear", { type })).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.settings.cacheInfo() });
      toast.success("已清除缓存");
    },
    onError: (e: any) => toast.error(e.response?.data?.error || "清除失败"),
  });
}
