import { Cpu } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { MetricRing } from "@/components/common/MetricRing";
import type { SystemMetrics } from "@/hooks/use-system-metrics";

/** 字节短格式: 1.5 GB / 11.4 GB */
export function formatBytesShort(n: number): string {
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

interface TelemetryCardProps {
  metrics?: SystemMetrics;
}

export function TelemetryCard({ metrics }: TelemetryCardProps) {
  return (
    <Card className="surface-card rounded-2xl overflow-hidden border-border/80 shadow-sm">
      <CardHeader className="p-3.5 sm:p-4 pb-2 border-b border-border/40 flex flex-row items-center justify-between">
        <div className="flex items-center gap-2">
          <Cpu className="w-4 h-4 text-primary" />
          <CardTitle className="text-xs sm:text-sm font-semibold">系统资源与硬件遥测</CardTitle>
        </div>
        <span className="text-[11px] text-muted-foreground font-mono">
          {metrics
            ? `${metrics.cpu.cores} 核心 · ${formatBytesShort(metrics.memory.totalBytes)} 内存`
            : "遥测就绪"}
        </span>
      </CardHeader>
      <CardContent className="p-0">
        <div className="grid grid-cols-1 sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-border/60">
          <div className="p-1 sm:p-2">
            <MetricRing
              label="CPU 占用"
              subLabel={metrics ? `${metrics.cpu.cores} 核心活跃` : "—"}
              percent={metrics?.cpu.percent ?? 0}
              size={84}
            />
          </div>
          <div className="p-1 sm:p-2">
            <MetricRing
              label="物理内存"
              subLabel={
                metrics
                  ? `${formatBytesShort(metrics.memory.usedBytes)} / ${formatBytesShort(
                      metrics.memory.totalBytes
                    )}`
                  : "—"
              }
              percent={metrics?.memory.percent ?? 0}
              size={84}
            />
          </div>
          <div className="p-1 sm:p-2">
            <MetricRing
              label={metrics?.disk.path ? `磁盘 (${metrics.disk.path})` : "磁盘使用"}
              subLabel={
                metrics
                  ? `${formatBytesShort(metrics.disk.usedBytes)} / ${formatBytesShort(
                      metrics.disk.totalBytes
                    )}`
                  : "—"
              }
              percent={metrics?.disk.percent ?? 0}
              size={84}
            />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
