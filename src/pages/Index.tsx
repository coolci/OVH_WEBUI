import { useState } from "react";
import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { Link } from "react-router-dom";
import {
  LayoutDashboard,
  ClipboardList,
  Server,
  CheckCircle2,
  Radar,
  RefreshCw,
  Zap,
} from "lucide-react";
import { PageHeader } from "@/components/common/PageHeader";
import { Button } from "@/components/ui/button";
import { QuickOrderDialog } from "@/components/orders/QuickOrderDialog";
import { MonitorSubscribeDialog } from "@/components/monitor/MonitorSubscribeDialog";
import { useStats } from "@/hooks/use-stats";
import { useQueueList, type QueueItem } from "@/hooks/use-queue";
import { useSystemMetrics, useAppVersion, useUpdateCheck } from "@/hooks/use-system-metrics";
import { useMonitorList } from "@/hooks/use-monitor";
import { useTelegramPollerStatus } from "@/hooks/use-settings";
import { useAccounts } from "@/hooks/use-accounts";
import { OperationBanner } from "@/components/dashboard/OperationBanner";
import { KpiCard } from "@/components/dashboard/KpiCard";
import { StockRadarSection } from "@/components/dashboard/StockRadarSection";
import { ActiveQueueCard } from "@/components/dashboard/ActiveQueueCard";
import { CoreServiceCard } from "@/components/dashboard/CoreServiceCard";
import { TelemetryCard } from "@/components/dashboard/TelemetryCard";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

/**
 * 仪表盘 v3.0 (Obsidian Slate 旗舰中枢 - 模块化解耦架构)
 */
