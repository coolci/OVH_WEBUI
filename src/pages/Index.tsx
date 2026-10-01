import { useState } from "react";
import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { Link } from "react-router-dom";
import {
  BarChart3,
  ClipboardList,
  Server,
  CheckCircle2,
  ChevronRight,
  Plus,
  Clock,
  Link2,
  Bot,
  Bell,
  Info,
  CheckCheck,
  Calendar,
  Zap,
  Activity,
  ShieldCheck,
  Radar,
  RefreshCw,
  type LucideIcon,
  Sparkles,
} from "lucide-react";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { EmptyState } from "@/components/common/EmptyState";
import { QuickOrderDialog } from "@/components/orders/QuickOrderDialog";
import { MonitorSubscribeDialog } from "@/components/monitor/MonitorSubscribeDialog";
import { Skeleton } from "@/components/common/Skeleton";
import { MetricRing } from "@/components/common/MetricRing";
import { useStats } from "@/hooks/use-stats";
import { useQueueList, type QueueItem } from "@/hooks/use-queue";
import { useSystemMetrics, useAppVersion, useUpdateCheck } from "@/hooks/use-system-metrics";
import { useMonitorList } from "@/hooks/use-monitor";
import { useTelegramPollerStatus } from "@/hooks/use-settings";
import { useAccounts } from "@/hooks/use-accounts";
import { isDcInStock } from "@/lib/datacenters";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

