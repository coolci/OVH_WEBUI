import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { useActiveAccount } from "@/hooks/use-active-account";
import { readPartialList, type PartialList } from "../partial-list";
import type {
  MitigationBlock,
  NetworkInterface,
  SecondaryDnsDomain,
  VirtualMacEntry,
  VrackEntry,
  IpSpecs,
  NetworkSpecs,
} from "./types";
import { useServerAccount, withAccount } from "./account";

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
        b.mitigations.some((m) => m.state === "creationPending" || m.state === "removalPending"),
      );
      return hasTransition ? 5000 : false;
    },
  });
}

export function useEnableMitigation(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) => {
      const res = await api.post(`/server-control/${serviceName}/mitigation/${vars.ip}?block=${encodeURIComponent(vars.block)}`, undefined, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.mitigation(serviceName, accountId) });
    },
  });
}

export function useDisableMitigation(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) => {
      const res = await api.delete(`/server-control/${serviceName}/mitigation/${vars.ip}?block=${encodeURIComponent(vars.block)}`, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.mitigation(serviceName, accountId) });
    },
  });
}

export function useServerIps(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.ips(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/ips`, withAccount(accountId));
      return (res.data?.ips || []) as Array<{ ip: string; type: string }>;
    },
    enabled: !!serviceName,
  });
}

export function useServerNetworkInterfaces(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.networkInterfaces(serviceName || "", accountId),
    queryFn: async (): Promise<PartialList<NetworkInterface>> => {
      const res = await api.get(`/server-control/${serviceName}/network-interfaces`, withAccount(accountId));
      return readPartialList<NetworkInterface>(res.data, "interfaces");
    },
    enabled: !!serviceName,
  });
}

export function useServerSecondaryDns(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.secondaryDns(serviceName || "", accountId),
    queryFn: async (): Promise<PartialList<SecondaryDnsDomain>> => {
      const res = await api.get(`/server-control/${serviceName}/secondary-dns`, withAccount(accountId));
      return readPartialList<SecondaryDnsDomain>(res.data, "domains");
    },
    enabled: !!serviceName,
  });
}

export function useAddServerSecondaryDns(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (domain: string) => {
      const res = await api.post(`/server-control/${serviceName}/secondary-dns`, { domain }, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.secondaryDns(serviceName, accountId) });
    },
  });
}

export function useDeleteServerSecondaryDns(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (domain: string) => {
      const res = await api.delete(`/server-control/${serviceName}/secondary-dns/${encodeURIComponent(domain)}`, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.secondaryDns(serviceName, accountId) });
    },
  });
}

export function useServerVirtualMac(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.virtualMac(serviceName || "", accountId),
    queryFn: async (): Promise<PartialList<VirtualMacEntry>> => {
      const res = await api.get(`/server-control/${serviceName}/virtual-mac`, withAccount(accountId));
      return readPartialList<VirtualMacEntry>(res.data, "virtualMacs");
    },
    enabled: !!serviceName,
  });
}

export function useServerVrack(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.vrack(serviceName || "", accountId),
    queryFn: async (): Promise<PartialList<VrackEntry>> => {
      const res = await api.get(`/server-control/${serviceName}/vrack`, withAccount(accountId));
      return readPartialList<VrackEntry>(res.data, "vracks");
    },
    enabled: !!serviceName,
  });
}

export function useServerIpSpecs(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.ipSpecs(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/ip-specs`, withAccount(accountId));
      return (res.data?.ipSpecs || null) as IpSpecs | null;
    },
    enabled: !!serviceName,
  });
}

export function useServerNetworkSpecs(serviceName: string | null, enabled = true) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.networkSpecs(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/network-specs`, withAccount(accountId));
      return (res.data?.network || null) as NetworkSpecs | null;
    },
    enabled: !!serviceName && enabled,
  });
}

export interface ReverseDnsRecord {
  ipReverse: string;
  reverse: string;
}

export function useServerReverseDns(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.reverseDns(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/reverse`, withAccount(accountId));
      return (res.data?.reverses || []) as ReverseDnsRecord[];
    },
    enabled: Boolean(serviceName),
  });
}

export function useSetServerReverseDns(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; reverse: string }) => {
      const res = await api.post(`/server-control/${serviceName}/reverse`, vars, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.reverseDns(serviceName, accountId) });
      qc.invalidateQueries({ queryKey: qk.serverControl.ips(serviceName, accountId) });
    },
  });
}

export function useDeleteServerReverseDns(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (ip: string) => {
      const res = await api.delete(`/server-control/${serviceName}/reverse/${encodeURIComponent(ip)}`, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.reverseDns(serviceName, accountId) });
      qc.invalidateQueries({ queryKey: qk.serverControl.ips(serviceName, accountId) });
    },
  });
}
