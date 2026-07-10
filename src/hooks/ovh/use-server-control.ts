import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";

export interface OwnedServer {
  serviceName: string;
  name: string;
  commercialRange: string;
  datacenter: string;
  state: string;
  status?: string;
  ip: string;
  os: string;
  orderId?: string | number;
}

export interface HardwareInfo {
  processorName: string;
  processorArchitecture: string;
  coresPerProcessor: number;
  threadsPerProcessor: number;
  memorySize?: { value: number; unit: string };
  diskGroups?: any[];
  expansionCards?: any[];
}

export interface ServiceInfo {
  status: string;
  expiration: string;
  creation: string;
  /** 是否启用自动续费(后端解析 OVH renew.automatic) */
  renewalType: boolean;
  /** 续费周期,单位月(1 / 3 / 6 / 12 等) */
  renewalPeriod: number;
  /** 到期是否自动删除服务 —— true 等于"到期断网回收" */
  renewalDeleteAtExpiration: boolean;
  /** OVH 是否强制自动续费(部分付费服务) */
  renewalForced: boolean;
  /** 是否要求手动支付(true 时余额扣款会跳过,需用户手动付) */
  renewalManualPayment: boolean;
  /** OVH 允许的续费周期列表(月数),前端 select 选项用 */
  possibleRenewPeriod: number[];
}

/**
 * 已购服务器列表（后端返回 { success, servers, total }）
 * 过滤逻辑照搬旧前端：只显示 state === 'ok' | 'active'，排除 expired / suspended / error
 */
export function useOwnedServers() {
  return useQuery({
    queryKey: qk.serverControl.list(),
    queryFn: async () => {
      const res = await api.get("/server-control/list");
      const raw = (res.data?.servers || []) as OwnedServer[];
      return raw.filter((s) => {
        const state = s.state?.toLowerCase();
        const status = s.status?.toLowerCase();
        if (status === "expired" || status === "suspended") return false;
        if (state === "error" || state === "suspended") return false;
        return state === "ok" || state === "active";
      });
    },
    staleTime: 60_000,
  });
}

/** 硬件信息（后端返回 { success, hardware: {...} }） */
export function useServerHardware(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.hardware(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/hardware`);
      return (res.data?.hardware || null) as HardwareInfo | null;
    },
    enabled: !!serviceName,
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
 * 修改续费策略。OVH 这个 endpoint 需要 PUT 整个 service 对象,
 * 后端代为 GET + merge + PUT,前端只传 mode + 可选 period。
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
    },
  });
}

/* ──────────── Engagement(合同期切换) ──────────── */

export interface EngagementPricing {
  pricingMode: string;
  description: string;
  duration: string;
  interval: number;
  price: { value: number; currencyCode?: string; text?: string };
  priceInUcents?: number;
  engagementConfiguration?: {
    defaultEndAction: string;
    duration: string;
    type: string;
  };
}

export interface EngagementInfo {
  currentPeriod?: { startDate: string; endDate: string };
  endRule?: { strategy: string; possibleStrategies: string[] };
}

export interface EngagementRequest {
  pricing?: EngagementPricing;
  requestDate?: string;
  order?: { orderId?: number; url?: string };
}

/** 当前 engagement(无合同期返回 null) */
export function useEngagement(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.engagement(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/engagement`);
      return (res.data?.engagement || null) as EngagementInfo | null;
    },
    enabled: !!serviceName,
  });
}

/** 可订购的 engagement 选项列表 */
export function useEngagementAvailable(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.engagementAvailable(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/engagement/available`);
      return (res.data?.pricings || []) as EngagementPricing[];
    },
    enabled: !!serviceName && enabled,
  });
}

/** 进行中的 engagement 变更请求 */
export function useEngagementRequest(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.engagementRequest(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/engagement/request`);
      return (res.data?.request || null) as EngagementRequest | null;
    },
    enabled: !!serviceName,
  });
}

/** 提交新的 engagement 请求 */
export function useCreateEngagementRequest(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { pricingMode: string }) => {
      const res = await api.post(`/server-control/${serviceName}/engagement/request`, vars);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.engagement(serviceName) });
      qc.invalidateQueries({ queryKey: qk.serverControl.engagementRequest(serviceName) });
    },
  });
}