/** 仪表盘 v2.0: 运维中控指挥条 + 4 KPI 指标卡 + 实时盯盘与补货信号雷达 + 活跃队列/系统状态 + 资源波形监控 */
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
  const [monitorRefreshing, setMonitorRefreshing] = useState(false);

  const handleOpenQuickOrder = (planCode?: string) => {
    setQuickOrderPlanCode(planCode);
    setQuickOrderOpen(true);
  };

  const handleOpenSubscribe = (planCode?: string) => {
    setSubscribePlanCode(planCode);
    setSubscribeOpen(true);
  };

  const handleRefreshMonitor = async () => {
    setMonitorRefreshing(true);
    try {
      await Promise.all([monitorList.refetch(), stats.refetch()]);
      toast.success("已刷新最新盯盘与探针状态");
    } finally {
      setTimeout(() => setMonitorRefreshing(false), 450);
    }
  };

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* 页面主标题区 */}
      <PageHeader
        icon={BarChart3}
        title="仪表盘"
        description="OVH 智能抢购与基础设施运行中枢"
        action={
          <div className="flex items-center gap-2">
            <Button
              asChild
              variant="outline"
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg border-border/80 hover:bg-secondary"
            >
              <Link to="/monitor">
                <Radar className="h-3.5 w-3.5 text-primary" />
                <span className="hidden sm:inline">盯盘监控</span>
                <span className="sm:hidden">盯盘</span>
              </Link>
            </Button>
            <Button
              onClick={() => handleOpenQuickOrder()}
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg shadow-sm"
            >
              <Zap className="h-3.5 w-3.5" />
              快速下单
            </Button>
          </div>
        }
      />

      {/* 顶部运维中控指挥条 */}
      <div className="surface-card rounded-2xl p-3.5 sm:p-4 border border-border/80 bg-card/75 backdrop-blur-md shadow-sm">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3">
          <div className="flex items-center gap-3 flex-wrap">
            <div className="flex items-center gap-2 pr-3.5 border-r border-border/70">
              <span className="relative flex h-2.5 w-2.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500" />
              </span>
              <span className="text-xs font-semibold tracking-tight text-foreground">
                系统全勤在线
              </span>
            </div>

            <div className="flex items-center gap-3.5 sm:gap-5 text-xs text-muted-foreground flex-wrap">
              <div className="inline-flex items-center gap-1.5" title="抢购队列处理器状态">
                <StatusDot tone={stats.data?.queueProcessorRunning ? "success" : "warning"} size="xs" />
                <span>队列调度: <strong className="font-mono text-foreground font-medium">{stats.data?.queueProcessorRunning ? "运行中" : "就绪"}</strong></span>
              </div>
              <div className="inline-flex items-center gap-1.5" title="库存监控引擎状态">
                <StatusDot tone={stats.data?.monitorRunning ? "success" : "muted"} size="xs" />
                <span>盯盘引擎: <strong className="font-mono text-foreground font-medium">{stats.data?.monitorRunning ? "监测中" : "待启动"}</strong></span>
              </div>
              <div className="inline-flex items-center gap-1.5" title="Telegram Bot 轮询状态">
                <StatusDot tone={poller.data?.running ? "success" : "warning"} size="xs" />
                <span>Telegram: <strong className="font-mono text-foreground font-medium">{poller.data?.running ? `@${poller.data.botUsername || "在线"}` : "未连接"}</strong></span>
              </div>
              {defaultAccount && (
                <div className="inline-flex items-center gap-1.5" title="当前默认下单扣费账户">
                  <ShieldCheck className="w-3.5 h-3.5 text-primary" />
                  <span>默认账户: <strong className="font-medium text-foreground">{defaultAccount.name}</strong></span>
                </div>
              )}
            </div>
          </div>

          <div className="flex items-center gap-2 self-end lg:self-auto text-[11px] text-muted-foreground font-mono">
            <span>活跃型号: <strong className="text-primary font-bold">{monitorCount}</strong></span>
            <span className="opacity-40">/</span>
            <span>库存服务器: <strong className="text-foreground font-medium">{stats.data?.totalServers ?? 0}</strong></span>
          </div>
        </div>
      </div>

      {/* 4 大现代化 KPI 卡片 */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
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
          hint={`历史总轮询捕获记录`}
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

      {/* 实时盯盘与补货信号雷达 */}
      <Card className="surface-card rounded-2xl border-border/80 overflow-hidden shadow-sm">
        <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40 flex flex-row items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="relative flex items-center justify-center">
              <span className={cn(
                "animate-ping absolute inline-flex h-3 w-3 rounded-full opacity-75",
                stats.data?.monitorRunning ? "bg-emerald-400" : "bg-muted"
              )} />
              <Radar className={cn(
                "w-4 h-4 relative",
                stats.data?.monitorRunning ? "text-primary" : "text-muted-foreground"
              )} />
            </div>
            <div>
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <span>实时盯盘与补货信号雷达</span>
                <span className={cn(
                  "text-[10px] px-2 py-0.5 rounded-full font-mono font-medium border",
                  stats.data?.monitorRunning
                    ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
                    : "bg-muted text-muted-foreground border-border"
                )}>
                  {stats.data?.monitorRunning ? "探针实时轮询中" : "监控引擎待机"}
                </span>
              </CardTitle>
            </div>
          </div>
          <div className="flex items-center gap-1.5 sm:gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleRefreshMonitor}
              disabled={monitorRefreshing || monitorList.isFetching}
              className="h-7 w-7 p-0 rounded-lg border-border/80 hover:bg-secondary flex-shrink-0"
              title="刷新探针状态"
            >
              <RefreshCw className={cn("h-3 w-3", (monitorRefreshing || monitorList.isFetching) && "animate-spin")} />
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={() => handleOpenSubscribe()}
              className="h-7 text-xs gap-1 px-2.5 rounded-lg border-border/80 hover:bg-secondary font-medium"
            >
              <Plus className="w-3 h-3" />
              <span>添加盯盘</span>
            </Button>
            <Link
              to="/monitor"
              className="text-xs text-muted-foreground hover:text-primary transition-colors inline-flex items-center gap-0.5 font-medium ml-1"
            >
              全部监控
              <ChevronRight className="w-3 h-3" />
            </Link>
          </div>
        </CardHeader>
        <CardContent className="p-4 sm:p-5">
          {monitorList.isPending ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-28 rounded-xl" />
              ))}
            </div>
          ) : (monitorList.data || []).length > 0 ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
              {(monitorList.data || []).map((sub) => {
                const targetDcs = (sub.datacenters && sub.datacenters.length > 0) ? sub.datacenters : ["gra", "rbx", "sbg", "bhs"];
                const inStockDcList = Object.entries(sub.lastStatus || {}).filter(([_, status]) => isDcInStock(status));
                const hasStock = inStockDcList.length > 0;

                return (
                  <div
                    key={sub.planCode}
                    className={cn(
                      "p-3.5 rounded-xl border transition-all duration-200 flex flex-col justify-between gap-2.5",
                      hasStock
                        ? "border-emerald-500/50 bg-emerald-500/10 shadow-sm"
                        : "border-border/70 hover:border-primary/40 bg-secondary/20 hover:bg-secondary/35"
                    )}
                  >
                    <div>
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-1.5 min-w-0">
                          <span className="font-mono text-sm font-bold text-foreground truncate">
                            {sub.planCode}
                          </span>
                          {sub.serverName && (
                            <span className="text-[11px] text-muted-foreground truncate">
                              ({sub.serverName})
                            </span>
                          )}
                        </div>
                        {sub.autoOrder ? (
                          <span className="text-[10px] font-mono px-2 py-0.5 rounded-md bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30 flex items-center gap-1 font-semibold flex-shrink-0">
                            <Zap className="w-2.5 h-2.5" />
                            自抢 x{sub.quantity || 1}
                          </span>
                        ) : (
                          <span className="text-[10px] font-mono px-2 py-0.5 rounded-md bg-secondary text-muted-foreground border border-border flex items-center gap-1 font-medium flex-shrink-0">
                            <Bell className="w-2.5 h-2.5" />
                            仅通知
                          </span>
                        )}
                      </div>

                      {/* 目标机房探针状态 */}
                      <div className="flex items-center gap-1.5 flex-wrap mt-2">
                        {targetDcs.slice(0, 6).map((dc) => {
                          const status = sub.lastStatus?.[dc.toLowerCase()];
                          const inStock = isDcInStock(status);
                          return (
                            <span
                              key={dc}
                              className={cn(
                                "text-[10px] font-mono uppercase px-1.5 py-0.5 rounded border transition-colors flex items-center gap-1",
                                inStock
                                  ? "bg-emerald-500/20 text-emerald-600 dark:text-emerald-300 border-emerald-500/40 font-bold"
                                  : "bg-background/60 text-muted-foreground/80 border-border/60"
                              )}
                              title={status ? `${dc.toUpperCase()}: ${status}` : `${dc.toUpperCase()}: 持续探测中`}
                            >
                              <span className={cn(
                                "w-1.5 h-1.5 rounded-full",
                                inStock ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                              )} />
                              {dc}
                              {inStock && <span className="text-[9px] font-normal">({status})</span>}
                            </span>
                          );
                        })}
                        {targetDcs.length > 6 && (
                          <span className="text-[10px] text-muted-foreground font-mono">
                            +{targetDcs.length - 6}
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="flex items-center justify-between pt-2 border-t border-border/40 text-[11px]">
                      {hasStock ? (
                        <div className="flex items-center gap-1 text-emerald-600 dark:text-emerald-400 font-semibold text-xs">
                          <CheckCircle2 className="w-3.5 h-3.5" />
                          <span>发现现货补货！</span>
                        </div>
                      ) : (
                        <span className="text-muted-foreground/75 font-mono text-[10.5px]">
                          持续盯盘中 · 待放货
                        </span>
                      )}
                      <div className="flex items-center gap-1.5">
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={() => handleOpenSubscribe(sub.planCode)}
                          className="h-6 text-[11px] px-2 rounded-md font-medium text-muted-foreground hover:text-foreground"
                        >
                          设置
                        </Button>
                        <Button
                          size="sm"
                          variant={hasStock ? "default" : "secondary"}
                          onClick={() => handleOpenQuickOrder(sub.planCode)}
                          className={cn(
                            "h-6 text-[11px] px-2.5 rounded-md font-medium",
                            hasStock && "bg-emerald-600 hover:bg-emerald-500 text-white"
                          )}
                        >
                          {hasStock ? "立即秒抢" : "下单"}
                        </Button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          ) : (
            /* 雷达空态巡航视窗 */
            <div className="py-6 sm:py-8 flex flex-col items-center justify-center text-center">
              <div className="relative flex items-center justify-center w-12 h-12 rounded-full bg-primary/10 text-primary mb-3">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-primary/20 opacity-75" />
                <Radar className="w-6 h-6" />
              </div>
              <h3 className="text-sm font-semibold text-foreground">监控雷达处于巡航待机状态</h3>
              <p className="text-xs text-muted-foreground mt-1 max-w-md">
                当前未挂载任何盯盘任务。添加型号后，后台将 24h 持续探测 OVH 库存，一旦放货秒速通知或自动下单。
              </p>

              {/* 热门抢购型号推荐池 */}
              <div className="mt-4 pt-4 border-t border-border/50 w-full max-w-xl">
                <div className="text-[11px] font-medium text-muted-foreground mb-2 flex items-center justify-center gap-1">
                  <Sparkles className="w-3 h-3 text-amber-500" />
                  <span>一键上架热门盯盘型号:</span>
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                  {[
                    { code: "24ska01", name: "KS-LE-1 爆款", desc: "高性价比独服" },
                    { code: "24ska02", name: "KS-LE-2 进阶", desc: "大内存计算" },
                    { code: "24sk602", name: "Rise 系列", desc: "高频主频" },
                    { code: "24sk40", name: "Eco 入门", desc: "低价轻量" },
                  ].map((item) => (
                    <button
                      key={item.code}
                      type="button"
                      onClick={() => handleOpenSubscribe(item.code)}
                      className="p-2 rounded-xl border border-border/70 hover:border-primary/50 bg-secondary/30 hover:bg-secondary/60 text-left transition-colors group touch-manipulation"
                    >
                      <div className="font-mono text-xs font-bold text-foreground group-hover:text-primary transition-colors">
                        {item.code}
                      </div>
                      <div className="text-[10px] text-muted-foreground mt-0.5 truncate">
                        {item.name}
                      </div>
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 中部：活跃队列 + 系统状态 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="surface-card rounded-2xl overflow-hidden lg:col-span-2 border-border/80 shadow-sm">
          <CardContent className="p-4 sm:p-6">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <ClipboardList className="w-4 h-4 text-primary" />
                <h2 className="text-[15px] font-semibold">活跃抢购队列</h2>
              </div>
              <Link to="/queue" className="text-xs text-muted-foreground hover:text-primary transition-colors inline-flex items-center gap-1 font-medium">
                查看全部 ({queue.data?.length || 0})
                <ChevronRight className="w-3 h-3" />
              </Link>
            </div>
            {queue.isPending ? (
              <div className="space-y-2">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-16 rounded-xl" />
                ))}
              </div>
            ) : activeQueue.length === 0 ? (
              <EmptyState
                icon={Calendar}
                title="暂无活跃抢购任务"
                action={
                  <Button asChild size="sm" className="h-8 text-xs font-medium rounded-lg">
                    <Link to="/queue">
                      <Plus className="w-3.5 h-3.5 mr-1" />
                      创建抢购任务
                    </Link>
                  </Button>
                }
              />
            ) : (
              <div className="space-y-2.5">
                {activeQueue.map((q) => (
                  <div
                    key={q.id}
                    className="flex items-center justify-between gap-3 rounded-xl px-4 py-3 bg-secondary/30 border border-border/70 hover:border-primary/40 transition-colors"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-sm font-mono text-foreground">{q.planCode}</span>
                        <span className="text-[10px] font-mono uppercase bg-primary/10 text-primary border border-primary/20 px-1.5 py-0.5 rounded">
                          {q.datacenter}
                        </span>
                      </div>
                      <div className="flex items-center gap-2 text-[11px] text-muted-foreground mt-1">
                        <span className="inline-flex items-center gap-1">
                          <Clock className="w-3 h-3 text-muted-foreground/70" />
                          已尝试 {q.retryCount + 1} 次
                        </span>
                        {q.retryInterval && (
                          <>
                            <span className="text-muted-foreground/40">·</span>
                            <span>间隔 {q.retryInterval}s</span>
                          </>
                        )}
                      </div>
                    </div>
                    <QueueStatusChip status={q.status} />
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* 系统运行状态 */}
        <Card className="surface-card rounded-2xl overflow-hidden border-border/80 shadow-sm">
          <CardContent className="p-4 sm:p-6">
            <div className="flex items-center gap-2 mb-4">
              <CheckCheck className="w-4 h-4 text-primary" />
              <h2 className="text-[15px] font-semibold">系统运行状态</h2>
            </div>
            <div className="space-y-1">
              <SystemRow
                icon={<Link2 className="w-3.5 h-3.5" />}
                label="API 连接"
                ok={!!stats.data}
                onText="已连接"
                offText="未连接"
              />
              <SystemRow
                icon={<Bot className="w-3.5 h-3.5" />}
                label="自动抢购调度"
                ok={(stats.data?.activeQueues || 0) > 0}
                onText="活跃轮询中"
                offText="空闲待机"
                neutralOff
              />
              <SystemRow
                icon={<Bell className="w-3.5 h-3.5" />}
                label="库存监控引擎"
                ok={!!stats.data?.monitorRunning}
                onText="实时运行中"
                offText="待启用"
                warnOff
              />
              <div className="flex justify-between items-center px-3 py-2.5 mt-2 border-t border-border/70 pt-3">
                <div className="inline-flex items-center gap-2 text-xs text-muted-foreground">
                  <Info className="w-3.5 h-3.5" />
                  系统版本
                </div>
                <div className="inline-flex items-center gap-2">
                  <span className="text-xs font-mono font-semibold">v{version.data || "—"}</span>
                  {update.data?.hasUpdate ? (
                    <a
                      href={update.data.url}
                      target="_blank"
                      rel="noreferrer noopener"
                      className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-primary/15 text-primary hover:bg-primary/25 transition-colors"
                      title={`有新版本 v${update.data.latest} 可用,点击查看`}
                    >
                      <Sparkles className="w-3 h-3" />
                      v{update.data.latest}
                    </a>
                  ) : null}
                </div>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* 系统硬件监控: CPU / 内存 / 磁盘 三个圆环 */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card className="surface-card rounded-2xl overflow-hidden border-border/80 shadow-sm">
          <CardContent className="p-0">
            <MetricRing
              label="CPU"
              subLabel={sys.data ? `${sys.data.cpu.cores} 核心` : "—"}
              percent={sys.data?.cpu.percent ?? 0}
            />
          </CardContent>
        </Card>
        <Card className="surface-card rounded-2xl overflow-hidden border-border/80 shadow-sm">
          <CardContent className="p-0">
            <MetricRing
              label="内存"
              subLabel={
                sys.data
                  ? `${formatBytesShort(sys.data.memory.usedBytes)} / ${formatBytesShort(sys.data.memory.totalBytes)}`
                  : "—"
              }
              percent={sys.data?.memory.percent ?? 0}
            />
          </CardContent>
        </Card>
        <Card className="surface-card rounded-2xl overflow-hidden border-border/80 shadow-sm">
          <CardContent className="p-0">
            <MetricRing
              label={sys.data?.disk.path || "磁盘"}
              subLabel={
                sys.data
                  ? `${formatBytesShort(sys.data.disk.usedBytes)} / ${formatBytesShort(sys.data.disk.totalBytes)}`
                  : "—"
              }
              percent={sys.data?.disk.percent ?? 0}
            />
          </CardContent>
        </Card>
      </div>

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
        mode={subscribePlanCode && (monitorList.data || []).some((s) => s.planCode === subscribePlanCode) ? "edit" : "create"}
        planCode={subscribePlanCode}
        serverName={(monitorList.data || []).find((s) => s.planCode === subscribePlanCode)?.serverName}
        lockPlanCode={!!subscribePlanCode && (monitorList.data || []).some((s) => s.planCode === subscribePlanCode)}
        initial={(monitorList.data || []).find((s) => s.planCode === subscribePlanCode) || null}
      />
    </div>
  );
}

/** 字节短格式: 1.5 GB / 11.4 GB */
function formatBytesShort(n: number): string {
  if (!n || n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB", "PB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}

/* ---------- 轻量微图折线 (Sparkline) ---------- */

function Sparkline({ points, color = "currentColor" }: { points: number[]; color?: string }) {
  const min = Math.min(...points);
  const max = Math.max(...points);
  const range = max - min || 1;
  const width = 56;
  const height = 22;
  const path = points
    .map((p, i) => {
      const x = (i / (points.length - 1)) * width;
      const y = height - ((p - min) / range) * (height - 4) - 2;
      return `${i === 0 ? "M" : "L"} ${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg width={width} height={height} className="overflow-visible opacity-60">
      <path d={path} fill="none" stroke={color} strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/* ---------- 小组件 ---------- */

function KpiCard({
  label,
  value,
  extra,
  hint,
  icon: Icon,
  linkTo,
  linkText,
  loading,
  sparkColor,
  sparkPoints,
}: {
  label: string;
  value: number | undefined;
  extra?: React.ReactNode;
  hint?: string;
  icon: LucideIcon;
  linkTo: string;
  linkText: string;
  loading?: boolean;
  sparkColor?: string;
  sparkPoints?: number[];
}) {
  return (
    <Card className="surface-card group hover:border-primary/40 transition-all duration-200 rounded-2xl overflow-hidden border-border/80 shadow-sm hover:-translate-y-0.5">
      <CardContent className="p-3.5 sm:p-5">
        <div className="flex items-center justify-between mb-2 sm:mb-3">
          <span className="text-[11px] sm:text-xs font-semibold uppercase tracking-wider text-muted-foreground/80">{label}</span>
          <div className="w-8 h-8 sm:w-10 sm:h-10 rounded-xl bg-secondary/60 border border-border/60 flex items-center justify-center transition-transform duration-200 group-hover:scale-105">
            <Icon className="w-4 h-4 sm:w-5 sm:h-5 text-foreground/90" strokeWidth={1.8} />
          </div>
        </div>
        <div className="flex items-baseline justify-between">
          <div className="flex items-baseline">
            {loading ? (
              <Skeleton className="w-16 h-8 sm:h-9 rounded-lg" />
            ) : (
              <span className="text-2xl sm:text-3xl font-bold font-mono leading-none tracking-tight text-foreground">
                {value ?? 0}
              </span>
            )}
            {extra}
          </div>
          {sparkPoints && (
            <div className="hidden xs:block">
              <Sparkline points={sparkPoints} color={sparkColor} />
            </div>
          )}
        </div>
        {hint && (
          <p className="text-[11px] text-muted-foreground/75 mt-1.5 truncate">
            {hint}
          </p>
        )}
        <Link
          to={linkTo}
          className="mt-3 inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-primary transition-colors"
        >
          {linkText}
          <ChevronRight className="w-3 h-3 group-hover:translate-x-0.5 transition-transform" />
        </Link>
      </CardContent>
    </Card>
  );
}

function QueueStatusChip({ status }: { status: string }) {
  if (status === "running")
    return (
      <Chip tone="success">
        <StatusDot tone="success" pulse size="xs" />
        运行中
      </Chip>
    );
  if (status === "pending")
    return (
      <Chip tone="warning">
        <StatusDot tone="warning" size="xs" />
        等待中
      </Chip>
    );
  return (
    <Chip tone="default">
      <StatusDot tone="muted" size="xs" />
      已暂停
    </Chip>
  );
}

function SystemRow({
  icon,
  label,
  ok,
  onText,
  offText,
  neutralOff,
  warnOff,
}: {
  icon: React.ReactNode;
  label: string;
  ok: boolean;
  onText: string;
  offText: string;
  neutralOff?: boolean;
  warnOff?: boolean;
}) {
  const dotTone = ok ? "success" : warnOff ? "warning" : neutralOff ? "muted" : "danger";
  return (
    <div className="flex justify-between items-center px-3 py-2.5 rounded-xl hover:bg-muted/50 dark:hover:bg-white/[0.04] transition-colors">
      <div className="inline-flex items-center gap-2 text-[13px] font-medium text-foreground/90">
        <span className="text-muted-foreground/80">{icon}</span>
        <span>{label}</span>
      </div>
      <div className="inline-flex items-center gap-1.5">
        <StatusDot tone={dotTone as any} pulse={ok} size="xs" />
        <span className={`text-xs ${ok ? "font-semibold text-foreground/90" : "text-muted-foreground/70"}`}>{ok ? onText : offText}</span>
      </div>
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
