import { useQuery, useMutation } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import type {
  OSTemplate,
  ReinstallArgs,
  ReinstallResult,
  InstallStatus,
} from "./types";

const TEMPLATES_LS_PREFIX = "ovh_sniper_templates_v2_";

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
    staleTime: 1000 * 60 * 60,
    gcTime: Infinity,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

export function useReinstallServer() {
  return useMutation({
    mutationFn: async (args: ReinstallArgs) => {
      const installData: any = {
        templateName: args.templateName,
        customHostname: args.customHostname || undefined,
        customizations: args.customizations && Object.keys(args.customizations).length > 0 ? args.customizations : undefined,
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
        const groups = new Map<string, any>();
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
        const DEFAULT_GID = "default";
        const gidKey = (v?: number) => (v && v > 0 ? String(v) : DEFAULT_GID);
        if (args.useSoftwareRaid && args.softwareRaidDisks && args.softwareRaidDisks > 0) {
          const dgid = groups.has(DEFAULT_GID) ? DEFAULT_GID : groups.keys().next().value as string;
          const g0 = groups.get(dgid);
          if (g0 && g0.partitioning) g0.partitioning.disks = args.softwareRaidDisks;
        }
        partitions.forEach((p) => {
          const gid = gidKey(p.diskGroupId);
          if (!groups.has(gid)) {
            const entry: any = { partitioning: { layout: [] } };
            if (gid !== DEFAULT_GID) entry.diskGroupId = Number(gid);
            groups.set(gid, entry);
          }
          const g = groups.get(gid);
          const ovhP: any = { mountPoint: p.mountpoint, fileSystem: p.filesystem, size: p.size || 0 };
          if (p.raid) {
            const m = p.raid.match(/raid(\d+)/);
            if (m) ovhP.raidLevel = parseInt(m[1]);
          }
          if (p.lvName?.trim()) ovhP.extras = { ...(ovhP.extras || {}), lv: { name: p.lvName.trim() } };
          if (p.filesystem === "zfs" && p.zpoolName?.trim()) {
            ovhP.extras = { ...(ovhP.extras || {}), zp: { name: p.zpoolName.trim() } };
          }
          g.partitioning.layout.push(ovhP);
        });

        if (args.hardwareRaid) {
          Object.entries(args.hardwareRaid).forEach(([gidStr, raidMode]) => {
            if (!raidMode) return;
            const parsed = parseInt(gidStr);
            const parsedKey = gidKey(parsed);
            const gid =
              groups.size === 1 && groups.has(DEFAULT_GID) && parsedKey === DEFAULT_GID
                ? DEFAULT_GID
                : parsedKey;
            if (!groups.has(gid)) {
              const entry: any = {};
              if (gid !== DEFAULT_GID) entry.diskGroupId = Number(gid);
              groups.set(gid, entry);
            }
            const g = groups.get(gid);
            if (!g.hardwareRaid) g.hardwareRaid = [];
            const level = parseInt(raidMode.replace("raid", ""), 10);
            if (Number.isNaN(level)) return;
            const diskCount = args.diskGroups?.[gidStr]?.disks?.length || 0;
            const item: { raidLevel: number; disks?: number; arrays?: number; spares?: number } = { raidLevel: level };
            if (diskCount > 0) item.disks = diskCount;
            if (args.hwRaidArrays && args.hwRaidArrays > 0 && level === 10) item.arrays = args.hwRaidArrays;
            if (args.hwRaidSpares && args.hwRaidSpares > 0) item.spares = args.hwRaidSpares;
            g.hardwareRaid.push(item);
          });
        }
        const storageArray = Array.from(groups.values());
        for (const keepGid of args.keepDiskGroups ?? []) {
          storageArray.push({ diskGroupId: keepGid, erase: false });
        }
        if (storageArray.length > 0) installData.storageConfig = storageArray;
      } else if (args.partitionSchemeName) {
        installData.partitionSchemeName = args.partitionSchemeName;
      }

      const res = await api.post(`/server-control/${args.serviceName}/install`, installData);
      return res.data as ReinstallResult;
    },
  });
}

export function useInstallStatus(serviceName: string | null, enabled = true) {
  return useQuery({
    queryKey: qk.serverControl.installStatus(serviceName || ""),
    queryFn: async () => {
      const res = await api.get(`/server-control/${serviceName}/install/status`);
      return {
        hasInstallation: res.data?.hasInstallation !== false,
        status: (res.data?.status || null) as InstallStatus | null,
      };
    },
    enabled: !!serviceName && enabled,
    refetchInterval: enabled ? 5_000 : false,
    staleTime: 0,
  });
}
