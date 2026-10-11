import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import i18n from "@/i18n";
import { qk } from "@/lib/query";
import { apiMessage } from "@/lib/api-error";
import type {
  BackupFtpAccessEntry,
  BackupFtpResult,
  BackupCloudResult,
} from "./types";

export function useServerMonitoring(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.monitoring(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/monitoring`);
      return !!res.data?.monitoring;
    },
    enabled: !!serviceName,
  });
}

export function useToggleMonitoring() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, enabled }: { serviceName: string; enabled: boolean }) => {
      const res = await api.put(`/server-control/${serviceName}/monitoring`, { monitoring: enabled });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.monitoring(vars.serviceName) });
    },
  });
}

export function useServerBurst(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.burst(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/burst`);
        return { burst: res.data?.burst || null, notAvailable: false } as any;
      } catch (e: any) {
        if (e?.response?.status === 404) {
          return { burst: null, notAvailable: true, error: e?.response?.data?.error } as any;
        }
        throw e;
      }
    },
    enabled: !!serviceName,
    retry: false,
  });
}

export function useSetBurst() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, status }: { serviceName: string; status: string }) => {
      const res = await api.put(`/server-control/${serviceName}/burst`, { status });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.burst(vars.serviceName) });
    },
  });
}

export function useServerFirewall(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.firewall(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/firewall`);
        return { firewall: res.data?.firewall || null, notAvailable: false } as any;
      } catch (e: any) {
        if (e?.response?.status === 404) {
          return { firewall: null, notAvailable: true, error: e?.response?.data?.error } as any;
        }
        throw e;
      }
    },
    enabled: !!serviceName,
    retry: false,
  });
}

export function useSetFirewall() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, enabled }: { serviceName: string; enabled: boolean }) => {
      const res = await api.put(`/server-control/${serviceName}/firewall`, { enabled });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.firewall(vars.serviceName) });
    },
  });
}

export function useServerBackupFtp(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.backupFtp(serviceName || ""),
    queryFn: async (): Promise<BackupFtpResult> => {
      try {
        const res = await api.get(`/server-control/${serviceName}/backup-ftp`);
        if (res.data?.success === false) {
          return {
            notAvailable: true,
            unknownService: res.data?.unknownService === true,
            error: res.data?.error,
            reason: res.data?.reason,
          };
        }
        let accessList: BackupFtpAccessEntry[] = [];
        let accessFailedCount = 0;
        let accessError: string | undefined;
        try {
          const accRes = await api.get(`/server-control/${serviceName}/backup-ftp/access`);
          accessList = (accRes.data?.accessList || []) as BackupFtpAccessEntry[];
          accessFailedCount = Number(accRes.data?.failedCount) || 0;
        } catch (e: any) {
          accessError = apiMessage(e) || e?.message || i18n.t("hooksMsg.server.backupAclLoadFailed");
        }
        return { backupFtp: res.data?.backupFtp || null, accessList, accessFailedCount, accessError };
      } catch (e: any) {
        if (e?.response?.status === 404) {
          if (e?.response?.data?.notAvailable === true) {
            return { notAvailable: true, error: e?.response?.data?.error };
          }
          return { notActivated: true };
        }
        return { notAvailable: true, error: apiMessage(e) || e?.message };
      }
    },
    enabled: !!serviceName,
  });
}

export function useActivateBackupFtp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/backup-ftp`);
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(serviceName) });
    },
  });
}

export function useDeleteBackupFtp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.delete(`/server-control/${serviceName}/backup-ftp`);
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(serviceName) });
    },
  });
}

export function useResetBackupFtpPassword() {
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/backup-ftp/password`);
      return res.data;
    },
  });
}

export function useBackupFtpAuthorizableBlocks(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: [...qk.serverControl.backupFtpAccess(serviceName || ""), "authorizable"] as const,
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/backup-ftp/authorizable-blocks`);
      return (res.data?.blocks || []) as string[];
    },
    enabled: !!serviceName && enabled,
  });
}

export function useAddBackupFtpAccess() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: {
      serviceName: string;
      ipBlock: string;
      ftp?: boolean;
      nfs?: boolean;
      cifs?: boolean;
    }) => {
      const res = await api.post(`/server-control/${vars.serviceName}/backup-ftp/access`, {
        ipBlock: vars.ipBlock,
        ftp: vars.ftp ?? true,
        nfs: !!vars.nfs,
        cifs: !!vars.cifs,
      });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(vars.serviceName) });
    },
  });
}

export function useDeleteBackupFtpAccess() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { serviceName: string; ipBlock: string }) => {
      const res = await api.delete(
        `/server-control/${vars.serviceName}/backup-ftp/access?ipBlock=${encodeURIComponent(vars.ipBlock)}`
      );
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(vars.serviceName) });
    },
  });
}

export function useServerBackupCloud(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.backupCloud(serviceName || ""),
    queryFn: async (): Promise<BackupCloudResult> => {
      try {
        const res = await api.get(`/server-control/${serviceName}/backup-cloud`);
        if (res.data?.success === false) {
          return {
            notAvailable: true,
            unknownService: res.data?.unknownService === true,
            error: res.data?.error,
            reason: res.data?.reason,
          };
        }
        return { backupCloud: res.data?.backupCloud || null };
      } catch (e: any) {
        if (e?.response?.status === 404) {
          if (e?.response?.data?.unknownService === true) {
            return {
              notAvailable: true,
              unknownService: true,
              error: e?.response?.data?.error,
              reason: e?.response?.data?.reason,
            };
          }
          if (e?.response?.data?.notActivated === true) {
            return { notActivated: true };
          }
        }
        return { notAvailable: true, error: apiMessage(e) || e?.message };
      }
    },
    enabled: !!serviceName,
  });
}

export function useActivateBackupCloud() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { serviceName: string; cloudProjectId?: string; projectDescription?: string }) => {
      const res = await api.post(`/server-control/${vars.serviceName}/backup-cloud`, {
        cloudProjectId: vars.cloudProjectId,
        projectDescription: vars.projectDescription,
      });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupCloud(vars.serviceName) });
    },
  });
}

export function useDeleteBackupCloud() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.delete(`/server-control/${serviceName}/backup-cloud`);
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupCloud(serviceName) });
    },
  });
}

export function useChangeBackupCloudPassword() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/backup-cloud/password`);
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupCloud(serviceName) });
    },
  });
}

export function useBackupCloudOfferDetails(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.backupCloudOffer(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/backup-cloud/offer-details`);
      return res.data;
    },
    enabled: !!serviceName,
  });
}
