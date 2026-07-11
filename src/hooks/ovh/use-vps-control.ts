import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";

/* ────────────── 类型定义 ────────────── */

export interface OwnedVps {
  serviceName: string;
  name: string;
  displayName: string;
  state: string;        // "running" / "stopped" / "migrating" / "installing"
  cluster: string;
  zone: string;
  keymap: string;
  netbootMode: string;
  offerType: string;
  slaMonitoring: boolean | null;
  lockStatus: string;
  model: string;        // vps.Model.name
  vcore: number;
  memoryMB: number;
  diskGB: number;
  status: string;       // billing status
  renewalType: boolean;
  error?: string;
}

export interface VpsServiceInfo {
  status: string;
  expiration: string;
  creation: string;
  renewalType: boolean;
  renewalPeriod: number;
  renewalDeleteAtExpiration: boolean;
  renewalForced: boolean;
  renewalManualPayment: boolean;
  possibleRenewPeriod: number[];
}

export interface VpsIp {
  ipAddress: string;
  reverse?: string;
  type?: string;
  version?: string;
  gateway?: string;
  geolocation?: string;
  macAddress?: string;
}

export interface VpsTemplate {
  /** EU 返回 long, US 返回 string —— 直接当 ID 回传给 reinstall 接口即可 */
  id: number | string;
  name: string;
  distribution: string;
  bitFormat: number;
  locale: string;
  availableLanguage: string[];
}

/** templates 接口返回的 kind:
 *   "templateId" - EU,reinstall body 用 templateId (long)
 *   "imageId"    - US,rebuild body 用 imageId (string),前端只展示用,后端按 endpoint 自动分路 */
export type TemplateKind = "templateId" | "imageId";

export interface VpsTask {
  id: number;
  type: string;
  state: string;       // todo / doing / done / cancelled / paused
  date: string;
  progress: number;
}

export interface VpsSnapshot {
  id: string;
  creationDate: string;
  description: string;
  region: string;
}

/* ────────────── List + Info + Status ────────────── */

export function useOwnedVps() {
  return useQuery({
    queryKey: qk.vpsControl.list(),
    queryFn: async () => {
      const res = await api.get("/vps-control/list");
      return (res.data?.vps || []) as OwnedVps[];
    },
    staleTime: 60_000,
  });
}

export function useVpsInfo(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.info(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/info`);
      return res.data?.info as Record<string, any> | null;
    },
    enabled: !!svc,
  });
}

/** VPS 网络服务存活探测(ping/dns/http/https/smtp/ssh) — 跟 info.state 不一样 */
export function useVpsServiceStatus(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.status(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/status`);
      return res.data?.status as Record<string, any> | null;
    },
    enabled: !!svc,
    staleTime: 30_000,
  });
}

export function useVpsServiceInfo(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.serviceInfo(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/serviceinfo`);
      return (res.data?.serviceInfo || null) as VpsServiceInfo | null;
    },
    enabled: !!svc,
  });
}

export function useUpdateVpsRenewal(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { mode: "auto" | "manual" | "delete"; period?: number }) => {
      const res = await api.put(`/vps-control/${svc}/serviceinfo/renewal`, vars);
      return res.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(svc) }),
  });
}

export function useVpsIps(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.ips(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/ips`);
      return (res.data?.ips || []) as VpsIp[];
    },
    enabled: !!svc,
  });
}

export function useSetVpsIpReverse(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; reverse: string }) => {
      const res = await api.put(`/vps-control/${svc}/ips/${vars.ip}/reverse`, { reverse: vars.reverse });
      return res.data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.ips(svc) }),
  });
}

export function useVpsDatacenter(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.datacenter(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/datacenter`);
      return res.data?.datacenter as { name: string; longName: string; country: string } | null;
    },
    enabled: !!svc,
  });
}

// VPS CPU/内存监控端点已被 OVH 全面 DEPRECATED:
//   /vps/{name}/monitoring  - DEPRECATED 2024-07,计划 2024-09 删除
//   /vps/{name}/statistics  - DEPRECATED 2023-11,计划 2024-01 删除
// OVH 没有提供新的 VPS 级 CPU/内存监控端点,只剩磁盘监控(/disks/{id}/use)。
// 移除监控功能,需要看负载请登录 VPS 用 top/htop。

/* ────────────── Power ────────────── */

export function useVpsStart(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${svc}/start`)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.list() });
      qc.invalidateQueries({ queryKey: qk.vpsControl.info(svc) });
    },
  });
}

