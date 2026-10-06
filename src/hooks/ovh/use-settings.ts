import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { errorMessage } from "@/components/common/LoadFailed";
import { toast } from "sonner";
import i18n from "@/i18n";
import { apiMessage } from "@/lib/api-error";

export interface SettingsConfig {
  appKey?: string;
  appSecret?: string;
  consumerKey?: string;
  endpoint?: string;
  zone?: string;
  iam?: string;
  tgToken?: string;
  tgChatId?: string;
  /** Telegram 回调地址：Telegram 把用户点按钮的动作推到这里（进） */
  /** 自定义通知地址：补货/下单结果由本程序 POST 到这里（出）。和上面那个方向相反 */
  notifyWebhookUrl?: string;
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

/** 读取后端 config */
export function useSettings() {
  return useQuery({
    queryKey: qk.settings.config(),
    queryFn: async () => (await api.get<SettingsConfig>("/settings")).data,
  });
}

/** 保存 config */
export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: SettingsConfig) => (await api.post("/settings", payload)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.settings.config() });
      qc.invalidateQueries({ queryKey: ["telegram", "verify"] });
      qc.invalidateQueries({ queryKey: qk.settings.telegramPoller() });
      qc.invalidateQueries({ queryKey: ["telegram"] });
      toast.success(i18n.t("hooksMsg.settings.saved"));
    },
    onError: (e: any) => toast.error(apiMessage(e) || i18n.t("hooksMsg.settings.saveFailed")),
  });
}

/** 缓存信息 */
export function useCacheInfo() {
  return useQuery({
    queryKey: qk.settings.cacheInfo(),
    queryFn: async () => (await api.get("/cache/info")).data,
  });
}

export function useClearCache() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (type: "all" | "memory" | "sqlite") =>
      (await api.post("/cache/clear", { type })).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.settings.cacheInfo() });
      toast.success(i18n.t("hooksMsg.settings.cacheCleared"));
    },
    onError: (e: any) => toast.error(apiMessage(e) || i18n.t("hooksMsg.settings.cacheClearFailed")),
  });
}

/**
 * 长轮询收取器的运行快照。
 */
export interface TelegramPollerStatus {
  running?: boolean;
  configured?: boolean;
  botUsername?: string;
  offset?: number;
  lastError?: string;
  lastPollAt?: string;
  lastUpdateAt?: string;
}

export interface TelegramPollerInfo extends TelegramPollerStatus {
  hasToken: boolean;
  success?: boolean;
  poller?: TelegramPollerStatus;
  polling?: TelegramPollerStatus;
}

/**
 * 长轮询的运行状态。
 */
export function useTelegramPoller(enabled = true) {
  return useQuery({
    queryKey: qk.settings.telegramPoller(),
    queryFn: async (): Promise<TelegramPollerInfo> => {
      const res = await api.get<any>("/telegram/poller");
      const data = res.data || {};
      const snap: TelegramPollerStatus = data.polling || data.poller || {};
      const running = data.running ?? snap.running ?? false;
      const configured = data.configured ?? snap.configured ?? false;
      const botUsername = data.botUsername || snap.botUsername || "";
      const hasToken = data.hasToken ?? (configured || !!botUsername);
      const lastError = data.lastError || snap.lastError || "";
      const offset = data.offset ?? snap.offset ?? 0;
      const lastUpdateAt = data.lastUpdateAt || snap.lastUpdateAt || snap.lastPollAt;

      const pollerObj: TelegramPollerStatus = {
        running,
        configured,
        botUsername,
        offset,
        lastError,
        lastUpdateAt,
        lastPollAt: lastUpdateAt,
      };

      return {
        success: data.success ?? true,
        hasToken,
        running,
        configured,
        botUsername,
        offset,
        lastError,
        lastUpdateAt,
        lastPollAt: lastUpdateAt,
        poller: pollerObj,
        polling: pollerObj,
      };
    },
    enabled,
    refetchInterval: 8000,
  });
}

export const useTelegramPollerStatus = useTelegramPoller;
