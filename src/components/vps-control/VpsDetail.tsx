import { useState, useEffect } from "react";
import {
  Power, PowerOff, RefreshCw, Monitor, HardDrive, Cpu, MemoryStick,
  MapPin, Globe, CalendarPlus, Repeat, AlertTriangle, ListTodo, Undo2,
} from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Dialog } from "@/components/ui/dialog";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { Skeleton } from "@/components/common/Skeleton";
import { DeviceMetaCapsules } from "@/components/common/DeviceMetaCapsules";
import { InfoCard } from "@/components/common/InfoCard";
import { VpsSnapshotPane } from "@/components/vps-control/VpsSnapshotPane";
import { VpsReinstallDialog } from "@/components/vps-control/VpsReinstallDialog";
import { VpsMitigationPane } from "@/components/vps-control/VpsMitigationPane";
import { VpsTasksDialog } from "@/components/vps-control/VpsTasksDialog";
import { VpsOptionsPanel } from "@/components/vps-control/VpsOptionsPanel";
import { ChangeContactInline, AliasEditor, VpsPowerDialogs } from "@/components/vps-control/VpsDialogs";
import { humanZoneShort } from "@/components/vps-control/vps-geo";
import { RenewalDialog } from "@/components/server-control/RenewalDialog";
import { EngagementDialog, type EngagementHooks } from "@/components/server-control/EngagementDialog";
import { RetractionDialog } from "@/components/server-control/RetractionDialog";
import { useRetraction } from "@/hooks/use-server-control";
import {
  useVpsServiceInfo, useVpsIps, useVpsCurrentOS,
  useVpsStart, useVpsStop, useVpsReboot, useVpsConsoleUrl,
  useUpdateVpsRenewal, useChangeVpsContact,
  useTerminateVps, useConfirmTerminateVps, useUpdateVpsTerminationPolicy,
  useVpsEngagement, useVpsEngagementAvailable, useVpsEngagementRequest,
  useCreateVpsEngagementRequest, useDeleteVpsEngagementRequest, useUpdateVpsEngagementEndRule,
  type OwnedVps,
} from "@/hooks/use-vps-control";
import { useHideIp, maskSensitive } from "@/hooks/use-hide-ip";
import { useServerAliases, useSetServerAlias, aliasOf } from "@/hooks/use-server-aliases";
import { formatDate } from "@/lib/format-os";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

interface VpsDetailProps {
  server: OwnedVps;
  aliases: ReturnType<typeof useServerAliases>;
  onSetAlias: ReturnType<typeof useSetServerAlias>;
  isUS: boolean;
  region: string;
  activeAccount?: string;
}

