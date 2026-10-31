import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { useActiveAccount } from "@/hooks/use-active-account";
import type { PartialList } from "../partial-list";
import type {
  MitigationBlock,
  NetworkInterface,
  SecondaryDnsDomain,
  VirtualMacEntry,
  VrackEntry,
} from "./types";

function readPartialList<T>(data: any, listKey: string): PartialList<T> {
  const failedCount = Number(data?.failedCount) || 0;
  return {
    items: (data?.[listKey] || []) as T[],
    partial: data?.partial === true || failedCount > 0,
    failedCount,
  };
}

export function useMitigation(serviceName: string | null) {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.serverControl.mitigation(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/mitigation`, { params: { account: accountId } });
      return (res.data?.ips || []) as MitigationBlock[];
    },
    enabled: !!serviceName && !!accountId,
    refetchInterval: (q) => {
      const data = q.state.data as MitigationBlock[] | undefined;
      if (!data) return false;
      const hasTransition = data.some((b) =>
        b.mitigations.some((m: any) => m.state === "creationPending" || m.state === "removalPending"),
      );
      return hasTransition ? 5000 : false;
    },
  });
}

export function useEnableMitigation(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) => {
      const res = await api.post(`/server-control/${serviceName}/mitigation/${vars.ip}?block=${encodeURIComponent(vars.block)}`);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.mitigation(serviceName) });
    },
  });
}

export function useDisableMitigation(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) => {
      const res = await api.delete(`/server-control/${serviceName}/mitigation/${vars.ip}?block=${encodeURIComponent(vars.block)}`);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.mitigation(serviceName) });
    },
  });
}

export function useServerIps(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.ips(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/ips`);
      return (res.data?.ips || []) as Array<{ ip: string; type: string }>;
    },
    enabled: !!serviceName,
  });
}

export function useServerNetworkInterfaces(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.networkInterfaces(serviceName || ""),
    queryFn: async (): Promise<PartialList<NetworkInterface>> => {
      const res = await api.get(`/server-control/${serviceName}/network-interfaces`);
      return readPartialList<NetworkInterface>(res.data, "interfaces");
    },
    enabled: !!serviceName,
  });
}

export function useServerSecondaryDns(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.secondaryDns(serviceName || ""),
    queryFn: async (): Promise<PartialList<SecondaryDnsDomain>> => {
      const res = await api.get(`/server-control/${serviceName}/secondary-dns`);
      return readPartialList<SecondaryDnsDomain>(res.data, "domains");
    },
    enabled: !!serviceName,
  });
}

export function useAddServerSecondaryDns(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (domain: string) => {
      const res = await api.post(`/server-control/${serviceName}/secondary-dns`, { domain });
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.secondaryDns(serviceName) });
    },
  });
}

export function useDeleteServerSecondaryDns(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (domain: string) => {
      const res = await api.delete(`/server-control/${serviceName}/secondary-dns/${encodeURIComponent(domain)}`);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.secondaryDns(serviceName) });
    },
  });
}

export function useServerVirtualMac(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.virtualMac(serviceName || ""),
    queryFn: async (): Promise<PartialList<VirtualMacEntry>> => {
      const res = await api.get(`/server-control/${serviceName}/virtual-mac`);
      return readPartialList<VirtualMacEntry>(res.data, "virtualMacs");
    },
    enabled: !!serviceName,
  });
}

export function useServerVrack(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.vrack(serviceName || ""),
    queryFn: async (): Promise<PartialList<VrackEntry>> => {
      const res = await api.get(`/server-control/${serviceName}/vrack`);
      return readPartialList<VrackEntry>(res.data, "vracks");
    },
    enabled: !!serviceName,
  });
}

export function useServerIpSpecs(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.ipSpecs(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/ip-specs`);
      return (res.data?.ipSpecs || null) as any;
    },
    enabled: !!serviceName,
  });
}

export function useServerNetworkSpecs(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.networkSpecs(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/network-specs`);
      return (res.data?.network || null) as any;
    },
    enabled: !!serviceName && enabled,
  });
}

export interface ReverseDnsRecord {
  ipReverse: string;
  reverse: string;
}

export function useServerReverseDns(serviceName: string | null) {
  return useQuery({
    queryKey: ["server-control", "reverse-dns", serviceName],
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/reverse`);
      return (res.data?.reverses || []) as ReverseDnsRecord[];
    },
    enabled: Boolean(serviceName),
  });
}

export function useSetServerReverseDns(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; reverse: string }) => {
      const res = await api.post(`/server-control/${serviceName}/reverse`, vars);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["server-control", "reverse-dns", serviceName] });
      qc.invalidateQueries({ queryKey: qk.serverControl.ips(serviceName) });
    },
  });
}

export function useDeleteServerReverseDns(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (ip: string) => {
      const res = await api.delete(`/server-control/${serviceName}/reverse/${encodeURIComponent(ip)}`);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["server-control", "reverse-dns", serviceName] });
      qc.invalidateQueries({ queryKey: qk.serverControl.ips(serviceName) });
    },
  });
}
