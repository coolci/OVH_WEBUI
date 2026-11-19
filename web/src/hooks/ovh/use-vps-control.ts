import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import type { PartialList } from "./partial-list";
import { useActiveVpsControlAccount } from "@/hooks/use-active-account";
import { useControlAccount, withAccount } from "./server-control/account";
import type { EngagementInfo, EngagementPricing, EngagementRequest, ServiceInfo } from "./server-control/types";

function useVpsAccount(overrideAccountId?: string): string {
  return useControlAccount(true, overrideAccountId);
}

/**
 * 统一失效单个 VPS 实例的完整生命周期关联缓存（列表、详情、状态、任务、当前系统、快照、计费服务信息）。
 * 覆盖重启、启停、重装、快照回滚、服务终止/取消等全部写操作，防止界面长期停留旧状态诱导重复提交。
 */
export function invalidateVpsLifecycle(
  qc: { invalidateQueries: (filters: { queryKey: readonly unknown[] }) => unknown },
  svc: string,
  accountId: string = "",
): void {
  qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
  qc.invalidateQueries({ queryKey: qk.vpsControl.info(svc, accountId) });
  qc.invalidateQueries({ queryKey: qk.vpsControl.status(svc, accountId) });
  qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(svc, accountId) });
  qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc, accountId) });
  qc.invalidateQueries({ queryKey: qk.vpsControl.currentOS(svc, accountId) });
  qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc, accountId) });
}

/** 二级 DNS:老接口返回域名字符串,新接口返回对象 */
export type VpsSecondaryDnsEntry = string | { domain: string; dns?: string; ipMaster?: string };

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
  /** billing status。serviceInfos 拉取失败时为 null —— 「没查到」不是一种状态，别渲染成「unknown」 */
  status: string | null;
  /** 是否自动续费。null = 这次没查到（serviceInfos 失败或 renew 为空），必须和 false（确实没开）分开显示 */
  renewalType: boolean | null;
  error?: string;
  /** 该 VPS 归属的 OVH 账户 ID */
  accountId?: string;
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
  /** 终止状态权威来源(lifecycle.pendingActions);读不到时后端不下发 */
  terminationScheduled?: boolean;
  /** 哪种终止(立即 / 到期 / 合同期结束),见独服侧同名字段说明 */
  terminationAction?: string;
  /** 读 lifecycle 失败,状态未经验证 */
  terminationStateUnknown?: boolean;
  terminationDate?: string;
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

export function useOwnedVps(overrideAccountId?: string) {
  const [activeId] = useActiveVpsControlAccount();
  const accountId = overrideAccountId || activeId;
  return useQuery({
    queryKey: qk.vpsControl.list(accountId),
    queryFn: async () => {
      const res = await api.get("/vps-control/list", { params: { account: accountId } });
      return ((res.data?.vps || []) as OwnedVps[]).map((v) => ({
        ...v,
        accountId: v.accountId || accountId,
      }));
    },
    staleTime: 60_000,
  });
}

export function useVpsInfo(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.info(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/info`, withAccount(accountId));
      return (res.data?.info || null) as ServiceInfo | null;
    },
    enabled: !!svc,
  });
}

export function useVpsServiceInfo(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.serviceInfo(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/serviceinfo`, withAccount(accountId));
      return (res.data?.serviceInfo || null) as VpsServiceInfo | null;
    },
    enabled: !!svc,
  });
}

export function useUpdateVpsRenewal(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { mode: "auto" | "manual" | "delete"; period?: number }) => {
      const res = await api.put(`/vps-control/${encodeURIComponent(svc)}/serviceinfo/renewal`, vars, withAccount(accountId));
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.info(svc, accountId) });
      qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
    },
  });
}