/** 撤销进行中的 engagement 请求 */
export function useDeleteEngagementRequest(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const res = await api.delete(`/server-control/${serviceName}/engagement/request`);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.engagementRequest(serviceName) });
    },
  });
}

/** 改 engagement 到期策略 */
export function useUpdateEngagementEndRule(serviceName: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { strategy: string }) => {
      const res = await api.put(`/server-control/${serviceName}/engagement/end-rule`, vars);
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.engagement(serviceName) });
    },
  });
}

/* ──────────── DDoS Mitigation ──────────── */

export interface MitigationIp {
  ipOnMitigation: string;
  state: string; // activated / pending / disabled / ...
  auto: boolean;
  permanent: boolean;
}

export interface MitigationBlock {
  ipBlock: string;
  mitigations: MitigationIp[];
  error?: string;
}

export function useMitigation(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.mitigation(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/mitigation`);
      return (res.data?.ips || []) as MitigationBlock[];
    },
    enabled: !!serviceName,
    // 过渡态自动轮询(creationPending/removalPending → ok 通常 30 秒-2 分钟)
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

/** IP 列表（后端返回 { success, ips: [{ ip, type, family, ... }] }） */
export function useServerIps(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.ips(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/ips`);
      return (res.data?.ips || []) as Array<{
        ip: string;
        type: string;
        family?: string;
        description?: string;
        routedTo?: string;
        inferred?: boolean;
      }>;
    },
    enabled: !!serviceName,
  });
}

/** 维护记录（后端返回 { success, interventions: [...] }） */
export function useServerInterventions(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.interventions(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/interventions`);
      return (res.data?.interventions || []) as any[];
    },
    enabled: !!serviceName,
  });
}

/** 网络接口（后端返回 { success, interfaces: [...] }） */
export function useServerNetworkInterfaces(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.networkInterfaces(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/network-interfaces`);
      return (res.data?.interfaces || []) as Array<{ mac: string; linkType?: string }>;
    },
    enabled: !!serviceName,
  });
}

export interface BootMode {
  id: number;
  bootType: string;
  description: string;
  kernel: string;
  active: boolean;
}

/** 启动模式（后端返回 { success, bootModes: [...] }） */
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

/** 切换启动模式（旧前端会随后自动调 reboot；这里把 reboot 留给调用方决定） */
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

export interface ServerTask {
  taskId: number;
  function: string;
  status: string;
  startDate: string;
  doneDate: string;
}

/** 服务器运维任务列表（后端返回 { success, tasks: [...] }） */
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

export interface OSTemplate {
  templateName: string;
  distribution: string;
  family: string;
  bitFormat: number;
}

/**
 * OS 模板列表（每台机器可用模板不同）（后端返回 { success, templates: [...] }）
 * 长期缓存：localStorage 持久化 + staleTime/gcTime 永不过期，
 * 只有用户点"刷新"才会重新拉取（dialog 里手动 refetch）。
 */
const TEMPLATES_LS_PREFIX = "ovh_sniper_templates_";
export function useServerTemplates(serviceName: string | null, enabled = true) {
  const lsKey = serviceName ? TEMPLATES_LS_PREFIX + serviceName : "";
  return useQuery({
    queryKey: qk.serverControl.osTemplates(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/templates`);
      const list = (res.data?.templates || []) as OSTemplate[];
      if (lsKey) {
        try {
          localStorage.setItem(lsKey, JSON.stringify(list));
          localStorage.setItem(lsKey + "_at", String(Date.now()));
        } catch { /* 配额满或隐私模式 */ }
      }
      return list;
    },
    initialData: () => {
      if (!lsKey) return undefined;
      try {
        const raw = localStorage.getItem(lsKey);
        return raw ? (JSON.parse(raw) as OSTemplate[]) : undefined;
      } catch {
        return undefined;
      }
    },
    initialDataUpdatedAt: () => {
      if (!lsKey) return undefined;
      try {
        const at = localStorage.getItem(lsKey + "_at");
        return at ? Number(at) : undefined;
      } catch {
        return undefined;
      }
    },
    enabled: !!serviceName && enabled,
    staleTime: Infinity,
    gcTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

/** 硬件磁盘组（用于自定义 RAID / 分区）（后端返回 { success, diskGroups: { [id]: {...} } }） */
export interface DiskGroupDisk {
  number: number;
  capacity: number;
  unit: string;
  technology?: string;
  interface?: string;
}
export interface DiskGroup {
  raidController?: string;
  disks: DiskGroupDisk[];
}

export function useServerDiskInfo(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.diskInfo(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/hardware-disk-info`);
      return (res.data?.diskGroups || {}) as Record<string, DiskGroup>;
    },
    enabled: !!serviceName && enabled,
    staleTime: 5 * 60_000,
  });
}

