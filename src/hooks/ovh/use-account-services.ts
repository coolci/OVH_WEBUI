import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/http";
import type { OwnedServer } from "./use-server-control";
import type { OwnedVps } from "./use-vps-control";

export interface AccountServiceItem {
  serviceName: string;
  name: string;
  displayName: string;
  type: "dedicated" | "vps";
  typeLabel: string;
  ip?: string;
  datacenter?: string;
  status?: string;
}

/**
 * 完整获取当前账户下的所有服务（包含全部独立服务器与全部 VPS 实例）
 * 不做任何状态过滤，确保故障机、待交付机、异常机都能被工单正常关联。
 */
export function useAccountServices(accountId?: string) {
  const serversQuery = useQuery({
    queryKey: ["all-owned-servers", accountId],
    queryFn: async () => {
      try {
        const res = await api.get<{ success?: boolean; servers?: OwnedServer[] }>(
          "/server-control/list",
          { params: accountId ? { account: accountId } : undefined }
        );
        return (res.data?.servers || []) as OwnedServer[];
      } catch {
        return [] as OwnedServer[];
      }
    },
    staleTime: 60_000,
  });

  const vpsQuery = useQuery({
    queryKey: ["all-owned-vps", accountId],
    queryFn: async () => {
      try {
        const res = await api.get<{ success?: boolean; vps?: OwnedVps[] }>(
          "/vps-control/list",
          { params: accountId ? { account: accountId } : undefined }
        );
        return (res.data?.vps || []) as OwnedVps[];
      } catch {
        return [] as OwnedVps[];
      }
    },
    staleTime: 60_000,
  });

  const servers = serversQuery.data || [];
  const vpsList = vpsQuery.data || [];

  const items: AccountServiceItem[] = [];

  // 1. 独立服务器
  for (const s of servers) {
    if (!s.serviceName) continue;
    items.push({
      serviceName: s.serviceName,
      name: s.name || s.serviceName,
      displayName: s.name && s.name !== s.serviceName ? `${s.name} (${s.serviceName.split(".")[0]})` : s.serviceName,
      type: "dedicated",
      typeLabel: "独立服务器",
      ip: s.ip,
      datacenter: s.datacenter,
      status: s.state || s.status,
    });
  }

  // 2. VPS 实例
  for (const v of vpsList) {
    if (!v.serviceName) continue;
    items.push({
      serviceName: v.serviceName,
      name: v.name || v.serviceName,
      displayName: v.displayName || v.name || v.serviceName,
      type: "vps",
      typeLabel: "VPS",
      datacenter: v.zone || v.cluster,
      status: v.state || v.status || undefined,
    });
  }

  return {
    services: items,
    isLoading: serversQuery.isLoading || vpsQuery.isLoading,
    isFetching: serversQuery.isFetching || vpsQuery.isFetching,
    refetch: () => {
      serversQuery.refetch();
      vpsQuery.refetch();
    },
  };
}
