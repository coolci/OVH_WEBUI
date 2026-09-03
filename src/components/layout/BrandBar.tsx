import { Link } from "react-router-dom";
import { Zap } from "lucide-react";
import { useAppVersion } from "@/hooks/use-system-metrics";

/** Sidebar header: type-only wordmark + quick order. */
export function BrandBar({ onQuickOrder }: { onQuickOrder: () => void }) {
  const { data: version } = useAppVersion();
  const ver = version && version !== "dev" ? version : null;

  return (
    <div className="shrink-0 border-b border-sidebar-border bg-card/30 px-3 pb-3 pt-3.5 pr-12 lg:pr-3">
      <Link
        to="/"
        className="group block rounded-lg p-1 -m-1 outline-none transition-colors hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="flex items-baseline gap-2">
          <span className="font-mono text-[17px] font-semibold tracking-tight text-foreground">
            OVH
          </span>
          <span className="text-[15px] font-medium text-primary">统御</span>
          {ver && (
            <span className="ml-auto self-center shrink-0 rounded border border-border/70 bg-muted/40 px-1.5 py-[2px] font-mono text-[9px] tabular-nums text-muted-foreground">
              v{ver}
            </span>
          )}
        </div>
        <div className="mt-1 text-[11px] leading-none text-muted-foreground">独服 · VPS</div>
      </Link>

      <button
        type="button"
        onClick={onQuickOrder}
        className="mt-2.5 flex h-8 w-full items-center justify-center gap-1.5 rounded-lg border border-primary/20 bg-primary/10 text-[12px] font-medium text-primary transition-colors hover:bg-primary/15 active:scale-[0.98]"
      >
        <Zap className="h-3.5 w-3.5 text-primary" strokeWidth={1.75} />
        快速下单
      </button>
    </div>
  );
}
