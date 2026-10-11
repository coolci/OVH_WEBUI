import { CheckCheck, Link2, Bot, Bell, Info, Sparkles } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { StatusDot } from "@/components/common/StatusDot";
import type { StatsResponse } from "@/hooks/use-stats";
import type { UpdateCheckResult } from "@/hooks/use-system-metrics";

interface SystemRowProps {
  icon: React.ReactNode;
  label: string;
  ok: boolean;
  onText: string;
  offText: string;
  neutralOff?: boolean;
  warnOff?: boolean;
}

export function SystemRow({
  icon,
  label,
  ok,
  onText,
  offText,
  neutralOff,
  warnOff,
}: SystemRowProps) {
  const dotTone = ok ? "success" : warnOff ? "warning" : neutralOff ? "muted" : "danger";
  return (
    <div className="flex justify-between items-center px-3 py-2 rounded-xl hover:bg-muted/50 dark:hover:bg-white/[0.04] transition-colors">
      <div className="inline-flex items-center gap-2 text-xs sm:text-[13px] font-medium text-foreground/90">
        <span className="text-muted-foreground/80">{icon}</span>
        <span>{label}</span>
      </div>
      <div className="inline-flex items-center gap-1.5">
        <StatusDot tone={dotTone} pulse={ok} size="xs" />
        <span className={`text-xs ${ok ? "font-semibold text-foreground/90" : "text-muted-foreground/70"}`}>
          {ok ? onText : offText}
        </span>
      </div>
    </div>
  );
}

interface CoreServiceCardProps {
  stats?: StatsResponse;
  version?: string;
  update?: UpdateCheckResult;
}

export function CoreServiceCard({ stats, version, update }: CoreServiceCardProps) {
  return (
    <Card className="surface-card rounded-2xl overflow-hidden border-border/80 shadow-sm">
      <CardContent className="p-3.5 sm:p-5">
        <div className="flex items-center gap-2 mb-3.5">
          <CheckCheck className="w-4 h-4 text-primary" />
          <h2 className="text-[14px] sm:text-[15px] font-semibold text-foreground">核心服务感知</h2>
        </div>
        <div className="space-y-1">
          <SystemRow
            icon={<Link2 className="w-3.5 h-3.5" />}
            label="OVH API 连通"
            ok={!!stats}
            onText="正常通信"
            offText="未连接"
          />
          <SystemRow
            icon={<Bot className="w-3.5 h-3.5" />}
            label="抢购调度引擎"
            ok={(stats?.activeQueues || 0) > 0}
            onText="活跃轮询中"
            offText="就绪待命"
            neutralOff
          />
          <SystemRow
            icon={<Bell className="w-3.5 h-3.5" />}
            label="库存监测引擎"
            ok={!!stats?.monitorRunning}
            onText="实时运行中"
            offText="待启用"
            warnOff
          />
          <div className="flex justify-between items-center px-3 py-2 mt-1.5 border-t border-border/70 pt-2.5">
            <div className="inline-flex items-center gap-2 text-xs text-muted-foreground">
              <Info className="w-3.5 h-3.5" />
              系统版本
            </div>
            <div className="inline-flex items-center gap-2">
              <span className="text-xs font-mono font-semibold">v{version || "—"}</span>
              {update?.hasUpdate ? (
                <a
                  href={update.url}
                  target="_blank"
                  rel="noreferrer noopener"
                  className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-primary/15 text-primary hover:bg-primary/25 transition-colors"
                  title={`有新版本 v${update.latest} 可用,点击查看`}
                >
                  <Sparkles className="w-3 h-3" />
                  v{update.latest}
                </a>
              ) : null}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
