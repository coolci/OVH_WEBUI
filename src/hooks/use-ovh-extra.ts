import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import i18n from "@/i18n";
import { api } from "@/lib/http";
import { apiMessage } from "@/lib/api-error";

/** 手动清理本程序留下的 OVH 遗留购物车(后台每 6 小时也会自动清理) */
export function useCleanupCarts() {
  return useMutation({
    mutationFn: async () => (await api.post<{ deleted: number }>("/ovh/carts/cleanup")).data,
    onSuccess: (data) => {
      if (data?.deleted) toast.success(i18n.t("ovhExtra.carts.done", { count: data.deleted }));
      else toast.info(i18n.t("ovhExtra.carts.none"));
    },
    onError: (e: unknown) => toast.error(i18n.t("ovhExtra.carts.failed", { reason: apiMessage(e) || "" })),
  });
}

export interface DefaultPaymentInfo {
  hasDefault: boolean;
  valid: boolean;
  type?: string;
  label?: string;
  status?: string;
  expiration?: string;
  reason?: string;
}

/** 账户默认支付方式(自动付款依赖它)。后端缓存 10 分钟 */
export function useDefaultPayment(accountId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ["ovh", "payment-default", accountId || ""],
    queryFn: async () => (await api.get<DefaultPaymentInfo>("/ovh/payment-default", { params: { account_id: accountId } })).data,
    enabled: enabled && !!accountId,
    staleTime: 5 * 60_000,
    retry: false,
  });
}

export interface OrderFollowUpStep {
  step: string;
  status: string;
  history: { date: string; label: string; description?: string }[];
}

/** 订单付款后的进度 + 已交付的服务名(点开时才查) */
export function useOrderFollowUp(accountId: string | undefined, orderId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ["ovh", "order-followup", accountId || "", orderId || ""],
    queryFn: async () =>
      (await api.get<{ steps: OrderFollowUpStep[]; services: string[] }>(
        `/ovh/orders/${encodeURIComponent(orderId || "")}/followup`,
        { params: { account_id: accountId } },
      )).data,
    enabled: enabled && !!accountId && !!orderId,
    staleTime: 60_000,
    retry: false,
  });
}

/** 某个独服型号能装的系统(OVH osAvailabilities,免登录;后端缓存 1 小时) */
export function useOSAvailability(planCode: string | undefined, accountId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ["ovh", "os-availability", planCode || "", accountId || ""],
    queryFn: async () =>
      (await api.get<{ os: string[] }>("/ovh/os-availability", { params: { planCode, account_id: accountId || undefined } })).data.os,
    enabled: enabled && !!planCode,
    staleTime: 60 * 60_000,
    retry: false,
  });
}

export interface ApiCredential {
  credentialId: number;
  applicationId: number;
  appName?: string;
  appDescription?: string;
  status: string;
  creation?: string;
  expiration?: string;
  lastUse?: string;
  allowedIPs?: string[];
  rules?: { method: string; path: string }[];
  current: boolean;
}

export function useApiCredentials(accountId: string | undefined) {
  return useQuery({
    queryKey: ["ovh", "security", "credentials", accountId || ""],
    queryFn: async () =>
      (await api.get<{ credentials: ApiCredential[]; truncated: boolean }>("/ovh/security/credentials", { params: { account_id: accountId } })).data,
    enabled: !!accountId,
    staleTime: 60_000,
    retry: false,
  });
}

export function useRevokeApiCredential(accountId: string | undefined) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: number) =>
      (await api.delete(`/ovh/security/credentials/${id}`, { params: { account_id: accountId } })).data,
    onSuccess: (_d, id) => {
      toast.success(i18n.t("ovhExtra.security.revoked", { id }));
      qc.invalidateQueries({ queryKey: ["ovh", "security", "credentials", accountId || ""] });
    },
    onError: (e: unknown) => toast.error(i18n.t("ovhExtra.security.revokeFailed", { reason: apiMessage(e) || "" })),
  });
}

export interface ApiLog {
  logId: number;
  date: string;
  method: string;
  path: string;
  route?: string;
  ip: string;
}

export function useApiLogs(accountId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: ["ovh", "security", "api-logs", accountId || ""],
    queryFn: async () =>
      (await api.get<{ logs: ApiLog[]; unsupported?: boolean }>("/ovh/security/api-logs", { params: { account_id: accountId } })).data,
    enabled: enabled && !!accountId,
    staleTime: 60_000,
    retry: false,
  });
}
