import { Bell, BellOff } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/common/StatusDot";
import { cn } from "@/lib/utils";

interface VPSMonitorHeaderCardProps {
  running: boolean;
  subscriptionsCount: number;
  checkInterval: number;
  onToggle: (running: boolean) => void;
  isToggling: boolean;
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div>
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="text-lg font-semibold">{value}</div>
    </div>
  );
}

export function VPSMonitorHeaderCard({
  running,
  subscriptionsCount,
  checkInterval,
  onToggle,
  isToggling,
}: VPSMonitorHeaderCardProps) {
  return (
    <Card className="surface-card rounded-xl border-border">
      <CardContent className="p-4 sm:p-5 space-y-3.5 sm:space-y-0 sm:flex sm:items-center sm:justify-between sm:gap-4">
        <div className="flex items-center justify-between sm:justify-start gap-3">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-lg bg-secondary border border-border/60 flex items-center justify-center text-foreground flex-shrink-0">
              {running ? (
                <Bell className="w-4 h-4 text-emerald-400" />
              ) : (
                <BellOff className="w-4 h-4 text-muted-foreground" />
              )}
            </div>
            <div>
              <div className="text-sm font-semibold text-foreground">VPS 监控状态</div>
              <div className="text-xs text-muted-foreground inline-flex items-center gap-1.5 mt-0.5">
                <StatusDot tone={running ? "success" : "muted"} size="xs" pulse={running} />
                <span>{running ? "轮询监控中" : "已停止"}</span>
              </div>
            </div>
          </div>

          {/* 移动端: 启停按钮置于卡片顶部右侧 */}
          <Button
            variant="outline"
            size="sm"
            className={cn(
              "sm:hidden h-8 text-xs gap-1.5 px-3 rounded-lg font-medium transition-all flex-shrink-0",
              running
                ? "border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20"
                : "border-border/80 hover:bg-secondary text-foreground"
            )}
            onClick={() => onToggle(running)}
            disabled={isToggling}
          >
            {running ? <BellOff className="w-3.5 h-3.5" /> : <Bell className="w-3.5 h-3.5" />}
            <span>{running ? "停止监控" : "启动监控"}</span>
          </Button>
        </div>

        {/* 指标与桌面端按钮 */}
        <div className="flex items-center justify-between sm:justify-end gap-6 text-sm border-t border-border/40 sm:border-t-0 pt-2.5 sm:pt-0">
          <div className="flex gap-6 text-sm">
            <Stat label="订阅数" value={subscriptionsCount} />
            <Stat label="检查间隔" value={`${checkInterval}s`} />
          </div>

          {/* 桌面端: 启停按钮置于卡片右侧 */}
          <Button
            variant="outline"
            size="sm"
            className={cn(
              "hidden sm:inline-flex h-8 text-xs gap-1.5 px-3 rounded-lg font-medium transition-all flex-shrink-0",
              running
                ? "border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20"
                : "border-border/80 hover:bg-secondary text-foreground"
            )}
            onClick={() => onToggle(running)}
            disabled={isToggling}
          >
            {running ? <BellOff className="w-3.5 h-3.5" /> : <Bell className="w-3.5 h-3.5" />}
            <span>{running ? "停止监控" : "启动监控"}</span>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