/** 硬件 RAID 支持情况（后端返回 { success, supported, profiles }） */
export function useServerRaidProfiles(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.raidProfiles(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/hardware-raid-profiles`);
        return {
          supported: res.data?.supported !== false,
          profiles: (res.data?.profiles || []) as any[],
        };
      } catch {
        return { supported: false, profiles: [] as any[] };
      }
    },
    enabled: !!serviceName && enabled,
    staleTime: 5 * 60_000,
  });
}

/** 分区方案列表（每个模板的内置方案）（后端返回 { success, schemes: [...] }） */
export interface PartitionScheme {
  name: string;
  priority: number;
}
export function useServerPartitionSchemes(serviceName: string | null, templateName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.partitionSchemes(serviceName || "", templateName || ""),
    queryFn: async () => {
      const res = await api.get(
        `/server-control/${serviceName}/partition-schemes?templateName=${encodeURIComponent(templateName || "")}`
      );
      return (res.data?.schemes || []) as PartitionScheme[];
    },
    enabled: !!serviceName && !!templateName,
    staleTime: 5 * 60_000,
  });
}

/** 自定义分区一行（前端模型，提交时再转 OVH layout） */
export interface CustomPartition {
  mountpoint: string;
  filesystem: string;
  size: number; // MB，0 表示剩余
  order: number;
  type: string;
  raid?: string; // raid0/raid1/...
  diskGroupId?: number;
}

/** 重装系统：完整版（template / hostname / Proxmox ZFS / 硬件 RAID / 软 RAID / 自定义分区 / 内置分区方案） */
export interface ReinstallArgs {
  serviceName: string;
  templateName: string;
  customHostname?: string;
  // Proxmox 9 + ZFS（仅当 templateName === 'proxmox9_64' 时）
  useProxmox9Zfs?: boolean;
  zfsRaidLevel?: 0 | 1;
  zfsVzSize?: number; // MB
  // 老路径：选择某个内置分区方案
  partitionSchemeName?: string;
  // 新路径：自定义存储（硬件 RAID + 软 RAID + 分区）
  hardwareRaid?: Record<number, string>; // diskGroupId → raidLevel (raid0/...)
  softwareRaidLevel?: string; // 仅当 useSoftwareRaid 为 true 时
  useSoftwareRaid?: boolean;
  customPartitions?: CustomPartition[];
  diskGroups?: Record<string, DiskGroup>; // 用于硬件 RAID 时拼 disks 列表
}

export function useReinstallServer() {
  return useMutation({
    mutationFn: async (args: ReinstallArgs) => {
      const installData: any = {
        templateName: args.templateName,
        customHostname: args.customHostname || undefined,
        useProxmox9Zfs: !!args.useProxmox9Zfs,
        zfsRaidLevel: args.useProxmox9Zfs ? args.zfsRaidLevel : undefined,
        zfsVzSize: args.useProxmox9Zfs ? args.zfsVzSize : undefined,
      };

      const useCustom = !!(
        args.useSoftwareRaid ||
        (args.hardwareRaid && Object.values(args.hardwareRaid).some((v) => !!v)) ||
        (args.customPartitions && args.customPartitions.length > 0)
      );

      if (useCustom) {
        // 按 diskGroupId 分组
        const groups = new Map<number, any>();
        let partitions = args.customPartitions || [];
        // 启用软 RAID 但未自定义分区 → 默认根分区软 RAID
        if (args.useSoftwareRaid && partitions.length === 0) {
          partitions = [
            {
              mountpoint: "/",
              filesystem: "ext4",
              size: 0,
              order: 1,
              type: "primary",
              raid: args.softwareRaidLevel || "raid1",
              diskGroupId: 0,
            },
          ];
        }
        partitions.forEach((p) => {
          const gid = p.diskGroupId ?? 0;
          if (!groups.has(gid)) groups.set(gid, { diskGroupId: gid, partitioning: { layout: [] } });
          const g = groups.get(gid);
          const ovhP: any = { mountPoint: p.mountpoint, fileSystem: p.filesystem, size: p.size || 0 };
          if (p.raid) {
            const m = p.raid.match(/raid(\d+)/);
            if (m) ovhP.raidLevel = parseInt(m[1]);
          }
          g.partitioning.layout.push(ovhP);
        });
        // 硬件 RAID
        if (args.hardwareRaid) {
          Object.entries(args.hardwareRaid).forEach(([gidStr, raidMode]) => {
            if (!raidMode) return;
            const gid = parseInt(gidStr);
            if (!groups.has(gid)) groups.set(gid, { diskGroupId: gid });
            const g = groups.get(gid);
            if (!g.hardwareRaid) g.hardwareRaid = [];
            const level = raidMode.replace("raid", "");
            g.hardwareRaid.push({
              disks: args.diskGroups?.[gidStr]?.disks?.map((d) => d.number) || [],
              mode: level,
              name: `raid${level}`,
              step: 1,
            });
          });
        }
        const storageArray = Array.from(groups.values());
        if (storageArray.length > 0) installData.storageConfig = storageArray;
      } else if (args.partitionSchemeName) {
        installData.partitionSchemeName = args.partitionSchemeName;
      }

      const res = await api.post(`/server-control/${args.serviceName}/install`, installData);
      return res.data;
    },
  });
}

/** 安装进度（前端轮询用，旧前端每 5s 轮一次）（后端返回 { success, hasInstallation, status: {...} }） */
export function useInstallStatus(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.installStatus(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/install/status`);
      return {
        hasInstallation: res.data?.hasInstallation !== false,
        status: res.data?.status || null,
      };
    },
    enabled: !!serviceName && enabled,
    refetchInterval: enabled ? 5_000 : false,
    staleTime: 0,
  });
}

