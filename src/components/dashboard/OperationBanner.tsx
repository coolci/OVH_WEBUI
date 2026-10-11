import { StatusDot } from "@/components/common/StatusDot";
import { ShieldCheck } from "lucide-react";
import type { StatsResponse } from "@/hooks/use-stats";
import type { TelegramPollerStatus } from "@/hooks/use-settings";
import type { OVHAccount } from "@/hooks/use-accounts";

interface OperationBannerProps {
  stats?: StatsResponse;
  poller?: TelegramPollerStatus;
  defaultAccount?: OVHAccount;
  monitorCount: number;
}

export function OperationBanner({
  stats,
  poller,
  defaultAccount,
  monitorCount,
}: OperationBannerProps) {
  return (
    <div className="surface-card rounded-2xl p-3 sm:p-4 border border-border/80 bg-card/75 backdrop-blur-md shadow-sm">
      {/* 桌面端流线布局 (md 及以上) */}
      <div className="hidden md:flex items-center justify-between gap-4">
        <div className="flex items-center gap-4 flex-1 min-w-0">
          {/* 系统在线状态徽章 */}
          <div className="flex items-center gap-2 pr-4 border-r border-border/70 flex-shrink-0">
            <span className="relative flex h-2.5 w-2.5">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500" />
            </span>
            <span className="text-xs font-semibold tracking-tight text-foreground whitespace-nowrap">
              系统全勤在线
            </span>
          </div>

          {/* 4 大核心调度服务状态 */}
          <div className="flex items-center gap-4 lg:gap-6 text-xs text-muted-foreground min-w-0 flex-1">
            <div className="inline-flex items-center gap-1.5 truncate" title="抢购队列处理器状态">
              <StatusDot tone={stats?.queueProcessorRunning ? "success" : "warning"} size="xs" />
              <span className="whitespace-nowrap">队列调度:</span>
              <strong className="font-mono text-foreground font-medium">
                {stats?.queueProcessorRunning ? "运行中" : "就绪"}
              </strong>
            </div>

            <div className="inline-flex items-center gap-1.5 truncate" title="库存监控引擎状态">
              <StatusDot tone={stats?.monitorRunning ? "success" : "muted"} size="xs" />
              <span className="whitespace-nowrap">盯盘引擎:</span>
              <strong className="font-mono text-foreground font-medium">
                {stats?.monitorRunning ? "监测中" : "待启动"}
              </strong>
            </div>

            <div className="inline-flex items-center gap-1.5 truncate" title="Telegram Bot 轮询状态">
              <StatusDot tone={poller?.running ? "success" : "warning"} size="xs" />
              <span className="whitespace-nowrap">Telegram:</span>
              <strong className="font-mono text-foreground font-medium truncate">
                {poller?.running ? `@${poller.botUsername || "在线"}` : "未连接"}
              </strong>
            </div>

            {defaultAccount && (
              <div className="inline-flex items-center gap-1.5 truncate" title="当前默认下单扣费账户">
                <ShieldCheck className="w-3.5 h-3.5 text-primary flex-shrink-0" />
                <span className="whitespace-nowrap">默认账户:</span>
                <strong className="font-medium text-foreground truncate">{defaultAccount.name}</strong>
              </div>
            )}
          </div>
        </div>

        {/* 右侧数量统计 */}
        <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono flex-shrink-0 pl-3 border-l border-border/70">
          <span>盯盘: <strong className="text-primary font-bold">{monitorCount}</strong></span>
          <span className="opacity-40">/</span>
          <span>现货: <strong className="text-foreground font-medium">{stats?.totalServers ?? 0}</strong></span>
        </div>
      </div>

      {/* 移动端对称 2x2 网格布局 (< md) */}
      <div className="md:hidden space-y-2.5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
              <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
            </span>
            <span className="text-xs font-semibold tracking-tight text-foreground">
              系统全勤在线
            </span>
          </div>
          <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground font-mono">
            <span>盯盘: <strong className="text-primary">{monitorCount}</strong></span>
            <span className="opacity-40">·</span>
            <span>现货: <strong className="text-foreground">{stats?.totalServers ?? 0}</strong></span>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-2 pt-2 border-t border-border/50 text-xs">
          <div className="flex items-center gap-1.5 p-1.5 rounded-lg bg-secondary/30 border border-border/60 min-w-0">
            <StatusDot tone={stats?.queueProcessorRunning ? "success" : "warning"} size="xs" />
            <span className="text-[11px] text-muted-foreground whitespace-nowrap">队列:</span>
            <span className="font-mono text-[11px] font-medium text-foreground truncate">
              {stats?.queueProcessorRunning ? "运行中" : "就绪"}
            </span>
          </div>

          <div className="flex items-center gap-1.5 p-1.5 rounded-lg bg-secondary/30 border border-border/60 min-w-0">
            <StatusDot tone={stats?.monitorRunning ? "success" : "muted"} size="xs" />
            <span className="text-[11px] text-muted-foreground whitespace-nowrap">盯盘:</span>
            <span className="font-mono text-[11px] font-medium text-foreground truncate">
              {stats?.monitorRunning ? "监测中" : "待启动"}
            </span>
          </div>

          <div className="flex items-center gap-1.5 p-1.5 rounded-lg bg-secondary/30 border border-border/60 min-w-0">
            <StatusDot tone={poller?.running ? "success" : "warning"} size="xs" />
            <span className="text-[11px] text-muted-foreground whitespace-nowrap">TG:</span>
            <span className="font-mono text-[11px] font-medium text-foreground truncate">
              {poller?.running ? `@${poller.botUsername || "在线"}` : "未连接"}
            </span>
          </div>

          <div className="flex items-center gap-1.5 p-1.5 rounded-lg bg-secondary/30 border border-border/60 min-w-0">
            <ShieldCheck className="w-3.5 h-3.5 text-primary flex-shrink-0" />
            <span className="text-[11px] text-muted-foreground whitespace-nowrap">账户:</span>
            <span className="text-[11px] font-medium text-foreground truncate">
              {defaultAccount?.name || "未配置"}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
