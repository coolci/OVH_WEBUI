import { catalogRefresh, type CatalogRefreshStatus } from "@/lib/catalog-refresh";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { toast } from "sonner";
import { useAccounts, findAccountByID } from "@/hooks/use-accounts";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import i18n from "@/i18n";
import { apiMessage } from "@/lib/api-error";

export interface ServerOption {
  label: string;
  value: string;
  family?: string;
}

export interface ServerPlan {
  planCode: string;
  name: string;
  description?: string;
  cpu: string;
  memory: string;
  storage: string;
  bandwidth: string;
  vrackBandwidth: string;
  defaultOptions: ServerOption[];
  availableOptions: ServerOption[];
  datacenters: {
    datacenter: string;
    dcName: string;
    region: string;
    availability: string;
    countryCode: string;
  }[];
  prerelease?: boolean;
  /** 本次目录刷新未能读到该型号库存，机房列表沿用上一份缓存 */
  stockStale?: boolean;
  /** 本程序首次在目录里看到它的 unix 秒;0 / 缺省 = 开始记录前就已在售 */
  firstSeen?: number;
}

const MONITOR_DC_API_ALIASES: Record<string, string> = {
  "par-a": "eu-west-par-a",
  "par-b": "eu-west-par-b",
  "par-c": "eu-west-par-c",
  tor: "ca-east-tor-a",
  "ca-east-tor": "ca-east-tor-a",
  mum: "ynm",
};

/** 独立无闭包副作用的服务器目录请求函数，支持显式指定 forceRefresh */
export async function fetchServers(opts: {
  showApiServers?: boolean;
  accountId?: string;
  forceRefresh?: boolean;
}): Promise<{ data: ServerPlan[]; updatedAt: number; failed: boolean }> {
  const showApiServers = opts.showApiServers ?? true;
  const params: Record<string, unknown> = { showApiServers };
  if (opts.accountId) params.account = opts.accountId;
  if (opts.forceRefresh) params.forceRefresh = true;
  const res = await api.get("/servers", { params });
  // 必须确保是数组:调用方直接 .find() / .map(),拿到对象就是整页白屏
  const raw = res.data?.servers ?? res.data;
  if (!Array.isArray(raw)) throw new Error("服务器目录响应格式异常");
  const info = res.data?.cacheInfo;
  return {
    data: raw as ServerPlan[],
    updatedAt: typeof info?.timestamp === "number" ? info.timestamp * 1000 : Date.now(),
    failed: !!info?.usingExpiredCache || !!info?.refreshFailed,
  };
}

/** 服务器目录实际使用的账户 ID：与 useServers 保持同一规则，供缓存徽章等配套组件复用。 */
export function useCatalogAccountId(explicitAccountId?: string): string {
  const [activeId] = useActiveServerControlAccount();
  const { data: accounts } = useAccounts();
  const resolvedId = explicitAccountId || activeId;
  return findAccountByID(accounts, resolvedId) ? resolvedId : "";
}

/** 目录到期后在可见页面自动刷新；旧缓存回退和网络失败均自动退避重试。 */
export function useServers(showApiServers: boolean = true, explicitAccountId?: string) {
  const qc = useQueryClient();
  // 目录跟着当前账户走:后端 /api/servers 支持 ?account=,按该账户的 zone(子公司)+ endpoint
  // 取目录并单独分桶缓存。只在这个 id 确实存在于账户列表里时才带上。
  const { isPending: accountsPending } = useAccounts();
  const accountId = useCatalogAccountId(explicitAccountId);
  const key = qk.servers.list(showApiServers, accountId);
  const statusKey = [...key, "refresh-status"];

  const refresh = catalogRefresh<ServerPlan[]>(qc, key, async () => {
    const hasPriorFailure = Boolean(qc.getQueryData<CatalogRefreshStatus>(statusKey)?.failures);
    const out = await fetchServers({
      showApiServers,
      accountId,
      forceRefresh: hasPriorFailure,
    });
    // 列表加载可能触发后端重拉目录，同步刷新该账户视角的缓存信息。
    qc.invalidateQueries({ queryKey: ["settings", "cache-info"] });
    return out;
  });

  // 只用来订阅 catalogRefresh 写进缓存的刷新状态,自己从不取数。
  const status = useQuery<CatalogRefreshStatus>({
    queryKey: refresh.statusKey,
    queryFn: () => qc.getQueryData<CatalogRefreshStatus>(refresh.statusKey) ?? { failures: 0, attemptedAt: 0 },
    enabled: false,
  });
  const q = useQuery({
    queryKey: key,
    ...refresh,
    enabled: !accountsPending,
  });

  /**
   * 显式发起带 forceRefresh=true 的请求并写回共享 QueryClient 缓存。
   * 不再使用局部 useRef + q.refetch()，彻底消除多组件挂载同一 queryKey 时后挂载实例闭包吞掉 forceRefresh 的缺陷。
   */
  const forceRefresh = async () => {
    const runForced = async () => {
      const previous = qc.getQueryData<CatalogRefreshStatus>(refresh.statusKey);
      try {
        const result = await fetchServers({
          showApiServers,
          accountId,
          forceRefresh: true,
        });
        qc.setQueryData<CatalogRefreshStatus>(refresh.statusKey, {
          updatedAt: result.updatedAt,
          expiresAt: result.updatedAt + 120_000,
          failures: result.failed ? (previous?.failures ?? 0) + 1 : 0,
          attemptedAt: Date.now(),
        });
        qc.setQueryData<ServerPlan[]>(key, result.data);
        return result.data;
      } catch (error) {
        qc.setQueryData<CatalogRefreshStatus>(refresh.statusKey, {
          ...previous,
          failures: (previous?.failures ?? 0) + 1,
          attemptedAt: Date.now(),
        });
        throw error;
      }
    };

    if (typeof qc.fetchQuery === "function") {
      return qc.fetchQuery({
        queryKey: key,
        staleTime: 0,
        queryFn: runForced,
      });
    }
    return runForced();
  };

  return Object.assign(q, {
    forceRefresh,
    refreshStatus: status.data,
    accountId,
    isRefreshing: q.isRefetching,
  });
}

/** 添加到监控订阅 */
export function useAddToMonitor() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: {
      planCode: string;
      datacenters: string[];
      /** 只盯这套配置(addon planCode)。空 = 该机型底下所有配置组合 */
      options?: string[];
      serverName?: string;
    }) => {
      const normalizedDcs = (payload.datacenters || []).map((dc) => {
        const c = dc.trim().toLowerCase();
        return MONITOR_DC_API_ALIASES[c] || c;
      });
      return (
        await api.post("/monitor/subscriptions", {
          ...payload,
          datacenters: normalizedDcs,
          notifyAvailable: true,
          notifyUnavailable: false,
        })
      ).data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.monitor.list() });
      toast.success(i18n.t("hooksMsg.servers.monitorAdded"));
    },
    onError: (e: unknown) => toast.error(apiMessage(e)),
  });
}
