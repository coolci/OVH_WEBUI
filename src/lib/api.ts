/**
 * OVH WebUI API 服务层 — 业务 facade（底层统一走 `@/lib/http` axios）
 *
 * - hooks 主路径：`import { api } from "@/lib/http"`（Axios 实例）
 * - 旧页面 / useApi：`import { api } from "@/lib/api"`（本对象，方法名语义化）
 */
import {
  apiRequest,
  API_KEY_STORAGE,
  BACKEND_URL_STORAGE,
  SERVER_CONTROL_ACCOUNT_KEY,
  getApiSecretKey,
  setApiSecretKey,
  clearApiSecretKey,
  getBackendUrl,
  setBackendUrl,
  getActiveServerControlAccount,
  setActiveServerControlAccount,
  ApiError,
} from "./http";
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
  OVHAccount,
  ContactChangeRequest,
  AccountInfo,
} from "./types";

export type * from "./types";

// 存储 / 传输 helpers（兼容 `import { getApiSecretKey } from "@/lib/api"`）
export {
  API_KEY_STORAGE,
  BACKEND_URL_STORAGE,
  SERVER_CONTROL_ACCOUNT_KEY,
  getApiSecretKey,
  setApiSecretKey,
  clearApiSecretKey,
  getBackendUrl,
  setBackendUrl,
  getActiveServerControlAccount,
  setActiveServerControlAccount,
  apiRequest,
  ApiError,
};

/** 把后端可能返回的数组 / 包一层 / null 统一成数组 */
function asArray<T>(raw: unknown, keys: string[] = []): T[] {
  if (Array.isArray(raw)) return raw as T[];
  if (raw && typeof raw === "object") {
    const obj = raw as Record<string, unknown>;
    // 常见包装：data / items / list / 业务 key
    const tryKeys = [...keys, "data", "items", "list", "results"];
    for (const k of tryKeys) {
      if (Array.isArray(obj[k])) return obj[k] as T[];
    }
  }
  return [];
}

/** Go 端常用 { status: "success"|"error", data?, message? } 或 { success: bool } */
function isOkPayload(raw: unknown): boolean {
  if (!raw || typeof raw !== "object") return false;
  const o = raw as Record<string, unknown>;
  if (o.success === true) return true;
  if (o.success === false) return false;
  if (o.status === "success" || o.status === "ok") return true;
  if (o.status === "error") return false;
  return true; // 无明确失败字段时视为成功
}

function normalizeActionResult(raw: unknown): {
  success: boolean;
  message?: string;
  error?: string;
  [k: string]: unknown;
} {
  if (!raw || typeof raw !== "object") {
    return { success: !!raw };
  }
  const o = raw as Record<string, unknown>;
  const success = isOkPayload(raw);
  return {
    ...o,
    success,
    message: (o.message as string) || (o.msg as string) || undefined,
    error: success
      ? undefined
      : (o.error as string) || (o.message as string) || "操作失败",
  };
}

type Settings = {
  appKey: string;
  appSecret: string;
  consumerKey: string;
  endpoint: string;
  tgToken: string;
  tgChatId: string;
  iam: string;
  zone: string;
};

async function getSettings(): Promise<Settings> {
  return apiRequest<Settings>("/api/settings");
}

async function saveSettings(config: Settings): Promise<{ status: string }> {
  return apiRequest<{ status: string }>("/api/settings", {
    method: "POST",
    body: JSON.stringify(config),
  });
}

async function getOvhAccountInfo(): Promise<{
  success: boolean;
  account?: AccountInfo;
  error?: string;
}> {
  const raw = await apiRequest<Record<string, unknown>>("/api/ovh/account/info");
  if (raw && typeof raw === "object" && "success" in raw) {
    return raw as { success: boolean; account?: AccountInfo; error?: string };
  }
  return { success: true, account: raw as AccountInfo };
}

