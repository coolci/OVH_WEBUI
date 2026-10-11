import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import i18n from "@/i18n";
import { qk } from "@/lib/query";
import { useActiveAccount } from "@/hooks/use-active-account";
import type { OwnedServer, ServiceInfo } from "./types";

/**
 * 已购服务器列表（后端返回 { success, servers, total }）
 */
export function useOwnedServers() {
  const [accountId] = useActiveAccount();
  return useQuery({
    queryKey: qk.serverControl.list(accountId),
    queryFn: async () => {
      const res = await api.get("/server-control/list", { params: { account: accountId } });
      const raw = (res.data?.servers || []) as OwnedServer[];
      return raw.filter((s) => {
        const state = s.state?.toLowerCase();
        if (s.error) return true;
        const status = s.status?.toLowerCase();
        if (status === "expired" || status === "suspended") return false;
        if (state === "error" || state === "suspended") return false;
        return state === "ok" || state === "active";
      });
    },
    staleTime: 60_000,
    enabled: !!accountId,
  });
}

/** 服务信息（后端返回 { success, serviceInfo: {...} }） */
export function useServerServiceInfo(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.serviceInfo(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/serviceinfo`);
      return (res.data?.serviceInfo || null) as ServiceInfo | null;
    },
    enabled: !!serviceName,
  });
}

/**
 * 修改续费策略。
 */
export function useUpdateRenewal(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { mode: "auto" | "manual" | "delete"; period?: number }) => {
      const res = await api.put(`/server-control/${serviceName}/serviceinfo/renewal`, vars);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.serviceInfo(serviceName) });
      qc.invalidateQueries({ queryKey: ["server-control", "list"] });
    },
  });
}

/** 终止状态的显示信息 */
export function terminationLabel(info: {
  terminationScheduled?: boolean;
  terminationAction?: string;
  terminationStateUnknown?: boolean;
  renewalDeleteAtExpiration?: boolean;
}): { text: string; danger: boolean; title: string } | null {
  if (info.terminationStateUnknown) {
    return {
      text: i18n.t("hooksMsg.server.termination.unknownText"),
      danger: true,
      title: i18n.t("hooksMsg.server.termination.unknownTitle"),
    };
  }
  const on = info.terminationScheduled ?? info.renewalDeleteAtExpiration;
  if (!on) return null;
  switch (info.terminationAction) {
    case "terminate":
      return {
        text: i18n.t("hooksMsg.server.termination.immediateText"),
        danger: true,
        title: i18n.t("hooksMsg.server.termination.immediateTitle"),
      };
    case "terminateAtEngagementDate":
      return {
        text: i18n.t("hooksMsg.server.termination.engagementText"),
        danger: false,
        title: i18n.t("hooksMsg.server.termination.engagementTitle"),
      };
    case "terminateAtExpirationDate":
    case "deleteAtExpiration":
      return {
        text: i18n.t("hooksMsg.server.termination.expirationText"),
        danger: false,
        title: i18n.t("hooksMsg.server.termination.expirationTitle"),
      };
    default:
      return {
        text: i18n.t("hooksMsg.server.termination.scheduledText"),
        danger: false,
        title: i18n.t("hooksMsg.server.termination.scheduledTitle"),
      };
  }
}

/**
 * 设置终止策略。
 */
export function useUpdateTerminationPolicy(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { policy: string }) =>
      (await api.put(`/server-control/${serviceName}/termination-policy`, vars)).data as {
        success: boolean;
        message: string;
      },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.serviceInfo(serviceName) });
    },
  });
}