function DashboardPage() {
  const stats = useStats();
  const queue = useQueueList();
  const sys = useSystemMetrics();
  const version = useAppVersion();
  const update = useUpdateCheck();
  const monitorList = useMonitorList();
  const poller = useTelegramPollerStatus();
  const accountsQ = useAccounts();

  const activeQueue: QueueItem[] = (queue.data || [])
    .filter((i) => ["running", "pending", "paused"].includes(i.status))
    .slice(0, 4);

  const defaultAccount = (accountsQ.data || []).find((a) => a.isDefault) || accountsQ.data?.[0];
  const monitorCount = monitorList.data?.length ?? 0;
  const successCount = stats.data?.purchaseSuccess ?? 0;
  const failedCount = stats.data?.purchaseFailed ?? 0;
  const totalAttempts = successCount + failedCount;
  const successRate = totalAttempts > 0 ? Math.round((successCount / totalAttempts) * 100) : 100;

  const [quickOrderOpen, setQuickOrderOpen] = useState(false);
  const [quickOrderPlanCode, setQuickOrderPlanCode] = useState<string | undefined>(undefined);
  const [subscribeOpen, setSubscribeOpen] = useState(false);
  const [subscribePlanCode, setSubscribePlanCode] = useState<string | undefined>(undefined);
  const [isRefreshingAll, setIsRefreshingAll] = useState(false);

  const handleOpenQuickOrder = (planCode?: string) => {
    setQuickOrderPlanCode(planCode);
    setQuickOrderOpen(true);
  };

  const handleOpenSubscribe = (planCode?: string) => {
    setSubscribePlanCode(planCode);
    setSubscribeOpen(true);
  };

  const handleRefreshAll = async () => {
    setIsRefreshingAll(true);
    try {
      await Promise.all([
        stats.refetch(),
        queue.refetch(),
        monitorList.refetch(),
        sys.refetch(),
        poller.refetch(),
      ]);
      toast.success("仪表盘核心数据已刷新至最新状态");
    } finally {
      setTimeout(() => setIsRefreshingAll(false), 500);
    }
  };

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* 1. 页面顶栏操作区 */}
      <PageHeader
        icon={LayoutDashboard}
        title="仪表盘"
        description="OVH 智能抢购与基础设施运行中枢"
        action={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefreshAll}
              disabled={isRefreshingAll}
              className="h-8 gap-1.5 text-xs font-medium rounded-lg border-border/80 hover:bg-secondary px-2.5 sm:px-3 touch-manipulation"
              title="全局刷新指标"
            >
              <RefreshCw className={cn("w-3.5 h-3.5", isRefreshingAll && "animate-spin")} />
              <span className="hidden sm:inline">刷新数据</span>
            </Button>
            <Button
              asChild
              variant="outline"
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg border-border/80 hover:bg-secondary px-2.5 sm:px-3 touch-manipulation"
            >
              <Link to="/monitor">
                <Radar className="w-3.5 h-3.5 text-primary" />
                <span className="hidden sm:inline">盯盘监控</span>
                <span className="sm:hidden">盯盘</span>
              </Link>
            </Button>
            <Button
              onClick={() => handleOpenQuickOrder()}
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg shadow-sm px-3 touch-manipulation"
            >
              <Zap className="w-3.5 h-3.5" />
              <span>快速下单</span>
            </Button>
          </div>
        }
      />

      {/* 2. 运维中控指挥条 (响应式对称布局) */}
      <OperationBanner
        stats={stats.data}
        poller={poller.data}
        defaultAccount={defaultAccount}
        monitorCount={monitorCount}
      />

      {/* 3. 4 大现代化 KPI 卡片 (对齐 md:grid-cols-4) */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 sm:gap-3.5">
        <KpiCard
          label="活跃队列"
          value={stats.data?.activeQueues}
          hint={activeQueue.length > 0 ? `${activeQueue.length} 正在发起重试` : "队列就绪中"}
          icon={ClipboardList}
          linkTo="/queue"
          linkText="管理队列"
          loading={stats.isPending}
          sparkColor="hsl(var(--primary))"
          sparkPoints={[2, 3, 2, 4, 3, 5, stats.data?.activeQueues ?? 2]}
        />
        <KpiCard
          label="盯盘型号"
          value={monitorCount}
          hint={stats.data?.monitorRunning ? "秒级轮询库存中" : "可随时添加订阅"}
          icon={Radar}
          linkTo="/monitor"
          linkText="监控列表"
          loading={monitorList.isPending}
          sparkColor="#8b5cf6"
          sparkPoints={[3, 4, 5, 4, 6, 7, monitorCount || 4]}
        />
        <KpiCard
          label="捕获成功"
          value={stats.data?.purchaseSuccess}
          extra={
            totalAttempts > 0 && (
              <span className="text-[11px] text-muted-foreground ml-2">
                成功率 <strong className="text-emerald-500 font-semibold">{successRate}%</strong>
              </span>
            )
          }
          hint="历史总轮询捕获记录"
          icon={CheckCircle2}
          linkTo="/history"
          linkText="查看记录"
          loading={stats.isPending}
          sparkColor="#10b981"
          sparkPoints={[1, 2, 3, 5, 4, 6, stats.data?.purchaseSuccess ?? 5]}
        />
        <KpiCard
          label="可用现货"
          value={stats.data?.availableServers}
          extra={
            stats.data && (
              <span className="text-[11px] text-muted-foreground ml-2">
                共 <span className="font-semibold text-foreground">{stats.data.totalServers}</span>
              </span>
            )
          }
          hint="可直接下单免抢购"
          icon={Server}
          linkTo="/servers"
          linkText="立即挑选"
          loading={stats.isPending}
          sparkColor="#0ea5e9"
          sparkPoints={[8, 12, 11, 14, 13, 16, stats.data?.availableServers ?? 12]}
        />
      </div>

      {/* 4. 实时盯盘与补货信号雷达 */}
      <StockRadarSection
        monitorRunning={stats.data?.monitorRunning}
        subscriptions={monitorList.data || []}
        isPending={monitorList.isPending}
        isFetching={monitorList.isFetching}
        onRefetch={() => monitorList.refetch()}
        onOpenSubscribe={handleOpenSubscribe}
        onOpenQuickOrder={handleOpenQuickOrder}
      />

      {/* 5. 核心两栏作业区：活跃队列 + 系统状态 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <ActiveQueueCard
          activeQueue={activeQueue}
          totalQueueCount={queue.data?.length || 0}
          isPending={queue.isPending}
        />
        <CoreServiceCard
          stats={stats.data}
          version={version.data}
          update={update.data}
        />
      </div>

      {/* 6. 系统硬件遥测舱 */}
      <TelemetryCard metrics={sys.data} />

      {/* 弹窗 */}
      <QuickOrderDialog
        open={quickOrderOpen}
        onOpenChange={(v) => {
          setQuickOrderOpen(v);
          if (!v) setQuickOrderPlanCode(undefined);
        }}
        initialPlanCode={quickOrderPlanCode}
      />

      <MonitorSubscribeDialog
        open={subscribeOpen}
        onOpenChange={(v) => {
          setSubscribeOpen(v);
          if (!v) setSubscribePlanCode(undefined);
        }}
        mode={
          subscribePlanCode &&
          (monitorList.data || []).some((s) => s.planCode === subscribePlanCode)
            ? "edit"
            : "create"
        }
        planCode={subscribePlanCode}
        serverName={
          (monitorList.data || []).find((s) => s.planCode === subscribePlanCode)?.serverName
        }
        lockPlanCode={
          !!subscribePlanCode &&
          (monitorList.data || []).some((s) => s.planCode === subscribePlanCode)
        }
        initial={
          (monitorList.data || []).find((s) => s.planCode === subscribePlanCode) || null
        }
      />
    </div>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>仪表盘 | OVH 统御</title>
    </Helmet>
    <AppLayout>
      <DashboardPage />
    </AppLayout>
  </>
);

export default Page;
