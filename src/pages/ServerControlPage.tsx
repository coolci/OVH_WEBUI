import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { Cpu, Terminal, Server, RefreshCw, Eye, EyeOff, CalendarClock, CalendarPlus, Repeat, Activity, Network, CalendarRange, Undo2 } from "lucide-react";
import { useEffect, useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import {
  useOwnedServers,
  useServerServiceInfo,
  useServerMonitoring,
  useToggleMonitoring,
  useRetraction,
  type OwnedServer,
} from "@/hooks/use-server-control";
import { useHideIp, maskSensitive } from "@/hooks/use-hide-ip";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import { useAccounts } from "@/hooks/use-accounts";
import { useServerAliases, useSetServerAlias, aliasOf } from "@/hooks/use-server-aliases";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { OverviewTab } from "@/components/server-control/OverviewTab";
import { PowerTab } from "@/components/server-control/PowerTab";
import { MaintenanceTab } from "@/components/server-control/MaintenanceTab";
import { AdvancedTab } from "@/components/server-control/AdvancedTab";
import { NetworkSpecsDialog } from "@/components/server-control/NetworkSpecsDialog";
import { RenewalDialog } from "@/components/server-control/RenewalDialog";
import { ReinstallDialog } from "@/components/server-control/ReinstallDialog";
import { RetractionDialog } from "@/components/server-control/RetractionDialog";
import { EngagementDialog } from "@/components/server-control/EngagementDialog";
import { DeviceMetaCapsules } from "@/components/common/DeviceMetaCapsules";
import { DeviceSwitcherCard } from "@/components/common/DeviceSwitcherCard";
import { toast } from "sonner";
import { cn } from "@/lib/utils";

const EMPTY_SERVERS: OwnedServer[] = [];

/** 服务器控制中心：顶部下拉切换服务器 + 4 tab 详情 */
function ServerControlPage() {
  const q = useOwnedServers();
  const { hidden, toggle } = useHideIp();
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [renaming, setRenaming] = useState<null | OwnedServer>(null);
  const [activeAccount, setActiveAccount] = useActiveServerControlAccount();
  const { data: accounts } = useAccounts();
  const { data: aliases } = useServerAliases();
  const servers = q.data || EMPTY_SERVERS;

  // 首次没选过账户 → 自动选默认账户
  useEffect(() => {
    if (!activeAccount && accounts && accounts.length > 0) {
      const def = accounts.find((a) => a.isDefault) || accounts[0];
      setActiveAccount(def.id);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accounts]);

  // 切换账户时,选中的 service 也清空(不同账户的服务器不一样)
  useEffect(() => {
    setSelectedName(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeAccount]);

  // 自动选中第一台（首次加载或切换列表后）
  useEffect(() => {
    if (servers.length === 0) {
      setSelectedName(null);
      return;
    }
    if (!selectedName || !servers.some((s) => s.serviceName === selectedName)) {
      setSelectedName(servers[0].serviceName);
    }
  }, [servers, selectedName]);

  const selected = servers.find((s) => s.serviceName === selectedName) || null;
  const activeAcc = accounts?.find((a) => a.id === activeAccount);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Cpu}
        title="服务器控制"
        description={
          activeAcc
            ? `管理 OVH 独立服务器 · 当前账户 ${activeAcc.name} (${activeAcc.zone})`
            : "管理 OVH 独立服务器"
        }
        action={
          <div className="flex items-center gap-2">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="sm" className="h-8 w-8 p-0 rounded-lg" onClick={toggle} aria-label={hidden ? "显示 IP / MAC" : "隐藏 IP"}>
                  {hidden ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>{hidden ? "已隐藏敏感信息 · 点击显示" : "隐藏 IP"}</TooltipContent>
            </Tooltip>
            <Button size="sm" className="h-8 gap-1.5 text-xs font-medium rounded-lg shadow-sm px-3" onClick={() => q.refetch()} disabled={q.isFetching} title="刷新服务器状态">
              <RefreshCw className={cn("w-3.5 h-3.5", q.isFetching && "animate-spin")} />
              <span>刷新</span>
            </Button>
          </div>
        }
      />

      {/* 一体化设备导航控制台 */}
      <DeviceSwitcherCard
        deviceType="server"
        accounts={(accounts || []).map((a) => ({
          id: a.id,
          name: a.name,
          zone: a.zone,
          isDefault: a.isDefault,
        }))}
        activeAccount={activeAccount || ""}
        onAccountChange={(v) => setActiveAccount(v || "")}
        items={servers.map((s) => ({
          serviceName: s.serviceName,
          name: aliasOf(aliases, s.serviceName, s.name),
          rawName: s.serviceName,
          datacenter: s.datacenter,
          subtext: s.commercialRange || "OVH 独服",
          state: s.state,
        }))}
        selectedName={selectedName || undefined}
        onSelectName={(name) => setSelectedName(name)}
        isFetching={q.isFetching}
        hidden={hidden}
        onRename={(serviceName) => {
          const s = servers.find((item) => item.serviceName === serviceName);
          if (s) setRenaming(s);
        }}
      />

      <RenameDialog
        server={renaming}
        currentAlias={renaming ? aliases?.[renaming.serviceName] || "" : ""}
        onClose={() => setRenaming(null)}
      />

      {q.isPending ? (
        <Skeleton className="h-[500px] rounded-xl" />
      ) : servers.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState
            icon={Server}
            title="暂无服务器"
            description="您的 OVH 账户下还没有独立服务器，或 API 没拿到数据"
          />
        </Card>
      ) : selected ? (
        <ServerTabs server={selected} />
      ) : null}
    </div>
  );
}



/** 服务器别名编辑对话框。alias 留空 + 保存 = 删除别名,恢复显示原 service_name。 */
function RenameDialog({
  server,
  currentAlias,
  onClose,
}: {
  server: OwnedServer | null;
  currentAlias: string;
  onClose: () => void;
}) {
  const { hidden } = useHideIp();
  const set = useSetServerAlias();
  const [value, setValue] = useState(currentAlias);
  useEffect(() => {
    setValue(currentAlias);
  }, [currentAlias, server?.serviceName]);

  if (!server) return null;
  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    await set.mutateAsync({ serviceName: server.serviceName, alias: value });
    onClose();
  };

  return (
    <Dialog open={!!server} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>设置别名</DialogTitle>
          <DialogDescription className="font-mono text-[11px]">
            {maskSensitive(server.serviceName, hidden)}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <Input
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="例如:kele(留空清除别名)"
            autoFocus
            maxLength={64}
          />
          <p className="text-[11px] text-muted-foreground">
            别名仅在本程序里显示,不会下发到 OVH。
          </p>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" disabled={set.isPending}>
              {set.isPending ? "保存中…" : value.trim() === "" ? "清除并保存" : "保存"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/**
 * 撤回期的悬停说明:把起止都写出来。
 * 撤回期是从下单起算的,机器常常下单后几天才交付。
 */
function retractionWindowText(r: { orderDate?: string; retractionDate?: string }): string {
  const fmt = (v?: string) => (v ? new Date(v).toLocaleString("zh-CN") : "");
  const end = fmt(r.retractionDate);
  const start = fmt(r.orderDate);
  if (!end) return "在撤回期内";
  if (!start) return `撤回期截止 ${end}（OVH 给的日期）`;
  return `撤回期 ${start} → ${end}\n从下单起算，不是从服务器开通起算`;
}

function ServerTabs({ server }: { server: OwnedServer }) {
  const info = useServerServiceInfo(server.serviceName);
  const monitoring = useServerMonitoring(server.serviceName);
  const toggleMon = useToggleMonitoring();
  const retraction = useRetraction(server.serviceName);
  const [retractOpen, setRetractOpen] = useState(false);
  const [netSpecsOpen, setNetSpecsOpen] = useState(false);
  const [renewalOpen, setRenewalOpen] = useState(false);
  const [reinstallOpen, setReinstallOpen] = useState(false);
  const [engagementOpen, setEngagementOpen] = useState(false);
  const { hidden } = useHideIp();
  const { data: aliases } = useServerAliases();

  const srvLabel = aliasOf(aliases, server.serviceName, server.name || server.serviceName);

  const handleToggleMonitoring = async () => {
    try {
      await toggleMon.mutateAsync({ serviceName: server.serviceName, enabled: !monitoring.data });
      toast.success(monitoring.data ? "OVH 监控已关闭" : "OVH 监控已开启");
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "操作失败");
    }
  };

  return (
    <div className="space-y-4">
      {/* 顶部设备摘要卡片 (移动端 App 风格) */}
      <Card className="surface-card rounded-xl border-border overflow-hidden">
        <CardContent className="p-3.5 sm:p-4 space-y-3">
          {/* 第一行: 别名/名称 + 状态 */}
          <div className="flex items-start justify-between gap-2.5">
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-semibold text-base sm:text-lg text-foreground truncate">
                  {maskSensitive(srvLabel, hidden)}
                </span>
                {server.commercialRange && (
                  <span className="px-2 py-0.5 rounded-md text-[11px] font-mono bg-secondary text-muted-foreground border border-border/60">
                    {server.commercialRange}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono flex-wrap">
                <span>{maskSensitive(server.serviceName, hidden)}</span>
                <span>·</span>
                <span>{(server.datacenter || "").toUpperCase()}</span>
                <span>·</span>
                <span>IP: {maskSensitive(server.ip, hidden)}</span>
              </div>
            </div>
            <Chip tone={server.state === "ok" ? "success" : "warning"} className="flex-shrink-0">
              <StatusDot tone={server.state === "ok" ? "success" : "warning"} pulse={server.state === "ok"} size="xs" />
              {server.state === "ok" ? "运行正常" : server.state}
            </Chip>
          </div>

          {/* 第二行: 属性胶囊 (撤单/系统/到期/开通/续费) */}
          <DeviceMetaCapsules
            retraction={retraction.data}
            onRetractClick={() => setRetractOpen(true)}
            os={{ rawName: server.os }}
            onOsClick={() => setReinstallOpen(true)}
            osClickTitle="点击进入重装系统"
            expiration={info.data?.expiration}
            creation={info.data?.creation}
            renewal={info.data}
            onRenewalClick={() => setRenewalOpen(true)}
          />
        </CardContent>
      </Card>

      {/* 快捷运维控制 (移动 App 风格 3 列对称网格) */}
      <div className="grid grid-cols-3 gap-2">
        <Button
          variant="outline"
          size="sm"
          className="h-9 text-xs font-normal border-border/80 justify-center"
          onClick={handleToggleMonitoring}
          disabled={toggleMon.isPending}
        >
          <Activity className={`w-3.5 h-3.5 mr-1.5 ${monitoring.data ? "text-success" : "text-muted-foreground"}`} />
          <span className="truncate">{monitoring.data ? "监控: 已开启" : "监控: 已关闭"}</span>
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="h-9 text-xs font-normal border-border/80 justify-center"
          onClick={() => setNetSpecsOpen(true)}
        >
          <Network className="w-3.5 h-3.5 mr-1.5 text-muted-foreground" />
          <span>网络规格</span>
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="h-9 text-xs font-normal border-border/80 justify-center"
          onClick={() => setEngagementOpen(true)}
        >
          <CalendarRange className="w-3.5 h-3.5 mr-1.5 text-muted-foreground" />
          <span>合同期</span>
        </Button>
      </div>

      {/* 分段选项卡 */}
      <Tabs defaultValue="overview" className="w-full">
        <TabsList className="grid grid-cols-4 w-full h-10 p-1 bg-muted/60 rounded-xl border border-border/40">
          <TabsTrigger value="overview" className="text-xs rounded-lg data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">概览</TabsTrigger>
          <TabsTrigger value="power" className="text-xs rounded-lg data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">电源</TabsTrigger>
          <TabsTrigger value="maintenance" className="text-xs rounded-lg data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">维护</TabsTrigger>
          <TabsTrigger value="advanced" className="text-xs rounded-lg data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">高级</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4">
          <OverviewTab server={server} />
        </TabsContent>
        <TabsContent value="power" className="mt-4">
          <PowerTab server={server} />
        </TabsContent>
        <TabsContent value="maintenance" className="mt-4">
          <MaintenanceTab server={server} />
        </TabsContent>
        <TabsContent value="advanced" className="mt-4">
          <AdvancedTab server={server} />
        </TabsContent>
      </Tabs>

      <NetworkSpecsDialog
        serviceName={server.serviceName}
        open={netSpecsOpen}
        onOpenChange={setNetSpecsOpen}
      />

      {info.data && (
        <RenewalDialog
          serviceName={server.serviceName}
          info={info.data}
          open={renewalOpen}
          onOpenChange={setRenewalOpen}
        />
      )}

      <ReinstallDialog
        serviceName={server.serviceName}
        open={reinstallOpen}
        onOpenChange={setReinstallOpen}
      />

      {retraction.data?.eligible && (
        <RetractionDialog
          serviceName={server.serviceName}
          displayName={srvLabel}
          info={retraction.data}
          open={retractOpen}
          onOpenChange={setRetractOpen}
        />
      )}

      <EngagementDialog
        serviceName={server.serviceName}
        open={engagementOpen}
        onOpenChange={setEngagementOpen}
      />
    </div>
  );
}

/** 紧凑胶囊:服务信息条的单元素。
 *  传 onClick → 视觉与右侧 outline 按钮(监控/网络规格)对齐:bg-background + accent hover,
 *  跟纯展示的胶囊(到期/开通/OS,bg-secondary/50)在外观上明确区分。 */
function InfoPill({
  icon,
  label,
  value,
  onClick,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  onClick?: () => void;
}) {
  // 注意:本项目 --accent 在亮色模式被定义为近黑色(用作强调对比),不能用作 hover bg。
  // 跟旁边 Button outline 变体对齐(用 hover:bg-muted,见 button.tsx)。
  const cls = [
    "inline-flex items-center gap-1.5 h-7 pl-2.5 pr-3 rounded-full border text-[12px]",
    onClick
      ? "border-border bg-background hover:bg-muted cursor-pointer transition-colors shadow-sm"
      : "border-border bg-secondary/50",
  ].join(" ");
  const inner = (
    <>
      <span className="flex items-center gap-1 text-muted-foreground">
        {icon}
        {label}
      </span>
      <span className="font-medium text-foreground">{value}</span>
    </>
  );
  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={cls}>
        {inner}
      </button>
    );
  }
  return <div className={cls}>{inner}</div>;
}

/** 续费状态友好文案。OVH 在 manager 后台标的 "Cancellation scheduled"
 *  其实就是 renew.deleteAtExpiration=true(到期不续 + 自动注销)。
 *
 *  - 到期注销         deleteAtExpiration=true (优先级最高,其它字段无意义)
 *  - 强制自动续费     forced=true (OVH 套餐限制,用户改不了)
 *  - 自动 / 手动      根据 automatic 显示,带 N 月周期
 */
function formatRenewal(info: {
  renewalType: boolean;
  renewalPeriod: number;
  renewalDeleteAtExpiration: boolean;
  renewalForced: boolean;
}): string {
  if (info.renewalDeleteAtExpiration) return "到期注销";
  const period = info.renewalPeriod > 0 ? ` · ${info.renewalPeriod}月` : "";
  if (info.renewalForced) return `强制自动${period}`;
  return (info.renewalType ? "自动" : "手动") + period;
}


const Page = () => (
  <>
    <Helmet>
      <title>服务器控制 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <ServerControlPage />
    </AppLayout>
  </>
);

export default Page;
