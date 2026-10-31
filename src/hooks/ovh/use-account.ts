import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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

import { useActiveAccount } from "./use-active-account";
import { useDefaultAccount } from "./use-accounts";

/** 当前账户的实际 ID:"跟随默认账户"也解析成具体 ID,付款/聊天必须钉在真实账户上。 */
export function useAccountScope(): string {
  const [selected] = useActiveAccount();
  const fallback = useDefaultAccount();
  return selected || fallback?.id || "";
}

/** OVH 账户信息（后端直接返回 OVH /me 字段） */
export function useAccountInfo() {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.account.info(accountId),
    queryFn: async () => (await api.get<AccountInfo>("/ovh/account/info")).data,
  });
}

/** 退款记录（后端直接返回数组） */
export function useRefunds() {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.account.refunds(accountId),
    queryFn: async () => (await api.get<RefundRecord[]>("/ovh/account/refunds")).data,
  });
}

/** 邮件历史（后端直接返回数组） */
export function useEmails() {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.account.emails(accountId),
    queryFn: async () => (await api.get<EmailHistoryEntry[]>("/ovh/account/email-history")).data,
  });
}

export interface OrderRecord {
  /** 后端固定的实际下单账户(不随查询期间默认账户变化) */
  accountId?: string;
  orderId?: number | string;
  /** OVH /me/order/{id}/status;notPaid 时可发起付款 */
  orderStatus?: string;
  paymentStatusError?: string;
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
  // 订单按账户区分:key 必须带 accountId,否则切换账户后沿用上一个账户的订单缓存
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: [...qk.account.info(accountId), "orders", limit] as const,
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

export interface OrderPaymentInfo {
  accountId: string;
  accountName: string;
  orderId: number;
  status: string;
  amount: number | null;
  currency: string;
  priceText: string;
  attempted: boolean;
  methods: { id: number; type: string; label: string; default: boolean }[];
}

/** 打开付款窗口时只读取可用方式,不产生付款请求。 */
export function useOrderPaymentInfo(accountId: string, orderId: number, open: boolean) {
  return useQuery({
    queryKey: qk.account.orderPayment(accountId, orderId),
    queryFn: async () =>
      (await api.get<OrderPaymentInfo>(`/ovh/account/orders/${orderId}/payment`, { params: { account: accountId } })).data,
    enabled: open && !!accountId,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
  });
}

/** 手动付款不自动重试(重试可能重复扣款);任何结果都刷新订单和付款登记。 */
export function usePayAccountOrder(orderId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: {
      accountId: string;
      paymentMethodId: number;
      paymentType: string;
      amount: number;
      currency: string;
      confirm: true;
    }) => {
      const { accountId: pinnedAccount, ...payload } = body;
      return (await api.post(`/ovh/account/orders/${orderId}/pay`, payload, { params: { account: pinnedAccount } })).data;
    },
    retry: false,
    onSettled: () => {
      // 订单列表的 key 用的是"选中账户"(可能为空=默认),这里直接刷新整个 account 域
      qc.invalidateQueries({ queryKey: ["account"] });
    },
  });
}

/** OVH 账户 SSH Key 列表 GET /me/sshKey */
export function useAccountSshKeys(accountIdOverride?: string | null) {
  const [activeAccount] = useActiveAccount();
  const accountId = accountIdOverride !== undefined ? accountIdOverride : activeAccount;
  return useQuery({
    queryKey: ["ovh", "account", "ssh-keys", accountId],
    queryFn: async () => {
      const res = await api.get<{ success: boolean; keys: string[] }>(
        "/ovh/account/ssh-keys",
        { params: accountId ? { account: accountId } : undefined }
      );
      return res.data?.keys || [];
    },
    enabled: Boolean(accountId),
  });
}