export function VpsDetail({
  server,
  aliases,
  onSetAlias,
  isUS,
  region,
  activeAccount,
}: VpsDetailProps) {
  const info = useVpsServiceInfo(server.serviceName);
  const ips = useVpsIps(server.serviceName);
  const currentOS = useVpsCurrentOS(server.serviceName);
  const { hidden } = useHideIp();
  const start = useVpsStart(server.serviceName);
  const stop = useVpsStop(server.serviceName);
  const reboot = useVpsReboot(server.serviceName);
  const console_ = useVpsConsoleUrl(server.serviceName);
  const terminate = useTerminateVps();
  const confirmTerm = useConfirmTerminateVps();
  const vpsTermPolicy = useUpdateVpsTerminationPolicy(server.serviceName);

  const [reinstallOpen, setReinstallOpen] = useState(false);
  const [stopOpen, setStopOpen] = useState(false);
  const [rebootOpen, setRebootOpen] = useState(false);
  const [terminateOpen, setTerminateOpen] = useState(false);
  const [termToken, setTermToken] = useState("");
  const [renewalOpen, setRenewalOpen] = useState(false);
  const [contactOpen, setContactOpen] = useState(false);
  const [engagementOpen, setEngagementOpen] = useState(false);
  const [tasksOpen, setTasksOpen] = useState(false);
  const [retractOpen, setRetractOpen] = useState(false);
  const retraction = useRetraction(server.serviceName, true);
  const renewalMutation = useUpdateVpsRenewal(server.serviceName);
  const contactMutation = useChangeVpsContact();

  useEffect(() => {
    setStopOpen(false);
    setRebootOpen(false);
    setTerminateOpen(false);
    setTermToken("");
  }, [server.serviceName]);

  const vpsEngagementHooks: EngagementHooks = {
    useEngagement: useVpsEngagement,
    useEngagementAvailable: useVpsEngagementAvailable,
    useEngagementRequest: useVpsEngagementRequest,
    useCreateEngagementRequest: useCreateVpsEngagementRequest,
    useDeleteEngagementRequest: useDeleteVpsEngagementRequest,
    useUpdateEngagementEndRule: useUpdateVpsEngagementEndRule,
  };

  const isRunning = server.state === "running";
  const isStopped = server.state === "stopped";

  const stateLabel: Record<string, { text: string; tone: "success" | "warning" | "danger" | "default" }> = {
    running: { text: "运行中", tone: "success" },
    stopped: { text: "已关机", tone: "warning" },
    stopping: { text: "关机中", tone: "warning" },
    rebooting: { text: "重启中", tone: "warning" },
    installing: { text: "装机中", tone: "warning" },
    backuping: { text: "备份中", tone: "warning" },
    upgrading: { text: "升级中", tone: "warning" },
    maintenance: { text: "维护中", tone: "warning" },
    rescued: { text: "救援模式", tone: "danger" },
  };
  const sLabel = stateLabel[server.state] || { text: server.state, tone: "default" as const };

  const handleStart = async () => {
    try {
      await start.mutateAsync();
      toast.success("启动任务已提交");
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "启动失败");
    }
  };
  const handleStop = async () => {
    try {
      await stop.mutateAsync();
      toast.success("关机任务已提交");
      setStopOpen(false);
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "关机失败");
    }
  };
  const handleReboot = async () => {
    try {
      await reboot.mutateAsync();
      toast.success("重启任务已提交");
      setRebootOpen(false);
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "重启失败");
    }
  };
  const handleConsole = async () => {
    try {
      const url = await console_.mutateAsync();
      if (url) {
        window.open(url, "_blank", "noopener,noreferrer");
        toast.success("控制台 URL 已生成,5 分钟内有效");
      } else {
        toast.error("获取控制台 URL 失败");
      }
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "获取失败");
    }
  };
  const handleTerminate = async () => {
    try {
      await terminate.mutateAsync({ serviceName: server.serviceName });
      toast.success("终止请求已提交,请查邮件获取 token");
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "提交失败");
    }
  };
  const handleConfirmTerm = async () => {
    if (!termToken) return toast.error("请输入邮件里的 token");
    try {
      await confirmTerm.mutateAsync({ serviceName: server.serviceName, token: termToken });
      toast.success("VPS 已确认终止");
      setTerminateOpen(false);
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "确认失败");
    }
  };

  const ipDisplay = ips.data && ips.data.length > 0 ? maskSensitive(ips.data[0].ipAddress, hidden) : "—";
  const vpsLabel = aliasOf(aliases.data, server.serviceName, server.displayName || server.serviceName);

  return (
    <div className="space-y-4">
      {/* 顶部设备摘要卡片 */}
      <Card className="surface-card rounded-xl border-border overflow-hidden">
        <CardContent className="p-3.5 sm:p-4 space-y-3">
          <div className="flex items-start justify-between gap-2.5">
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-semibold text-base sm:text-lg text-foreground truncate">
                  {maskSensitive(vpsLabel, hidden)}
                </span>
                {server.model && (
                  <span className="px-2 py-0.5 rounded-md text-[11px] font-mono bg-secondary text-muted-foreground border border-border/60">
                    {server.model}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono flex-wrap">
                <span>{maskSensitive(server.serviceName, hidden)}</span>
                <span>·</span>
                <span>IP: {ipDisplay}</span>
              </div>
            </div>
            <Chip tone={sLabel.tone} className="flex-shrink-0">
              <StatusDot tone={sLabel.tone === "default" ? "muted" : sLabel.tone} pulse={isRunning} size="xs" />
              {sLabel.text}
            </Chip>
          </div>

          <DeviceMetaCapsules
            retraction={retraction.data}
            onRetractClick={() => setRetractOpen(true)}
            os={{
              rawName: currentOS.data?.name,
              distribution: currentOS.data?.distribution,
            }}
            onOsClick={() => setReinstallOpen(true)}
            osClickTitle="点击进入系统重建 / 重装"
            expiration={info.data?.expiration}
            creation={info.data?.creation}
            renewal={info.data}
            onRenewalClick={() => setRenewalOpen(true)}
          />
        </CardContent>
      </Card>

      {/* 硬件规格 4 格卡片 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 sm:gap-3">
        <InfoCard icon={<Cpu className="w-4 h-4 text-primary" />} label="vCore" value={String(server.vcore || "—")} />
        <InfoCard
          icon={<MemoryStick className="w-4 h-4 text-primary" />}
          label="内存"
          value={server.memoryMB ? `${(server.memoryMB / 1024).toFixed(0)} GB` : "—"}
        />
        <InfoCard
          icon={<HardDrive className="w-4 h-4 text-primary" />}
          label="磁盘"
          value={server.diskGB ? `${server.diskGB} GB` : "—"}
        />
        <InfoCard
          icon={<MapPin className="w-4 h-4 text-primary" />}
          label="区域"
          value={humanZoneShort(server.zone, server.cluster)}
        />
      </div>

      {/* 快捷操作区 */}
      <Card className="surface-card rounded-xl border-border">
        <CardContent className="p-3 sm:p-4 space-y-2.5">
          <div className="grid grid-cols-3 gap-2">
            {isStopped ? (
              <Button size="sm" onClick={handleStart} disabled={start.isPending} className="h-10 text-xs font-medium">
                <Power className="w-3.5 h-3.5 mr-1" />
                开机
              </Button>
            ) : (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setStopOpen(true)}
                disabled={stop.isPending}
                className="h-10 text-xs font-medium border-border/80 hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30"
              >
                <PowerOff className="w-3.5 h-3.5 mr-1 text-destructive" />
                关机
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => setRebootOpen(true)}
              disabled={reboot.isPending || !isRunning}
              className="h-10 text-xs font-medium border-border/80"
            >
              <RefreshCw className="w-3.5 h-3.5 mr-1 text-primary" />
              重启
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={handleConsole}
              disabled={console_.isPending}
              className="h-10 text-xs font-medium border-border/80"
            >
              <Monitor className="w-3.5 h-3.5 mr-1 text-muted-foreground" />
              Web控制台
            </Button>
          </div>

          <div className="grid grid-cols-3 gap-2 pt-1 border-t border-border/40">
            <Button variant="outline" size="sm" onClick={() => setReinstallOpen(true)} className="h-9 text-xs font-normal border-border/70 justify-center">
              <HardDrive className="w-3.5 h-3.5 mr-1.5 text-muted-foreground" />
              系统重建
            </Button>
            <Button variant="outline" size="sm" onClick={() => setEngagementOpen(true)} className="h-9 text-xs font-normal border-border/70 justify-center">
              <CalendarPlus className="w-3.5 h-3.5 mr-1.5 text-muted-foreground" />
              合同期
            </Button>
            <Button variant="outline" size="sm" onClick={() => setTasksOpen(true)} className="h-9 text-xs font-normal border-border/70 justify-center">
              <ListTodo className="w-3.5 h-3.5 mr-1.5 text-muted-foreground" />
              任务历史
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* 分段选项卡 */}
      <Tabs defaultValue="overview" className="w-full">
        <TabsList className="grid grid-cols-4 w-full h-10 p-1 bg-muted/60 rounded-xl border border-border/40">
          <TabsTrigger value="overview" className="text-xs rounded-lg font-medium">概览</TabsTrigger>
          <TabsTrigger value="snapshot" className="text-xs rounded-lg font-medium">快照</TabsTrigger>
          <TabsTrigger value="ddos" className="text-xs rounded-lg font-medium">DDoS</TabsTrigger>
          <TabsTrigger value="maintenance" className="text-xs rounded-lg font-medium">维护</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4 space-y-4">
          {server.lockStatus && server.lockStatus !== "unlocked" && (
            <div className="border border-destructive/40 bg-destructive/5 rounded-xl p-3 flex items-start gap-2.5">
              <AlertTriangle className="w-4 h-4 text-destructive mt-0.5 flex-shrink-0" />
              <div className="text-[12px]">
                <p className="font-semibold text-destructive">VPS 已锁定</p>
                <p className="text-muted-foreground mt-0.5">
                  状态: <code className="font-mono">{server.lockStatus}</code> · 通常因投诉被 OVH 临时冻结
                </p>
              </div>
            </div>
          )}
          {server.netbootMode === "rescue" && (
            <div className="border border-amber-500/40 bg-amber-500/5 rounded-xl p-3 flex items-start gap-2.5">
              <AlertTriangle className="w-4 h-4 text-amber-600 dark:text-amber-400 mt-0.5 flex-shrink-0" />
              <div className="text-[12px]">
                <p className="font-semibold text-amber-700 dark:text-amber-300">救援模式</p>
                <p className="text-muted-foreground mt-0.5">
                  下次重启会进入 OVH 救援镜像。修完故障后请切回 local 模式。
                </p>
              </div>
            </div>
          )}

          <div className="border border-border rounded-2xl overflow-hidden">
            <div className="px-4 py-3 border-b border-border flex items-center gap-2">
              <Globe className="w-4 h-4 text-muted-foreground" />
              <h3 className="text-sm font-semibold">IP 地址</h3>
              <span className="text-[11px] text-muted-foreground ml-auto">主 IP: {ipDisplay}</span>
            </div>
            {ips.isPending ? (
              <div className="p-4">
                <Skeleton className="h-16 rounded-md" />
              </div>
            ) : (ips.data || []).length === 0 ? (
              <p className="px-4 py-6 text-sm text-muted-foreground text-center">无 IP</p>
            ) : (
              <div className="divide-y divide-border">
                {(ips.data || []).map((ip) => (
                  <div key={ip.ipAddress} className="px-4 py-3 flex items-center gap-2 text-[13px] flex-wrap">
                    <code className="font-mono">{maskSensitive(ip.ipAddress, hidden)}</code>
                    {ip.version && <span className="text-[10px] text-muted-foreground">{ip.version}</span>}
                    {ip.type && <span className="text-[10px] text-muted-foreground">{ip.type}</span>}
                    {ip.geolocation && <span className="text-[10px] text-muted-foreground">{ip.geolocation}</span>}
                    {ip.reverse && (
                      <span className="ml-auto text-[11px] text-muted-foreground font-mono truncate" title={ip.reverse}>
                        ↩ {maskSensitive(ip.reverse, hidden)}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          <p className="text-[11px] text-muted-foreground px-1">
            {server.model && <>型号 <code className="font-mono">{server.model}</code></>}
            {info.data?.creation && <> · 开通 {formatDate(info.data.creation)}</>}
            {server.cluster && <> · 集群 <code className="font-mono">{server.cluster}</code></>}
            {server.slaMonitoring && <> · 已开 SLA 监控</>}
          </p>

          <VpsOptionsPanel serviceName={server.serviceName} region={region} />
        </TabsContent>

        <TabsContent value="snapshot" className="mt-4">
          <VpsSnapshotPane serviceName={server.serviceName} />
        </TabsContent>

        <TabsContent value="ddos" className="mt-4">
          <VpsMitigationPane serviceName={server.serviceName} />
        </TabsContent>

        <TabsContent value="maintenance" className="mt-4 space-y-4">
          <div className="border border-border rounded-2xl p-4 space-y-3">
            <h3 className="text-sm font-semibold">服务器别名</h3>
            <p className="text-[12px] text-muted-foreground">本地别名,不影响 OVH 真实名(`{maskSensitive(server.serviceName, hidden)}`)</p>
            <AliasEditor serviceName={server.serviceName} aliases={aliases} onSetAlias={onSetAlias} accountId={activeAccount} />
          </div>

          {!isUS ? (
            <div className="border border-border rounded-2xl p-4 space-y-3">
              <h3 className="text-sm font-semibold">变更联系人</h3>
              <p className="text-[12px] text-muted-foreground">切换 OVH admin / tech / billing NIC</p>
              <Button variant="outline" size="sm" onClick={() => setContactOpen(true)}>
                <Repeat className="w-3.5 h-3.5 mr-1" />
                变更联系人
              </Button>
            </div>
          ) : (
            <div className="border border-border rounded-2xl p-4 space-y-2 bg-secondary/30">
              <h3 className="text-sm font-semibold text-muted-foreground">美区限制</h3>
              <p className="text-[12px] text-muted-foreground">
                US OVHcloud 是独立公司,变更联系人等接口不可用。
              </p>
            </div>
          )}

          {/* 14 天撤单 */}
          <div className={cn(
            "border rounded-2xl p-4 space-y-2",
            retraction.data?.eligible ? "border-warning/50 bg-warning/5" : "border-border"
          )}>
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <div className="flex items-center gap-2">
                <Undo2 className={cn("w-4 h-4", retraction.data?.eligible ? "text-warning" : "text-muted-foreground")} />
                <h3 className="text-sm font-semibold">14 天无理由撤单（退款）</h3>
              </div>
              {retraction.data?.eligible && <Chip tone="warning">可退款</Chip>}
            </div>
            <p className="text-[12px] text-muted-foreground leading-relaxed">
              {retraction.isPending
                ? "正在查询撤单资格..."
                : retraction.data?.eligible
                  ? `当前处于 14 天撤单期内（还剩 ${retraction.data.hoursLeft ? (retraction.data.hoursLeft > 48 ? `${Math.ceil(retraction.data.hoursLeft / 24)} 天` : `${retraction.data.hoursLeft} 小时`) : "在期内"}）。`
                  : "当前不可撤单或已超期。"}
            </p>
            <Button
              variant={retraction.data?.eligible ? "destructive" : "outline"}
              size="sm"
              onClick={() => setRetractOpen(true)}
            >
              {retraction.data?.eligible ? "申请撤单 (可全额退款)" : "查看撤单说明"}
            </Button>
          </div>

          <div className="border border-destructive/40 bg-destructive/5 rounded-2xl p-4 space-y-2">
            <div className="flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 text-destructive" />
              <h3 className="text-sm font-semibold text-destructive">终止 VPS</h3>
            </div>
            <p className="text-[12px] text-muted-foreground">提交终止请求后 OVH 邮件发 token,确认后立即销毁。</p>
            <div className="flex gap-2 flex-wrap">
              <Button variant="destructive" size="sm" onClick={handleTerminate} disabled={terminate.isPending}>
                <CalendarPlus className="w-3.5 h-3.5 mr-1" />
                提交终止请求
              </Button>
              <Button variant="outline" size="sm" onClick={() => setTerminateOpen(true)}>
                有 token,直接确认终止
              </Button>
            </div>
          </div>
        </TabsContent>
      </Tabs>

      {/* 弹窗集合 */}
      <VpsReinstallDialog
        serviceName={server.serviceName}
        open={reinstallOpen}
        onOpenChange={setReinstallOpen}
      />

      {info.data && (
        <RenewalDialog
          serviceName={server.serviceName}
          info={info.data}
          open={renewalOpen}
          onOpenChange={setRenewalOpen}
          mutation={renewalMutation}
          termination={{ policy: vpsTermPolicy }}
        />
      )}

      <RetractionDialog
        serviceName={server.serviceName}
        displayName={vpsLabel}
        info={retraction.data}
        open={retractOpen}
        onOpenChange={setRetractOpen}
        isVps={true}
      />

      <Dialog open={contactOpen} onOpenChange={setContactOpen}>
        <ChangeContactInline
          serviceName={server.serviceName}
          onClose={() => setContactOpen(false)}
          mutation={contactMutation}
        />
      </Dialog>

      <EngagementDialog
        serviceName={server.serviceName}
        open={engagementOpen}
        onOpenChange={setEngagementOpen}
        hooks={vpsEngagementHooks}
      />

      <VpsTasksDialog
        serviceName={server.serviceName}
        open={tasksOpen}
        onOpenChange={setTasksOpen}
      />

      <VpsPowerDialogs
        stopOpen={stopOpen}
        setStopOpen={setStopOpen}
        onStop={handleStop}
        isStopPending={stop.isPending}
        rebootOpen={rebootOpen}
        setRebootOpen={setRebootOpen}
        onReboot={handleReboot}
        isRebootPending={reboot.isPending}
        terminateOpen={terminateOpen}
        setTerminateOpen={setTerminateOpen}
        termToken={termToken}
        setTermToken={setTermToken}
        onConfirmTerm={handleConfirmTerm}
        isConfirmTermPending={confirmTerm.isPending}
      />
    </div>
  );
}
