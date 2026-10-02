import { Link } from "react-router-dom";
import { Zap } from "lucide-react";
import { useAppVersion } from "@/hooks/use-system-metrics";

/** Sidebar header: type-only wordmark + quick order. */
export function BrandBar({ onQuickOrder }: { onQuickOrder: () => void }) {
  const { data: version } = useAppVersion();
  const ver = version && version !== "dev" ? version : null;

  return (
    <div className="shrink-0 border-b border-sidebar-border bg-card/20 px-3.5 pb-3 pt-3 pr-12 lg:pr-3.5">
      <Link
        to="/"
        className="group flex items-center gap-2.5 rounded-xl p-1.5 -m-1 outline-none transition-all hover:bg-sidebar-accent/50 focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="w-8 h-8 rounded-lg bg-primary/10 border border-primary/25 text-primary flex items-center justify-center shrink-0 shadow-sm transition-transform group-hover:scale-105">
          <Zap className="w-4 h-4 fill-primary/20" strokeWidth={2.2} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5 leading-none">
            <span className="font-mono text-sm font-bold tracking-tight text-foreground">
              OVH
            </span>
            <span className="text-sm font-semibold text-primary">统御</span>
            {ver && (
              <span className="ml-auto shrink-0 rounded border border-border/60 bg-muted/50 px-1 py-0.5 font-mono text-[9px] tabular-nums text-muted-foreground leading-none">
                v{ver}
              </span>
            )}
          </div>
          <div className="mt-1 text-[10.5px] leading-none text-muted-foreground/75 font-medium">
            基础设施运维中枢
          </div>
        </div>
      </Link>

      <button
        type="button"
        onClick={onQuickOrder}
        className="mt-2.5 flex h-8 w-full items-center justify-center gap-1.5 rounded-lg border border-primary/25 bg-primary/10 text-xs font-semibold text-primary shadow-sm transition-all hover:bg-primary/15 hover:border-primary/40 active:scale-[0.98]"
      >
        <Zap className="h-3.5 w-3.5" strokeWidth={2} />
        快速下单
      </button>
    </div>
  );
}