async function wrapList(
  path: string,
  key: string
): Promise<{ success: boolean; [k: string]: unknown }> {
  try {
    const raw = await apiRequest<unknown>(path);
    const list = asArray(raw, [key, "data"]);
    return { success: true, [key]: list, data: list };
  } catch (e) {
    return { success: false, [key]: [], data: [], error: e instanceof Error ? e.message : String(e) };
  }
}

export const api = {
  health: () => apiRequest<{ status: string; time?: string }>("/health", {}, { silent401: true }),

  getStats: () => apiRequest<Stats>("/api/stats"),

  getSystemMetrics: () => apiRequest<Record<string, unknown>>("/api/system/metrics"),
  getVersion: () => apiRequest<{ version?: string; [k: string]: unknown }>("/api/version"),

  getSettings,
  /** @deprecated 使用 getSettings */
  getConfig: getSettings,

  saveSettings,
  /** @deprecated 使用 saveSettings */
  saveConfig: saveSettings,

  verifyAuth: () =>
    apiRequest<{ valid: boolean }>("/api/verify-auth", { method: "POST", body: "{}" }),

  // ==================== 多账户 ====================
  listAccounts: async (): Promise<{ accounts: OVHAccount[]; total: number }> => {
    const raw = await apiRequest<unknown>("/api/accounts");
    const accounts = asArray<OVHAccount>(raw, ["accounts"]);
    return { accounts, total: accounts.length };
  },
  createAccount: (body: Record<string, unknown>) =>
    apiRequest<{ account?: OVHAccount; valid?: boolean }>("/api/accounts", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateAccount: (id: string, body: Record<string, unknown>) =>
    apiRequest(`/api/accounts/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteAccount: (id: string) => apiRequest(`/api/accounts/${id}`, { method: "DELETE" }),
  setDefaultAccount: (id: string) =>
    apiRequest(`/api/accounts/${id}/set-default`, { method: "POST" }),
  verifyAccount: (id: string) =>
    apiRequest(`/api/accounts/${id}/verify`, { method: "POST" }),

  // ==================== 服务器列表 ====================
  getServers: async (): Promise<ServerPlan[]> => {
    // Go: 必须 showApiServers=true 才会调 OVH 拉列表（否则只读空缓存）
    const raw = await apiRequest<unknown>("/api/servers?showApiServers=true");
    return asArray<ServerPlan>(raw, ["servers", "data"]);
  },
  /** Go 后端为懒加载缓存；force 时 forceRefresh=true 重拉 */
  refreshServers: async (force?: boolean) => {
    const q = force
      ? "/api/servers?showApiServers=true&forceRefresh=true"
      : "/api/servers?showApiServers=true";
    const raw = await apiRequest<unknown>(q);
    const servers = asArray<ServerPlan>(raw, ["servers", "data"]);
    return { status: "ok", count: servers.length, servers };
  },
  getCatalog: () => apiRequest<Record<string, unknown>>("/api/catalog"),
  getAvailability: (planCode: string) =>
    apiRequest(`/api/availability/${encodeURIComponent(planCode)}`),

  // ==================== 队列 ====================
  getQueue: async (): Promise<QueueItem[]> => {
    const raw = await apiRequest<unknown>("/api/queue");
    return asArray<QueueItem>(raw, ["queue", "items"]);
  },
  addQueueItem: (item: {
    planCode: string;
    datacenter: string;
    options?: string[];
    retryInterval?: number;
  }) =>
    apiRequest<{ status?: string; id?: string }>("/api/queue", {
      method: "POST",
      body: JSON.stringify(item),
    }),
  removeQueueItem: (id: string) => apiRequest(`/api/queue/${id}`, { method: "DELETE" }),
  clearQueue: () =>
    apiRequest<Record<string, any>>("/api/queue/clear", { method: "DELETE" }),
  updateQueueStatus: (id: string, status: string) =>
    apiRequest(`/api/queue/${id}/status`, {
      method: "PUT",
      body: JSON.stringify({ status }),
    }),
  quickOrder: (order: {
    planCode: string;
    datacenter: string;
    options?: string[];
    account_id: string;
  }) =>
    apiRequest<any>("/api/queue/quick-order", {
      method: "POST",
      body: JSON.stringify(order),
    }),
  // Go 队列处理器启动时即运行；保留兼容方法供旧 UI 调用
  getQueueProcessorStatus: async () => {
    try {
      const s = await apiRequest<Stats>("/api/stats");
      return {
        running: !!s.queueProcessorRunning,
        processedCount: 0,
        lastProcessTime: null as number | null,
      };
    } catch {
      return { running: true, processedCount: 0, lastProcessTime: null as number | null };
    }
  },
  startQueueProcessor: async () => ({
    status: "ok",
    message: "队列处理器由后端守护，无需手动启动",
  }),
  stopQueueProcessor: async () => ({
    status: "ok",
    message: "队列处理器为守护线程，停止请重启后端进程",
  }),

  // ==================== 历史 / 日志 ====================
  getPurchaseHistory: async (): Promise<HistoryEntry[]> => {
    const raw = await apiRequest<unknown>("/api/purchase-history");
    return asArray<HistoryEntry>(raw, ["history", "items"]);
  },
  clearPurchaseHistory: () => apiRequest("/api/purchase-history", { method: "DELETE" }),
  getLogs: async (
    limit?: number,
    extra?: { level?: string; source?: string; order?: "asc" | "desc" }
  ): Promise<LogEntry[]> => {
    const qs = new URLSearchParams();
    // 默认限量，避免无参拉满 1000 条
    qs.set("limit", String(limit && limit > 0 ? Math.min(limit, 500) : 200));
    qs.set("order", extra?.order || "desc");
    if (extra?.level) qs.set("level", extra.level);
    if (extra?.source) qs.set("source", extra.source);
    const raw = await apiRequest<unknown>(`/api/logs?${qs.toString()}`);
    return asArray<LogEntry>(raw, ["logs"]);
  },
  clearLogs: () => apiRequest("/api/logs", { method: "DELETE" }),
  flushLogs: () => apiRequest("/api/logs/flush", { method: "POST" }),

  // ==================== 独服监控 ====================
  getSubscriptions: async (): Promise<Subscription[]> => {
    const raw = await apiRequest<unknown>("/api/monitor/subscriptions");
    return asArray<Subscription>(raw, ["subscriptions"]);
  },
  addSubscription: (sub: Record<string, unknown>) =>
    apiRequest<Record<string, any>>("/api/monitor/subscriptions", {
      method: "POST",
      body: JSON.stringify(sub),
    }),
  removeSubscription: (planCode: string) =>
    apiRequest<Record<string, any>>(`/api/monitor/subscriptions/${encodeURIComponent(planCode)}`, {
      method: "DELETE",
    }),
  clearSubscriptions: () =>
    apiRequest<Record<string, any>>("/api/monitor/subscriptions/clear", { method: "DELETE" }),
  batchAddAllServers: (options?: Record<string, unknown>) =>
    apiRequest<Record<string, any>>("/api/monitor/subscriptions/batch-add-all", {
      method: "POST",
      body: JSON.stringify(options || {}),
    }),
  getSubscriptionHistory: (planCode: string) =>
    apiRequest<Record<string, any>>(
      `/api/monitor/subscriptions/${encodeURIComponent(planCode)}/history`
    ),
  startMonitor: () => apiRequest("/api/monitor/start", { method: "POST" }),
  stopMonitor: () => apiRequest("/api/monitor/stop", { method: "POST" }),
  getMonitorStatus: () => apiRequest<MonitorStatus>("/api/monitor/status"),
  updateMonitorInterval: (interval: number) =>
    apiRequest("/api/monitor/interval", {
      method: "PUT",
      body: JSON.stringify({ interval }),
    }),
  testNotification: () =>
    apiRequest<any>("/api/monitor/test-notification", { method: "POST" }),

  // ==================== VPS 监控 ====================
  getVpsSubscriptions: async (): Promise<VpsSubscription[]> => {
    const raw = await apiRequest<unknown>("/api/vps-monitor/subscriptions");
    return asArray<VpsSubscription>(raw, ["subscriptions"]);
  },
  addVpsSubscription: (sub: Record<string, unknown>) =>
    apiRequest("/api/vps-monitor/subscriptions", {
      method: "POST",
      body: JSON.stringify(sub),
    }),
  removeVpsSubscription: (id: string) =>
    apiRequest<any>(`/api/vps-monitor/subscriptions/${id}`, { method: "DELETE" }),
  clearVpsSubscriptions: () =>
    apiRequest<any>("/api/vps-monitor/subscriptions/clear", { method: "DELETE" }),
  startVpsMonitor: () => apiRequest<any>("/api/vps-monitor/start", { method: "POST" }),
  stopVpsMonitor: () => apiRequest<any>("/api/vps-monitor/stop", { method: "POST" }),
  getVpsMonitorStatus: () => apiRequest<MonitorStatus>("/api/vps-monitor/status"),
  updateVpsMonitorInterval: (interval: number) =>
    apiRequest<any>("/api/vps-monitor/interval", {
      method: "PUT",
      body: JSON.stringify({ interval }),
    }),
  manualCheckVps: (planCode: string, ovhSubsidiary?: string) =>
    apiRequest<any>(`/api/vps-monitor/check/${encodeURIComponent(planCode)}`, {
      method: "POST",
      body: JSON.stringify({ ovhSubsidiary: ovhSubsidiary || "IE" }),
    }),
  getVpsSubscriptionHistory: (id: string) =>
    apiRequest<any>(`/api/vps-monitor/subscriptions/${id}/history`),

  // ==================== 服务器控制 ====================
  getMyServers: () =>
    apiRequest<{ success: boolean; servers: ManagedServer[]; total: number }>(
      "/api/server-control/list"
    ),

  getServerHardware: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/hardware`),
  getServerTemplates: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/templates`),
  getServerIps: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/ips`),
  getServiceInfo: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/serviceinfo`),
  getServerTasks: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/tasks`),
  rebootServer: (serviceName: string, type?: "hardware" | "soft") =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/reboot`, {
      method: "POST",
      body: JSON.stringify({ type: type || "soft" }),
    }),
  reinstallServer: (serviceName: string, templateName: string, options?: Record<string, unknown>) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/install`, {
      method: "POST",
      body: JSON.stringify({ templateName, ...options }),
    }),
  getInstallStatus: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/install/status`),
  changeContact: (
    serviceName: string,
    contacts: { contactAdmin?: string; contactTech?: string; contactBilling?: string }
  ) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/change-contact`, {
      method: "POST",
      body: JSON.stringify(contacts),
    }),
  getInterventions: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/interventions`),
  getPlannedInterventions: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/planned-interventions`),
  getIpmiAccess: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/console`),
  getBurstStatus: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/burst`),
  toggleBurst: (serviceName: string, enable: boolean) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/burst`, {
      method: "PUT",
      body: JSON.stringify({ enabled: enable }),
    }),
  getFirewallStatus: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/firewall`),
  toggleFirewall: (serviceName: string, enable: boolean) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/firewall`, {
      method: "PUT",
      body: JSON.stringify({ enabled: enable }),
    }),
  getBackupFtp: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/backup-ftp`),
  activateBackupFtp: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/backup-ftp`, {
      method: "POST",
    }),
  getBackupFtpPassword: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/backup-ftp/password`, {
      method: "POST",
    }),
  getBiosSettings: (serviceName: string) =>
    apiRequest<Record<string, any>>(`/api/server-control/${encodeURIComponent(serviceName)}/bios-settings`),
  getServerStatistics: (
    serviceName: string,
    period?: "daily" | "hourly" | "weekly" | "monthly" | "yearly"
  ) =>
    apiRequest<Record<string, any>>(
      `/api/server-control/${encodeURIComponent(serviceName)}/statistics${period ? `?period=${period}` : ""}`
    ),
  getMrtg: (serviceName: string, params?: Record<string, string>) => {
    const q = params ? "?" + new URLSearchParams(params).toString() : "";
    return apiRequest(`/api/server-control/${encodeURIComponent(serviceName)}/mrtg${q}`);
  },
  terminateServer: (serviceName: string) =>
    apiRequest<any>(`/api/server-control/${encodeURIComponent(serviceName)}/terminate`, {
      method: "POST",
    }),
  confirmTermination: (serviceName: string, token: string) =>
    apiRequest<any>(`/api/server-control/${encodeURIComponent(serviceName)}/confirm-termination`, {
      method: "POST",
      body: JSON.stringify({ token }),
    }),
  moveIp: (serviceName: string, ip: string, targetService: string) =>
    apiRequest<any>(`/api/server-control/${encodeURIComponent(serviceName)}/ip/move`, {
      method: "POST",
      body: JSON.stringify({ ip, to: targetService }),
    }),

  getCacheInfo: () => apiRequest<any>("/api/cache/info"),
  clearCache: (type?: "all" | "memory" | "files") =>
    apiRequest<any>("/api/cache/clear", {
      method: "POST",
      body: JSON.stringify({ type: type || "all" }),
    }),

  // ==================== OVH 账户信息 ====================
  getOvhAccountInfo,
  getOvhCreditBalance: async () => {
    try {
      const raw = await apiRequest<unknown>("/api/ovh/account/credit-balance");
      const data = asArray(raw, ["data"]);
      return { success: true, data };
    } catch (e) {
      return { success: false, data: [], error: e instanceof Error ? e.message : String(e) };
    }
  },
  getOvhSubAccounts: async () => {
    try {
      const raw = await apiRequest<unknown>("/api/ovh/account/sub-accounts");
      const data = asArray(raw, ["data"]);
      return { success: true, data };
    } catch (e) {
      return { success: false, data: [], error: e instanceof Error ? e.message : String(e) };
    }
  },
  getOvhBills: async (limit?: number) => {
    const r = await wrapList(
      `/api/ovh/account/bills${limit ? `?limit=${limit}` : ""}`,
      "bills"
    );
    return r as { success: boolean; bills: unknown[] };
  },
  getOvhEmails: async (limit?: number) => {
    const r = await wrapList(
      `/api/ovh/account/email-history${limit ? `?limit=${limit}` : ""}`,
      "emails"
    );
    return r as { success: boolean; emails: unknown[] };
  },
  getOvhRefunds: async (limit?: number) => {
    const r = await wrapList(
      `/api/ovh/account/refunds${limit ? `?limit=${limit}` : ""}`,
      "refunds"
    );
    return r as { success: boolean; refunds: unknown[] };
  },
  getOvhOrders: async (limit?: number) => {
    const r = await wrapList(
      `/api/ovh/account/orders${limit ? `?limit=${limit}` : ""}`,
      "orders"
    );
    return r as { success: boolean; orders: unknown[] };
  },
  getContactChangeRequests: async () => {
    try {
      const raw = await apiRequest<unknown>("/api/ovh/contact-change-requests");
      // Go: { status: "success", data: [...] }
      const requests = asArray<ContactChangeRequest>(raw, ["requests", "data"]);
      return { success: true, requests, data: requests };
    } catch (e) {
      return {
        success: false,
        requests: [] as ContactChangeRequest[],
        data: [] as ContactChangeRequest[],
        error: e instanceof Error ? e.message : String(e),
      };
    }
  },
  acceptContactChange: async (id: number, token?: string) =>
    normalizeActionResult(
      await apiRequest(`/api/ovh/contact-change-requests/${id}/accept`, {
        method: "POST",
        body: JSON.stringify({ token: token || "" }),
      })
    ),
  refuseContactChange: async (id: number, token?: string) =>
    normalizeActionResult(
      await apiRequest(`/api/ovh/contact-change-requests/${id}/refuse`, {
        method: "POST",
        body: JSON.stringify({ token: token || "" }),
      })
    ),
  resendContactChangeEmail: async (id: number) =>
    normalizeActionResult(
      await apiRequest(`/api/ovh/contact-change-requests/${id}/resend-email`, {
        method: "POST",
        body: "{}",
      })
    ),

  // ==================== Telegram ====================
  setTelegramWebhook: (webhookUrl: string) =>
    apiRequest<any>("/api/telegram/set-webhook", {
      method: "POST",
      body: JSON.stringify({ webhook_url: webhookUrl }),
    }),
  getTelegramWebhookInfo: async (): Promise<{
    success: boolean;
    webhook_info?: unknown;
    error?: string;
  }> => {
    try {
      const raw = await apiRequest<unknown>("/api/telegram/get-webhook-info");
      if (raw && typeof raw === "object" && "success" in (raw as object)) {
        return raw as { success: boolean; webhook_info?: unknown; error?: string };
      }
      return { success: true, webhook_info: raw };
    } catch (e) {
      return { success: false, error: e instanceof Error ? e.message : String(e) };
    }
  },
  verifyTelegram: () => apiRequest("/api/telegram/verify"),

  getOvhBalance: async () => {
    const r = await api.getOvhCreditBalance();
    const first = Array.isArray(r.data) && r.data[0] ? (r.data[0] as Record<string, unknown>) : null;
    const balance = first?.balance as { value?: number; currencyCode?: string } | undefined;
    return {
      success: r.success,
      balance: balance
        ? { value: Number(balance.value || 0), currencyCode: String(balance.currencyCode || "") }
        : undefined,
      data: r.data,
    };
  },
  manualCheckDedicated: (planCode: string) =>
    apiRequest(`/api/availability/${encodeURIComponent(planCode)}`, { method: "POST" }),
  updateSubscription: (planCode: string, options: Record<string, unknown>) =>
    apiRequest<any>(`/api/monitor/subscriptions/${encodeURIComponent(planCode)}`, {
      method: "PUT",
      body: JSON.stringify(options),
    }),
  updateVpsSubscription: (id: string, options: Record<string, unknown>) =>
    apiRequest<any>(`/api/vps-monitor/subscriptions/${id}`, {
      method: "PUT",
      body: JSON.stringify(options),
    }),
  getTelegramOrderModes: () =>
    Promise.resolve({
      success: true,
      modes: [
        { mode: "stock", description: "查库存", example: "/stock planCode" },
        { mode: "queue", description: "入队", example: "/queue planCode dc" },
        { mode: "monitor", description: "添加监控", example: "/monitor planCode" },
        { mode: "price", description: "查价格", example: "/price planCode dc" },
        { mode: "buy", description: "快速下单", example: "/buy planCode dc" },
      ],
    }),
  /** 网页端执行与 Bot 相同的 /stock /queue /buy /monitor /price 语义 */
  telegramQuickOrder: (order: Record<string, unknown>) =>
    apiRequest<any>("/api/telegram/quick-order", {
      method: "POST",
      body: JSON.stringify(order),
    }),
  /** 向 Telegram 注册 Bot 命令菜单（setMyCommands） */
  registerTelegramCommands: () =>
    apiRequest<any>("/api/telegram/register-commands", {
      method: "POST",
      body: "{}",
    }),
  batchAddAllVps: (options?: Record<string, unknown>) =>
    apiRequest<any>("/api/vps-monitor/subscriptions", {
      method: "POST",
      body: JSON.stringify(options || {}),
    }),
  getServerDetails: async (serviceName: string) => {
    const r = await api.getMyServers();
    return {
      success: true,
      server: r.servers?.find((s) => s.serviceName === serviceName),
    };
  },
  getServerPrice: async (
    planCode: string,
    datacenter: string,
    options?: string[],
    accountId?: string
  ): Promise<any> => {
    return apiRequest(`/api/servers/${encodeURIComponent(planCode)}/price`, {
      method: "POST",
      body: JSON.stringify({
        datacenter,
        options: options || [],
        accountId: accountId || undefined,
      }),
    });
  },
};

export default api;
