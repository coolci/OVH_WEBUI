import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { toast } from "sonner";
import i18n from "@/i18n";
import { qk } from "@/lib/query";
import { apiMessage, bodyMessage, errorStatus, errorField, type BodyMessage } from "@/lib/api-error";
import type { ContactChangeRequest } from "@/lib/types";
import { readPartialList, type PartialList } from "../partial-list";
import type {
  EngagementInfo,
  EngagementPricing,
  EngagementRequest,
  EngagementEndStrategy,
  ServerOptionEntry,
  ContactChangeRequestList,
  RetractionInfo,
  SplaListResult,
} from "./types";
import { useControlAccount, useServerAccount, withAccount } from "./account";

export function useEngagement(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.engagement(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/engagement`, withAccount(accountId));
      return (res.data?.engagement || null) as EngagementInfo | null;
    },
    enabled: !!serviceName,
  });
}

export function useEngagementAvailable(serviceName: string | null, enabled = true) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.engagementAvailable(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/engagement/available`, withAccount(accountId));
      return (res.data?.pricings || []) as EngagementPricing[];
    },
    enabled: !!serviceName && enabled,
  });
}

export function useEngagementRequest(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.engagementRequest(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/engagement/request`, withAccount(accountId));
      return (res.data?.request || null) as EngagementRequest | null;
    },
    enabled: !!serviceName,
  });
}

export function useCreateEngagementRequest(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { pricingMode: string }) => {
      const res = await api.post(`/server-control/${serviceName}/engagement/request`, vars, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.engagement(serviceName, accountId) });
      qc.invalidateQueries({ queryKey: qk.serverControl.engagementRequest(serviceName, accountId) });
    },
  });
}

export function useDeleteEngagementRequest(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const res = await api.delete(`/server-control/${serviceName}/engagement/request`, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.engagementRequest(serviceName, accountId) });
    },
  });
}

export function useUpdateEngagementEndRule(serviceName: string) {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { strategy: EngagementEndStrategy | string; confirm?: boolean }) => {
      const res = await api.put(`/server-control/${serviceName}/engagement/end-rule`, vars, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.engagement(serviceName, accountId) });
    },
  });
}

export function useServerOrderable(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.orderable(serviceName || "", accountId),
    queryFn: async () => {
      const [bw, tr, ip] = await Promise.all([
        api.get(`/server-control/${serviceName}/orderable/bandwidth`, withAccount(accountId)).catch(() => ({ data: { success: false } })),
        api.get(`/server-control/${serviceName}/orderable/traffic`, withAccount(accountId)).catch(() => ({ data: { success: false } })),
        api.get(`/server-control/${serviceName}/orderable/ip`, withAccount(accountId)).catch(() => ({ data: { success: false } })),
      ]);
      return {
        bandwidth: bw.data?.success ? bw.data.orderable : null,
        traffic: tr.data?.success ? tr.data.orderable : null,
        ip: ip.data?.success ? ip.data.orderable : null,
      };
    },
    enabled: !!serviceName,
  });
}

export function useServerOptions(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.options(serviceName || "", accountId),
    queryFn: async (): Promise<PartialList<ServerOptionEntry>> => {
      const res = await api.get(`/server-control/${serviceName}/options`, withAccount(accountId));
      return readPartialList<ServerOptionEntry>(res.data, "options");
    },
    enabled: !!serviceName,
  });
}

export function useChangeContact() {
  const accountId = useServerAccount();
  return useMutation({
    mutationFn: async (args: { serviceName: string; admin?: string; tech?: string; billing?: string }) => {
      const res = await api.post(`/server-control/${args.serviceName}/change-contact`, {
        contactAdmin: args.admin || undefined,
        contactTech: args.tech || undefined,
        contactBilling: args.billing || undefined,
      }, withAccount(accountId));
      return res.data;
    },
  });
}

export function useContactChangeRequests(enabled = true) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.contactRequests(accountId),
    queryFn: async (): Promise<ContactChangeRequestList> => {
      try {
        const res = await api.get(`/ovh/contact-change-requests`, withAccount(accountId));
        return {
          requests: (res.data?.data || res.data?.requests || []) as ContactChangeRequest[],
          unsupported: false,
          failedCount: Number(res.data?.failed) || 0,
        };
      } catch (e: unknown) {
        if (errorStatus(e) === 501) {
          return {
            requests: [],
            unsupported: true,
            message: errorField(e, "message") || i18n.t("hooksMsg.server.contactUnsupported"),
            failedCount: 0,
          };
        }
        throw e;
      }
    },
    enabled,
  });
}

export function useContactRequestAction() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: { id: number | string; action: "accept" | "refuse" | "resend"; token?: string }) => {
      if (args.action === "resend") {
        const res = await api.post(`/ovh/contact-change-requests/${args.id}/resend-email`, undefined, withAccount(accountId));
        return res.data;
      }
      const res = await api.post(`/ovh/contact-change-requests/${args.id}/${args.action}`, {
        token: args.token,
      }, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["server-control", "contact-requests"] });
    },
  });
}

export function useRetraction(serviceName: string | null, isVps = false) {
  const accountId = useControlAccount(isVps);
  const prefix = isVps ? "/vps-control" : "/server-control";
  const domain = isVps ? "vps-control" : "server-control";
  return useQuery<RetractionInfo>({
    queryKey: [domain, "retraction", serviceName, accountId],
    queryFn: async () =>
      (await api.get<RetractionInfo>(`${prefix}/${encodeURIComponent(serviceName!)}/retraction`, withAccount(accountId))).data,
    enabled: !!serviceName,
    staleTime: 5 * 60_000,
    retry: false,
    refetchOnWindowFocus: false,
  });
}

export function useRequestRetraction(serviceName: string, isVps = false) {
  const accountId = useControlAccount(isVps);
  const prefix = isVps ? "/vps-control" : "/server-control";
  const domain = isVps ? "vps-control" : "server-control";
  const qc = useQueryClient();
  return useMutation({
    // orderId 必传:后端只撤销用户在窗口里确认过的那张订单,不在提交时重新猜订单
    mutationFn: async (p: { orderId: number; reason: string; comment: string }) =>
      (
        await api.post(`${prefix}/${encodeURIComponent(serviceName)}/retraction`, {
          ...p,
          confirm: true,
        }, withAccount(accountId))
      ).data,
    onSuccess: (d: BodyMessage) => {
      qc.invalidateQueries({ queryKey: [domain, "retraction"] });
      qc.invalidateQueries({ queryKey: [domain, "list"] });
      toast.success(bodyMessage(d) || i18n.t("hooksMsg.server.retractionSubmitted"));
    },
    onError: (e: unknown) => toast.error(apiMessage(e) || i18n.t("hooksMsg.server.retractionFailed")),
  });
}

export function useSplaList(serviceName: string | null, enabled = true) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.spla(serviceName || "", accountId),
    queryFn: async (): Promise<SplaListResult> => {
      const d = (await api.get(`/server-control/${serviceName}/spla`, withAccount(accountId))).data;
      return {
        list: Array.isArray(d?.splaList) ? d.splaList : [],
        partial: d?.partial === true,
      };
    },
    enabled: !!serviceName && enabled,
    staleTime: 60_000,
  });
}

export function hasActiveSpla(r: SplaListResult | undefined, type: string): boolean {
  if (!r) return false;
  return r.list.some(
    (x) => x.type === type && String(x.status || "").toLowerCase() !== "terminated"
  );
}
