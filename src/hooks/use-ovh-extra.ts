import { useMutation, useQuery } from "@tanstack/react-query";
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