export function useVpsStop(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${svc}/stop`)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.list() });
      qc.invalidateQueries({ queryKey: qk.vpsControl.info(svc) });
    },
  });
}

export function useVpsReboot(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${svc}/reboot`)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.list() });
      qc.invalidateQueries({ queryKey: qk.vpsControl.info(svc) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc) });
    },
  });

export function useVpsConsoleUrl(svc: string) {
  return useMutation({
    mutationFn: async () => {
      const res = await api.post(`/vps-control/${svc}/console`);
      return res.data?.url as string;
    },
  });
}

export function useVpsSetPassword(svc: string) {
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${svc}/password`)).data,
  });
}

/* ────────────── Reinstall ────────────── */

/** 当前安装的系统信息(EU /distribution / US /images/current) */
export function useVpsCurrentOS(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.currentOS(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/current-os`);
      return res.data?.currentOS as {
        id: number | string;
        name: string;
        distribution: string;
        bitFormat: number;
        locale: string;
        source: string;
      } | null;
    },
    enabled: !!svc,
    staleTime: 5 * 60_000,
  });
}

export function useVpsTemplates(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.templates(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/templates`);
      return (res.data?.templates || []) as VpsTemplate[];
    },
    enabled: !!svc,
    staleTime: 5 * 60_000,
  });
}

export function useReinstallVps(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: {
      templateId: number | string;
      language?: string;
      sshKey?: string[];
      doNotSendPassword?: boolean;
      softwareId?: number[];
    }) => (await api.post(`/vps-control/${svc}/reinstall`, vars)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc) });
    },
  });
}

/* ────────────── Tasks ────────────── */

export function useVpsTasks(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.tasks(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/tasks`);
      return (res.data?.tasks || []) as VpsTask[];
    },
    enabled: !!svc,
  });
}

export function useVpsTask(svc: string | null, taskId: number | string | null, refetchInterval = 0) {
  return useQuery({
    queryKey: qk.vpsControl.task(svc || "", taskId || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/tasks/${taskId}`);
      return res.data?.task as VpsTask | null;
    },
    enabled: !!svc && !!taskId,
    refetchInterval,
  });
}

/* ────────────── Snapshot ────────────── */

export function useVpsSnapshot(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.snapshot(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/snapshot`);
      return res.data?.snapshot as VpsSnapshot | null;
    },
    enabled: !!svc,
  });
}

export function useCreateVpsSnapshot(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { description?: string }) => (await api.post(`/vps-control/${svc}/snapshot`, vars)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc) }),
  });
}

export function useUpdateVpsSnapshot(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { description: string }) => (await api.put(`/vps-control/${svc}/snapshot`, vars)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc) }),
  });
}

export function useRevertVpsSnapshot(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${svc}/snapshot/revert`)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.list() });
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc) });
    },
  });
}

export function useDeleteVpsSnapshot(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => (await api.delete(`/vps-control/${svc}/snapshot`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc) }),
  });
}

/* ────────────── Misc ────────────── */

export function useChangeVpsContact() {
  return useMutation({
    mutationFn: async (vars: { serviceName: string; admin?: string; tech?: string; billing?: string }) => {
      const body: Record<string, string> = {};
      if (vars.admin) body.contactAdmin = vars.admin;
      if (vars.tech) body.contactTech = vars.tech;
      if (vars.billing) body.contactBilling = vars.billing;
      return (await api.post(`/vps-control/${vars.serviceName}/change-contact`, body)).data;
    },
  });
}

export function useTerminateVps() {
  return useMutation({
    mutationFn: async (vars: { serviceName: string }) =>
      (await api.post(`/vps-control/${vars.serviceName}/terminate`)).data,
  });
}

export function useConfirmTerminateVps() {
  return useMutation({
    mutationFn: async (vars: { serviceName: string; token: string; reason?: string; commentary?: string }) =>
      (await api.post(`/vps-control/${vars.serviceName}/confirm-termination`, {
        token: vars.token,
        reason: vars.reason,
        commentary: vars.commentary,
      })).data,
  });
}

export function useVpsSecondaryDns(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.secondaryDns(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/secondary-dns`);
      return (res.data?.domains || []) as any[];
    },
    enabled: !!svc,
  });
}

