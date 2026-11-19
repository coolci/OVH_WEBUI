import { useQuery, useMutation } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import type {
  OSTemplate,
  CustomizeQuestion,
  DiskGroup,
  ReinstallArgs,
  ReinstallResult,
  InstallStatus,
} from "./types";
import { useServerAccount, withAccount } from "./account";

export const TEMPLATES_LS_PREFIX = "ovh_sniper_templates_v2_";

export function getServerTemplatesCacheKey(serviceName: string | null | undefined, accountId?: string): string {
  if (!serviceName) return "";
  return `${TEMPLATES_LS_PREFIX}${accountId || "default"}:${serviceName}`;
}

export function clearServerTemplatesStorage(
  serviceName: string | null | undefined,
  accountId?: string,
  storage?: Pick<Storage, "removeItem"> | null,
): void {
  const lsKey = getServerTemplatesCacheKey(serviceName, accountId);
  if (!lsKey) return;
  const target = storage ?? (typeof localStorage !== "undefined" ? localStorage : null);
  if (!target) return;
  try {
    target.removeItem(lsKey);
    target.removeItem(`${lsKey}_at`);
  } catch {
    /* 配额或隐私模式忽略 */
  }
}

export function invalidateServerTemplatesCache(
  qc: {
    removeQueries?: (filters: { queryKey: readonly unknown[] }) => void;
    invalidateQueries: (filters: { queryKey: readonly unknown[] }) => unknown;
  },
  serviceName: string,
  accountId?: string,
  storage?: Pick<Storage, "removeItem"> | null,
): void {
  if (!serviceName) return;
  clearServerTemplatesStorage(serviceName, accountId, storage);
  if (accountId) clearServerTemplatesStorage(serviceName, "", storage);
  const exactKey = qk.serverControl.osTemplates(serviceName, accountId || "");
  qc.removeQueries?.({ queryKey: exactKey });
  qc.invalidateQueries({ queryKey: exactKey });
  qc.invalidateQueries({ queryKey: ["server-control", "os-templates", serviceName] });
}

