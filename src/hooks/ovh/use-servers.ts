import { useRef } from "react";
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
  /** 本程序首次在目录里看到它的 unix 秒;0 / 缺省 = 开始记录前就已在售 */
  firstSeen?: number;
}

/** 目录到期后在可见页面自动刷新；旧缓存回退和网络失败均自动退避重试。 */
export function useServers(showApiServers: boolean = true, explicitAccountId?: string) {
  const qc = useQueryClient();
  // 目录跟着当前账户走:后端 /api/servers 支持 ?account=,按该账户的 zone(子公司)+ endpoint
  // 取目录并单独分桶缓存。不传的话永远是默认账户的视角 —— 切到美区账户后,页面上的机型集合、
  // 机房状态、价格分别来自三个不同的账户/站点,互相对不上。
  // 只在这个 id 确实存在于账户列表里时才带上:后端对未知 account 直接 400,
  // 而 localStorage 里可能留着已删账户的 id,那会让整页变成一条红错。
  const [activeId] = useActiveServerControlAccount();
  const { data: accounts, isPending: accountsPending } = useAccounts();
  const resolvedId = explicitAccountId || activeId;
  const accountId = findAccountByID(accounts, resolvedId) ? resolvedId : "";
  const key = qk.servers.list(showApiServers, accountId);
  const force = useRef(false);
  const refresh = catalogRefresh<ServerPlan[]>(qc, key, async () => {
    const params: Record<string, unknown> = { showApiServers };
    if (accountId) params.account = accountId;
    if (force.current || qc.getQueryData<CatalogRefreshStatus>([...key, "refresh-status"])?.failures) params.forceRefresh = true;
    force.current = false;
    const res = await api.get("/servers", { params });
    // 必须确保是数组:调用方直接 .find() / .map(),拿到对象就是整页白屏
    // (「Something went wrong!」那种,比任何报错都难查)。
    // 后端正常返回 { servers: [...] },这里防的是响应结构意外的情况 ——
    // 反向代理插了一页、网关返回 200 带错误体、以后重构改了字段名。
    const raw = res.data?.servers ?? res.data;
    if (!Array.isArray(raw)) throw new Error("服务器目录响应格式异常");
    const info = res.data?.cacheInfo;
    return {
      data: raw as ServerPlan[],
      updatedAt: typeof info?.timestamp === "number" ? info.timestamp * 1000 : Date.now(),
      failed: !!info?.usingExpiredCache || !!info?.refreshFailed,
    };
  });
  // 只用来订阅 catalogRefresh 写进缓存的刷新状态,自己从不取数。仍要给 queryFn:
  // 没有它时,任何按 ["servers", ...] 前缀触发的刷新都会让 react-query 报 "No queryFn"
  // (控制台里刷屏),返回缓存里的现值即可,不会覆盖真实状态。
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

  const forceRefresh = async () => {
    force.current = true;
    await q.refetch();
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
    }) =>
      (await api.post("/monitor/subscriptions", { ...payload, notifyAvailable: true, notifyUnavailable: false })).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.monitor.list() });
      toast.success(i18n.t("hooksMsg.servers.monitorAdded"));
    },
    onError: (e: unknown) => toast.error(apiMessage(e)),
  });
}