export function useVpsIps(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.ips(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/ips`, withAccount(accountId));
      return (res.data?.ips || []) as VpsIp[];
    },
    enabled: !!svc,
  });
}

export function useSetVpsIpReverse(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { ip: string; reverse: string }) => {
      const res = await api.put(
        `/vps-control/${encodeURIComponent(svc)}/ips/${encodeURIComponent(vars.ip)}/reverse`,
        { reverse: vars.reverse },
        withAccount(accountId),
      );
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.ips(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc, accountId) });
    },
  });
}

export function useVpsDatacenter(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.datacenter(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/datacenter`, withAccount(accountId));
      return res.data?.datacenter as { name: string; longName: string; country: string } | null;
    },
    enabled: !!svc,
  });
}

/* ────────────── Power ────────────── */

export function useVpsStart(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${encodeURIComponent(svc)}/start`, undefined, withAccount(accountId))).data,
    onSuccess: () => {
      invalidateVpsLifecycle(qc, svc, accountId);
    },
  });
}

export function useVpsStop(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${encodeURIComponent(svc)}/stop`, undefined, withAccount(accountId))).data,
    onSuccess: () => {
      invalidateVpsLifecycle(qc, svc, accountId);
    },
  });
}

export function useVpsReboot(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () => (await api.post(`/vps-control/${encodeURIComponent(svc)}/reboot`, undefined, withAccount(accountId))).data,
    onSuccess: () => {
      invalidateVpsLifecycle(qc, svc, accountId);
    },
  });
}

export function useVpsConsoleUrl(svc: string, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () => {
      const res = await api.post(`/vps-control/${encodeURIComponent(svc)}/console`, undefined, withAccount(accountId));
      return res.data?.url as string;
    },
  });
}

/* ────────────── Reinstall ────────────── */

export function useVpsCurrentOS(svc: string | null, enabled = true, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.currentOS(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/current-os`, withAccount(accountId));
      return res.data?.currentOS as {
        id: number | string;
        name: string;
        distribution: string;
        bitFormat: number;
        locale: string;
        source: string;
      } | null;
    },
    enabled: !!svc && enabled,
    staleTime: 5 * 60_000,
  });
}

export interface VpsTemplateList extends PartialList<VpsTemplate> {
  kind: TemplateKind | "";
}

export function useVpsTemplates(svc: string | null, enabled = true, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.templates(svc || "", accountId),
    queryFn: async (): Promise<VpsTemplateList> => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/templates`, withAccount(accountId));
      const failedCount = Number(res.data?.failed) || 0;
      return {
        items: (res.data?.templates || []) as VpsTemplate[],
        kind: (res.data?.kind || "") as TemplateKind | "",
        partial: res.data?.partial === true || failedCount > 0,
        failedCount,
      };
    },
    enabled: !!svc && enabled,
    staleTime: 5 * 60_000,
  });
}

export function useReinstallVps(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: {
      templateId: number | string;
      sshKey?: string[];
      doNotSendPassword?: boolean;
    }) => {
      const validKeys = (vars.sshKey || [])
        .map((k) => (k || "").trim())
        .filter((k) => k.length > 0 && !/^(ssh-(rsa|ed25519|dss)|ecdsa-sha2-)/i.test(k) && !k.includes(","));
      if (vars.doNotSendPassword && validKeys.length === 0) {
        throw new Error("未选择有效的 OVH SSH 密钥时禁止勾选“不发送密码邮件”，否则重装后新 VPS 将永久无法登录。");
      }
      const payload = {
        templateId: vars.templateId,
        sshKey: validKeys.length > 0 ? [validKeys[0]] : undefined,
        doNotSendPassword: validKeys.length > 0 ? Boolean(vars.doNotSendPassword) : false,
      };
      return (await api.post(`/vps-control/${encodeURIComponent(svc)}/reinstall`, payload, withAccount(accountId))).data;
    },
    onSuccess: () => {
      invalidateVpsLifecycle(qc, svc, accountId);
    },
  });
}

/* ────────────── Tasks ────────────── */

