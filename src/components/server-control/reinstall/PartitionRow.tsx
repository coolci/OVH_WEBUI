import { X as XIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { CustomPartition } from "@/hooks/use-server-control";
import { FILESYSTEMS, SOFTWARE_RAID_LEVELS } from "./constants";
import { useTranslation } from "react-i18next";

/** 单行自定义分区编辑（mountpoint / filesystem / size / 磁盘组 / RAID） */
export function PartitionRow({
  partition,
  diskGroupIds,
  onChange,
  onRemove,
  filesystems,
  lvmReady,
  softRaidOnlyMirroring,
}: {
  partition: CustomPartition;
  diskGroupIds: number[];
  onChange: (p: CustomPartition) => void;
  onRemove: () => void;
  /** 官方 installationTemplate.filesystems:该 OS 实际支持的文件系统;缺省退回全集 */
  filesystems?: string[];
  /** 官方 lvmReady:false 时该 OS 不支持 LVM,LV 名输入不可用 */
  lvmReady?: boolean;
  /** 官方 softRaidOnlyMirroring:true 时软 RAID 只允许 0/1 */
  softRaidOnlyMirroring?: boolean;
}) {
  const { t } = useTranslation();
  // 官方口径:文件系统下拉按所选模板的 filesystems 过滤(模板没给元数据时退回全集)
  const fsOptions = (filesystems && filesystems.length > 0 ? filesystems : FILESYSTEMS).filter(
    (fs) => FILESYSTEMS.includes(fs)
  );
  const raidOptions = softRaidOnlyMirroring
    ? SOFTWARE_RAID_LEVELS.filter((l) => l.value === "raid0" || l.value === "raid1")
    : SOFTWARE_RAID_LEVELS;
  return (
    <div className="border border-border rounded-xl p-2.5 flex items-center gap-2 text-[12px] bg-background flex-wrap">
      <Input
        value={partition.mountpoint}
        onChange={(e) => onChange({ ...partition, mountpoint: e.target.value })}
        placeholder={t("ctrl.reinstall.part.mountpoint")}
        className="h-8 w-32"
      />
      <Select value={partition.filesystem} onValueChange={(v) => onChange({ ...partition, filesystem: v })}>
        <SelectTrigger className="h-8 w-24">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {fsOptions.map((fs) => (
            <SelectItem key={fs} value={fs}>
              {fs}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        type="number"
        min={0}
        value={partition.size}
        onChange={(e) => onChange({ ...partition, size: parseInt(e.target.value) || 0 })}
        placeholder="MB"
        className="h-8 w-24"
        title={t("ctrl.reinstall.part.sizeHint")}
      />
      <span className="text-[11px] text-muted-foreground">MB</span>
      {diskGroupIds.length > 1 && (
        <Select
          value={String(partition.diskGroupId ?? "")}
          onValueChange={(v) => onChange({ ...partition, diskGroupId: parseInt(v) })}
        >
          <SelectTrigger className="h-8 w-24">
            <SelectValue placeholder={t("ctrl.reinstall.part.diskGroup")} />
          </SelectTrigger>
          <SelectContent>
            {diskGroupIds.map((gid) => (
              <SelectItem key={gid} value={String(gid)}>
                {t("ctrl.reinstall.disk.group", { id: gid })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
      <Select
        value={partition.raid || ""}
        onValueChange={(v) => onChange({ ...partition, raid: v === " " ? undefined : v })}
      >
        <SelectTrigger className="h-8 w-28">
          <SelectValue placeholder={t("ctrl.reinstall.part.noRaid")} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value=" ">{t("ctrl.reinstall.part.noRaid")}</SelectItem>
          {raidOptions.map((l) => (
            <SelectItem key={l.value} value={l.value}>
              {l.value.toUpperCase()}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button type="button" variant="outline" size="sm" onClick={onRemove} className="ml-auto h-8 w-8 p-0">
        <XIcon className="w-3.5 h-3.5" />
      </Button>
      {/* 官方 extras(第二行):ZFS 显示 zpool 名;非 ZFS 且 OS 支持 LVM 显示逻辑卷名。
          官方 auto-fix:同 RAID 的 LV 自动归同一 VG、同名 zpool 合并 —— 用户只需命名即可控制分组 */}
      {partition.filesystem === "zfs" && (
        <Input
          value={partition.zpoolName ?? ""}
          onChange={(e) => onChange({ ...partition, zpoolName: e.target.value })}
          placeholder={t("ctrl.reinstall.part.zpool")}
          className="h-8 w-36 font-mono text-[11px]"
          title={t("ctrl.reinstall.part.zpoolHint")}
        />
      )}
      {partition.filesystem !== "zfs" && lvmReady && (
        <Input
          value={partition.lvName ?? ""}
          onChange={(e) => onChange({ ...partition, lvName: e.target.value })}
          placeholder={t("ctrl.reinstall.part.lv")}
          className="h-8 w-36 font-mono text-[11px]"
          title={t("ctrl.reinstall.part.lvHint")}
        />
      )}
    </div>
  );
}
