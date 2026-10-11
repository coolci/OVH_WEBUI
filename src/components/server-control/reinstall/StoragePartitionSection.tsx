import { HardDrive, Database, Plus, Cog, Zap, Wand2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/common/Skeleton";
import { LoadFailed, errorMessage } from "@/components/common/LoadFailed";
import { useTranslation } from "react-i18next";
import { StorageModeOption } from "./StorageModeOption";
import { PartitionRow } from "./PartitionRow";
import { HARDWARE_RAID_LEVELS, SOFTWARE_RAID_LEVELS, type StorageMode } from "./constants";
import type {
  useServerDiskInfo,
  useServerRaidProfiles,
  useServerPartitionSchemes,
  CustomPartition,
} from "@/hooks/use-server-control";

export interface StoragePartitionSectionProps {
  noPartitioning: boolean;
  storageMode: StorageMode;
  setStorageMode: (m: StorageMode) => void;
  isProxmox9: boolean;
  templateName: string;
  zfsRaidLevel: 0 | 1;
  setZfsRaidLevel: (lvl: 0 | 1) => void;
  zfsRaid1Impossible: boolean;
  zfsVzSize: number;
  setZfsVzSize: (sz: number) => void;
  vzMaxKnown: boolean;
  vzMaxGB: number;
  zfsCap: { singleDiskGB: number; diskCount: number; usableMB: number; maxVzGB: number };
  disk: ReturnType<typeof useServerDiskInfo>;
  raid: ReturnType<typeof useServerRaidProfiles>;
  ps: ReturnType<typeof useServerPartitionSchemes>;
  partitionSchemeName: string;
  setPartitionSchemeName: (v: string) => void;
  useCustomStorage: boolean;
  keepDiskGroups: Set<number>;
  setKeepDiskGroups: (s: Set<number>) => void;
  hardwareRaid: Record<number, string>;
  setHardwareRaid: (hr: Record<number, string>) => void;
  hwRaidArrays: string;
  setHwRaidArrays: (v: string) => void;
  hwRaidSpares: string;
  setHwRaidSpares: (v: string) => void;
  useSoftwareRaid: boolean;
  setUseSoftwareRaid: (v: boolean) => void;
  softwareRaidLevel: string;
  setSoftwareRaidLevel: (v: string) => void;
  softRaidOnlyMirroring: boolean;
  softwareRaidDisks: string;
  setSoftwareRaidDisks: (v: string) => void;
  onOpenSmart: () => void;
  customPartitions: CustomPartition[];
  setCustomPartitions: (p: CustomPartition[]) => void;
  osFilesystems?: string[];
  lvmReady: boolean;
}

export function StoragePartitionSection({
  noPartitioning,
  storageMode,
  setStorageMode,
  isProxmox9,
  templateName,
  zfsRaidLevel,
  setZfsRaidLevel,
  zfsRaid1Impossible,
  zfsVzSize,
  setZfsVzSize,
  vzMaxKnown,
  vzMaxGB,
  zfsCap,
  disk,
  raid,
  ps,
  partitionSchemeName,
  setPartitionSchemeName,
  useCustomStorage,
  keepDiskGroups,
  setKeepDiskGroups,
  hardwareRaid,
  setHardwareRaid,
  hwRaidArrays,
  setHwRaidArrays,
  hwRaidSpares,
  setHwRaidSpares,
  useSoftwareRaid,
  setUseSoftwareRaid,
  softwareRaidLevel,
  setSoftwareRaidLevel,
  softRaidOnlyMirroring,
  softwareRaidDisks,
  setSoftwareRaidDisks,
  onOpenSmart,
  customPartitions,
  setCustomPartitions,
  osFilesystems,
  lvmReady,
}: StoragePartitionSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="space-y-5">
      {/* 官方 noPartitioning(如 ESXi):分区由软件发布方决定,自定义存储整体不可用 */}
      {noPartitioning && (
        <div className="border border-info/40 bg-info/5 rounded-2xl p-3 text-[12px] flex items-start gap-2">
          <Zap className="w-4 h-4 text-info mt-0.5 flex-shrink-0" />
          <p className="text-foreground/80 leading-relaxed">{t("ctrl.reinstall.noPartitioning")}</p>
        </div>
      )}

      {/* 存储配置模式（三者互斥，见 storageMode 注释） */}
      {!noPartitioning && (
        <div className="border border-border rounded-2xl p-4 space-y-2">
          <div className="flex items-center gap-2">
            <Cog className="w-4 h-4 text-muted-foreground" />
            <h4 className="text-[13px] font-semibold">{t("ctrl.reinstall.storage.title")}</h4>
          </div>
          <p className="text-[11px] text-muted-foreground">
            {t("ctrl.reinstall.storage.desc")}
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-1.5 text-[12px]">
            <StorageModeOption
              checked={storageMode === "default"}
              onSelect={() => setStorageMode("default")}
              label={t("ctrl.reinstall.mode.default")}
              hint={t("ctrl.reinstall.mode.defaultHint")}
            />
            {isProxmox9 && (
              <StorageModeOption
                checked={storageMode === "zfs"}
                onSelect={() => setStorageMode("zfs")}
                label={t("ctrl.reinstall.mode.zfs")}
                hint={t("ctrl.reinstall.mode.zfsHint")}
              />
            )}
            <StorageModeOption
              checked={storageMode === "scheme"}
              onSelect={() => setStorageMode("scheme")}
              label={t("ctrl.reinstall.mode.scheme")}
              hint={t(templateName ? "ctrl.reinstall.mode.schemeHint" : "ctrl.reinstall.mode.schemeHintDisabled")}
              disabled={!templateName}
            />
            <StorageModeOption
              checked={storageMode === "custom"}
              onSelect={() => setStorageMode("custom")}
              label={t("ctrl.reinstall.mode.custom")}
              hint={t("ctrl.reinstall.mode.customHint")}
            />
          </div>
        </div>
      )}

      {/* Proxmox 9 ZFS 配置 */}
      {isProxmox9 && storageMode === "zfs" && (
        <div className="border border-success/40 bg-success/5 rounded-2xl p-4 space-y-3">
          <div className="flex items-center gap-2">
            <Database className="w-4 h-4 text-success" />
            <h4 className="text-[13px] font-semibold">{t("ctrl.reinstall.zfs.title")}</h4>
          </div>
          <p className="text-[11px] text-muted-foreground">
            {t("ctrl.reinstall.zfs.desc")}
          </p>

          <div className="space-y-3">
            <div>
              <label className="block text-[12px] mb-1.5">{t("ctrl.reinstall.zfs.raidLevel")}</label>
              <div className="flex gap-4 text-[13px]">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    checked={zfsRaidLevel === 1}
                    onChange={() => setZfsRaidLevel(1)}
                    className="w-4 h-4"
                  />
                  {t("ctrl.reinstall.zfs.raid1")}
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="radio"
                    checked={zfsRaidLevel === 0}
                    onChange={() => setZfsRaidLevel(0)}
                    className="w-4 h-4"
                  />
                  {t("ctrl.reinstall.zfs.raid0")}
                </label>
              </div>
            </div>
            {zfsRaid1Impossible && (
              <p className="text-[11px] text-destructive">
                {t("ctrl.reinstall.zfs.singleDiskWarn")}
              </p>
            )}
            <div>
              <label className="block text-[12px] mb-1.5">{t("ctrl.reinstall.zfs.vzLabel")}</label>
              <Input
                type="number"
                min={10}
                max={vzMaxKnown ? zfsCap.maxVzGB : undefined}
                value={Math.floor(zfsVzSize / 1024)}
                onChange={(e) => {
                  const v = Math.max(10, Math.min(vzMaxGB, parseInt(e.target.value) || 100));
                  setZfsVzSize(v * 1024);
                }}
                className="w-40"
              />
              <p className="text-[11px] text-muted-foreground mt-1">
                {t("ctrl.reinstall.zfs.vzHint", {
                  max: vzMaxKnown
                    ? `${zfsCap.maxVzGB} GB`
                    : disk.isError
                      ? t("ctrl.reinstall.zfs.maxUnknownFailed")
                      : t("ctrl.reinstall.zfs.maxUnknownLoading"),
                })}
                {zfsCap.singleDiskGB > 0 && (
                  <>
                    {" "}
                    {t("ctrl.reinstall.zfs.vzDetail", {
                      disks: zfsCap.diskCount,
                      size: zfsCap.singleDiskGB,
                      level: zfsRaidLevel,
                      usable: Math.floor(zfsCap.usableMB / 1024),
                    })}
                  </>
                )}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* 内置分区方案（仅 scheme 模式） */}
      {storageMode === "scheme" && templateName && (
        <div>
          <label className="block text-[12px] font-semibold mb-1.5">{t("ctrl.reinstall.mode.scheme")}</label>
          {ps.isPending ? (
            <Skeleton className="h-9 rounded-md" />
          ) : ps.isError ? (
            <LoadFailed
              compact
              icon={Cog}
              title={t("ctrl.reinstall.scheme.loadFailed")}
              error={ps.error}
              onRetry={() => ps.refetch()}
            />
          ) : (ps.data || []).length === 0 ? (
            <p className="text-[12px] text-muted-foreground">{t("ctrl.reinstall.scheme.empty")}</p>
          ) : (
            <Select value={partitionSchemeName} onValueChange={setPartitionSchemeName}>
              <SelectTrigger>
                <SelectValue placeholder={t("ctrl.reinstall.mode.default")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value=" ">{t("ctrl.reinstall.mode.default")}</SelectItem>
                {(ps.data || []).map((s) => (
                  <SelectItem key={s.name} value={s.name}>
                    {t("ctrl.reinstall.scheme.item", { name: s.name, priority: s.priority })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      )}

      {/* 高级存储配置 */}
      <div className="border-t border-border pt-4">
        {useCustomStorage && (
          <div className="flex items-center gap-2 text-[13px] font-semibold mb-3">
            <Cog className="w-4 h-4" />
            {t("ctrl.reinstall.advTitle")}
          </div>
        )}

        {useCustomStorage && (
          <div className="space-y-4 border border-border rounded-2xl p-4 bg-secondary/30">
            {/* 磁盘组 + 硬件 RAID */}
            {disk.isPending ? (
              <Skeleton className="h-20 rounded-md" />
            ) : disk.isError ? (
              <LoadFailed
                compact
                icon={HardDrive}
                title={t("ctrl.reinstall.disk.loadFailed")}
                error={disk.error}
                onRetry={() => disk.refetch()}
              />
            ) : Object.keys(disk.data || {}).length === 0 ? (
              <p className="text-[12px] text-muted-foreground">{t("ctrl.reinstall.disk.empty")}</p>
            ) : (
              <div className="space-y-3">
                <h4 className="text-[12px] font-semibold">{t("ctrl.reinstall.disk.groupTitle")}</h4>
                {Object.entries(disk.data || {}).map(([gidStr, group]) => {
                  const gid = parseInt(gidStr);
                  return (
                    <div key={gid} className="border border-border rounded-xl p-3 space-y-2 bg-background">
                      <div className="flex items-center gap-2 text-[12px]">
                        <HardDrive className="w-3.5 h-3.5 text-muted-foreground" />
                        <span className="font-semibold">{t("ctrl.reinstall.disk.group", { id: gid })}</span>
                        {group.raidController && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded-full border border-border">
                            {group.raidController}
                          </span>
                        )}
                        <label className="ml-auto flex items-center gap-1.5 cursor-pointer text-[11px] text-muted-foreground">
                          <input
                            type="checkbox"
                            checked={keepDiskGroups.has(gid)}
                            onChange={(e) => {
                              const next = new Set(keepDiskGroups);
                              if (e.target.checked) next.add(gid);
                              else next.delete(gid);
                              setKeepDiskGroups(next);
                            }}
                            className="w-3.5 h-3.5"
                          />
                          {t("ctrl.reinstall.disk.keepData")}
                        </label>
                      </div>
                      <div className="grid grid-cols-2 gap-1.5 text-[11px] text-muted-foreground">
                        {group.disks.map((d, idx) => (
                          <div key={idx} className="flex items-center gap-1.5">
                            <span className="w-1.5 h-1.5 rounded-full bg-foreground/40" />
                            {d.capacity}
                            {d.unit} {d.technology || ""} {d.interface || ""}
                          </div>
                        ))}
                      </div>
                      <div>
                        <label className="block text-[11px] text-muted-foreground mb-1">{t("ctrl.reinstall.hwRaid.label")}</label>
                        {raid.isPending ? (
                          <Skeleton className="h-9 rounded-md" />
                        ) : raid.isError ? (
                          <p className="text-[11px] text-destructive">
                            {t("ctrl.reinstall.hwRaid.readFailed", { err: errorMessage(raid.error) })}
                          </p>
                        ) : !raid.data?.supported ? (
                          <p className="text-[11px] text-warning">
                            {t("ctrl.reinstall.hwRaid.unsupported")}
                          </p>
                        ) : (
                          <Select
                            value={hardwareRaid[gid] || ""}
                            onValueChange={(v) => setHardwareRaid({ ...hardwareRaid, [gid]: v === " " ? "" : v })}
                          >
                            <SelectTrigger className="h-9">
                              <SelectValue placeholder={t("ctrl.reinstall.hwRaid.none")} />
                            </SelectTrigger>
                            <SelectContent>
                              {HARDWARE_RAID_LEVELS.map((l) => (
                                <SelectItem key={l.value || "none"} value={l.value || " "}>
                                  {t(l.label)}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}

            {/* 官方 hardwareRaid 高级参数:RAID10 阵列数 + 热备盘 */}
            {raid.data?.supported && Object.values(hardwareRaid).some((v) => v && v !== "") && (
              <div className="flex flex-wrap items-center gap-3 text-[11px]">
                {Object.values(hardwareRaid).includes("raid10") && (
                  <label className="flex items-center gap-1.5">
                    <span className="text-muted-foreground">{t("ctrl.reinstall.hwRaid.arrays")}</span>
                    <Input
                      type="number"
                      min={1}
                      value={hwRaidArrays}
                      onChange={(e) => setHwRaidArrays(e.target.value.replace(/\D/g, ""))}
                      placeholder={t("ctrl.reinstall.hwRaid.auto")}
                      className="h-7 w-20"
                    />
                  </label>
                )}
                <label className="flex items-center gap-1.5">
                  <span className="text-muted-foreground">{t("ctrl.reinstall.hwRaid.spares")}</span>
                  <Input
                    type="number"
                    min={0}
                    value={hwRaidSpares}
                    onChange={(e) => setHwRaidSpares(e.target.value.replace(/\D/g, ""))}
                    placeholder={t("ctrl.reinstall.hwRaid.none")}
                    className="h-7 w-20"
                  />
                </label>
              </div>
            )}

            {/* 软 RAID */}
            <div className="border-t border-border pt-3">
              <label className="flex items-center gap-2 cursor-pointer text-[12px] font-semibold mb-2">
                <input
                  type="checkbox"
                  checked={useSoftwareRaid}
                  onChange={(e) => setUseSoftwareRaid(e.target.checked)}
                  className="w-4 h-4"
                />
                <HardDrive className="w-3.5 h-3.5" />
                {t("ctrl.reinstall.swRaid.label")}
              </label>
              {useSoftwareRaid && (
                <div className="pl-6 space-y-2">
                  <Select value={softwareRaidLevel} onValueChange={setSoftwareRaidLevel}>
                    <SelectTrigger className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {SOFTWARE_RAID_LEVELS.filter((l) =>
                        softRaidOnlyMirroring ? l.value === "raid0" || l.value === "raid1" : true
                      ).map((l) => (
                        <SelectItem key={l.value} value={l.value}>
                          {t(l.label)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <div className="flex items-center gap-2">
                    <Input
                      type="number"
                      min={1}
                      value={softwareRaidDisks}
                      onChange={(e) =>
                        setSoftwareRaidDisks(e.target.value.replace(/\D/g, ""))
                      }
                      placeholder={t("ctrl.reinstall.swRaid.disksPlaceholder")}
                      className="h-8 w-28"
                    />
                    <p className="text-[11px] text-muted-foreground">
                      {t("ctrl.reinstall.swRaid.disksHint")}
                    </p>
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    {t("ctrl.reinstall.swRaid.desc")}
                  </p>
                </div>
              )}
            </div>

            {/* 自定义分区 */}
            <div className="border-t border-border pt-3">
              <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
                <h4 className="text-[12px] font-semibold">{t("ctrl.reinstall.part.title")}</h4>
                <div className="flex flex-wrap gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant="secondary"
                    onClick={onOpenSmart}
                    disabled={!disk.data || Object.keys(disk.data).length === 0}
                  >
                    <Wand2 className="w-3.5 h-3.5 mr-1" />
                    {t("ctrl.reinstall.smart.title")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      setCustomPartitions([
                        ...customPartitions,
                        {
                          mountpoint: "/",
                          filesystem: "ext4",
                          size: 0,
                          order: customPartitions.length + 1,
                          type: "primary",
                          raid: useSoftwareRaid ? softwareRaidLevel : undefined,
                          diskGroupId: undefined,
                        },
                      ])
                    }
                  >
                    <Plus className="w-3.5 h-3.5 mr-1" />
                    {t("ctrl.reinstall.part.add")}
                  </Button>
                </div>
              </div>
              <p className="text-[11px] text-muted-foreground mb-2">{t("ctrl.reinstall.part.desc")}</p>
              {customPartitions.length > 0 && (
                <div className="space-y-2">
                  {customPartitions.map((p, idx) => (
                    <PartitionRow
                      key={idx}
                      partition={p}
                      diskGroupIds={Object.keys(disk.data || {}).map((s) => parseInt(s))}
                      filesystems={osFilesystems}
                      lvmReady={lvmReady}
                      softRaidOnlyMirroring={softRaidOnlyMirroring}
                      onChange={(np) => {
                        const next = [...customPartitions];
                        next[idx] = np;
                        setCustomPartitions(next);
                      }}
                      onRemove={() => setCustomPartitions(customPartitions.filter((_, i) => i !== idx))}
                    />
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