export function useVpsTasks(
  svc: string | null,
  options?: { refetchInterval?: number | false },
  overrideAccountId?: string,
) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.tasks(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/tasks`, withAccount(accountId));
      return (res.data?.tasks || []) as VpsTask[];
    },
    enabled: !!svc,
    refetchInterval: options?.refetchInterval,
  });
}

export function useVpsTask(
  svc: string | null,
  taskId: number | string | null,
  refetchInterval = 0,
  overrideAccountId?: string,
) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.task(svc || "", taskId || "", accountId),
    queryFn: async () => {
      const res = await api.get(
        `/vps-control/${encodeURIComponent(svc || "")}/tasks/${encodeURIComponent(String(taskId || ""))}`,
        withAccount(accountId),
      );
      return res.data?.task as VpsTask | null;
    },
    enabled: !!svc && !!taskId,
    refetchInterval,
  });
}

/* ────────────── Snapshot ────────────── */

export function useVpsSnapshot(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.snapshot(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/snapshot`, withAccount(accountId));
      return res.data?.snapshot as VpsSnapshot | null;
    },
    enabled: !!svc,
  });
}

export function useCreateVpsSnapshot(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { description?: string }) =>
      (await api.post(`/vps-control/${encodeURIComponent(svc)}/snapshot`, vars, withAccount(accountId))).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc, accountId) });
      qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
    },
  });
}

export function useUpdateVpsSnapshot(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { description: string }) =>
      (await api.put(`/vps-control/${encodeURIComponent(svc)}/snapshot`, vars, withAccount(accountId))).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc, accountId) });
    },
  });
}

export function useRevertVpsSnapshot(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () =>
      (await api.post(`/vps-control/${encodeURIComponent(svc)}/snapshot/revert`, { confirm: true }, withAccount(accountId))).data,
    onSuccess: () => {
      invalidateVpsLifecycle(qc, svc, accountId);
    },
  });
}

export function useDeleteVpsSnapshot(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () =>
      (await api.delete(`/vps-control/${encodeURIComponent(svc)}/snapshot`, withAccount(accountId))).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.snapshot(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(svc, accountId) });
      qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
    },
  });
}

/* ────────────── Misc ────────────── */

export function useChangeVpsContact(overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { serviceName: string; admin?: string; tech?: string; billing?: string }) => {
      const body: Record<string, string> = {};
      if (vars.admin) body.contactAdmin = vars.admin;
      if (vars.tech) body.contactTech = vars.tech;
      if (vars.billing) body.contactBilling = vars.billing;
      return (await api.post(`/vps-control/${encodeURIComponent(vars.serviceName)}/change-contact`, body, withAccount(accountId))).data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(vars.serviceName, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.tasks(vars.serviceName, accountId) });
    },
  });
}

export function useUpdateVpsTerminationPolicy(serviceName: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { policy: string }) =>
      (await api.put(
        `/vps-control/${encodeURIComponent(serviceName)}/termination-policy`,
        { policy: vars.policy },
        withAccount(accountId),
      )).data,
    onSuccess: () => {
      invalidateVpsLifecycle(qc, serviceName, accountId);
    },
  });
}

export function useTerminateVps(overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { serviceName: string }) =>
      (await api.post(`/vps-control/${encodeURIComponent(vars.serviceName)}/terminate`, undefined, withAccount(accountId))).data,
    onSuccess: (_, vars) => {
      invalidateVpsLifecycle(qc, vars.serviceName, accountId);
    },
  });
}

export function useConfirmTerminateVps(overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { serviceName: string; token: string; reason?: string; commentary?: string }) =>
      (await api.post(
        `/vps-control/${encodeURIComponent(vars.serviceName)}/confirm-termination`,
        {
          token: vars.token,
          reason: vars.reason,
          commentary: vars.commentary,
        },
        withAccount(accountId),
      )).data,
    onSuccess: (_, vars) => {
      invalidateVpsLifecycle(qc, vars.serviceName, accountId);
    },
  });
}

