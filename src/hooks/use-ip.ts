import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import api from "@/lib/api";
import { useActiveAccount } from "@/hooks/use-active-account";
import type { IpAsset, IpReverse, IpFirewall, IpFirewallRule } from "@/lib/types";

export function useIps(overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["ips", accountId],
    queryFn: async () => {
      const res = await api.getIps(accountId);
      return (res.ips || []) as IpAsset[];
    },
    staleTime: 30_000,
  });
}

export function useMoveIp() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({ ip, to, accountId }: { ip: string; to: string; accountId?: string }) => {
      return api.moveIpToService(ip, to, accountId || activeAccountId || undefined);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ips"] });
    },
  });
}

export function useIpReverse(ip: string | null, overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["ip-reverse", ip, accountId],
    queryFn: async () => {
      if (!ip) return [];
      const res = await api.getIpReverse(ip, accountId);
      return (res.reverses || []) as IpReverse[];
    },
    enabled: !!ip,
  });
}

export function useSetIpReverse() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ip,
      ipReverse,
      reverse,
      accountId,
    }: {
      ip: string;
      ipReverse: string;
      reverse: string;
      accountId?: string;
    }) => {
      return api.setIpReverse(ip, ipReverse, reverse, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ip-reverse", variables.ip] });
    },
  });
}

export function useDeleteIpReverse() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ip,
      ipReverse,
      accountId,
    }: {
      ip: string;
      ipReverse: string;
      accountId?: string;
    }) => {
      return api.deleteIpReverse(ip, ipReverse, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ip-reverse", variables.ip] });
    },
  });
}

export function useIpFirewall(ip: string | null, overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["ip-firewall", ip, accountId],
    queryFn: async () => {
      if (!ip) return [];
      const res = await api.getIpFirewall(ip, accountId);
      return (res.firewalls || []) as IpFirewall[];
    },
    enabled: !!ip,
  });
}

export function useCreateIpFirewall() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ip,
      ipOnFirewall,
      accountId,
    }: {
      ip: string;
      ipOnFirewall: string;
      accountId?: string;
    }) => {
      return api.createIpFirewall(ip, ipOnFirewall, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ip-firewall", variables.ip] });
    },
  });
}

export function useToggleIpFirewall() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ip,
      ipOnFirewall,
      enabled,
      accountId,
    }: {
      ip: string;
      ipOnFirewall: string;
      enabled: boolean;
      accountId?: string;
    }) => {
      return api.toggleIpFirewall(ip, ipOnFirewall, enabled, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ip-firewall", variables.ip] });
    },
  });
}

export function useIpFirewallRules(ip: string | null, ipOnFirewall: string | null, overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["ip-firewall-rules", ip, ipOnFirewall, accountId],
    queryFn: async () => {
      if (!ip || !ipOnFirewall) return [];
      const res = await api.getIpFirewallRules(ip, ipOnFirewall, accountId);
      return (res.rules || []) as IpFirewallRule[];
    },
    enabled: !!ip && !!ipOnFirewall,
  });
}

export function useCreateIpFirewallRule() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ip,
      rule,
      accountId,
    }: {
      ip: string;
      rule: Record<string, unknown>;
      accountId?: string;
    }) => {
      return api.createIpFirewallRule(ip, rule, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ip-firewall-rules", variables.ip] });
    },
  });
}

export function useDeleteIpFirewallRule() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ip,
      ipOnFirewall,
      sequence,
      accountId,
    }: {
      ip: string;
      ipOnFirewall: string;
      sequence: number;
      accountId?: string;
    }) => {
      return api.deleteIpFirewallRule(ip, ipOnFirewall, sequence, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["ip-firewall-rules", variables.ip] });
    },
  });
}