// ───────────────────────────────── BIOS / Monitoring ─────────────────────────────────

/** BIOS 设置（response.data 即结果对象） */
export function useServerBiosSettings(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.biosSettings(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/bios-settings`);
        const sgxRes = await api.get(`/server-control/${serviceName}/bios-settings/sgx`).catch(() => null);
        return {
          settings: res.data || {},
          sgx: sgxRes?.data?.sgx ?? sgxRes?.data?.data ?? sgxRes?.data ?? null,
        };
      } catch {
        return { settings: {}, sgx: null };
      }
    },
    enabled: !!serviceName && enabled,
  });
}

/** OVH 监控开关（res.data.monitoring → boolean） */
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

// ───────────────────────────────── Burst / Firewall ─────────────────────────────────

/** Burst：res.data.burst（结构含 status / capacity 等）；某些服务器不支持，会返回 404 */
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

/** 防火墙：res.data.firewall（结构含 state / mode / model 等）；某些服务器不支持，会返回 404 */
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

// ───────────────────────────────── Backup FTP ─────────────────────────────────

/** Backup FTP：可能 notAvailable / notActivated / 正常对象 */
export function useServerBackupFtp(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.backupFtp(serviceName || ""),
    queryFn: async () => {
      try {
        const res = await api.get(`/server-control/${serviceName}/backup-ftp`);
        if (res.data?.success === false) {
          return { notAvailable: true, error: res.data?.error } as any;
        }
        // 尝试同时取 access 列表
        let accessList: any[] = [];
        try {
          const accRes = await api.get(`/server-control/${serviceName}/backup-ftp/access`);
          accessList = accRes.data?.accessList || [];
        } catch {
          /* 访问列表拿不到不算失败 */
        }
        return { backupFtp: res.data?.backupFtp || null, accessList } as any;
      } catch (e: any) {
        if (e?.response?.status === 404) return { notActivated: true } as any;
        return { notAvailable: true, error: e?.response?.data?.error || e?.message } as any;
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

// ───────────────────────────────── Secondary DNS / vMAC / vRack ─────────────────────────────────

export function useServerSecondaryDns(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.secondaryDns(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/secondary-dns`);
      return (res.data?.domains || []) as any[];
    },
    enabled: !!serviceName,
  });
}

