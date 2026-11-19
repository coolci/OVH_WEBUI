import { Search, X } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

interface QueueFilterBarProps {
  search: string;
  onSearchChange: (value: string) => void;
  statusFilter: string;
  onStatusFilterChange: (value: string) => void;
  totalCount: number;
  runningCount: number;
  pausedCount: number;
  failedCount: number;
  completedCount: number;
}

export function QueueFilterBar({
  search,
  onSearchChange,
  statusFilter,
  onStatusFilterChange,
  totalCount,
  runningCount,
  pausedCount,
  failedCount,
  completedCount,
}: QueueFilterBarProps) {
  return (
    <Card className="surface-card rounded-xl border-border">
      <CardContent className="p-3 sm:p-3.5 flex flex-col lg:flex-row lg:items-center gap-2.5">
        <div className="relative flex-1 w-full min-w-0">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="搜索型号 / 机房 / 账户..."
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="pl-8 pr-8 rounded-lg h-8 text-xs bg-background/60 border-border/80 focus-visible:ring-1 focus-visible:ring-primary/40"
          />
          {search && (
            <button
              type="button"
              className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-md p-0.5 text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => onSearchChange("")}
              aria-label="清空搜索"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
        <div className="flex items-center gap-1 overflow-x-auto no-scrollbar w-full lg:w-auto shrink-0 p-1 bg-muted/50 rounded-lg border border-border/60">
          {[
            { id: "all", label: "全部", count: totalCount },
            { id: "running", label: "运行中", count: runningCount },
            { id: "paused", label: "已暂停", count: pausedCount },
            { id: "failed", label: "失败", count: failedCount },
            { id: "completed", label: "已下单", count: completedCount },
          ].map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => onStatusFilterChange(tab.id)}
              className={cn(
                "px-2.5 py-1 text-xs font-medium rounded-md transition-colors shrink-0 whitespace-nowrap flex items-center gap-1.5",
                statusFilter === tab.id
                  ? "bg-background text-foreground font-semibold shadow-sm border border-border/60"
                  : "text-muted-foreground hover:bg-secondary/50 hover:text-foreground"
              )}
            >
              <span>{tab.label}</span>
              <span
                className={cn(
                  "text-[10px] font-mono tabular-nums px-1.5 py-0.5 leading-none rounded-full",
                  statusFilter === tab.id
                    ? "bg-primary/10 text-primary"
                    : "bg-muted text-muted-foreground"
                )}
              >
                {tab.count}
              </span>
            </button>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