export function useServerTemplates(serviceName: string | null, enabled = true, overrideAccountId?: string) {
  const accountId = useServerAccount(overrideAccountId);
  const lsKey = getServerTemplatesCacheKey(serviceName, accountId);
  return useQuery({
    queryKey: qk.serverControl.osTemplates(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${encodeURIComponent(serviceName || "")}/templates`, withAccount(accountId));
      const list = (res.data?.templates || []) as OSTemplate[];
      if (lsKey && typeof localStorage !== "undefined") {
        try {
          localStorage.setItem(lsKey, JSON.stringify(list));
          localStorage.setItem(lsKey + "_at", String(Date.now()));
        } catch { /* 配额满或隐私模式 */ }
      }
      return list;
    },
    initialData: () => {
      if (!lsKey || typeof localStorage === "undefined") return undefined;
      try {
        const raw = localStorage.getItem(lsKey);
        return raw ? (JSON.parse(raw) as OSTemplate[]) : undefined;
      } catch {
        return undefined;
      }
    },
    initialDataUpdatedAt: () => {
      if (!lsKey || typeof localStorage === "undefined") return undefined;
      try {
        const at = localStorage.getItem(lsKey + "_at");
        return at ? Number(at) : undefined;
      } catch {
        return undefined;
      }
    },
    enabled: !!serviceName && enabled,
    staleTime: 1000 * 60 * 60,
    gcTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

/**
 * 将 OVH 磁盘容量与单位统一换算为 GB（支持 B/KB/Ko/KiB/MB/Mo/MiB/GB/Go/GiB/TB/To/TiB/PB/Po/PiB）。
 * 修复将 MB/Mo 直接当作 GB 放大 1024 倍的问题。
 */
export function diskCapacityToGB(capacity: number, unit?: string): number {
  if (!Number.isFinite(capacity) || capacity <= 0) return 0;
  const u = (unit || "GB").trim().toLowerCase();
  switch (u) {
    case "p":
    case "pb":
    case "po":
    case "pib":
      return capacity * 1024 * 1024;
    case "t":
    case "tb":
    case "to":
    case "tib":
      return capacity * 1024;
    case "":
    case "g":
    case "gb":
    case "go":
    case "gib":
      return capacity;
    case "m":
    case "mb":
    case "mo":
    case "mib":
      return capacity / 1024;
    case "k":
    case "kb":
    case "ko":
    case "kib":
      return capacity / (1024 * 1024);
    case "b":
    case "o":
    case "byte":
    case "bytes":
    case "octet":
    case "octets":
      return capacity / (1024 * 1024 * 1024);
    default:
      if (u.startsWith("p")) return capacity * 1024 * 1024;
      if (u.startsWith("t")) return capacity * 1024;
      if (u.startsWith("m")) return capacity / 1024;
      if (u.startsWith("k")) return capacity / (1024 * 1024);
      return capacity;
  }
}

/**
 * 确定默认系统安装盘组：
 * 1. 显式传入的 defaultDiskGroupId
 * 2. 硬件规格标记了 isDefault (来自 hardware.defaultDiskGroupId) 的磁盘组
 * 3. OVH 官方默认 diskGroupId === 1
 * 4. 最小正整数编号磁盘组（避免盲取 Object.values(groups)[0]）
 */
export function pickDefaultDiskGroup(
  groups?: Record<string, DiskGroup> | null,
  explicitDefaultId?: number,
): { id: number; group: DiskGroup } | null {
  if (!groups) return null;
  const parsed = Object.entries(groups)
    .map(([key, group]) => {
      const id = Number(group?.id ?? key);
      return { id: Number.isInteger(id) && id > 0 ? id : 0, group };
    })
    .filter((item) => item.id > 0 && item.group);

  if (parsed.length === 0) return null;

  if (explicitDefaultId && Number.isInteger(explicitDefaultId) && explicitDefaultId > 0) {
    const exact = parsed.find((x) => x.id === explicitDefaultId);
    if (exact) return exact;
  }

  const marked = parsed.find((x) => x.group.isDefault === true);
  if (marked) return marked;

  const groupOne = parsed.find((x) => x.id === 1);
  if (groupOne) return groupOne;

  parsed.sort((a, b) => a.id - b.id);
  return parsed[0] ?? null;
}

export function minDisksForHwRaid(raidLevel: number, totalDisks = 0): number {
  switch (raidLevel) {
    case 0:
      return totalDisks === 1 ? 1 : 2;
    case 1:
      return 2;
    case 5:
      return 3;
    case 6:
    case 10:
      return 4;
    case 50:
      return 6;
    case 60:
      return 8;
    default:
      return 2;
  }
}

/**
 * 按当前模板问答白名单归一化并校验 customizations：
 * - 过滤非当前模板残留键值，防止跨模板污染
 * - 拦截 mandatory / required 必填项为空
 * - 将 type === "number" / "long" / "integer" 转为数值类型提交
 */
export function normalizeCustomizations(
  questions: CustomizeQuestion[],
  answers: Record<string, string | number | boolean>,
): {
  value?: Record<string, string | number | boolean>;
  missingMandatory: string[];
  invalidNumbers: string[];
} {
  const out: Record<string, string | number | boolean> = {};
  const missingMandatory: string[] = [];
  const invalidNumbers: string[] = [];

  for (const q of questions) {
    if (!q?.name) continue;
    const isMandatory = Boolean(q.required || q.mandatory);
    const raw = answers[q.name];
    const qLabel = q.description || q.name;

    if (q.type === "boolean") {
      if (typeof raw === "boolean") {
        out[q.name] = raw;
      } else if (isMandatory) {
        missingMandatory.push(qLabel);
      }
      continue;
    }

    if (q.type === "number" || q.type === "long" || q.type === "integer") {
      const str = raw === undefined || raw === null ? "" : String(raw).trim();
      if (!str) {
        if (isMandatory) missingMandatory.push(qLabel);
        continue;
      }
      const num = Number(str);
      if (!Number.isFinite(num) || ((q.type === "long" || q.type === "integer") && !Number.isInteger(num))) {
        invalidNumbers.push(qLabel);
        continue;
      }
      out[q.name] = num;
      continue;
    }

    const strVal = typeof raw === "string" ? raw : raw === undefined || raw === null ? "" : String(raw);
    const trimmed = strVal.trim();
    if (!trimmed) {
      if (isMandatory) missingMandatory.push(qLabel);
      continue;
    }
    out[q.name] = q.name === "postInstallationScript" ? strVal : trimmed;
  }

  return {
    value: Object.keys(out).length > 0 ? out : undefined,
    missingMandatory,
    invalidNumbers,
  };
}

/** 发给后端 /install 的 OVH storage 配置(每个磁盘组一项) */
export interface OvhPartition {
  mountPoint: string;
  fileSystem: string;
  size: number;
  raidLevel?: number;
  extras?: { lv?: { name: string }; zp?: { name: string } };
}

export interface StorageGroup {
  diskGroupId?: number;
  erase?: boolean;
  partitioning?: { layout: OvhPartition[]; disks?: number };
  hardwareRaid?: { raidLevel: number; disks?: number; arrays?: number; spares?: number }[];
}

interface InstallPayload {
  templateName: string;
  customHostname?: string;
  customizations?: ReinstallArgs["customizations"];
  useProxmox9Zfs: boolean;
  zfsRaidLevel?: ReinstallArgs["zfsRaidLevel"];
  zfsVzSize?: ReinstallArgs["zfsVzSize"];
  storageConfig?: StorageGroup[];
  partitionSchemeName?: string;
}

function resolveGroupMapValue(
  source: Record<number, number> | number | undefined,
  gid: number,
): number | undefined {
  if (typeof source === "number") return Number.isFinite(source) ? source : undefined;
  if (source && typeof source === "object") {
    const val = (source as Record<string, number | undefined>)[String(gid)];
    return typeof val === "number" && Number.isFinite(val) ? val : undefined;
  }
  return undefined;
}

export function buildInstallStorageConfig(args: ReinstallArgs): StorageGroup[] | undefined {
  const useCustom = !!(
    args.useSoftwareRaid ||
    (args.hardwareRaid && Object.values(args.hardwareRaid).some((v) => !!v)) ||
    (args.customPartitions && args.customPartitions.length > 0) ||
    (args.keepDiskGroups && args.keepDiskGroups.length > 0)
  );

  if (!useCustom) return undefined;

  const groups = new Map<string, StorageGroup>();
  let partitions = args.customPartitions || [];
  if (args.useSoftwareRaid && partitions.length === 0) {
    partitions = [
      {
        mountpoint: "/",
        filesystem: "ext4",
        size: 0,
        order: 1,
        type: "primary",
        raid: args.softwareRaidLevel || "raid1",
      },
    ];
  }

  const hasKeep = Boolean(args.keepDiskGroups && args.keepDiskGroups.length > 0);
  const defaultPick = pickDefaultDiskGroup(args.diskGroups, args.defaultDiskGroupId);
  const activeHwRaidGids = Object.entries(args.hardwareRaid || {})
    .filter(([, mode]) => !!mode)
    .map(([k]) => parseInt(k, 10))
    .filter((n) => Number.isInteger(n) && n > 0);

  // 未启用保留数据盘时，为未显式指定 diskGroupId 的分区归一化绑定真实默认磁盘组 ID，
  // 消除虚拟 "default" 键与真实硬件 RAID 盘组键分裂成两条重复 diskGroupId 的问题。
  // 当启用了 keepDiskGroups 时，严格保留 SS-04 要求（禁止隐式回退默认安装盘组）。
  const normalizedDefaultGid: number | undefined = !hasKeep
    ? (defaultPick?.id ?? (activeHwRaidGids.length === 1 ? activeHwRaidGids[0] : undefined))
    : undefined;

  const DEFAULT_GID = "default";
  const gidKey = (v?: number) => {
    const resolved = v && v > 0 ? v : normalizedDefaultGid;
    return resolved && resolved > 0 ? String(resolved) : DEFAULT_GID;
  };

  partitions.forEach((p) => {
    const gid = gidKey(p.diskGroupId);
    if (!groups.has(gid)) {
      const entry: StorageGroup = { partitioning: { layout: [] } };
      if (gid !== DEFAULT_GID) entry.diskGroupId = Number(gid);
      groups.set(gid, entry);
    }
    const g = groups.get(gid)!;
    if (!g.partitioning) g.partitioning = { layout: [] };
    const ovhP: OvhPartition = { mountPoint: p.mountpoint, fileSystem: p.filesystem, size: p.size || 0 };
    if (p.raid) {
      const m = p.raid.match(/raid(\d+)/);
      if (m?.[1]) ovhP.raidLevel = parseInt(m[1], 10);
    }
    if (p.lvName?.trim()) ovhP.extras = { ...(ovhP.extras || {}), lv: { name: p.lvName.trim() } };
    if (p.filesystem === "zfs" && p.zpoolName?.trim()) {
      ovhP.extras = { ...(ovhP.extras || {}), zp: { name: p.zpoolName.trim() } };
    }
    g.partitioning.layout.push(ovhP);
  });

  if (args.useSoftwareRaid && args.softwareRaidDisks && args.softwareRaidDisks > 0) {
    const defaultKey = normalizedDefaultGid ? String(normalizedDefaultGid) : DEFAULT_GID;
    const dgid = groups.has(defaultKey) ? defaultKey : (groups.keys().next().value as string);
    const g0 = groups.get(dgid);
    if (g0 && g0.partitioning) g0.partitioning.disks = args.softwareRaidDisks;
  }

  if (args.hardwareRaid) {
    Object.entries(args.hardwareRaid).forEach(([gidStr, raidMode]) => {
      if (!raidMode) return;
      const parsed = parseInt(gidStr, 10);
      if (!Number.isInteger(parsed) || parsed <= 0) return;
      const gid = String(parsed);
      // 若此前因未提供 diskGroups 且多组 RAID 导致存在单一未绑定 DEFAULT_GID 分区组，且未启用 keepDiskGroups，
      // 将其提升合并至默认盘组（优先 defaultDiskGroupId 或 1 或首个 RAID 盘组），绝不制造两条重复组
      if (!hasKeep && groups.has(DEFAULT_GID)) {
        const targetDefault = args.defaultDiskGroupId || ( args.hardwareRaid?.["1"] ? 1 : activeHwRaidGids[0] );
        if (targetDefault === parsed && !groups.has(gid)) {
          const orphan = groups.get(DEFAULT_GID)!;
          groups.delete(DEFAULT_GID);
          orphan.diskGroupId = parsed;
          groups.set(gid, orphan);
        }
      }
      if (!groups.has(gid)) {
        groups.set(gid, { diskGroupId: parsed });
      }
      const g = groups.get(gid)!;
      if (!g.hardwareRaid) g.hardwareRaid = [];
      const level = parseInt(raidMode.replace("raid", ""), 10);
      if (Number.isNaN(level)) return;

      const dg = args.diskGroups?.[gidStr] as (DiskGroup & { numberOfDisks?: number }) | undefined;
      const totalDisks = dg?.disks?.length || dg?.numberOfDisks || 0;
      const rawSpares = resolveGroupMapValue(args.hwRaidSpares, parsed);
      const spares = rawSpares !== undefined ? rawSpares : 0;
      if (!Number.isInteger(spares) || spares < 0) {
        throw new Error(`磁盘组 ${parsed} 的热备盘数量必须为非负整数。`);
      }

      const rawArrays = resolveGroupMapValue(args.hwRaidArrays, parsed);
      const supportsArrays = level === 10 || level === 50 || level === 60;
      if (rawArrays !== undefined && (!Number.isInteger(rawArrays) || rawArrays <= 0)) {
        throw new Error(`磁盘组 ${parsed} 的 RAID 子阵列数(arrays)必须为正整数。`);
      }

      const minDisks = minDisksForHwRaid(level, totalDisks);
      const item: { raidLevel: number; disks?: number; arrays?: number; spares?: number } = { raidLevel: level };

      if (totalDisks > 0) {
        const disksCount = totalDisks - spares;
        if (disksCount < minDisks) {
          throw new Error(
            `磁盘组 ${parsed} 物理盘数(${totalDisks})扣除热备盘(${spares})后仅剩 ${disksCount} 块成员盘，不足以组建 RAID ${level}（至少需要 ${minDisks} 块成员盘）。`
          );
        }
        if (supportsArrays && rawArrays !== undefined && rawArrays > Math.floor(disksCount / 2)) {
          throw new Error(
            `磁盘组 ${parsed} 的成员盘数(${disksCount})不足以划分 ${rawArrays} 个 RAID ${level} 子阵列。`
          );
        }
        item.disks = disksCount;
      }

      if (spares > 0) item.spares = spares;
      if (supportsArrays && rawArrays && rawArrays > 0) {
        item.arrays = rawArrays;
      }
      g.hardwareRaid.push(item);
    });
  }

  const storageArray = Array.from(groups.values());
  if (args.keepDiskGroups?.length) {
    if (args.useProxmox9Zfs) throw new Error("保留磁盘数据时请关闭 ZFS 预设，并为安装磁盘组明确配置分区。");
    const keep = new Set(args.keepDiskGroups);
    if (keep.size !== args.keepDiskGroups.length || [...keep].some((id) => !Number.isInteger(id) || id <= 0)) {
      throw new Error("保留数据的磁盘组编号必须为不重复的正整数。");
    }
    if (!storageArray.some((g) => g.partitioning) || storageArray.some((g) => !g.diskGroupId)) {
      throw new Error("保留数据时必须为另一磁盘组明确配置安装分区，不能使用默认磁盘组或仅勾选保留数据。");
    }
    if (storageArray.some((g) => g.diskGroupId !== undefined && keep.has(g.diskGroupId))) {
      throw new Error("同一磁盘组不能同时安装系统和保留数据。");
    }
  }
  for (const keepGid of args.keepDiskGroups ?? []) {
    storageArray.push({ diskGroupId: keepGid, erase: false });
  }
  return storageArray.length > 0 ? storageArray : undefined;
}

export function useReinstallServer(overrideAccountId?: string) {
  const accountId = useServerAccount(overrideAccountId);
  return useMutation({
    mutationFn: async (args: ReinstallArgs & { accountId?: string }) => {
      const targetAcc = args.accountId || accountId;
      const installData: InstallPayload = {
        templateName: args.templateName,
        customHostname: args.customHostname || undefined,
        customizations: args.customizations && Object.keys(args.customizations).length > 0 ? args.customizations : undefined,
        useProxmox9Zfs: !!args.useProxmox9Zfs,
        zfsRaidLevel: args.useProxmox9Zfs ? args.zfsRaidLevel : undefined,
        zfsVzSize: args.useProxmox9Zfs ? args.zfsVzSize : undefined,
      };

      const storageArray = buildInstallStorageConfig(args);
      if (storageArray) {
        installData.storageConfig = storageArray;
      } else if (args.partitionSchemeName) {
        installData.partitionSchemeName = args.partitionSchemeName;
      }

      const res = await api.post(`/server-control/${encodeURIComponent(args.serviceName)}/install`, installData, withAccount(targetAcc));
      return res.data as ReinstallResult;
    },
  });
}

export function useInstallStatus(serviceName: string | null, enabled = true, overrideAccountId?: string) {
  const accountId = useServerAccount(overrideAccountId);
  return useQuery({
    queryKey: qk.serverControl.installStatus(serviceName || "", accountId),
    queryFn: async () => {
      const res = await api.get(`/server-control/${encodeURIComponent(serviceName || "")}/install/status`, withAccount(accountId));
      return {
        hasInstallation: res.data?.hasInstallation !== false,
        status: (res.data?.status || null) as InstallStatus | null,
      };
    },
    enabled: !!serviceName && enabled,
    refetchInterval: (query) => {
      if (!enabled) return false;
      const d = query.state.data;
      if (d) {
        if (!d.hasInstallation) return false;
        if (d.status && (d.status.allDone || d.status.hasError)) return false;
      }
      return 5_000;
    },
    staleTime: 0,
  });
}
