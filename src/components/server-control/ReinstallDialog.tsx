import { useEffect, useMemo, useState } from "react";
import { HardDrive, AlertTriangle, Zap } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { errorMessage } from "@/components/common/LoadFailed";
import {
  useServerTemplates,
  useReinstallServer,
  useServerDiskInfo,
  useServerRaidProfiles,
  useServerPartitionSchemes,
  type CustomPartition,
} from "@/hooks/use-server-control";
import { detectOsKind } from "@/components/server-control/OsIcon";
import { useTheme } from "@/hooks/use-theme";
import { toast } from "sonner";
import { buildSmartPlan } from "@/lib/smart-storage";
import { Trans, useTranslation } from "react-i18next";
import { OS_GROUPS, type StorageMode } from "./reinstall/constants";
import { OsTemplateSection } from "./reinstall/OsTemplateSection";
import { StoragePartitionSection } from "./reinstall/StoragePartitionSection";
import { PostInstallSection } from "./reinstall/PostInstallSection";
import { SmartPlanDialog } from "./reinstall/SmartPlanDialog";

/**
 * 重装系统对话框（协调器组件）：
 * - OS 模板搜索 + 选择 (OsTemplateSection)
 * - 存储模式与磁盘 RAID 分区配置 (StoragePartitionSection)
 * - 自定义 Hostname 与系统个性化参数 (PostInstallSection)
 * - 智能分区规划向导 (SmartPlanDialog)
 */
