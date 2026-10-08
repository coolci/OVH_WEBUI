import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  Archive,
  ArrowDownWideNarrow,
  CheckCircle2,
  CircleDot,
  Inbox,
  Plus,
  RefreshCw,
  Search,
  X,
} from "lucide-react";
import { type SupportTicket } from "@/hooks/ovh/use-tickets";
import { Skeleton } from "@/components/common/Skeleton";
import { getProductLabel, getStateMeta } from "./labels";
import { formatDateTime } from "@/lib/format-os";
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
  onSearchQueryChange: (query: string) => void;
  isLoading: boolean;
  isFetching: boolean;
  isError?: boolean;
  onRefresh: () => void;
  onCreate: () => void;
  warning?: string;
  incomplete?: boolean;
}

function shortDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const now = new Date();
  if (date.toDateString() === now.toDateString())
    return date.toLocaleTimeString("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (date.toDateString() === yesterday.toDateString()) return "昨天";
  return date.toLocaleDateString("zh-CN", {
    month: "numeric",
    day: "numeric",
    ...(date.getFullYear() !== now.getFullYear() ? { year: "numeric" } : {}),
  });
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
  isError,
  onRefresh,
  onCreate,
  warning,
  incomplete,
}: TicketListProps) {
  const [localSearch, setLocalSearch] = useState(searchQuery);
  const sortedTickets = useMemo(
    () =>
      [...tickets].sort(
        (a, b) =>
          Date.parse(b.updateDate || b.creationDate) -
          Date.parse(a.updateDate || a.creationDate),
      ),
    [tickets],
  );

  useEffect(() => {
    setLocalSearch(searchQuery);
  }, [searchQuery]);
  useEffect(() => {
    if (localSearch === searchQuery) return;
    const timer = window.setTimeout(
      () => onSearchQueryChange(localSearch),
      350,
    );
    return () => window.clearTimeout(timer);
  }, [localSearch, searchQuery, onSearchQueryChange]);

  return (
    <div className="ticket-queue-content">
      <div className="ticket-queue-header">
        <div className="ticket-queue-title">
          <h2>
            <Inbox size={16} />
            工单队列
          </h2>
          <button
            className="support-icon-button"
            aria-label="刷新工单列表"
            title="刷新工单列表"
            disabled={isFetching}
            onClick={onRefresh}
          >
            <RefreshCw size={14} className={cn(isFetching && "animate-spin")} />
          </button>
        </div>
        <form
          className="support-search"
          onSubmit={(event) => {
            event.preventDefault();
            onSearchQueryChange(localSearch);
          }}
        >
          <Search size={15} />
          <input
            aria-label="搜索工单"
            value={localSearch}
            onChange={(event) => setLocalSearch(event.target.value)}
            placeholder="搜索主题、编号或服务…"
          />
          {localSearch && (
            <button
              type="button"
              aria-label="清空搜索"
              onClick={() => {
                setLocalSearch("");
                onSearchQueryChange("");
              }}
            >
              <X size={14} />
            </button>
          )}
        </form>
        <div
          className="ticket-queue-filters"
          role="group"
          aria-label="工单状态筛选"
        >
          {[
            { value: "all", label: "全部工单" },
            { value: "open", label: "处理中" },
            { value: "closed", label: "已关闭" },
          ].map((tab) => (
            <button
              key={tab.value}
              className={cn(statusFilter === tab.value && "is-active")}
              aria-pressed={statusFilter === tab.value}
              onClick={() => onStatusFilterChange(tab.value)}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>
      <div className="ticket-queue-sort">
        <span>
          <ArrowDownWideNarrow size={12} />
          最近更新
        </span>
        <button
          className={cn("ticket-archive-toggle", archivedFilter && "is-active")}
          aria-label="包含归档"
          aria-pressed={archivedFilter}
          onClick={() => onArchivedFilterChange(!archivedFilter)}
        >
          <Archive size={12} />
          包含归档
        </button>
      </div>
      {(incomplete || warning || (isError && tickets.length > 0)) && (
        <div className="support-inline-warning" role="status">
          <AlertCircle size={14} />
          <span>
            {warning ||
              (isError
                ? "同步失败，当前显示上次加载的工单。"
                : "部分工单暂未加载完整，请稍后刷新。")}
          </span>
        </div>
      )}
      <div
        className="ticket-queue-list support-scrollbar"
        aria-busy={isLoading}
      >
        {isLoading ? (
          <div className="space-y-5 p-5">
            {[1, 2, 3, 4].map((item) => (
              <div
                key={item}
                className="space-y-3 border-b border-border/50 pb-5"
              >
                <Skeleton className="h-3 w-1/3 rounded" />
                <Skeleton className="h-4 w-5/6 rounded" />
                <Skeleton className="h-3 w-2/3 rounded" />
              </div>
            ))}
          </div>
        ) : isError && tickets.length === 0 ? (
          <div className="support-empty-state">
            <AlertCircle size={26} />
            <h3>工单加载失败</h3>
            <p>请检查连接后重新加载。</p>
            <button className="support-text-button" onClick={onRefresh}>
              重新加载工单
            </button>
          </div>
        ) : tickets.length === 0 ? (
          <div className="support-empty-state">
            <Inbox size={29} strokeWidth={1.4} />
            <h3>{searchQuery ? "没有找到相关工单" : "当前没有工单"}</h3>
            <p>
              {searchQuery
                ? "尝试其他关键词或调整筛选条件。"
                : "发起咨询，让支持团队协助处理。"}
            </p>
            {!searchQuery && (
              <button className="support-text-button" onClick={onCreate}>
                <Plus size={14} />
                新建工单
              </button>
            )}
          </div>
        ) : (
          sortedTickets.map((ticket) => {
            const state = getStateMeta(ticket.state);
            return (
              <button
                key={ticket.ticketId}
                className={cn(
                  "ticket-queue-item",
                  selectedTicketId === ticket.ticketId && "is-selected",
                )}
                aria-current={
                  selectedTicketId === ticket.ticketId ? "true" : undefined
                }
                onClick={() => onSelectTicket(ticket)}
              >
                <span className="ticket-item-meta">
                  <span>#{ticket.ticketNumber || ticket.ticketId}</span>
                  <time
                    title={formatDateTime(
                      ticket.updateDate || ticket.creationDate,
                    )}
                    dateTime={ticket.updateDate || ticket.creationDate}
                  >
                    {shortDate(ticket.updateDate || ticket.creationDate)}
                  </time>
                </span>
                <span className="ticket-item-subject">
                  {ticket.subject || "未命名工单"}
                </span>
                <span className="ticket-item-service">
                  {ticket.serviceName || getProductLabel(ticket.product)}
                </span>
                <span className="ticket-item-footer">
                  <span className={cn("support-status", `is-${state.variant}`)}>
                    {ticket.state === "closed" ? (
                      <CheckCircle2 size={11} />
                    ) : (
                      <CircleDot size={11} />
                    )}
                    {state.label}
                  </span>
                  <span>
                    {ticket.lastMessageFrom === "customer"
                      ? "最近回复：我"
                      : ticket.lastMessageFrom
                        ? "最近回复：支持团队"
                        : "尚无回复"}
                  </span>
                </span>
              </button>
            );
          })
        )}
      </div>
      <footer className="ticket-queue-footer">
        <span>
          当前显示 {tickets.length} 条
          {tickets.length === 50 ? " · 可搜索更多" : ""}
        </span>
        <span className="flex items-center gap-1.5">
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full",
              isError ? "bg-amber-500" : "bg-primary/60",
            )}
          />
          {isFetching ? "同步中" : "自动同步"}
        </span>
      </footer>
    </div>
  );
}
