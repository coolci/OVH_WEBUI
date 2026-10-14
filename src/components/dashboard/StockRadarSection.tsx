import { Link } from "react-router-dom";
import {
  Radar,
  RefreshCw,
  Plus,
  ChevronRight,
  CheckCircle2,
  Zap,
  Bell,
  Sparkles,
} from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { StatusDot } from "@/components/common/StatusDot";
import { isDcInStock } from "@/lib/datacenters";
import { cn } from "@/lib/utils";
import type { MonitorSubscription } from "@/hooks/use-monitor";

interface StockRadarSectionProps {
  monitorRunning?: boolean;
  subscriptions: MonitorSubscription[];
  isPending: boolean;
  isFetching: boolean;
  onRefetch: () => void;
  onOpenSubscribe: (planCode?: string) => void;
  onOpenQuickOrder: (planCode?: string) => void;
}

const RECOMMENDED_PLANS = [
  { code: "24sk102", name: "KS-1 爆款入门", desc: "高性价比独服" },
  { code: "24sk202", name: "KS-2 进阶主力", desc: "大内存计算" },
  { code: "24sk40-v1", name: "KS-4 轻量入门", desc: "低价轻量" },
  { code: "24sk602", name: "KS-6 大内存神机", desc: "Epyc · 128G" },
];

export function StockRadarSection({
  monitorRunning,
  subscriptions,
  isPending,
  isFetching,
  onRefetch,
  onOpenSubscribe,
  onOpenQuickOrder,
}: StockRadarSectionProps) {
  return (
    <Card className="surface-card rounded-2xl border-border/80 overflow-hidden shadow-sm">
      <CardHeader className="p-3.5 sm:p-5 pb-3 border-b border-border/40 flex flex-row items-center justify-between gap-2">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="relative flex items-center justify-center flex-shrink-0">
            <span
              className={cn(
                "animate-ping absolute inline-flex h-3 w-3 rounded-full opacity-75",
                monitorRunning ? "bg-primary" : "bg-muted"
              )}
            />
            <Radar
              className={cn(
                "w-4 h-4 relative",
                monitorRunning ? "text-primary" : "text-muted-foreground"
              )}
            />
          </div>
          <div className="min-w-0">
            <CardTitle className="text-sm font-semibold flex items-center gap-2 flex-wrap">
              <span className="truncate">实时盯盘与补货信号雷达</span>
              <Chip
                tone={monitorRunning ? "success" : "default"}
                className="flex-shrink-0 font-mono text-[10px]"
              >
                {monitorRunning ? "探针实时轮询中" : "监控引擎待机"}
              </Chip>
            </CardTitle>
          </div>
        </div>

        <div className="flex items-center gap-1.5 sm:gap-2 flex-shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={onRefetch}
            disabled={isFetching}
            className="h-8 w-8 p-0 rounded-lg border-border/80 hover:bg-secondary flex-shrink-0"
            title="刷新探针状态"
          >
            <RefreshCw className={cn("w-3.5 h-3.5", isFetching && "animate-spin")} />
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => onOpenSubscribe()}
            className="h-8 text-xs gap-1.5 px-2.5 sm:px-3 rounded-lg border-border/80 hover:bg-secondary font-medium"
          >
            <Plus className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">添加盯盘</span>
            <span className="sm:hidden">添加</span>
          </Button>
          <Link
            to="/monitor"
            className="text-xs text-muted-foreground hover:text-primary transition-colors inline-flex items-center gap-0.5 font-medium ml-1"
          >
            <span className="hidden sm:inline">全部监控</span>
            <ChevronRight className="w-3.5 h-3.5" />
          </Link>
        </div>
      </CardHeader>

      <CardContent className="p-3.5 sm:p-5">
        {isPending ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-28 rounded-xl" />
            ))}
          </div>
        ) : subscriptions.length > 0 ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {subscriptions.map((sub) => {
              const targetDcs =
                sub.datacenters && sub.datacenters.length > 0
                  ? sub.datacenters
                  : ["gra", "rbx", "sbg", "bhs"];
              const inStockDcList = Object.entries(sub.lastStatus || {}).filter(
                ([_, status]) => isDcInStock(status)
              );
              const hasStock = inStockDcList.length > 0;

              return (
                <div
                  key={sub.planCode}
                  className={cn(
                    "p-3.5 rounded-xl border transition-all duration-200 flex flex-col justify-between gap-2.5",
                    hasStock
                      ? "border-primary/50 bg-primary/10 shadow-sm"
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
                        <Chip tone="success" className="flex-shrink-0 rounded-md font-mono text-[10px] font-semibold">
                          <Zap className="w-3 h-3" />
                          自抢 x{sub.quantity || 1}
                        </Chip>
                      ) : (
                        <Chip tone="default" className="flex-shrink-0 rounded-md font-mono text-[10px] text-muted-foreground">
                          <Bell className="w-3 h-3" />
                          仅通知
                        </Chip>
                      )}
                    </div>

                    {/* 目标机房探针状态 */}
                    <div className="flex items-center gap-1.5 flex-wrap mt-2">
                      {targetDcs.slice(0, 6).map((dc) => {
                        const status = sub.lastStatus?.[dc.toLowerCase()];
                        const inStock = isDcInStock(status);
                        return (
                          <Chip
                            key={dc}
                            tone={inStock ? "success" : "default"}
                            className={cn(
                              "uppercase font-mono text-[10px] rounded-md",
                              inStock ? "font-bold" : "bg-background/60 text-muted-foreground/80"
                            )}
                            title={
                              status
                                ? `${dc.toUpperCase()}: ${status}`
                                : `${dc.toUpperCase()}: 持续探测中`
                            }
                          >
                            <StatusDot
                              tone={inStock ? "success" : "muted"}
                              size="xs"
                              pulse={inStock}
                            />
                            {dc}
                            {inStock && (
                              <span className="text-[9px] font-normal">({status})</span>
                            )}
                          </Chip>
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
                      <div className="flex items-center gap-1 text-primary font-semibold text-xs">
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
                        onClick={() => onOpenSubscribe(sub.planCode)}
                        className="h-7 text-[11px] px-2 rounded-md font-medium text-muted-foreground hover:text-foreground"
                      >
                        设置
                      </Button>
                      <Button
                        size="sm"
                        variant={hasStock ? "default" : "secondary"}
                        onClick={() => onOpenQuickOrder(sub.planCode)}
                        className="h-7 text-[11px] px-2.5 rounded-md font-medium gap-1"
                      >
                        {hasStock && <Zap className="w-3 h-3" />}
                        <span>{hasStock ? "立即秒抢" : "下单"}</span>
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
                <Sparkles className="w-3.5 h-3.5 text-amber-500" />
                <span>一键上架热门盯盘型号:</span>
              </div>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                {RECOMMENDED_PLANS.map((item) => (
                  <button
                    key={item.code}
                    type="button"
                    onClick={() => onOpenSubscribe(item.code)}
                    title={`${item.code} - ${item.name} (${item.desc})`}
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
  );
}
