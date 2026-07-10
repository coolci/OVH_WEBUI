import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import {
  FileText,
  RefreshCw,
  Trash2,
  Search,
  ChevronDown,
  AlertTriangle,
} from "lucide-react";
import { memo, useEffect, useMemo, useRef, useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useLogs, useClearLogs, type LogEntry } from "@/hooks/use-logs";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 80; // 前端一次渲染条数，避免 200+ DOM 卡顿
const LIMIT_OPTIONS = [100, 200, 300, 500] as const;

/** 防抖 */
function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

function LogsPage() {
  const [autoRefresh, setAutoRefresh] = useState(false); // 默认关，避免常驻轮询
  const [limit, setLimit] = useState<number>(200);
  const [search, setSearch] = useState("");
  const [levelFilter, setLevelFilter] = useState<string>("all");
  const [confirmClear, setConfirmClear] = useState(false);
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);

  const debouncedSearch = useDebounced(search, 280);

  // 级别过滤下推到服务端；关键字仅客户端（消息内容）
  const logs = useLogs({
    limit,
    level: levelFilter === "all" ? undefined : levelFilter,
    order: "desc",
    autoRefresh,
    refreshIntervalMs: 12_000,
  });
  const clear = useClearLogs();

  const items = logs.data?.logs || [];
  const total = logs.data?.total ?? items.length;
  const truncated = logs.data?.truncated ?? false;

  const filtered = useMemo(() => {
    const s = debouncedSearch.trim().toLowerCase();
    if (!s) return items;
    return items.filter((l) => `${l.message} ${l.source}`.toLowerCase().includes(s));
  }, [items, debouncedSearch]);

  // 筛选/limit 变化时重置可视窗口
  useEffect(() => {
    setVisibleCount(PAGE_SIZE);
  }, [limit, levelFilter, debouncedSearch, items.length]);

  const windowed = useMemo(
    () => filtered.slice(0, visibleCount),
    [filtered, visibleCount]
  );
  const hasMore = visibleCount < filtered.length;

  // 仅当用户靠近底部时自动跟滚（避免刷新打断浏览）
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const stickBottomRef = useRef(true);
  useEffect(() => {
    const el = scrollerRef.current;
    if (!el) return;
    const onScroll = () => {
      const dist = el.scrollHeight - el.scrollTop - el.clientHeight;
      stickBottomRef.current = dist < 80;
    };
    el.addEventListener("scroll", onScroll, { passive: true });
    return () => el.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    if (!autoRefresh || !stickBottomRef.current) return;
    // desc 列表顶部是最新：贴顶
    const el = scrollerRef.current;
    if (el && el.scrollTop < 40) {
      /* already near top */
    }
  }, [windowed, autoRefresh]);

  return (
    <div className="space-y-5 sm:space-y-6">
      <PageHeader
        icon={FileText}
        title="系统日志"
        description="限量拉取 · 本地筛选 · 避免一次渲染过多"
        action={
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => logs.refetch()}
              disabled={logs.isFetching}
            >
              <RefreshCw className={cn("w-4 h-4", logs.isFetching && "animate-spin")} />
              刷新
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setConfirmClear(true)}
              disabled={items.length === 0}
            >
              <Trash2 className="w-4 h-4" />
              清空
            </Button>
          </div>
        }
      />

      <Card>
        <CardContent className="p-4 sm:p-5">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4 lg:items-center">
            <div className="relative sm:col-span-2 lg:col-span-1">
              <Search className="pointer-events-none absolute left-3.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder="搜索消息 / 来源…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="rounded-full pl-9"
              />
            </div>
            <Select value={levelFilter} onValueChange={setLevelFilter}>
              <SelectTrigger className="rounded-full">
                <SelectValue placeholder="级别" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">所有级别</SelectItem>
                <SelectItem value="INFO">INFO</SelectItem>
                <SelectItem value="WARNING">WARNING</SelectItem>
                <SelectItem value="ERROR">ERROR</SelectItem>
                <SelectItem value="DEBUG">DEBUG</SelectItem>
              </SelectContent>
            </Select>
            <Select
              value={String(limit)}
              onValueChange={(v) => setLimit(Number(v))}
            >
              <SelectTrigger className="rounded-full">
                <SelectValue placeholder="条数" />
              </SelectTrigger>
              <SelectContent>
                {LIMIT_OPTIONS.map((n) => (
                  <SelectItem key={n} value={String(n)}>
                    最近 {n} 条
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <label className="inline-flex cursor-pointer items-center gap-2 text-sm">
              <Checkbox
                checked={autoRefresh}
                onCheckedChange={(c) => setAutoRefresh(c === true)}
              />
              <span className="text-muted-foreground">
                自动刷新 <span className="font-mono text-[11px]">12s</span>
              </span>
            </label>
          </div>
          {(truncated || total > items.length) && (
            <p className="mt-3 flex items-start gap-1.5 text-[11px] text-muted-foreground">
              <AlertTriangle className="mt-0.5 h-3 w-3 flex-shrink-0 text-warning" />
              内存中共 {total} 条匹配日志，当前仅加载最新 {items.length} 条。增大「最近 N 条」可看更多。
            </p>
          )}
        </CardContent>
      </Card>

      <Card className="overflow-hidden">
        <div className="flex items-center justify-between border-b border-border px-4 py-2.5 text-[12px]">
          <span className="font-semibold">日志流</span>
          <span className="font-mono text-muted-foreground">
            显示 {windowed.length}
            {filtered.length !== windowed.length ? ` / ${filtered.length}` : ""}
            {items.length !== filtered.length ? `（筛选自 ${items.length}）` : ""}
            {logs.isFetching ? " · 更新中…" : ""}
          </span>
        </div>

        {logs.isPending && items.length === 0 ? (
          <div className="space-y-2 p-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="h-8" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState icon={FileText} title="没有日志" description="试试调整级别或搜索关键字" />
        ) : (
          <>
            <div
              ref={scrollerRef}
              className="max-h-[min(70vh,calc(100dvh-320px))] overflow-y-auto overscroll-contain"
            >
              <div className="divide-y divide-border/80">
                {windowed.map((log) => (
                  <LogRow key={log.id} log={log} />
                ))}
              </div>
            </div>
            {hasMore && (
              <div className="border-t border-border p-3 text-center">
                <Button
                  variant="outline"
                  size="sm"
                  className="rounded-full"
                  onClick={() => setVisibleCount((n) => n + PAGE_SIZE)}
                >
                  <ChevronDown className="h-3.5 w-3.5" />
                  再显示 {Math.min(PAGE_SIZE, filtered.length - visibleCount)} 条
                </Button>
              </div>
            )}
          </>
        )}
      </Card>

      <Dialog open={confirmClear} onOpenChange={setConfirmClear}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认清空日志？</DialogTitle>
            <DialogDescription>将清空后端内存与日志文件，不可撤销。</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmClear(false)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                clear.mutate();
                setConfirmClear(false);
              }}
            >
              确认清空
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

const LogRow = memo(function LogRow({ log }: { log: LogEntry }) {
  const tone =
    log.level === "ERROR"
      ? "danger"
      : log.level === "WARNING" || log.level === "WARN"
        ? "warning"
        : log.level === "DEBUG"
          ? "default"
          : "info";

  let timeLabel = log.timestamp;
  try {
    timeLabel = new Date(log.timestamp).toLocaleString("zh-CN", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    /* keep raw */
  }

  return (
    <div className="flex items-start gap-2.5 px-3 py-2 text-[12px] hover:bg-muted/40 sm:gap-3 sm:px-4">
      <span className="w-[4.5rem] flex-shrink-0 font-mono text-[11px] text-muted-foreground sm:w-32">
        {timeLabel}
      </span>
      <Chip tone={tone as "danger" | "warning" | "default" | "info"} className="w-14 justify-center font-mono sm:w-16">
        {log.level === "WARNING" ? "WARN" : log.level}
      </Chip>
      <span className="hidden w-24 flex-shrink-0 truncate font-mono text-muted-foreground sm:inline sm:w-28">
        [{log.source}]
      </span>
      <span className="min-w-0 flex-1 break-words leading-relaxed text-foreground/90">
        <span className="mr-1.5 font-mono text-[10px] text-muted-foreground sm:hidden">
          [{log.source}]
        </span>
        {log.message}
      </span>
    </div>
  );
});

const Page = () => (
  <>
    <Helmet>
      <title>系统日志 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <LogsPage />
    </AppLayout>
  </>
);

export default Page;
