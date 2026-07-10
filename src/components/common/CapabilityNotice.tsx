import type { ComponentType } from "react";
import { Info, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";

interface CapabilityNoticeProps {
  icon?: ComponentType<{ className?: string; strokeWidth?: number | string }>;
  title: string;
  description?: string;
  /** 建议的替代能力，如「使用下方 MRTG 流量」 */
  alternative?: string;
  tone?: "muted" | "info" | "warning";
  className?: string;
  compact?: boolean;
}

/**
 * 能力不可用态：机型未开通 / OVH 无该 API 时展示，避免把 404 当成系统故障。
 * 行业实践：soft capability + 清晰替代路径，而非红色错误。
 */
export function CapabilityNotice({
  icon: Icon = Info,
  title,
  description,
  alternative,
  tone = "info",
  className,
  compact,
}: CapabilityNoticeProps) {
  const toneCls =
    tone === "warning"
      ? "border-warning/25 bg-warning/[0.06]"
      : tone === "muted"
        ? "border-border/80 bg-muted/30"
        : "border-accent/20 bg-accent/[0.05]";

  const iconCls =
    tone === "warning"
      ? "text-warning"
      : tone === "muted"
        ? "text-muted-foreground"
        : "text-accent";

  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-2xl border",
        toneCls,
        compact ? "p-4" : "p-5 sm:p-6",
        className
      )}
    >
      <div
        className="pointer-events-none absolute -right-8 -top-8 h-28 w-28 rounded-full opacity-40 blur-2xl"
        style={{
          background:
            tone === "warning"
              ? "hsl(var(--warning) / 0.25)"
              : "hsl(var(--accent) / 0.2)",
        }}
        aria-hidden
      />
      <div className={cn("relative flex gap-3", compact ? "items-start" : "items-start sm:items-center")}>
        <div
          className={cn(
            "flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl border border-border/60 bg-background/40",
            iconCls
          )}
        >
          <Icon className="h-[18px] w-[18px]" strokeWidth={1.75} />
        </div>
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-[13px] font-semibold tracking-tight text-foreground">{title}</p>
          {description && (
            <p className="text-[12px] leading-relaxed text-muted-foreground">{description}</p>
          )}
          {alternative && (
            <p className="mt-2 inline-flex items-center gap-1.5 text-[11px] font-medium text-primary">
              <Sparkles className="h-3 w-3" />
              {alternative}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