export function useAddVpsSecondaryDns(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { domain: string; ip: string }) =>
      (await api.post(`/vps-control/${svc}/secondary-dns`, vars)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.secondaryDns(svc) }),
  });
}

export function useDeleteVpsSecondaryDns(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (domain: string) =>
      (await api.delete(`/vps-control/${svc}/secondary-dns/${domain}`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.secondaryDns(svc) }),
  });
}

export function useVpsOptions(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.options(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/options`);
      return (res.data?.options || []) as any[];
    },
    enabled: !!svc,
  });
}

export function useDeleteVpsOption(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (option: string) =>
      (await api.delete(`/vps-control/${svc}/options/${option}`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.options(svc) }),
  });
}

/* ────────────── Engagement(合同期) ────────────── */

export function useVpsEngagement(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.engagement(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/engagement`);
      return res.data?.engagement as { currentPeriod?: any; endRule?: any } | null;
    },
    enabled: !!svc,
  });
}

export function useVpsEngagementAvailable(svc: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.vpsControl.engagementAvailable(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/engagement/available`);
      return (res.data?.pricings || []) as any[];
    },
    enabled: !!svc && enabled,
  });
}

export function useVpsEngagementRequest(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.engagementRequest(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/engagement/request`);
      return res.data?.request as any | null;
    },
    enabled: !!svc,
  });
}

export function useCreateVpsEngagementRequest(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { pricingMode: string }) =>
      (await api.post(`/vps-control/${svc}/engagement/request`, vars)).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.engagement(svc) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.engagementRequest(svc) });
    },
  });
}

export function useDeleteVpsEngagementRequest(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () =>
      (await api.delete(`/vps-control/${svc}/engagement/request`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.engagementRequest(svc) }),
  });
}

export function useUpdateVpsEngagementEndRule(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { strategy: string }) =>
      (await api.put(`/vps-control/${svc}/engagement/end-rule`, vars)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.engagement(svc) }),
  });
}

/* ────────────── DDoS Mitigation ────────────── */

export interface VpsMitigationIp {
  ipOnMitigation: string;
  state: string;
  auto: boolean;
  permanent: boolean;
}

export interface VpsMitigationBlock {
  ipBlock: string;
  mitigations: VpsMitigationIp[];
  error?: string;
}

export function useVpsMitigation(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.mitigation(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/mitigation`);
      return (res.data?.ips || []) as VpsMitigationBlock[];
    },
    enabled: !!svc,
    // 有过渡态(creationPending/removalPending)时每 5 秒轮询一次,稳定就停
    refetchInterval: (q) => {
      const data = q.state.data as VpsMitigationBlock[] | undefined;
      if (!data) return false;
      const hasTransition = data.some((b) =>
        b.mitigations.some((m) => m.state === "creationPending" || m.state === "removalPending"),
      );
      return hasTransition ? 5000 : false;
    },
  });
}

export function useEnableVpsMitigation(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) =>
      (await api.post(`/vps-control/${svc}/mitigation/${vars.ip}?block=${encodeURIComponent(vars.block)}`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.mitigation(svc) }),
  });
}

export function useDisableVpsMitigation(svc: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) =>
      (await api.delete(`/vps-control/${svc}/mitigation/${vars.ip}?block=${encodeURIComponent(vars.block)}`)).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.mitigation(svc) }),
  });
}

export function useVpsAutomatedBackup(svc: string | null) {
  return useQuery({
    queryKey: qk.vpsControl.automatedBackup(svc || ""),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${svc}/automated-backup`);
      return res.data?.automatedBackup as { rotation: number; schedule: string; state: string } | null;
    },
    enabled: !!svc,
  });
}
