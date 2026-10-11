import { useMemo } from "react";
import { Bell, ShoppingCart, Cpu, MemoryStick, HardDrive, Wifi } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/common/StatusDot";
import { type ServerPlan } from "@/hooks/use-servers";
import { type PriceInfo, formatPrice } from "@/hooks/use-availability";
import { OVH_DATACENTERS, isDcInStock, lookupDcStatus, mergeDcAvailability } from "@/lib/datacenters";
import { cn } from "@/lib/utils";

interface ServerCardProps {
  server: ServerPlan;
  realtimeDcMap?: Record<string, string>;
  price?: PriceInfo;
  isMonitored?: boolean;
  onView: () => void;
  onMonitor: () => void;
}

export function ServerCard({
  server,
  realtimeDcMap,
  price,
  isMonitored,
  onView,
  onMonitor,
}: ServerCardProps) {
  const dcMap = useMemo(
    () => mergeDcAvailability(server.datacenters, realtimeDcMap),
    [server.datacenters, realtimeDcMap]
  );

  const dcStatuses = OVH_DATACENTERS.map((dc) => ({
    dc,
    isOk: isDcInStock(lookupDcStatus(dcMap, dc)),
  }));
  const total = dcStatuses.length;
  const okCount = dcStatuses.filter((s) => s.isOk).length;

  return (
    <Card className="surface-card rounded-xl border-border hover:border-border/80 transition-all duration-200 shadow-sm overflow-hidden flex flex-col justify-between">
      <CardContent className="p-4 sm:p-5 flex flex-col gap-4 h-full">
        {/* 头部：型号 + 当前可用标识 */}
        <div className="flex items-center justify-between gap-2.5">
          <h3 className="font-mono text-[15px] font-bold tracking-tight text-foreground truncate">
            {server.planCode}
          </h3>
          {okCount > 0 ? (
            <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium border border-emerald-500/25 bg-emerald-500/10 text-emerald-500 dark:text-emerald-400 flex-shrink-0">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
              <span>{okCount} 机房有货</span>
            </div>
          ) : (
            <div className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium border border-border/80 bg-secondary/60 text-muted-foreground flex-shrink-0">
              <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/50" />
              <span>暂时缺货</span>
            </div>
          )}
        </div>

        {/* 次行：名称 + 价格 */}
        <div className="flex items-baseline justify-between gap-2 -mt-1.5 text-xs">
          <p className="text-muted-foreground truncate flex-1 min-w-0" title={server.name}>
            {server.name}
          </p>
          <div className="text-sm font-bold tabular-nums text-foreground flex-shrink-0">
            {price ? formatPrice(price) : <span className="text-muted-foreground font-normal text-xs">—</span>}
          </div>
        </div>

        {/* 规格 2x2 */}
        <div className="grid grid-cols-2 gap-2 text-[12px] bg-secondary/30 p-2.5 rounded-lg border border-border/50">
          <SpecRow icon={<Cpu className="w-3.5 h-3.5" />} text={server.cpu} />
          <SpecRow icon={<MemoryStick className="w-3.5 h-3.5" />} text={server.memory} />
          <SpecRow icon={<HardDrive className="w-3.5 h-3.5" />} text={server.storage} />
          <SpecRow icon={<Wifi className="w-3.5 h-3.5" />} text={server.bandwidth} />
        </div>

        {/* DC 点阵：12 个标准 OVH DC */}
        <div className="flex flex-wrap items-center gap-1.5 py-1">
          {dcStatuses.map(({ dc, isOk }) => (
            <span
              key={dc.code}
              title={`${dc.name} · ${dc.region}`}
              className={cn(
                "inline-flex items-center gap-1 px-1.5 h-5 rounded-full border text-[10px] font-mono transition-colors",
                isOk
                  ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-300"
                  : "border-border/60 bg-muted/30 text-muted-foreground/60"
              )}
            >
              <StatusDot tone={isOk ? "success" : "danger"} size="xs" pulse={isOk} />
              {dc.code.toUpperCase()}
            </span>
          ))}
        </div>

        {/* 操作按钮 */}
        <div className="flex items-center gap-2.5 pt-2.5 mt-auto border-t border-border/40">
          <Button
            variant="outline"
            size="sm"
            className="flex-1 rounded-lg h-8 text-xs gap-1.5 border-border/80 hover:bg-secondary font-medium"
            onClick={onMonitor}
          >
            <Bell className="w-3.5 h-3.5 text-muted-foreground" />
            <span>{isMonitored ? "监控设置" : "监控"}</span>
          </Button>
          <Button
            size="sm"
            className="flex-1 rounded-lg h-8 text-xs gap-1.5 font-medium shadow-sm"
            onClick={onView}
          >
            <ShoppingCart className="w-3.5 h-3.5" />
            <span>抢购</span>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function SpecRow({ icon, text }: { icon: React.ReactNode; text: string }) {
  return (
    <div className="flex items-center gap-2 min-w-0">
      <span className="text-primary/80 flex-shrink-0">{icon}</span>
      <span className="truncate text-xs font-medium text-foreground/90" title={text}>{text}</span>
    </div>
  );
}
