import { useState, useMemo } from "react";
import { Wifi, ArrowDown, ArrowUp, RefreshCw } from "lucide-react";
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  CartesianGrid,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
} from "recharts";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { useMrtgTraffic, type MrtgPeriod, type MrtgInterface } from "@/hooks/use-mrtg";
import { cn } from "@/lib/utils";

const PERIOD_LABEL: Record<MrtgPeriod, string> = {
  hourly: "过去 1 小时",
  daily: "过去 24 小时",
  weekly: "过去 7 天",
  monthly: "过去 30 天",
  yearly: "过去 1 年",
};

/** bps → 友好显示 */
function formatBandwidth(bps: number): string {
  if (bps >= 1_000_000_000) return `${(bps / 1_000_000_000).toFixed(2)} Gbps`;
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(2)} Mbps`;
  if (bps >= 1_000) return `${(bps / 1_000).toFixed(2)} Kbps`;
  return `${bps.toFixed(0)} bps`;
}

/**
 * MRTG 流量监控 — 主性能数据源
 */
export function MrtgTrafficChart({ serviceName }: { serviceName: string }) {
  const [period, setPeriod] = useState<MrtgPeriod>("daily");
  const { download, upload, isPending, isFetching, isError, refetch } = useMrtgTraffic(
    serviceName,
    period
  );

  const merged = useMemo(() => {
    if (!download?.interfaces || !upload?.interfaces) return [];
    return download.interfaces
      .map((d: MrtgInterface) => {
        const u = upload.interfaces.find((x) => x.mac === d.mac);
        if (!d.data?.length || !u?.data?.length) return null;
        return { mac: d.mac, download: d, upload: u };
      })
      .filter((x): x is { mac: string; download: MrtgInterface; upload: MrtgInterface } => x !== null);
  }, [download, upload]);

  // 全局汇总 KPI（所有网卡）
  const globalKpi = useMemo(() => {
    if (!merged.length) return null;
    let dlSum = 0;
    let ulSum = 0;
    let dlMax = 0;
    let ulMax = 0;
    let n = 0;
    for (const { download: d, upload: u } of merged) {
      for (let i = 0; i < d.data.length; i++) {
        const dv = d.data[i]?.value?.value || 0;
        const uv = u.data[i]?.value?.value || 0;
        dlSum += dv;
        ulSum += uv;
        dlMax = Math.max(dlMax, dv);
        ulMax = Math.max(ulMax, uv);
        n++;
      }
    }
    if (!n) return null;
    return {
      dlAvg: dlSum / n,
      ulAvg: ulSum / n,
      dlMax,
      ulMax,
      nics: merged.length,
    };
  }, [merged]);

  return (
    <div className="terminal-card overflow-hidden">
      <div className="flex flex-col gap-3 border-b border-border/70 bg-gradient-to-r from-primary/[0.07] via-transparent to-accent/[0.04] px-4 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:px-5">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="icon-well h-9 w-9">
            <Wifi className="h-4 w-4 text-primary" strokeWidth={1.75} />
          </div>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold tracking-tight">带宽时序</h3>
            <p className="truncate font-mono text-[11px] text-muted-foreground">{serviceName}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Select value={period} onValueChange={(v) => setPeriod(v as MrtgPeriod)}>
            <SelectTrigger className="h-9 w-[9.5rem] rounded-full border-border/80 bg-background/50 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(PERIOD_LABEL) as MrtgPeriod[]).map((p) => (
                <SelectItem key={p} value={p}>
                  {PERIOD_LABEL[p]}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant="outline"
            size="sm"
            className="h-9 rounded-full"
            onClick={() => refetch()}
            disabled={isFetching}
          >
            <RefreshCw className={cn("h-3.5 w-3.5", isFetching && "animate-spin")} />
            <span className="hidden sm:inline">刷新</span>
          </Button>
        </div>
      </div>

      <div className="space-y-5 p-4 sm:p-5">
        {globalKpi && (
          <div className="grid grid-cols-2 gap-2.5 sm:grid-cols-4">
            <KpiTile
              label="↓ 平均下载"
              value={formatBandwidth(globalKpi.dlAvg)}
              tone="success"
            />
            <KpiTile
              label="↑ 平均上传"
              value={formatBandwidth(globalKpi.ulAvg)}
              tone="warning"
            />
            <KpiTile
              label="↓ 峰值下载"
              value={formatBandwidth(globalKpi.dlMax)}
              tone="success"
            />
            <KpiTile
              label="网卡数"
              value={String(globalKpi.nics)}
              tone="accent"
            />
          </div>
        )}

        {isPending ? (
          <Skeleton className="h-[380px] rounded-2xl" />
        ) : isError ? (
          <EmptyState
            icon={Wifi}
            title="MRTG 数据加载失败"
            description="请检查账户权限或稍后重试"
            action={
              <Button variant="outline" size="sm" onClick={() => refetch()}>
                重试
              </Button>
            }
          />
        ) : merged.length === 0 ? (
          <EmptyState
            icon={Wifi}
            title="暂无流量数据"
            description="该服务器尚未上报 MRTG 数据，或所选周期内无采样点。"
          />
        ) : (
          <div className="space-y-5">
            {merged.map(({ mac, download: d, upload: u }) => (
              <InterfaceChart key={mac} mac={mac} download={d} upload={u} period={period} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function KpiTile({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone: "success" | "warning" | "accent";
}) {
  const ring =
    tone === "success"
      ? "from-success/15 to-transparent"
      : tone === "warning"
        ? "from-warning/15 to-transparent"
        : "from-accent/15 to-transparent";
  const valueCls =
    tone === "success"
      ? "text-success"
      : tone === "warning"
        ? "text-warning"
        : "text-accent";
  return (
    <div
      className={cn(
        "rounded-xl border border-border/80 bg-gradient-to-br p-3 sm:p-3.5",
        ring
      )}
    >
      <div className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className={cn("mt-1 font-mono text-sm font-semibold tabular-nums sm:text-[15px]", valueCls)}>
        {value}
      </div>
    </div>
  );
}

function InterfaceChart({
  mac,
  download,
  upload,
  period,
}: {
  mac: string;
  download: MrtgInterface;
  upload: MrtgInterface;
  period: MrtgPeriod;
}) {
  const chartData = useMemo(
    () =>
      download.data.map((dp, i) => {
        const up = upload.data[i];
        return {
          time: new Date(dp.timestamp * 1000).toLocaleString("zh-CN", {
            month: "2-digit",
            day: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
          }),
          download: dp.value?.value || 0,
          upload: up?.value?.value || 0,
        };
      }),
    [download, upload]
  );

  const stats = useMemo(() => {
    const dl = chartData.map((d) => d.download);
    const ul = chartData.map((d) => d.upload);
    const tot = chartData.map((d) => d.download + d.upload);
    const avg = (arr: number[]) => (arr.length ? arr.reduce((a, b) => a + b, 0) / arr.length : 0);
    return {
      dlCur: dl[dl.length - 1] || 0,
      dlAvg: avg(dl),
      dlMax: Math.max(0, ...dl),
      ulCur: ul[ul.length - 1] || 0,
      ulAvg: avg(ul),
      ulMax: Math.max(0, ...ul),
      totMax: Math.max(0, ...tot),
      points: chartData.length,
    };
  }, [chartData]);

  return (
    <div className="rounded-2xl border border-border/80 bg-background/30 p-3.5 sm:p-4">
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <span className="text-[12px] font-semibold">网卡</span>
        <code className="rounded-md border border-border/70 bg-muted/50 px-2 py-0.5 font-mono text-[11px]">
          {mac}
        </code>
        <span className="text-[11px] text-muted-foreground">
          {PERIOD_LABEL[period]} · {stats.points} 点
        </span>
      </div>

      <div className="mb-4 grid grid-cols-1 gap-2.5 sm:grid-cols-2">
        <StatBlock
          label="下载"
          icon={<ArrowDown className="h-3.5 w-3.5" />}
          tone="success"
          cur={stats.dlCur}
          avg={stats.dlAvg}
          max={stats.dlMax}
        />
        <StatBlock
          label="上传"
          icon={<ArrowUp className="h-3.5 w-3.5" />}
          tone="warning"
          cur={stats.ulCur}
          avg={stats.ulAvg}
          max={stats.ulMax}
        />
      </div>

      <div className="h-[260px] w-full sm:h-[300px]">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 8, right: 8, left: -8, bottom: 0 }}>
            <defs>
              <linearGradient id="dlFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="hsl(var(--success))" stopOpacity={0.35} />
                <stop offset="100%" stopColor="hsl(var(--success))" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="ulFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="hsl(var(--warning))" stopOpacity={0.28} />
                <stop offset="100%" stopColor="hsl(var(--warning))" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 6" stroke="hsl(var(--border))" vertical={false} />
            <XAxis
              dataKey="time"
              stroke="hsl(var(--muted-foreground))"
              tickLine={false}
              axisLine={false}
              style={{ fontSize: 10 }}
              minTickGap={28}
            />
            <YAxis
              stroke="hsl(var(--muted-foreground))"
              tickLine={false}
              axisLine={false}
              style={{ fontSize: 10 }}
              width={48}
              tickFormatter={(v) => formatBandwidth(v).replace(/\s.*/, "")}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: "hsl(var(--popover) / 0.96)",
                border: "1px solid hsl(var(--border))",
                borderRadius: 12,
                fontSize: 12,
                boxShadow: "0 12px 40px -12px hsl(0 0% 0% / 0.5)",
              }}
              formatter={(value: number, name: string) => [
                formatBandwidth(Number(value)),
                name === "download" ? "↓ 下载" : "↑ 上传",
              ]}
            />
            <Legend
              wrapperStyle={{ paddingTop: 4, fontSize: 11 }}
              formatter={(value) => (value === "download" ? "↓ 下载" : "↑ 上传")}
            />
            <Area
              type="monotone"
              dataKey="download"
              name="download"
              stroke="hsl(var(--success))"
              strokeWidth={2}
              fill="url(#dlFill)"
              dot={false}
              animationDuration={700}
            />
            <Area
              type="monotone"
              dataKey="upload"
              name="upload"
              stroke="hsl(var(--warning))"
              strokeWidth={2}
              fill="url(#ulFill)"
              dot={false}
              animationDuration={700}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function StatBlock({
  label,
  tone,
  icon,
  cur,
  avg,
  max,
}: {
  label: string;
  tone: "success" | "warning";
  icon: React.ReactNode;
  cur: number;
  avg: number;
  max: number;
}) {
  const toneText = tone === "success" ? "text-success" : "text-warning";
  return (
    <div className="rounded-xl border border-border/70 bg-muted/20 p-3">
      <div className={cn("mb-2 flex items-center gap-1.5 text-[12px] font-semibold", toneText)}>
        {icon}
        {label}带宽
      </div>
      <div className="grid grid-cols-3 gap-1.5 text-[11px]">
        <Slot label="当前" value={formatBandwidth(cur)} />
        <Slot label="平均" value={formatBandwidth(avg)} bold />
        <Slot label="峰值" value={formatBandwidth(max)} />
      </div>
    </div>
  );
}

function Slot({ label, value, bold }: { label: string; value: string; bold?: boolean }) {
  return (
    <div>
      <div className="mb-0.5 text-muted-foreground">{label}</div>
      <div className={cn("font-mono tabular-nums", bold ? "font-bold" : "font-semibold")}>
        {value}
      </div>
    </div>
  );
}