export function ReinstallDialog({
  serviceName,
  open,
  onOpenChange,
}: {
  serviceName: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const tpl = useServerTemplates(serviceName, open);
  const disk = useServerDiskInfo(serviceName, open);
  const raid = useServerRaidProfiles(serviceName, open);
  const mut = useReinstallServer();
  const { t } = useTranslation();
  const brandDark = useTheme().resolved === "dark";

  // 基本
  const [search, setSearch] = useState("");
  const [templateName, setTemplateName] = useState("");
  const [activeGroup, setActiveGroup] = useState<ReturnType<typeof detectOsKind> | null>(null);
  const [hostname, setHostname] = useState("");

  // 存储模式与配置
  const [storageMode, setStorageMode] = useState<StorageMode>("default");
  const [zfsRaidLevel, setZfsRaidLevel] = useState<0 | 1>(1);
  const [zfsVzSize, setZfsVzSize] = useState(100 * 1024); // MB

  // 高级存储
  const [hardwareRaid, setHardwareRaid] = useState<Record<number, string>>({});
  const [useSoftwareRaid, setUseSoftwareRaid] = useState(false);
  const [softwareRaidLevel, setSoftwareRaidLevel] = useState("raid1");
  const [softwareRaidDisks, setSoftwareRaidDisks] = useState("");
  const [hwRaidArrays, setHwRaidArrays] = useState("");
  const [hwRaidSpares, setHwRaidSpares] = useState("");
  const [keepDiskGroups, setKeepDiskGroups] = useState<Set<number>>(new Set());
  const [customizationAnswers, setCustomizationAnswers] = useState<Record<string, string | boolean>>({});
  const [customPartitions, setCustomPartitions] = useState<CustomPartition[]>([]);
  const [showSmart, setShowSmart] = useState(false);

  // 内置分区方案
  const ps = useServerPartitionSchemes(serviceName, templateName || null);
  const smartPlan = useMemo(
    () => buildSmartPlan(disk.data || {}, detectOsKind(templateName)),
    [disk.data, templateName]
  );
  const [partitionSchemeName, setPartitionSchemeName] = useState("");

  // 确认
  const [confirming, setConfirming] = useState(false);

  // 重置 selectedScheme 当模板变化时
  useEffect(() => {
    setPartitionSchemeName("");
  }, [templateName]);

  // 换模板时收敛存储模式
  useEffect(() => {
    if (templateName !== "proxmox9_64") {
      setStorageMode((m) => (m === "zfs" ? "default" : m));
    } else {
      setStorageMode((m) => (m === "default" ? "zfs" : m));
    }
  }, [templateName]);

  // 已选模板对应的发行版自动展开到左栏
  useEffect(() => {
    if (!templateName || activeGroup) return;
    const item = (tpl.data || []).find((x) => x.templateName === templateName);
    if (item) setActiveGroup(detectOsKind(item.templateName, item.distribution, item.family));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [templateName, tpl.data]);

  const filtered = useMemo(() => {
    const all = tpl.data || [];
    if (!search) return all;
    const s = search.toLowerCase();
    return all.filter(
      (item) => item.templateName.toLowerCase().includes(s) || item.distribution.toLowerCase().includes(s) || item.family.toLowerCase().includes(s)
    );
  }, [tpl.data, search]);

  const groupedTemplates = useMemo(() => {
    const buckets = new Map<string, typeof filtered>();
    for (const item of filtered) {
      const k = detectOsKind(item.templateName, item.distribution, item.family);
      const arr = buckets.get(k);
      if (arr) arr.push(item);
      else buckets.set(k, [item]);
    }
    for (const arr of buckets.values()) {
      arr.sort((a, b) => a.templateName.localeCompare(b.templateName));
    }
    return OS_GROUPS
      .filter((g) => buckets.has(g.kind))
      .map((g) => ({ ...g, items: buckets.get(g.kind)! }));
  }, [filtered]);

  const isProxmox9 = templateName === "proxmox9_64";
  const useProxmox9Zfs = isProxmox9 && storageMode === "zfs";
  const useCustomStorage = storageMode === "custom";

  const selectedTpl = useMemo(
    () => (tpl.data || []).find((x) => x.templateName === templateName),
    [tpl.data, templateName]
  );
  const osFilesystems = selectedTpl?.filesystems;
  const lvmReady = selectedTpl?.lvmReady !== false;
  const noPartitioning = selectedTpl?.noPartitioning === true;
  const softRaidOnlyMirroring = selectedTpl?.softRaidOnlyMirroring === true;
  const customizeQuestions = selectedTpl?.customizeQuestions || [];

  useEffect(() => {
    if (noPartitioning && storageMode !== "default") setStorageMode("default");
  }, [noPartitioning, storageMode]);

  const blockingErrors: { label: string; error: unknown; retry: () => void }[] = [];
  if (tpl.isError) {
    blockingErrors.push({ label: "ctrl.reinstall.dep.templates", error: tpl.error, retry: () => tpl.refetch() });
  }
  if ((useCustomStorage || useProxmox9Zfs) && disk.isError) {
    blockingErrors.push({ label: "ctrl.reinstall.dep.diskGroups", error: disk.error, retry: () => disk.refetch() });
  }
  if (useCustomStorage && raid.isError) {
    blockingErrors.push({ label: "ctrl.reinstall.dep.hwRaid", error: raid.error, retry: () => raid.refetch() });
  }
  if (storageMode === "scheme" && ps.isError) {
    blockingErrors.push({ label: "ctrl.reinstall.dep.schemes", error: ps.error, retry: () => ps.refetch() });
  }
  const blocked = blockingErrors.length > 0;
  const failedList = blocked
    ? blockingErrors.map((b) => t(b.label)).join(t("ctrl.reinstall.listSep"))
    : "";

  useEffect(() => {
    if (blocked) setConfirming(false);
  }, [blocked]);

  const zfsCap = useMemo(() => {
    const groups = disk.data || {};
    const first = Object.values(groups)[0];
    if (!first?.disks?.length) return { singleDiskGB: 0, diskCount: 0, usableMB: 0, maxVzGB: 0 };
    const d = first.disks[0];
    const singleDiskGB = d.unit?.toLowerCase().startsWith("t") ? d.capacity * 1024 : d.capacity;
    const diskCount = first.disks.length;
    const totalGB = zfsRaidLevel === 0 ? singleDiskGB * diskCount : singleDiskGB;
    const usableMB = Math.floor(totalGB * 1024 * 0.92);
    const BOOT_MB = 1024;
    const SWAP_MB = 8192;
    const ROOT_RESERVE_MB = 20 * 1024;
    const maxVzGB = Math.max(10, Math.floor((usableMB - BOOT_MB - SWAP_MB - ROOT_RESERVE_MB) / 1024));
    return { singleDiskGB, diskCount, usableMB, maxVzGB };
  }, [disk.data, zfsRaidLevel]);

  const zfsRaid1Impossible = zfsRaidLevel !== 0 && zfsCap.diskCount > 0 && zfsCap.diskCount < 2;
  const vzMaxKnown = zfsCap.maxVzGB > 0 && zfsCap.diskCount > 0;
  const vzMaxGB = vzMaxKnown ? zfsCap.maxVzGB : Number.MAX_SAFE_INTEGER;

  useEffect(() => {
    if (!vzMaxKnown) return;
    setZfsVzSize((cur) => Math.min(cur, zfsCap.maxVzGB * 1024));
  }, [vzMaxKnown, zfsCap.maxVzGB]);

  const handleSubmit = async () => {
    if (!templateName) {
      toast.error(t("ctrl.reinstall.toast.selectTemplate"));
      return;
    }
    if (blocked) {
      setConfirming(false);
      toast.error(t("ctrl.reinstall.toast.blockedRetry", { list: failedList }));
      return;
    }
    const hn = hostname.trim();
    if (hn && !/^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/.test(hn)) {
      toast.error(t("ctrl.reinstall.toast.hostnameInvalid"));
      return;
    }
    if (useProxmox9Zfs && zfsRaid1Impossible) {
      toast.error(t("ctrl.reinstall.toast.zfsRaid1Impossible"));
      return;
    }
    if (!confirming) {
      setConfirming(true);
      return;
    }
    try {
      const res = await mut.mutateAsync({
        serviceName,
        templateName,
        customHostname: hostname.trim() || undefined,
        customizations: Object.keys(customizationAnswers).length > 0 ? customizationAnswers : undefined,
        useProxmox9Zfs,
        zfsRaidLevel: useProxmox9Zfs ? zfsRaidLevel : undefined,
        zfsVzSize: useProxmox9Zfs ? zfsVzSize : undefined,
        partitionSchemeName: storageMode === "scheme" && partitionSchemeName ? partitionSchemeName : undefined,
        hardwareRaid: useCustomStorage ? hardwareRaid : undefined,
        useSoftwareRaid: useCustomStorage && useSoftwareRaid,
        softwareRaidLevel: useCustomStorage && useSoftwareRaid ? softwareRaidLevel : undefined,
        softwareRaidDisks:
          useCustomStorage && useSoftwareRaid && softwareRaidDisks ? parseInt(softwareRaidDisks) : undefined,
        hwRaidArrays:
          useCustomStorage && hwRaidArrays ? parseInt(hwRaidArrays) : undefined,
        hwRaidSpares:
          useCustomStorage && hwRaidSpares ? parseInt(hwRaidSpares) : undefined,
        keepDiskGroups:
          useCustomStorage && keepDiskGroups.size > 0 ? Array.from(keepDiskGroups) : undefined,
        customPartitions: useCustomStorage ? customPartitions : undefined,
        diskGroups: useCustomStorage ? disk.data : undefined,
      });
      toast.success(t("ctrl.reinstall.toast.submitted"));
      (res?.warnings || []).forEach((w) => toast.warning(w, { duration: 6000 }));
      onOpenChange(false);
      reset();
    } catch (e: any) {
      toast.error(errorMessage(e));
    }
  };

  const reset = () => {
    setSearch("");
    setTemplateName("");
    setHostname("");
    setStorageMode("default");
    setZfsRaidLevel(1);
    setZfsVzSize(100 * 1024);
    setHardwareRaid({});
    setUseSoftwareRaid(false);
    setSoftwareRaidLevel("raid1");
    setSoftwareRaidDisks("");
    setHwRaidArrays("");
    setHwRaidSpares("");
    setKeepDiskGroups(new Set());
    setCustomizationAnswers({});
    setCustomPartitions([]);
    setPartitionSchemeName("");
    setConfirming(false);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v);
        if (!v) reset();
      }}
    >
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <HardDrive className="w-5 h-5 text-destructive" />
            {t("ctrl.reinstall.title")}
          </DialogTitle>
          <DialogDescription>{t("ctrl.reinstall.desc")}</DialogDescription>
        </DialogHeader>

        <div className="overflow-y-auto flex-1 -mx-6 px-6 space-y-5">
          {/* 读失败总览 */}
          {blocked && (
            <div className="border border-destructive/40 bg-destructive/5 rounded-2xl p-3 space-y-2">
              <div className="flex items-start gap-2 text-[12px]">
                <AlertTriangle className="w-4 h-4 text-destructive mt-0.5 flex-shrink-0" />
                <div className="leading-relaxed">
                  <p className="font-semibold">{t("ctrl.reinstall.blockedTitle")}</p>
                  <p className="text-muted-foreground">{t("ctrl.reinstall.blockedDesc")}</p>
                </div>
              </div>
              <div className="space-y-1.5 pl-6">
                {blockingErrors.map((b) => (
                  <div key={b.label} className="flex items-start justify-between gap-2 text-[11px]">
                    <span className="min-w-0">
                      <span className="font-semibold">{t("ctrl.reinstall.depFailed", { what: t(b.label) })}</span>
                      <span className="text-muted-foreground"> · {errorMessage(b.error)}</span>
                    </span>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="h-6 px-2 text-[11px] flex-shrink-0"
                      onClick={b.retry}
                    >
                      {t("common.retry")}
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Windows 提示 */}
          <div className="border border-info/40 bg-info/5 rounded-2xl p-3 text-[12px] flex items-start gap-2">
            <Zap className="w-4 h-4 text-info mt-0.5 flex-shrink-0" />
            <div className="text-foreground/80 leading-relaxed space-y-1">
              <p>{t("ctrl.reinstall.winHintRefresh")}</p>
              <p>
                <Trans i18nKey="ctrl.reinstall.winHintStd" components={{ b: <span className="font-semibold" /> }} />
              </p>
            </div>
          </div>

          {/* OS 模板选择区 */}
          <OsTemplateSection
            tpl={tpl}
            search={search}
            setSearch={setSearch}
            templateName={templateName}
            setTemplateName={setTemplateName}
            activeGroup={activeGroup}
            setActiveGroup={setActiveGroup}
            filtered={filtered}
            groupedTemplates={groupedTemplates}
            brandDark={brandDark}
          />

          {/* 存储模式与磁盘配置 */}
          <StoragePartitionSection
            noPartitioning={noPartitioning}
            storageMode={storageMode}
            setStorageMode={setStorageMode}
            isProxmox9={isProxmox9}
            templateName={templateName}
            zfsRaidLevel={zfsRaidLevel}
            setZfsRaidLevel={setZfsRaidLevel}
            zfsRaid1Impossible={zfsRaid1Impossible}
            zfsVzSize={zfsVzSize}
            setZfsVzSize={setZfsVzSize}
            vzMaxKnown={vzMaxKnown}
            vzMaxGB={vzMaxGB}
            zfsCap={zfsCap}
            disk={disk}
            raid={raid}
            ps={ps}
            partitionSchemeName={partitionSchemeName}
            setPartitionSchemeName={setPartitionSchemeName}
            useCustomStorage={useCustomStorage}
            keepDiskGroups={keepDiskGroups}
            setKeepDiskGroups={setKeepDiskGroups}
            hardwareRaid={hardwareRaid}
            setHardwareRaid={setHardwareRaid}
            hwRaidArrays={hwRaidArrays}
            setHwRaidArrays={setHwRaidArrays}
            hwRaidSpares={hwRaidSpares}
            setHwRaidSpares={setHwRaidSpares}
            useSoftwareRaid={useSoftwareRaid}
            setUseSoftwareRaid={setUseSoftwareRaid}
            softwareRaidLevel={softwareRaidLevel}
            setSoftwareRaidLevel={setSoftwareRaidLevel}
            softRaidOnlyMirroring={softRaidOnlyMirroring}
            softwareRaidDisks={softwareRaidDisks}
            setSoftwareRaidDisks={setSoftwareRaidDisks}
            onOpenSmart={() => setShowSmart(true)}
            customPartitions={customPartitions}
            setCustomPartitions={setCustomPartitions}
            osFilesystems={osFilesystems}
            lvmReady={lvmReady}
          />

          {/* 自定义 Hostname 与系统特定问题 */}
          <PostInstallSection
            hostname={hostname}
            setHostname={setHostname}
            customizeQuestions={customizeQuestions}
            customizationAnswers={customizationAnswers}
            setCustomizationAnswers={setCustomizationAnswers}
          />

          {/* 危险操作提示 */}
          {confirming && (
            <div className="border border-destructive/40 bg-destructive/5 rounded-2xl p-3 text-[12px] flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 text-destructive mt-0.5 flex-shrink-0" />
              <div className="leading-relaxed">
                <Trans i18nKey="ctrl.reinstall.confirmWarn" components={{ b: <span className="font-semibold" /> }} />
              </div>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!templateName || mut.isPending || blocked}
            title={blocked ? t("ctrl.reinstall.submitBlockedTitle", { list: failedList }) : undefined}
          >
            {mut.isPending
              ? t("ctrl.reinstall.submitting")
              : blocked
                ? t("ctrl.reinstall.submitBlocked")
                : confirming
                  ? t("ctrl.reinstall.submitConfirm")
                  : t("ctrl.reinstall.next")}
          </Button>
        </DialogFooter>
      </DialogContent>

      {/* 智能配置确认对话框 */}
      <SmartPlanDialog
        showSmart={showSmart}
        setShowSmart={setShowSmart}
        smartPlan={smartPlan}
        onApply={(partitions) => setCustomPartitions(partitions)}
      />
    </Dialog>
  );
}
