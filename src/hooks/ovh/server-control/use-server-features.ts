import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import i18n from "@/i18n";
import { qk } from "@/lib/query";
import { apiMessage, errorStatus, errorBody, errorField, errorText } from "@/lib/api-error";
import type {
  BackupFtpAccessEntry,
  BackupFtpResult,
  BackupCloudResult,
  BurstResult,
  FirewallResult,
} from "./types";
import { useServerAccount, withAccount } from "./account";

export function useServerMonitoring(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.monitoring(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/monitoring`, withAccount(accountId));
      return !!res.data?.monitoring;
    },
    enabled: !!serviceName,
  });
}

export function useToggleMonitoring() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, enabled }: { serviceName: string; enabled: boolean }) => {
      const res = await api.put(`/server-control/${serviceName}/monitoring`, { monitoring: enabled }, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.monitoring(vars.serviceName, accountId) });
    },
  });
}

export function useServerBurst(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.burst(serviceName || "", accountId),
    queryFn: async (): Promise<BurstResult> => {
      try {
        const res = await api.get(`/server-control/${serviceName}/burst`, withAccount(accountId));
        return { burst: res.data?.burst || null, notAvailable: false };
      } catch (e: unknown) {
        if (errorStatus(e) === 404) {
          return { burst: null, notAvailable: true, error: errorField(e, "error") };
        }
        throw e;
      }
    },
    enabled: !!serviceName,
    retry: false,
  });
}

export function useSetBurst() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, status }: { serviceName: string; status: string }) => {
      const res = await api.put(`/server-control/${serviceName}/burst`, { status }, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.burst(vars.serviceName, accountId) });
    },
  });
}

export function useServerFirewall(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.firewall(serviceName || "", accountId),
    queryFn: async (): Promise<FirewallResult> => {
      try {
        const res = await api.get(`/server-control/${serviceName}/firewall`, withAccount(accountId));
        return { firewall: res.data?.firewall || null, notAvailable: false };
      } catch (e: unknown) {
        if (errorStatus(e) === 404) {
          return { firewall: null, notAvailable: true, error: errorField(e, "error") };
        }
        throw e;
      }
    },
    enabled: !!serviceName,
    retry: false,
  });
}

export function useSetFirewall() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, enabled }: { serviceName: string; enabled: boolean }) => {
      const res = await api.put(`/server-control/${serviceName}/firewall`, { enabled }, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.firewall(vars.serviceName, accountId) });
    },
  });
}

export function useServerBackupFtp(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.backupFtp(serviceName || "", accountId),
    queryFn: async (): Promise<BackupFtpResult> => {
      try {
        const res = await api.get(`/server-control/${serviceName}/backup-ftp`, withAccount(accountId));
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
          const accRes = await api.get(`/server-control/${serviceName}/backup-ftp/access`, withAccount(accountId));
          accessList = (accRes.data?.accessList || []) as BackupFtpAccessEntry[];
          accessFailedCount = Number(accRes.data?.failedCount) || 0;
        } catch (e: unknown) {
          accessError = apiMessage(e) || errorText(e) || i18n.t("hooksMsg.server.backupAclLoadFailed");
        }
        return { backupFtp: res.data?.backupFtp || null, accessList, accessFailedCount, accessError };
      } catch (e: unknown) {
        if (errorStatus(e) === 404) {
          if (errorBody(e).notAvailable === true) {
            return { notAvailable: true, error: errorField(e, "error") };
          }
          return { notActivated: true };
        }
        return { notAvailable: true, error: apiMessage(e) || errorText(e) };
      }
    },
    enabled: !!serviceName,
  });
}

export function useActivateBackupFtp() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/backup-ftp`, undefined, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(serviceName, accountId) });
    },
  });
}

export function useDeleteBackupFtp() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.delete(`/server-control/${serviceName}/backup-ftp`, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(serviceName, accountId) });
    },
  });
}

export function useResetBackupFtpPassword() {
  const accountId = useServerAccount();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/backup-ftp/password`, undefined, withAccount(accountId));
      return res.data;
    },
  });
}

export function useBackupFtpAuthorizableBlocks(serviceName: string | null, enabled = true) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: [...qk.serverControl.backupFtpAccess(serviceName || "", accountId), "authorizable"] as const,
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/backup-ftp/authorizable-blocks`, withAccount(accountId));
      return (res.data?.blocks || []) as string[];
    },
    enabled: !!serviceName && enabled,
  });
}

export function useAddBackupFtpAccess() {
  const accountId = useServerAccount();
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
      }, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(vars.serviceName, accountId) });
    },
  });
}

export function useDeleteBackupFtpAccess() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { serviceName: string; ipBlock: string }) => {
      const res = await api.delete(
        `/server-control/${vars.serviceName}/backup-ftp/access?ipBlock=${encodeURIComponent(vars.ipBlock)}`,
        withAccount(accountId)
      );
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupFtp(vars.serviceName, accountId) });
    },
  });
}

export function useServerBackupCloud(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.backupCloud(serviceName || "", accountId),
    queryFn: async (): Promise<BackupCloudResult> => {
      try {
        const res = await api.get(`/server-control/${serviceName}/backup-cloud`, withAccount(accountId));
        if (res.data?.success === false) {
          return {
            notAvailable: true,
            unknownService: res.data?.unknownService === true,
            error: res.data?.error,
            reason: res.data?.reason,
          };
        }
        return { backupCloud: res.data?.backupCloud || null };
      } catch (e: unknown) {
        if (errorStatus(e) === 404) {
          if (errorBody(e).unknownService === true) {
            return {
              notAvailable: true,
              unknownService: true,
              error: errorField(e, "error"),
              reason: errorField(e, "reason"),
            };
          }
          if (errorBody(e).notActivated === true) {
            return { notActivated: true };
          }
        }
        return { notAvailable: true, error: apiMessage(e) || errorText(e) };
      }
    },
    enabled: !!serviceName,
  });
}

export function useActivateBackupCloud() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { serviceName: string; cloudProjectId?: string; projectDescription?: string }) => {
      const res = await api.post(`/server-control/${vars.serviceName}/backup-cloud`, {
        cloudProjectId: vars.cloudProjectId,
        projectDescription: vars.projectDescription,
      }, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupCloud(vars.serviceName, accountId) });
    },
  });
}

export function useDeleteBackupCloud() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.delete(`/server-control/${serviceName}/backup-cloud`, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupCloud(serviceName, accountId) });
    },
  });
}

export function useChangeBackupCloudPassword() {
  const accountId = useServerAccount();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/backup-cloud/password`, undefined, withAccount(accountId));
      return res.data;
    },
    onSuccess: (_, serviceName) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.backupCloud(serviceName, accountId) });
    },
  });
}

export function useBackupCloudOfferDetails(serviceName: string | null) {
  const accountId = useServerAccount();
  return useQuery({
    queryKey: qk.serverControl.backupCloudOffer(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/backup-cloud/offer-details`, withAccount(accountId));
      return res.data;
    },
    enabled: !!serviceName,
  });
}
