import { Link } from "react-router-dom";
import { Zap } from "lucide-react";
import { useAppVersion } from "@/hooks/use-system-metrics";

/** Sidebar header: type-only wordmark + quick order, one block. */
export function BrandBar({ onQuickOrder }: { onQuickOrder: () => void }) {
  const { data: version } = useAppVersion();
  const ver = version && version !== "dev" ? version : null;

  return (
    <div className="shrink-0 border-b border-sidebar-border/80 px-3 pb-3 pt-3 pr-12 lg:pr-3">
      <Link
        to="/"
        className="flex items-start justify-between gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <div className="min-w-0">
          <div className="truncate text-[13px] font-semibold leading-none tracking-tight">
            <span className="text-muted-foreground">OVH</span>
            <span className="mx-[5px] font-normal text-border">/</span>
            <span className="text-foreground">WebUI</span>
          </div>
          <div className="mt-1.5 truncate text-[11px] leading-none text-muted-foreground">
            独服控制台
          </div>
        </div>
        {ver && (
          <span className="shrink-0 rounded border border-border/70 px-1.5 py-[3px] font-mono text-[10px] tabular-nums leading-none text-muted-foreground">
            {ver}
          </span>
        )}
      </Link>

      <button
        type="button"
        onClick={onQuickOrder}
        className="mt-3 flex h-8 w-full items-center justify-center gap-1.5 rounded-md border border-border/80 bg-transparent text-[12px] font-medium text-foreground/85 transition-colors hover:border-primary/45 hover:bg-primary/10 hover:text-primary"
      >
        <Zap className="h-3.5 w-3.5" strokeWidth={1.75} />
        快速下单
      </button>
    </div>
  );
}