export function useServerVirtualMac(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.virtualMac(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/virtual-mac`);
      return (res.data?.virtualMacs || []) as any[];
    },
    enabled: !!serviceName,
  });
}

export function useServerVrack(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.vrack(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/vrack`);
      return (res.data?.vracks || []) as any[];
    },
    enabled: !!serviceName,
  });
}

// ───────────────────────────────── Orderable / Options / IP Specs / Network Specs ─────────────────────────────────

/** 可订购服务：并发取 bandwidth / traffic / ip 三项 */
export function useServerOrderable(serviceName: string | null) {
  return useQuery({
    queryKey: qk.serverControl.orderable(serviceName || ""),
    queryFn: async () => {
      const [bw, tr, ip] = await Promise.all([
        api.get(`/server-control/${serviceName}/orderable/bandwidth`).catch(() => ({ data: { success: false } })),
        api.get(`/server-control/${serviceName}/orderable/traffic`).catch(() => ({ data: { success: false } })),
        api.get(`/server-control/${serviceName}/orderable/ip`).catch(() => ({ data: { success: false } })),
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
  return useQuery({
    queryKey: qk.serverControl.options(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/options`);
      return (res.data?.options || []) as any[];
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

// ───────────────────────────────── Interventions（创建工单） ─────────────────────────────────

/** 创建硬件干预工单（硬盘 / 内存 / 散热 等）—— 旧前端 POST /interventions */
export function useCreateIntervention() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: { serviceName: string; type: string; details?: string; comment?: string }) => {
      const res = await api.post(`/server-control/${args.serviceName}/interventions`, {
        type: args.type,
        details: args.details,
        comment: args.comment,
      });
      return res.data;
    },
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: qk.serverControl.interventions(vars.serviceName) });
    },
  });
}

// ───────────────────────────────── Contact change ─────────────────────────────────

/** 提交变更联系人请求(POST /change-contact)
 *  字段名要跟 OVH API 一致: contactAdmin / contactTech / contactBilling */
export function useChangeContact() {
  return useMutation({
    mutationFn: async (args: { serviceName: string; admin?: string; tech?: string; billing?: string }) => {
      const res = await api.post(`/server-control/${args.serviceName}/change-contact`, {
        contactAdmin: args.admin || undefined,
        contactTech: args.tech || undefined,
        contactBilling: args.billing || undefined,
      });
      return res.data;
    },
  });
}

/** 查询所有变更联系人请求（用户全局而非按服务器）。后端返回 { success, data: [...] } */
export function useContactChangeRequests(enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.contactRequests(),
    queryFn: async () => {
      const res = await api.get(`/ovh/contact-change-requests`);
      return (res.data?.data || res.data?.requests || []) as any[];
    },
    enabled,
  });
}

/** 操作单个变更请求（接受 / 拒绝 / 重发邮件） */
export function useContactRequestAction() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: { id: number | string; action: "accept" | "refuse" | "resend"; token?: string }) => {
      if (args.action === "resend") {
        const res = await api.post(`/ovh/contact-change-requests/${args.id}/resend-email`);
        return res.data;
      }
      const res = await api.post(`/ovh/contact-change-requests/${args.id}/${args.action}`, {
        token: args.token,
      });
      return res.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: qk.serverControl.contactRequests() });
    },
  });
}

// ───────────────────────────────── Tasks · 可用时间段 ─────────────────────────────────

/** 任务的可用时间段（旧前端 GET /tasks/{id}/available-timeslots?periodStart=&periodEnd=） */
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

/** 重启服务器（mutation 封装） */
export function useRebootServer() {
  return useMutation({
    mutationFn: async (serviceName: string) => {
      const res = await api.post(`/server-control/${serviceName}/reboot`);
      return res.data;
    },
  });
}
