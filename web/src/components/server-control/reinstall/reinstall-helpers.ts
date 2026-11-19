import type { CustomPartition } from "@/hooks/use-server-control";
import type { StorageMode } from "./constants";

/**
 * 构建重装高危配置指纹：一旦模板、存储模式、保留数据盘、软/硬 RAID、热备盘、阵列数、
 * 自定义分区表、主机名或定制问答发生任何变动，自动复位二次确认态 confirming。
 */
export function buildReinstallConfirmFingerprint(state: {
  templateName: string;
  storageMode: StorageMode;
  partitionSchemeName: string;
  zfsRaidLevel: number;
  zfsVzSize: number;
  keepDiskGroups: Iterable<number>;
  hardwareRaid: Record<number, string>;
  hwRaidArrays: Record<number, string | number>;
  hwRaidSpares: Record<number, string | number>;
  useSoftwareRaid: boolean;
  softwareRaidLevel: string;
  softwareRaidDisks: string;
  customPartitions: CustomPartition[];
  hostname?: string;
  customizationAnswers?: Record<string, string | number | boolean>;
}): string {
  const sortedKeep = Array.from(state.keepDiskGroups).sort((a, b) => a - b);
  return JSON.stringify({
    templateName: state.templateName,
    storageMode: state.storageMode,
    partitionSchemeName: state.partitionSchemeName,
    zfsRaidLevel: state.zfsRaidLevel,
    zfsVzSize: state.zfsVzSize,
    keepDiskGroups: sortedKeep,
    hardwareRaid: state.hardwareRaid,
    hwRaidArrays: state.hwRaidArrays,
    hwRaidSpares: state.hwRaidSpares,
    useSoftwareRaid: state.useSoftwareRaid,
    softwareRaidLevel: state.softwareRaidLevel,
    softwareRaidDisks: state.softwareRaidDisks,
    customPartitions: state.customPartitions,
    hostname: state.hostname || "",
    customizationAnswers: state.customizationAnswers || {},
  });
}
