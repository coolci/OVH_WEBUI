import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { toast } from "sonner";
import i18n from "@/i18n";
import { qk } from "@/lib/query";
import { apiMessage, bodyMessage } from "@/lib/api-error";
import type { BootMode, ServerTask, RescueStatus } from "./types";

export function useServerBootModes(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.bootModes(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/boot-mode`);
      return (res.data?.bootModes || []) as BootMode[];
    },
    enabled: !!serviceName && enabled,
  });
}

export function useSetServerBootMode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ serviceName, bootId }: { serviceName: string; bootId: number }) => {
      const res = await api.put(`/server-control/${serviceName}/boot-mode`, { bootId });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.bootModes(vars.serviceName) });
    },
  });
}

export function useServerTasks(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.tasks(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/tasks`);
      return (res.data?.tasks || []) as ServerTask[];
    },
    enabled: !!serviceName && enabled,
  });
}

export function useRebootServer() {
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/reboot`);
      return res.data;
    },
  });
}

export function useRescueStatus(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.rescue(serviceName || ""),
    queryFn: async (): Promise<RescueStatus> => {
      const d = (await api.get(`/server-control/${serviceName}/rescue`)).data;
      return {
        inRescue: d?.inRescue === true,
        currentBoot: Number(d?.currentBoot) || 0,
        rescueMail: d?.rescueMail || "",
        boots: Array.isArray(d?.boots) ? d.boots : [],
      };
    },
    enabled: !!serviceName && enabled,
    staleTime: 30_000,
  });
}

export function useEnterRescue(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    // expectedBoot = 打开窗口时看到的启动项;期间被别处改过的话后端返回 409,不会按过期页面重启。
    // 邮箱留空时必须显式声明 useDefaultEmail,否则后端拒绝(省略 rescueMail 会沿用旧地址)。
    mutationFn: async (v: { email?: string; sshKey?: string; bootId?: number; expectedBoot?: number }) =>
      (
        await api.post(`/server-control/${serviceName}/rescue`, {
          ...v,
          useDefaultEmail: !v.email && !v.sshKey,
          confirm: true,
        })
      ).data,
    onSuccess: (d: any) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.bootModes(serviceName) });
      toast.success(bodyMessage(d) || i18n.t("hooksMsg.server.rescueEntered"));
      qc.invalidateQueries({ queryKey: qk.serverControl.rescue(serviceName) });
    },
    onError: (e: any) => toast.error(apiMessage(e) || i18n.t("hooksMsg.server.rescueEnterFailed")),
  });
}

export function useExitRescue(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (expectedBoot?: number) =>
      (await api.post(`/server-control/${serviceName}/rescue/exit`, { confirm: true, expectedBoot })).data,
    onSuccess: (d: any) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.bootModes(serviceName) });
      toast.success(bodyMessage(d) || i18n.t("hooksMsg.server.rescueExited"));
      qc.invalidateQueries({ queryKey: qk.serverControl.rescue(serviceName) });
    },
    onError: (e: any) => toast.error(apiMessage(e) || i18n.t("hooksMsg.server.rescueExitFailed")),
  });
}

export function useTaskTimeslots(
  serviceName: string | null,
  taskId: number | null,
  periodStart: string,
  periodEnd: string,
  enabled = true
) {
  return useQuery({
    queryKey: qk.serverControl.taskTimeslots(serviceName || "", taskId || 0, periodStart, periodEnd),
    queryFn: async () => {
      const res = await api.get(
        `/server-control/${serviceName}/tasks/${taskId}/available-timeslots?periodStart=${encodeURIComponent(periodStart)}&periodEnd=${encodeURIComponent(periodEnd)}`
      );
      return {
        timeslots: (res.data?.timeslots || []) as any[],
        scheduleNotRequired: !!res.data?.scheduleNotRequired,
      };
    },
    enabled: !!serviceName && !!taskId && enabled,
  });
}

export function useScheduleTask() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: {
      serviceName: string;
      taskId: number;
      wantedBeginingDate: string;
      hasPerformedBackup: boolean;
    }) => {
      const res = await api.post(`/server-control/${args.serviceName}/tasks/${args.taskId}/schedule`, {
        wantedBeginingDate: args.wantedBeginingDate,
        hasPerformedBackup: args.hasPerformedBackup,
      });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.tasks(vars.serviceName) });
    },
  });
}
