import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";

export interface AccountInfo {
  customerCode: string;
  nichandle: string;
  email: string;
  firstname?: string;
  name?: string;
  city?: string;
  country?: string;
  kycValidated?: boolean;
  state?: string;
  currency?: { code: string; symbol: string };
  /** OVH 子公司：IE / FR / DE / US / CA / ASIA / SG / AU / IN 等。决定结算货币和价格档 */
  ovhSubsidiary?: string;
}

export interface RefundRecord {
  refundId: string;
  orderId: string;
  date: string;
  priceWithTax: { value: number; text: string; currencyCode: string };
  pdfUrl?: string;
}

export interface EmailHistoryEntry {
  id: number;
  date: string;
  subject: string;
  body: string;
}

/** OVH 账户信息（后端直接返回 OVH /me 字段） */
export function useAccountInfo() {
  return useQuery({
    queryKey: qk.account.info(),
    queryFn: async () => (await api.get<AccountInfo>("/ovh/account/info")).data,
  });
}

/** 退款记录（后端直接返回数组） */
export function useRefunds() {
  return useQuery({
    queryKey: qk.account.refunds(),
    queryFn: async () => (await api.get<RefundRecord[]>("/ovh/account/refunds")).data,
  });
}

/** 邮件历史（后端直接返回数组） */
export function useEmails() {
  return useQuery({
    queryKey: qk.account.emails(),
    queryFn: async () => (await api.get<EmailHistoryEntry[]>("/ovh/account/email-history")).data,
  });
}

export interface OrderRecord {
  orderId?: number | string;
  date?: string;
  expirationDate?: string;
  password?: string;
  pdfUrl?: string;
  priceWithTax?: { value?: number; text?: string; currencyCode?: string };
  priceWithoutTax?: { value?: number; text?: string; currencyCode?: string };
  url?: string;
  [key: string]: unknown;
}

/** OVH 订单列表 GET /me/order 详情 */
export function useOrders(limit = 30) {
  return useQuery({
    queryKey: [...qk.account.info(), "orders", limit] as const,
    queryFn: async () => {
      const res = await api.get<OrderRecord[] | { orders?: OrderRecord[] }>(
        "/ovh/account/orders",
        { params: { limit } }
      );
      const data = res.data;
      if (Array.isArray(data)) return data;
      if (data && typeof data === "object" && Array.isArray((data as any).orders)) {
        return (data as any).orders as OrderRecord[];
      }
      return [] as OrderRecord[];
    },
  });
}
