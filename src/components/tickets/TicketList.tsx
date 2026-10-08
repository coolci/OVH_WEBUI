import { useState } from "react";
import { type SupportTicket } from "@/hooks/ovh/use-tickets";
import { formatDateTime } from "@/lib/format-os";
import { getStateMeta, getProductLabel } from "./labels";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import {
  Search,
  X,
  RefreshCw,
  AlertTriangle,
  Ticket,
  ChevronRight,
  Server,
  Archive,
  Layers,
  Inbox,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface TicketListProps {
  tickets: SupportTicket[];
  selectedTicketId?: number | null;
  onSelectTicket: (ticket: SupportTicket) => void;
  statusFilter: string;
  onStatusFilterChange: (status: string) => void;
  archivedFilter: boolean;
  onArchivedFilterChange: (archived: boolean) => void;
  searchQuery: string;
  onSearchQueryChange: (q: string) => void;
  isLoading: boolean;
  isFetching: boolean;
  onRefresh: () => void;
  warning?: string;
  incomplete?: boolean;
}

export function TicketList({
  tickets,
  selectedTicketId,
  onSelectTicket,
  statusFilter,
  onStatusFilterChange,
  archivedFilter,
  onArchivedFilterChange,
  searchQuery,
  onSearchQueryChange,
  isLoading,
  isFetching,
  onRefresh,
  warning,
  incomplete,
}: TicketListProps) {
  const [localSearch, setLocalSearch] = useState(searchQuery);

  // 统计各类数量
  const counts = {
    all: tickets.length,
    open: tickets.filter((t) => t.state === "open").length,
    closed: tickets.filter((t) => t.state === "closed").length,
  };

  const statusTabs = [
    { value: "all", label: "全部", count: counts.all },
    { value: "open", label: "处理中", count: counts.open },
    { value: "closed", label: "已关闭", count: counts.closed },
  ];

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSearchQueryChange(localSearch);
  };

  const handleClearSearch = () => {
    setLocalSearch("");
    onSearchQueryChange("");
  };

  return (
    <div className="flex h-full min-h-0 flex-col bg-card/40 border border-border/70 rounded-2xl shadow-[0_4px_24px_-4px_rgba(0,0,0,0.12)] overflow-hidden backdrop-blur-xl">
      {/* ── 顶栏标题与统计 ── */}
      <div className="flex-none p-3.5 sm:p-4 border-b border-border/60 bg-card/60 backdrop-blur-lg space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2.5">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary/10 border border-primary/20 text-primary shadow-xs">
              <Ticket className="h-4 w-4" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <span className="font-semibold text-sm tracking-tight text-foreground">
                  工单索引
                </span>
                <span className="font-mono text-[11px] px-2 py-0.5 rounded-full bg-secondary/80 text-secondary-foreground border border-border/50 font-medium">
                  {tickets.length} 条
                </span>
              </div>
            </div>
          </div>

          <Button
            variant="ghost"
            size="icon"
            onClick={onRefresh}
            disabled={isFetching}
            className="h-7 w-7 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-all"
            title="刷新工单列表"
          >
            <RefreshCw
              className={cn("h-3.5 w-3.5 transition-all", isFetching && "animate-spin text-primary")}
            />
          </Button>
        </div>

        {/* ── 仿 Linear 风格搜索条 ── */}
        <form onSubmit={handleSearchSubmit} className="relative group">
          <Search className="absolute left-3 top-2.5 h-3.5 w-3.5 text-muted-foreground/70 group-focus-within:text-primary transition-colors" />
          <Input
            value={localSearch}
            onChange={(e) => setLocalSearch(e.target.value)}
            onBlur={() => onSearchQueryChange(localSearch)}
            placeholder="按编号、标题或服务搜索..."
            className="h-8.5 pl-8.5 pr-8 text-xs bg-background/60 hover:bg-background/90 focus:bg-background border-border/70 focus-visible:border-primary/50 focus-visible:ring-2 focus-visible:ring-primary/20 rounded-xl transition-all shadow-xs"
          />
          {localSearch ? (
            <button
              type="button"
              onClick={handleClearSearch}
              className="absolute right-2.5 top-2.5 text-muted-foreground hover:text-foreground p-0.5 rounded transition-colors"
            >
              <X className="h-3 w-3" />
            </button>
          ) : (
            <kbd className="hidden sm:inline-flex absolute right-2.5 top-2 h-4.5 select-none items-center gap-1 rounded border border-border/60 bg-muted/60 px-1.5 font-mono text-[10px] text-muted-foreground">
              ↵
            </kbd>
          )}
        </form>

        {/* ── macOS 风格滑动分段按钮与归档开关 ── */}
        <div className="flex items-center justify-between gap-2 pt-0.5">
          <div className="flex items-center bg-muted/50 p-1 rounded-xl border border-border/40 text-xs w-full sm:w-auto">
            {statusTabs.map((tab) => {
              const active = statusFilter === tab.value;
              return (
                <button
                  key={tab.value}
                  type="button"
                  onClick={() => onStatusFilterChange(tab.value)}
                  className={cn(
                    "flex-1 sm:flex-none flex items-center justify-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium transition-all duration-150",
                    active
                      ? "bg-card text-foreground font-semibold shadow-[0_1px_4px_rgba(0,0,0,0.12)] border border-border/40"
                      : "text-muted-foreground hover:text-foreground hover:bg-card/40"
                  )}
                >
                  <span>{tab.label}</span>
                </button>
              );
            })}
          </div>

          <button
            type="button"
            onClick={() => onArchivedFilterChange(!archivedFilter)}
            className={cn(
              "flex items-center gap-1.5 px-2.5 py-1 rounded-lg border text-[11px] transition-all shrink-0",
              archivedFilter
                ? "bg-secondary text-foreground border-border/80 font-medium"
                : "text-muted-foreground border-transparent hover:border-border/50 hover:bg-muted/40"
            )}
            title="切换是否显示历史归档工单"
          >
            <Archive className="h-3 w-3" />
            <span className="hidden sm:inline">包含归档</span>
          </button>
        </div>
      </div>

      {/* ── 异常告警横幅 ── */}
      {incomplete && (
        <div className="flex-none px-3.5 py-2 bg-amber-500/10 border-b border-amber-500/20 text-xs text-amber-600 dark:text-amber-400 flex items-center gap-2">
          <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">{warning || "部分工单详情未能完全拉取，已展示基础列表"}</span>
        </div>
      )}

      {/* ── 工单列表容器 ── */}
      <div className="flex-1 overflow-y-auto p-2 sm:p-2.5 space-y-2">
        {isLoading ? (
          <div className="space-y-2.5 p-1">
            <Skeleton className="h-20 w-full rounded-xl" />
            <Skeleton className="h-20 w-full rounded-xl" />
            <Skeleton className="h-20 w-full rounded-xl" />
            <Skeleton className="h-20 w-full rounded-xl" />
          </div>
        ) : tickets.length === 0 ? (
          <div className="py-14 px-4 text-center">
            <EmptyState
              icon={Inbox}
              title="当前筛选下无工单"
              description={
                searchQuery
                  ? "没有匹配该搜索关键字的工单记录"
                  : "当前状态下未查询到工单，可点击上方「创建工单」发起咨询"
              }
            />
          </div>
        ) : (
          tickets.map((t) => {
            const isSelected = selectedTicketId === t.ticketId;
            const stateMeta = getStateMeta(t.state);

            return (
              <div
                key={t.ticketId}
                onClick={() => onSelectTicket(t)}
                className={cn(
                  "group relative flex flex-col p-3.5 rounded-xl cursor-pointer transition-all duration-200 border text-left",
                  isSelected
                    ? "border-primary/50 bg-gradient-to-r from-primary/12 via-primary/5 to-card/60 shadow-[0_4px_16px_-4px_rgba(16,185,129,0.18)]"
                    : "border-border/50 bg-card/40 hover:bg-card/80 hover:border-border/80 hover:shadow-xs"
                )}
              >
                {/* 选中高亮左边指示条 */}
                {isSelected && (
                  <div className="absolute left-0 top-2.5 bottom-2.5 w-1 rounded-r bg-primary shadow-[0_0_8px_hsl(var(--primary))]" />
                )}

                {/* 顶栏：工单号 + 状态指示灯与 Badge + 时间戳 */}
                <div className="flex items-center justify-between gap-2 mb-1.5 pl-0.5">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="font-mono text-xs font-semibold px-1.5 py-0.5 rounded-md bg-secondary text-secondary-foreground border border-border/60">
                      #{t.ticketNumber || t.ticketId}
                    </span>
                    <span
                      className={cn(
                        "inline-flex items-center gap-1.5 text-[11px] px-2 py-0.5 rounded-md border font-medium",
                        stateMeta.badgeClass
                      )}
                    >
                      <span className={cn("h-1.5 w-1.5 rounded-full", stateMeta.dotClass)} />
                      {stateMeta.label}
                    </span>
                  </div>

                  <span className="text-[11px] font-mono text-muted-foreground whitespace-nowrap">
                    {formatDateTime(t.updateDate || t.creationDate).slice(5)}
                  </span>
                </div>

                {/* 主题：2 行智能截断 */}
                <h3
                  className={cn(
                    "text-xs sm:text-[13px] font-medium line-clamp-2 leading-relaxed mb-2.5 tracking-tight transition-colors pl-0.5",
                    isSelected
                      ? "text-foreground font-semibold"
                      : "text-foreground/90 group-hover:text-primary"
                  )}
                >
                  {t.subject}
                </h3>

                {/* 底栏元数据：产品/服务标识 + 箭头指示 */}
                <div className="flex items-center justify-between gap-2 text-[11px] text-muted-foreground pl-0.5">
                  <div className="flex items-center gap-1.5 truncate">
                    {t.product && (
                      <span className="px-1.5 py-0.5 rounded bg-muted/60 text-muted-foreground border border-border/40 font-medium">
                        {getProductLabel(t.product)}
                      </span>
                    )}
                    {t.serviceName && (
                      <span className="font-mono text-[10px] bg-muted/80 text-foreground/80 px-1.5 py-0.5 rounded border border-border/50 truncate max-w-[150px] flex items-center gap-1">
                        <Server className="h-2.5 w-2.5 opacity-60 shrink-0" />
                        {t.serviceName}
                      </span>
                    )}
                  </div>

                  <div className="flex items-center gap-1 shrink-0">
                    <ChevronRight
                      className={cn(
                        "h-3.5 w-3.5 transition-transform duration-200 group-hover:translate-x-0.5",
                        isSelected ? "text-primary" : "text-muted-foreground/30"
                      )}
                    />
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