export function useVpsSecondaryDns(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.secondaryDns(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/secondary-dns`, withAccount(accountId));
      return (res.data?.domains || []) as VpsSecondaryDnsEntry[];
    },
    enabled: !!svc,
  });
}

export function useAddVpsSecondaryDns(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { domain: string; ip?: string }) =>
      (await api.post(`/vps-control/${encodeURIComponent(svc)}/secondary-dns`, vars, withAccount(accountId))).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.secondaryDns(svc, accountId) }),
  });
}

export function useDeleteVpsSecondaryDns(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (domain: string) =>
      (await api.delete(
        `/vps-control/${encodeURIComponent(svc)}/secondary-dns/${encodeURIComponent(domain)}`,
        withAccount(accountId),
      )).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.secondaryDns(svc, accountId) }),
  });
}

export interface VpsOption {
  option: string;
  state?: string;
  manageEndpointsAvailable?: boolean;
  unsupportedReason?: string;
  region?: string;
  [k: string]: unknown;
}

export function useVpsOptions(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.options(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/options`, withAccount(accountId));
      return (res.data?.options || []) as VpsOption[];
    },
    enabled: !!svc,
  });
}

/* ────────────── Engagement(合同期) ────────────── */

export function useVpsEngagement(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.engagement(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/engagement`, withAccount(accountId));
      return (res.data?.engagement || null) as EngagementInfo | null;
    },
    enabled: !!svc,
  });
}

export function useVpsEngagementAvailable(svc: string | null, enabled = true, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.engagementAvailable(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/engagement/available`, withAccount(accountId));
      return (res.data?.pricings || []) as EngagementPricing[];
    },
    enabled: !!svc && enabled,
  });
}

export function useVpsEngagementRequest(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.engagementRequest(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/engagement/request`, withAccount(accountId));
      return (res.data?.request || null) as EngagementRequest | null;
    },
    enabled: !!svc,
  });
}

export function useCreateVpsEngagementRequest(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { pricingMode: string }) =>
      (await api.post(`/vps-control/${encodeURIComponent(svc)}/engagement/request`, vars, withAccount(accountId))).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.engagement(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.engagementRequest(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(svc, accountId) });
      qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
    },
  });
}

export function useDeleteVpsEngagementRequest(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async () =>
      (await api.delete(`/vps-control/${encodeURIComponent(svc)}/engagement/request`, withAccount(accountId))).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.engagementRequest(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(svc, accountId) });
      qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
    },
  });
}

export function useUpdateVpsEngagementEndRule(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { strategy: string; confirm?: boolean }) =>
      (await api.put(`/vps-control/${encodeURIComponent(svc)}/engagement/end-rule`, vars, withAccount(accountId))).data,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.vpsControl.engagement(svc, accountId) });
      qc.invalidateQueries({ queryKey: qk.vpsControl.serviceInfo(svc, accountId) });
      qc.invalidateQueries({ queryKey: ["vps-control", "list"] });
    },
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
  ipAddress?: string;
  mitigations: VpsMitigationIp[];
  error?: string;
}

export function useVpsMitigation(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.mitigation(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/mitigation`, withAccount(accountId));
      return (res.data?.ips || []) as VpsMitigationBlock[];
    },
    enabled: !!svc,
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

export function useEnableVpsMitigation(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) =>
      (await api.post(
        `/vps-control/${encodeURIComponent(svc)}/mitigation/${encodeURIComponent(vars.ip)}?block=${encodeURIComponent(vars.block)}`,
        undefined,
        withAccount(accountId),
      )).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.mitigation(svc, accountId) }),
  });
}

export function useDisableVpsMitigation(svc: string, overrideAccountId?: string) {
  const qc = useQueryClient();
  const accountId = useVpsAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (vars: { ip: string; block: string }) =>
      (await api.delete(
        `/vps-control/${encodeURIComponent(svc)}/mitigation/${encodeURIComponent(vars.ip)}?block=${encodeURIComponent(vars.block)}`,
        withAccount(accountId),
      )).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.vpsControl.mitigation(svc, accountId) }),
  });
}

export function useVpsAutomatedBackup(svc: string | null, overrideAccountId?: string) {
  const accountId = useVpsAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.vpsControl.automatedBackup(svc || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/vps-control/${encodeURIComponent(svc || "")}/automated-backup`, withAccount(accountId));
      return res.data?.automatedBackup as { rotation: number; schedule: string; state: string } | null;
    },
    enabled: !!svc,
  });
}
