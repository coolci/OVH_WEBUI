import { queryOptions, useQueries, useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { useActiveAccount } from "@/hooks/use-active-account";
import { useOwnedServers } from "./use-server-core";
import type {
  HardwareInfo,
  HardwareLottery,
  DiskGroup,
  PartitionScheme,
  FaultyDisk,
} from "./types";

export function serverHardwareQueryOptions(serviceName: string, accountId: string) {
  return queryOptions({
    queryKey: qk.serverControl.hardware(serviceName, accountId),
    queryFn: async ({ signal }) => {
      const res = await api.get(`/server-control/${encodeURIComponent(serviceName)}/hardware`, {
        params: { account: accountId },
        signal,
      });
      const hw = (res.data?.hardware || null) as HardwareInfo | null;
      return hw ? { ...hw, lottery: (res.data?.lottery || null) as HardwareLottery | null } : null;
    },
    staleTime: 10 * 60_000,
    enabled: !!serviceName && !!accountId,
  });
}

/** 硬件信息（后端返回 { success, hardware: {...}, lottery: {...} }） */
export function useServerHardware(serviceName: string | null) {
  const [accountId] = useActiveAccount();
  return useQuery(serverHardwareQueryOptions(serviceName || "", accountId));
}

/** 检查当前账户的全部机器，进入控制页之前也能在导航看到中奖提醒。 */
export function useServerLotterySummary() {
  const [accountId] = useActiveAccount();
  const qc = useQueryClient();
  const serversQuery = useOwnedServers();
  const servers = serversQuery.data || [];
  const hardwareQueries = useQueries({
    queries: servers.map((server) => serverHardwareQueryOptions(server.serviceName, accountId)),
  });
  const byServiceName = new Map<string, HardwareLottery>();
  servers.forEach((server, index) => {
    const lottery = hardwareQueries[index].data?.lottery;
    if (lottery?.checked && lottery.won) byServiceName.set(server.serviceName, lottery);
  });

  const refresh = async () => {
    const result = await serversQuery.refetch();
    await Promise.all((result.data || []).map((server) => qc.invalidateQueries({
      queryKey: qk.serverControl.hardware(server.serviceName, accountId),
      exact: true,
    })));
  };

  return {
    serversQuery,
    byServiceName,
    winnerCount: byServiceName.size,
    isFetching: serversQuery.isFetching || hardwareQueries.some((query) => query.isFetching),
    refresh,
  };
}

export function useServerDiskInfo(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.diskInfo(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/hardware-disk-info`);
      return (res.data?.diskGroups || {}) as Record<string, DiskGroup>;
    },
    enabled: !!serviceName && enabled,
    staleTime: 5 * 60_000,
  });
}

export function useServerRaidProfiles(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.raidProfiles(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/hardware-raid-profiles`);
        return {
          supported: res.data?.supported !== false,
          profiles: (res.data?.profiles || []) as any[],
        };
      } catch (e: any) {
        const status = e?.response?.status;
        if (status === 404 || status === 501) {
          return { supported: false, profiles: [] as any[] };
        }
        throw e;
      }
    },
    enabled: !!serviceName && enabled,
    staleTime: 5 * 60_000,
  });
}

export function useServerPartitionSchemes(serviceName: string | null, templateName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.partitionSchemes(serviceName || "", templateName || ""),
    queryFn: async () => {
      const res = await api.get(
        `/server-control/${serviceName}/partition-schemes?templateName=${encodeURIComponent(templateName || "")}`
      );
      return (res.data?.schemes || []) as PartitionScheme[];
    },
    enabled: !!serviceName && !!templateName,
    staleTime: 5 * 60_000,
  });
}

export function useServerBiosSettings(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.biosSettings(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/bios-settings`);
        const sgxRes = await api.get(`/server-control/${serviceName}/bios-settings/sgx`).catch(() => null);
        return {
          settings: (res.data || {}) as any,
          sgx: sgxRes?.data?.sgx ?? sgxRes?.data?.data ?? sgxRes?.data ?? null,
        };
      } catch (e: any) {
        const status = e?.response?.status;
        if (status === 404 || status === 501) {
          return { settings: {} as any, sgx: null };
        }
        throw e;
      }
    },
    enabled: !!serviceName && enabled,
  });
}

export function useCreateIntervention() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: {
      serviceName: string;
      type: string;
      details?: string;
      comment?: string;
      disks?: FaultyDisk[];
      inverse?: boolean;
      slots?: string[];
    }) => {
      const res = await api.post<{
        success: boolean;
        message: string;
        ticketNumber?: string;
        ticketId?: string;
        notice?: string;
      }>(`/server-control/${args.serviceName}/hardware/replace`, {
        componentType: args.type,
        details: args.details,
        comment: args.comment,
        ...(args.disks?.length ? { disks: args.disks, inverse: !!args.inverse } : {}),
        ...(args.slots?.length ? { slots: args.slots } : {}),
      });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.interventions(vars.serviceName) });
    },
  });
}

export function useServerInterventions(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.interventions(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/interventions`);
      return (res.data?.interventions || []) as any[];
    },
    enabled: !!serviceName,
  });
}
