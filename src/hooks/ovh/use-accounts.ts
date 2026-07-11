import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, getActiveServerControlAccount, setActiveServerControlAccount } from "@/lib/http";
import { toast } from "sonner";

export interface OVHAccount {
  id: string;
  name: string;
  endpoint: string;
  zone: string;
  appKey: string;
  appSecret: string;
  consumerKey: string;
  iam: string;
  isDefault: boolean;
  createdAt: string;
}

export interface AccountInput {
  name: string;
  zone: string;
  endpoint?: string; // 可空, 后端按 zone 推
  appKey: string;
  appSecret: string;
  consumerKey: string;
  iam?: string;
  setDefault?: boolean;
}

const ACCOUNTS_KEY = ["accounts", "list"] as const;

/** 全部账户列表(默认账户排首位) */
export function useAccounts() {
  return useQuery({
    queryKey: ACCOUNTS_KEY,
    queryFn: async () => {
      const res = await api.get<{ accounts: OVHAccount[] }>("/accounts");
      return res.data.accounts || [];
    },
    staleTime: 5 * 60_000,
  });
}

/** 默认账户(取列表中 isDefault, 没有就第一个) */
export function useDefaultAccount(): OVHAccount | null {
  const q = useAccounts();
  const list = q.data || [];
  return list.find((a) => a.isDefault) || list[0] || null;
}

/** 创建账户。后端会自动调 /me 验证,返回 valid 字段 */
export function useCreateAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: AccountInput) => {
      const res = await api.post<{ account: OVHAccount; valid: boolean }>("/accounts", input);
      return res.data;
    },
    onSuccess: (data) => {
      qc.invalidateQueries({ queryKey: ACCOUNTS_KEY });
      // 新账户立即设为活跃，避免 localStorage 仍指向旧 ID
      if (data?.account?.id) {
        setActiveServerControlAccount(data.account.id);
        void qc.invalidateQueries({ queryKey: ["server-control"] });
        void qc.invalidateQueries({ queryKey: ["vps-control"] });
        void qc.invalidateQueries({ queryKey: ["account"] });
      }
      if (data.valid) {
        toast.success(`账户 ${data.account.name} 创建成功`);
      } else {
        toast.warning(`账户创建失败或 OVH 验证未通过，请检查凭据`);
      }
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || "创建失败"),
  });
}

/** 更新账户 */
export function useUpdateAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: Partial<AccountInput> }) => {
      const res = await api.put<{ account: OVHAccount; valid: boolean }>(`/accounts/${id}`, input);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ACCOUNTS_KEY });
      toast.success("账户已更新");
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || "更新失败"),
  });
}

/** 删除账户(级联删除关联的 queue/history/sniper 记录) */
export function useDeleteAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await api.delete(`/accounts/${id}`);
      return id;
    },
    onSuccess: (id) => {
      if (getActiveServerControlAccount() === id) {
        setActiveServerControlAccount("");
      }
      qc.invalidateQueries({ queryKey: ACCOUNTS_KEY });
      qc.invalidateQueries({ queryKey: ["queue"] });
      qc.invalidateQueries({ queryKey: ["history"] });
      qc.invalidateQueries({ queryKey: ["server-control"] });
      qc.invalidateQueries({ queryKey: ["vps-control"] });
      qc.invalidateQueries({ queryKey: ["account"] });
      toast.success("账户已删除,关联数据一并清理");
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || "删除失败"),
  });
}

/** 把指定账户标为默认 */
export function useSetDefaultAccount() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => (await api.post(`/accounts/${id}/set-default`)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ACCOUNTS_KEY });
      toast.success("已设为默认账户");
    },
    onError: (e: any) => toast.error(e?.response?.data?.error || "设默认失败"),
  });
}

/** 重新验证账户凭据(调 OVH /me) */
export function useVerifyAccount() {
  return useMutation({
    mutationFn: async (id: string) =>
      (await api.post<{ valid: boolean }>(`/accounts/${id}/verify`)).data,
    onSuccess: (data) => {
      if (data.valid) {
        toast.success("OVH 凭据验证通过");
      } else {
        toast.error("OVH 凭据验证失败,检查 AppKey / AppSecret / ConsumerKey");
      }
    },
  });
}

/** 按 ID 查账户(从 useAccounts 缓存里找,不发请求) */
export function findAccountByID(accounts: OVHAccount[] | undefined, id: string): OVHAccount | undefined {
  if (!accounts || !id) return undefined;
  return accounts.find((a) => a.id === id);
}

/** zone 颜色映射, 用于账户 chip 区分(EU 蓝 / US 红 / CA 绿 等) */
export function accountChipColor(zone: string): string {
  const z = zone.toUpperCase();
  if (z === "US") return "bg-red-100 text-red-700 dark:bg-red-950/40 dark:text-red-300";
  if (z === "CA" || z === "QC") return "bg-green-100 text-green-700 dark:bg-green-950/40 dark:text-green-300";
  if (z === "ASIA" || z === "SG" || z === "AU" || z === "IN") return "bg-orange-100 text-orange-700 dark:bg-orange-950/40 dark:text-orange-300";
  // EU 系
  return "bg-blue-100 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300";
}
