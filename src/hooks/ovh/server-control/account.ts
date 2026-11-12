import type { AxiosRequestConfig } from "axios";
import { useActiveServerControlAccount, useActiveVpsControlAccount } from "@/hooks/use-active-account";

/**
 * 服务器控制 hooks 用的活跃账户 ID(随本页状态更新,跨标签页切换时也会同步)。
 *
 * 支持传入 overrideAccountId 锁定特定服务器归属的账户 ID，避免跨标签页切换账户时发生竞态操作串号。
 * 所有 /server-control/* 请求都用它显式带 ?account=,query key 也带上它。
 * 显式传参让请求始终发往渲染当前界面的那个账户。
 */
export function useServerAccount(overrideAccountId?: string): string {
  const [accountId] = useActiveServerControlAccount();
  return overrideAccountId || accountId;
}

/**
 * 独服 / VPS 共用的 hooks(撤单等)用:VPS 模式与拦截器规则一致,
 * 优先 VPS 活跃账户,没选时退回独服账户。
 * 支持传入 overrideAccountId 显式指定账户。
 */
export function useControlAccount(isVps: boolean, overrideAccountId?: string): string {
  const [serverAccount] = useActiveServerControlAccount();
  const [vpsAccount] = useActiveVpsControlAccount();
  if (overrideAccountId) return overrideAccountId;
  return isVps ? vpsAccount || serverAccount : serverAccount;
}

/** 给请求配置加上 ?account=<accountId>。accountId 为空时原样返回(交给拦截器兜底)。 */
export function withAccount<T extends AxiosRequestConfig>(accountId: string, config?: T): T {
  const base = (config ?? {}) as T;
  if (!accountId) return base;
  return {
    ...base,
    params: { ...((base.params as Record<string, unknown> | undefined) ?? {}), account: accountId },
  };
}
