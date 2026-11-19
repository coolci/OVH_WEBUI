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
  priceWithTax?: { value?: number; text?: string; currencyCode?: string };
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
    queryFn: async () =>
      (
        await api.get<AccountInfo>("/ovh/account/info", {
          params: accountId ? { account: accountId } : undefined,
        })
      ).data,
  });
}

export interface AccountRecords<T> { items: T[]; failed: number; total: number }
function records<T>(items: T[], failed: unknown, total?: number): AccountRecords<T> {
  const count = Math.max(0, Number(failed) || 0);
  if (count > 0 && items.length === 0) throw new Error("记录详情读取失败，请重试");
  return { items, failed: count, total: total ?? items.length + count };
}

export function useRefunds() {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.account.refunds(accountId),
    queryFn: async () => {
      const res = await api.get<RefundRecord[]>("/ovh/account/refunds", { params: accountId ? { account: accountId } : undefined });
      return records(res.data, res.headers?.["x-partial-failures"]);
    },
  });
}

export function useEmails() {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.account.emails(accountId),
    queryFn: async () => {
      const res = await api.get<EmailHistoryEntry[]>("/ovh/account/email-history", { params: accountId ? { account: accountId } : undefined });
      return records(res.data, res.headers?.["x-partial-failures"]);
    },
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
      const res = await api.get<OrderRecord[] | { orders?: OrderRecord[]; failed?: number; total?: number }>(
        "/ovh/account/orders",
        { params: { limit, ...(accountId ? { account: accountId } : {}) } }
      );
      const data = res.data;
      if (Array.isArray(data)) return records(data, res.headers?.["x-partial-failures"]);
      return records(data?.orders || [], data?.failed, data?.total);
    },
  });
}

export interface OrderPaymentBalance {
  name: string;
  type: string;
  amount: number;
  currency: string;
  text: string;
  enough: boolean;
}

export interface OrderPaymentInfo {
  accountId: string;
  accountName: string;
  orderId: number;
  status: string;
  amount: number | null;
  currency: string;
  priceText: string;
  expirationDate: string;
  managerUrl: string;
  attempted: boolean;
  methods: { id: number; type: string; label: string; default: boolean }[];
  /** 可用信用余额（预付款/代金券/赠送/押金）；enough=同币种且足够付清整单 */
  balances?: OrderPaymentBalance[];
  balanceError?: string;
}

/** 付款请求体：绑定支付方式，或用账户余额付清整单（POST /me/order/{id}/balance） */
export type OrderPayBody =
  | {
      kind?: "method";
      accountId: string;
      paymentMethodId: number;
      paymentType: string;
      amount: number;
      currency: string;
      confirm: true;
    }
  | {
      kind: "balance";
      accountId: string;
      balanceName: string;
      amount: number;
      currency: string;
      confirm: true;
    };

/** 信用余额（GET /me/credit/balance 及详情），只读 */
export interface CreditBalance {
  balanceName: string;
  type: string;
  usable: boolean;
  amount?: { value?: number; currencyCode?: string; text?: string };
  expiring?: { amount?: { value?: number; currencyCode?: string; text?: string }; expirationDate?: string }[];
}

export function useCreditBalances() {
  const accountId = useAccountScope();
  return useQuery({
    queryKey: qk.account.creditBalance(accountId),
    queryFn: async () => {
      const res = await api.get<{ data?: CreditBalance[]; failed?: number }>("/ovh/account/credit-balance", {
        params: accountId ? { account: accountId } : undefined,
      });
      return { list: res.data?.data || [], failed: res.data?.failed || 0 };
    },
    enabled: !!accountId,
    staleTime: 60_000,
  });
}

/** 预付款账户（GET /me/ovhAccount，仅 EU/CA）；supported=false 表示该站点无充值接口 */
export interface TopupInfo {
  accountId: string;
  supported: boolean;
  accounts: {
    id: string;
    balance: number | null;
    currency: string;
    balanceText: string;
    canBeCredited: boolean;
    isActive: boolean;
  }[];
}

export function useTopupInfo(accountId: string) {
  return useQuery({
    queryKey: qk.account.topup(accountId),
    queryFn: async () => (await api.get<TopupInfo>("/ovh/account/topup", { params: { account: accountId } })).data,
    enabled: !!accountId,
    staleTime: 60_000,
    retry: false,
  });
}

export interface TopupOrder {
  accountId: string;
  orderId: number;
  url?: string;
  amount: number | null;
  currency: string;
  priceText: string;
}

/** 生成充值订单（仅生成待付款订单，不扣款）；不自动重试 */
export function useCreateTopup(accountId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: { ovhAccountId: string; amount: number }) =>
      (await api.post<TopupOrder>("/ovh/account/topup", { ...body, confirm: true }, { params: { account: accountId } })).data,
    retry: false,
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["account"] });
    },
  });
}

export interface RedeemResult {
  balanceName: string;
  type: string;
  amount: number | null;
  currency: string;
  text: string;
  expirationDate?: string;
}

/** 兑换券码到余额（POST /me/credit/code）；不自动重试 */
export function useRedeemCode(accountId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (code: string) =>
      (await api.post<RedeemResult>("/ovh/account/credit-code", { code }, { params: { account: accountId } })).data,
    retry: false,
    onSettled: () => {
      qc.invalidateQueries({ queryKey: qk.account.creditBalance(accountId) });
      qc.invalidateQueries({ queryKey: qk.account.topup(accountId) });
    },
  });
}

/** 打开付款窗口时只读取可用方式,不产生付款请求。 */
export function useOrderPaymentInfo(accountId: string, orderId: number, open: boolean) {
  const qc = useQueryClient();
  return useQuery({
    queryKey: qk.account.orderPayment(accountId, orderId),
    queryFn: async () => {
      const data = (await api.get<OrderPaymentInfo>(`/ovh/account/orders/${orderId}/payment`, { params: { account: accountId } })).data;
      // 后端将这次实时查询结果同步到购买历史，再让页面读取新状态。
      void qc.invalidateQueries({ queryKey: qk.history() });
      return data;
    },
    enabled: open && !!accountId,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => query.state.status !== "error" && query.state.data?.attempted &&
      ["notPaid", "unknown"].includes(query.state.data.status) ? 5_000 : false,
    refetchIntervalInBackground: false,
  });
}

/** 手动付款不自动重试(重试可能重复扣款);任何结果都刷新订单、余额和付款登记。 */
export function usePayAccountOrder(orderId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (body: OrderPayBody): Promise<{ status?: string; orderStatus?: string; message?: string }> => {
      const { accountId: pinnedAccount, kind, ...payload } = body;
      const path = kind === "balance" ? "pay-balance" : "pay";
      return (await api.post(`/ovh/account/orders/${orderId}/${path}`, payload, { params: { account: pinnedAccount } })).data;
    },
    retry: false,
    onSettled: () => {
      // 订单列表的 key 用的是"选中账户"(可能为空=默认),这里直接刷新整个 account 域（含余额与预付款缓存）
      qc.invalidateQueries({ queryKey: ["account"] });
      qc.invalidateQueries({ queryKey: qk.history() });
      qc.invalidateQueries({ queryKey: ["history-cancellation"] });
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
