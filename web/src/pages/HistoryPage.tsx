import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { History, RefreshCw, Trash2, Search, ExternalLink, AlertCircle, Hourglass, RotateCcw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Chip } from "@/components/common/Chip";
import { AccountChip } from "@/components/common/AccountChip";
import { TimingChip } from "@/components/common/TimingChip";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { LoadFailed } from "@/components/common/LoadFailed";
import { CURRENCY_UNKNOWN_HINT, formatMoney } from "@/lib/money";
import { describeOptionCodes } from "@/lib/option-groups";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useHistory,
  useClearHistory,
  useRefreshOrderStatus,
  type PurchaseHistory,
} from "@/hooks/use-history";
import { cn, safeHttpsUrl } from "@/lib/utils";
import { OrderDurationBadge } from "@/components/orders/OrderDurationField";
import { OrderProgressButton } from "@/components/history/OrderProgressButton";
import { CancelOrderButton } from "@/components/history/CancelOrderButton";
import { HistoryPaymentButton } from "@/components/history/HistoryPaymentButton";
import { Checkbox } from "@/components/ui/checkbox";
import { BulkCleanupBar, BulkCleanupDialog } from "@/components/common/BulkCleanup";
import { useBulkCleanup } from "@/hooks/use-bulk-cleanup";
import { historyCleanupTarget } from "@/lib/bulk-cleanup";
import { DragSelectArea } from "@/components/common/DragSelectArea";
import { normalizeHistorySort, sortHistory } from "@/lib/history-sort";

/**
 * 订单支付状态 → 标签。取值是 OVH 的 billing.order.OrderStatusEnum,三区一致。
 * 这才是用户真正关心的:「下单成功」只说明订单建了,付没付、过没过期看这里。
 */
function orderStatusView(item: PurchaseHistory): {
  label: string;
  tone: "success" | "warning" | "danger" | "info" | "default";
  paid: boolean;
  closed: boolean;
  title: string;
} {
  if (item.abandonedAt && !["checking", "delivering", "delivered", "documentsRequested", "cancelling", "cancelled"].includes(item.orderStatus || "")) {
    return { label: "已放弃·待作废", tone: "default", paid: false, closed: true, title: "本系统已放弃订单，等待 OVH 到期作废；付款期限内可撤销放弃" };
  }
  if (item.retractionState && item.retractionState !== "failed" && !["cancelling", "cancelled"].includes(item.orderStatus || "")) {
    return { label: item.retractionState === "submitted" ? "撤单已申请" : "撤单待核对", tone: "warning", paid: true, closed: true, title: "请在 OVH 核对撤单及退款结果，不要重复提交" };
  }
  switch (item.orderStatus) {
    case "notPaid":
      return {
        label: "待付款",
        tone: "warning",
        paid: false,
        closed: false,
        title: "订单已创建尚未付款;倒计时结束前未付款会作废",
      };
    case "checking":
      return {
        label: "付款核验中",
        tone: "info",
        paid: true,
        closed: false,
        title: "OVH 已收到付款正在核验",
      };
    case "delivering":
      return {
        label: "已付款·交付中",
        tone: "success",
        paid: true,
        closed: false,
        title: "已付款 OVH 正在交付服务器",
      };
    case "delivered":
      return {
        label: "已付款·已交付",
        tone: "success",
        paid: true,
        closed: true,
        title: "已付款并交付",
      };
    case "cancelling":
      return {
        label: "取消中",
        tone: "danger",
        paid: false,
        closed: true,
        title: "订单正在取消",
      };
    case "cancelled":
      return {
        label: "已取消",
        tone: "danger",
        paid: false,
        closed: true,
        title: "订单已取消;过期未付款也会走到这里",
      };
    case "documentsRequested":
      return {
        label: "需补材料",
        tone: "warning",
        paid: false,
        closed: false,
        title: "OVH 要求补充证件/材料后才处理",
      };
    case "unknown":
      return {
        label: "状态未知",
        tone: "default",
        paid: false,
        closed: false,
        title: "OVH 返回 unknown",
      };
    default:
      return {
        label: "状态未查到",
        tone: "default",
        paid: false,
        closed: false,
        title: "还没向 OVH 读到订单状态，点「刷新状态」再试",
      };
  }
}

/**
 * 成交价 + 币种。币种缺失时只显示金额并在 title 里说明，绝不补 "EUR"。
 */
function HistoryPrice({ item, strike }: { item: PurchaseHistory; strike: boolean }) {
  const value = item.price?.withoutTax != null ? item.price.withoutTax : item.price?.withTax;
  if (value == null) return <span className="text-muted-foreground">—</span>;
  const currency = (item.price?.currencyCode || "").trim();
  return (
    <span
      className={`font-mono font-medium text-success ${strike ? "line-through" : ""}`}
      title={currency ? undefined : CURRENCY_UNKNOWN_HINT}
    >
      {formatMoney(value, currency || null)}
      {!currency && <span className="text-muted-foreground"> (币种未知)</span>}
    </span>
  );
}

/** 把毫秒倒计时格式化为 `2天5时12分` / `12分` / `已过期` */
function formatCountdown(remainingMs: number): string {
  if (remainingMs <= 0) return "已过期";
  const totalMinutes = Math.floor(remainingMs / 60_000);
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;
  if (days > 0) return `${days}天${hours}时${minutes}分`;
  if (hours > 0) return `${hours}时${minutes}分`;
  return `${minutes}分`;
}

/**
 * 付款截止时间,只用 OVH 返回的 expirationDate。拿不到就是"未知",绝不自己推算:
 * 促销订单可能 15 分钟不付就作废,按 15 天推算会让人以为还有两周,其实早就作废了。
 */
function getExpirationMs(item: PurchaseHistory): number | null {
  if (!item.expirationTime) return null;
  const ms = new Date(item.expirationTime).getTime();
  return Number.isFinite(ms) ? ms : null;
}

/** 付款窗口:付了、取消了、交付了都不再显示;没有 orderId 就没有 OVH 订单可付 */
function paymentWindow(item: PurchaseHistory, st: { paid: boolean; closed: boolean }, now: number) {
  const showCountdown = item.status === "success" && !!item.orderId && !st.paid && !st.closed;
  const expirationMs = showCountdown ? getExpirationMs(item) : null;
  const deadlineKnown = expirationMs != null;
  const remainingMs = deadlineKnown ? expirationMs - now : 0;
  const isExpired = showCountdown && deadlineKnown && remainingMs <= 0;
  // 24 小时内进入告警色;截止时间未知时一律按紧急处理
  const isUrgent = showCountdown && !isExpired && (!deadlineKnown || remainingMs < 24 * 60 * 60 * 1000);
  return { showCountdown, expirationMs, deadlineKnown, remainingMs, isExpired, isUrgent };
}

function DeadlineChip({ w }: { w: ReturnType<typeof paymentWindow> }) {
  if (!w.deadlineKnown) {
    return (
      <Chip tone="warning" title="暂未从 OVH 取到付款截止时间(后台每 10 分钟会自动补查,点页面上的「刷新」可立即重查)。促销订单可能 15 分钟内不付款就作废,请尽快付款">
        <Hourglass className="w-3 h-3" />
        截止未知·尽快付款
      </Chip>
    );
  }
  return (
    <Chip
      tone={w.isExpired ? "danger" : w.isUrgent ? "warning" : "info"}
      title={`到期截止时间: ${new Date(w.expirationMs as number).toLocaleString("zh-CN")}`}
    >
      <Hourglass className="w-3 h-3" />
      {formatCountdown(w.remainingMs)}
    </Chip>
  );
}

function HistoryPage() {
  const list = useHistory();
  const clear = useClearHistory();
  const refreshStatus = useRefreshOrderStatus();
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "success" | "failed">("all");
  const [sort, setSort] = useState(() => {
    try { return normalizeHistorySort(localStorage.getItem("ovh_history_sort")); }
    catch { return normalizeHistorySort(null); }
  });
  const [confirmClear, setConfirmClear] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  // 每分钟刷新一次 now，让所有行的倒计时同步推进
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);

  const rawItems = list.data;
  const items = useMemo(() => rawItems || [], [rawItems]);
  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    return sortHistory(items.filter((i) => {
      if (statusFilter !== "all" && i.status !== statusFilter) return false;
      if (s && !`${i.planCode} ${i.datacenter} ${i.orderId || ""}`.toLowerCase().includes(s)) return false;
      return true;
    }), sort);
  }, [items, search, statusFilter, sort]);
  const cleanup = useBulkCleanup(filtered, historyCleanupTarget, "history");

  return (
    <DragSelectArea className="space-y-5" selectedKeys={cleanup.selectedKeys} onSelectionChange={cleanup.selectKeys}
      onDeleteSelected={cleanup.requestCleanup} disabled={cleanup.locked || list.isError || clear.isPending}>
      <PageHeader
        icon={History}
        title="抢购历史"
        description="查看服务器购买历史记录"
        action={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg border-border/80 hover:bg-secondary px-3"
              onClick={() => refreshStatus.mutate()}
              disabled={list.isFetching || refreshStatus.isPending}
              title="向 OVH 查询未到终态订单的支付状态，然后重载列表"
            >
              <RefreshCw
                className={cn(
                  "w-3.5 h-3.5",
                  (list.isFetching || refreshStatus.isPending) && "animate-spin"
                )}
              />
              <span>刷新</span>
            </Button>
            <Button
              variant="soft-destructive"
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg px-3"
              onClick={() => setConfirmClear(true)}
              disabled={items.length === 0 || cleanup.locked || list.isError || clear.isPending}
              title="清空所有历史"
            >
              <Trash2 className="w-3.5 h-3.5" />
              <span>清空全部</span>
            </Button>
          </div>
        }
      />

      <Card className="surface-card rounded-xl border-border">
        <CardContent className="p-3 sm:p-3.5">
          <div className="grid grid-cols-1 sm:grid-cols-[1fr_140px_188px] gap-2.5 items-center">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
              <Input
                placeholder="搜索型号 / 机房 / 订单号..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-8 rounded-lg h-8 text-xs bg-background/60 border-border/80 focus-visible:ring-1 focus-visible:ring-primary/40"
              />
            </div>
            <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v as typeof statusFilter)}>
              <SelectTrigger className="rounded-lg h-8 text-xs bg-background/60 border-border/80">
                <SelectValue placeholder="所有状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">所有状态</SelectItem>
                <SelectItem value="success">成功</SelectItem>
                <SelectItem value="failed">失败</SelectItem>
              </SelectContent>
            </Select>
            <Select value={sort} onValueChange={(value) => {
              const next = normalizeHistorySort(value);
              setSort(next);
              try { localStorage.setItem("ovh_history_sort", next); } catch { /* private mode */ }
            }}>
              <SelectTrigger aria-label="排序方式" className="rounded-lg h-8 text-xs bg-background/60 border-border/80">
                <SelectValue placeholder="排序方式" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="paid-first">已付款优先</SelectItem>
                <SelectItem value="newest">下单时间：最新在前</SelectItem>
                <SelectItem value="oldest">下单时间：最早在前</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="mt-2 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
            <span>
              {sort === "paid-first"
                ? "已付款·交付中、已付款·已交付的订单置顶，同组按下单时间从新到旧排列。"
                : "按下单时间排序展示历史记录。"}
            </span>
            <span className="font-mono tabular-nums shrink-0">
              {filtered.length}/{items.length} 条
            </span>
          </div>
        </CardContent>
      </Card>

      <BulkCleanupBar {...cleanup} disabled={cleanup.locked || list.isError || list.isPending || clear.isPending}
        onToggleAll={cleanup.toggleAll} onClearSelection={cleanup.clearSelection} onCleanup={cleanup.requestCleanup} />

      {list.isPending ? (
        <Card className="surface-card rounded-xl border-border">
          <CardContent className="p-4 space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
          </CardContent>
        </Card>
      ) : list.isError ? (
        <Card className="surface-card rounded-xl border-border">
          <LoadFailed
            icon={History}
            title="抢购历史读取失败 —— 不是「你没有订单」,是我们没读到"
            error={list.error}
            onRetry={() => list.refetch()}
          />
          <p className="px-6 pb-5 text-[11px] text-muted-foreground text-center">
            未付款的订单仍在走 15 分钟倒计时,别把这片空白当成"没有订单"。
            请重试,或直接去 OVH 管理面板确认待付款的订单。
          </p>
        </Card>
      ) : filtered.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState icon={History} title="没有匹配的订单" />
        </Card>
      ) : (
        <>
          {/* 桌面 / 平板:横向表格 */}
          <Card className="surface-card rounded-xl border-border overflow-hidden hidden md:block">
            <div className="table-scroll">
              <table className="w-full min-w-[820px]">
                <thead>
                  <tr className="text-left text-[11px] font-medium text-muted-foreground border-b border-border/60 bg-muted/35">
                    <th className="pl-4 py-2.5 w-10"><span className="sr-only">选择记录</span></th>
                    <th className="px-3 py-2.5">型号 / 账户</th>
                    <th className="px-3 py-2.5">配置</th>
                    <th className="px-3 py-2.5 text-right">价格</th>
                    <th className="px-3 py-2.5">状态</th>
                    <th className="px-3 py-2.5">下单时间</th>
                    <th className="px-3 py-2.5" title="付款窗口:下单不会自动扣款,倒计时结束前未付款订单作废">
                      付款剩余
                    </th>
                    <th className="pr-4 pl-3 py-2.5 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/45">
                  {filtered.map((item) => (
                    <HistoryRow key={item.id} item={item} now={now} selected={cleanup.isSelected(item)}
                      onSelect={() => cleanup.toggle(item)} selectionDisabled={cleanup.locked} />
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          {/* 手机:卡片堆叠,每条订单一张卡 */}
          <div className="md:hidden space-y-3">
            {filtered.map((item) => (
              <HistoryCard key={item.id} item={item} now={now} selected={cleanup.isSelected(item)}
                onSelect={() => cleanup.toggle(item)} selectionDisabled={cleanup.locked} />
            ))}
          </div>
        </>
      )}

      <BulkCleanupDialog {...cleanup.dialogProps} />
      <Dialog open={confirmClear} onOpenChange={setConfirmClear}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认清空所有历史？</DialogTitle>
            <DialogDescription>所有抢购历史将被删除，此操作不可撤销。</DialogDescription>
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
    </DragSelectArea>
  );
}

interface HistorySelectionProps {
  item: PurchaseHistory; now: number; selected: boolean; selectionDisabled: boolean; onSelect: () => void;
}

function HistoryRow({ item, now, selected, selectionDisabled, onSelect }: HistorySelectionProps) {
  const st = orderStatusView(item);
  const w = paymentWindow(item, st, now);
  const { showCountdown, isExpired } = w;

  return (
    <tr data-selection-key={item.id} className={`text-[13px] align-middle hover:bg-muted/45 transition-colors ${selected ? "bg-primary/5" : ""} ${isExpired ? "opacity-60" : ""}`}>
      <td className="pl-4 py-3"><Checkbox checked={selected} disabled={selectionDisabled} onCheckedChange={onSelect}
        aria-label={`选择历史记录 ${item.planCode} ${item.orderId || item.id}`} /></td>
      <td className="px-3 py-3">
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 flex-wrap">
            <span className={`font-mono font-semibold text-foreground ${isExpired ? "line-through" : ""}`}>
              {item.planCode}
            </span>
            <span className={`px-1.5 py-0.5 rounded bg-secondary text-foreground/85 border border-border/60 font-mono text-[10px] font-semibold uppercase leading-none ${isExpired ? "line-through" : ""}`}>
              {item.datacenter}
            </span>
            <OrderDurationBadge value={item.duration} />
          </div>
          <div className="flex items-center gap-1.5 flex-wrap">
            <AccountChip accountId={item.accountId} />
            <TimingChip totalMs={item.totalMs} phases={item.timing} />
          </div>
          {item.couponWarning && (
            <p className="text-[11px] font-normal text-warning break-words" role="status">
              {item.couponWarning}
            </p>
          )}
        </div>
      </td>
      <td
        className={`px-3 py-3 text-xs text-muted-foreground max-w-[200px] truncate ${isExpired ? "line-through" : ""}`}
        title={item.options && item.options.length > 0 ? item.options.join("\n") : undefined}
      >
        {item.options && item.options.length > 0 ? describeOptionCodes(item.options) : "默认配置"}
      </td>
      <td className="px-3 py-3 text-right whitespace-nowrap tabular-nums">
        <HistoryPrice item={item} strike={isExpired} />
      </td>
      <td className="px-3 py-3 whitespace-nowrap">
        {item.status === "success" ? (
          <Chip tone={st.tone} title={st.title}>
            {st.label}
          </Chip>
        ) : (
          <Chip tone="danger">失败</Chip>
        )}
      </td>
      <td className="px-3 py-3 text-[11px] text-muted-foreground font-mono tabular-nums whitespace-nowrap">
        {new Date(item.purchaseTime).toLocaleString("zh-CN", {
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
        })}
      </td>
      <td className="px-3 py-3 whitespace-nowrap">
        {showCountdown ? (
          <DeadlineChip w={w} />
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>
      <td className="pr-4 pl-3 py-3">
        <div className="flex items-center justify-end gap-1.5 flex-wrap">
          {item.status === "success" && item.orderId && <OrderProgressButton accountId={item.accountId} orderId={item.orderId} />}
          <HistoryPaymentButton item={item} />
          <CancelOrderButton item={item} />
          <BuyAgainButton item={item} />
          {item.status === "success" && safeHttpsUrl(item.orderUrl) ? (
            <a
              href={safeHttpsUrl(item.orderUrl)}
              target="_blank"
              rel="noopener noreferrer"
              aria-disabled={isExpired}
              className={`inline-flex items-center gap-1 text-foreground hover:underline text-[12px] px-1 ${isExpired ? "pointer-events-none opacity-50" : ""
                }`}
            >
              <ExternalLink className="w-3 h-3" />
              订单
            </a>
          ) : item.status === "failed" && item.errorMessage ? (
            <button
              type="button"
              onClick={() => toast.info(item.errorMessage)}
              className="inline-flex items-center gap-1 text-destructive hover:underline text-[12px] px-1"
            >
              <AlertCircle className="w-3 h-3" />
              错误
            </button>
          ) : null}
        </div>
      </td>
    </tr>
  );
}

/** 手机端的订单卡片渲染。跟 HistoryRow 字段一一对应,但堆叠成卡片。 */
function HistoryCard({ item, now, selected, selectionDisabled, onSelect }: HistorySelectionProps) {
  const st = orderStatusView(item);
  const w = paymentWindow(item, st, now);
  const { showCountdown, isExpired } = w;

  return (
    <Card data-selection-key={item.id} className={cn("surface-card rounded-xl border-border hover:border-border/90 transition-colors", selected && "border-primary bg-primary/5", isExpired && "opacity-60")}>
      <CardContent className="p-3.5 space-y-2.5">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2 flex-wrap min-w-0">
            <Checkbox checked={selected} disabled={selectionDisabled} onCheckedChange={onSelect}
              aria-label={`选择历史记录 ${item.planCode} ${item.orderId || item.id}`} />
            <span className={`font-mono font-bold text-sm ${isExpired ? "line-through" : ""}`}>
              {item.planCode}
            </span>
            <span className="px-1.5 py-0.5 rounded bg-secondary text-foreground/85 border border-border/60 font-mono text-[10px] font-semibold uppercase leading-none">
              {item.datacenter}
            </span>
            <OrderDurationBadge value={item.duration} />
          </div>
          {item.status === "success" ? (
            <Chip tone={st.tone} title={st.title} className="shrink-0">
              {st.label}
            </Chip>
          ) : (
            <Chip tone="danger" className="shrink-0">失败</Chip>
          )}
        </div>
        <div className="flex items-center gap-1.5 flex-wrap">
          <AccountChip accountId={item.accountId} />
          <TimingChip totalMs={item.totalMs} phases={item.timing} />
          {item.couponWarning && (
            <p className="w-full text-[11px] font-normal text-warning break-words" role="status">
              {item.couponWarning}
            </p>
          )}
        </div>
        <div
          className={`text-[11px] text-muted-foreground break-all ${isExpired ? "line-through" : ""}`}
          title={item.options && item.options.length > 0 ? item.options.join("\n") : undefined}
        >
          {item.options && item.options.length > 0 ? describeOptionCodes(item.options) : "默认配置"}
        </div>
        <div className="flex items-center justify-between gap-2 text-[11px] tabular-nums">
          <span className="text-muted-foreground font-mono">
            {new Date(item.purchaseTime).toLocaleString("zh-CN", {
              month: "2-digit",
              day: "2-digit",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </span>
          {(item.price?.withoutTax ?? item.price?.withTax) != null ? <HistoryPrice item={item} strike={isExpired} /> : null}
        </div>
        <div className="flex items-center justify-between gap-2 flex-wrap pt-2 border-t border-border/45">
          {showCountdown ? (
            <DeadlineChip w={w} />
          ) : (
            <span />
          )}
          <div className="flex items-center gap-2 flex-wrap">
            {item.status === "success" && item.orderId && <OrderProgressButton accountId={item.accountId} orderId={item.orderId} />}
            <HistoryPaymentButton item={item} />
            <CancelOrderButton item={item} />
            <BuyAgainButton item={item} />
            {item.status === "success" && safeHttpsUrl(item.orderUrl) ? (
              <a
                href={safeHttpsUrl(item.orderUrl)}
                target="_blank"
                rel="noopener noreferrer"
                className={`inline-flex items-center gap-1 text-foreground hover:underline text-[12px] ${isExpired ? "pointer-events-none opacity-50" : ""
                  }`}
              >
                <ExternalLink className="w-3 h-3" />
                订单
              </a>
            ) : item.status === "failed" && item.errorMessage ? (
              <button
                type="button"
                onClick={() => toast.info(item.errorMessage)}
                className="inline-flex items-center gap-1 text-destructive hover:underline text-[12px]"
              >
                <AlertCircle className="w-3 h-3" />
                错误详情
              </button>
            ) : null}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

/** 购买历史「再抢一单」：把原订单的型号、账户、机房、配置与周期一并带入队列新建弹窗。 */
function BuyAgainButton({ item }: { item: PurchaseHistory }) {
  const navigate = useNavigate();
  if (!item.planCode) return null;
  const openCreate = () => {
    const params = new URLSearchParams({ create: item.planCode });
    if (item.accountId) params.set("account", item.accountId);
    if (item.datacenter) params.set("dc", item.datacenter);
    if (item.options?.length) params.set("options", item.options.join(","));
    if (item.duration) params.set("duration", item.duration);
    navigate(`/queue?${params.toString()}`);
  };
  return (
    <Button
      variant="ghost"
      size="sm"
      className="h-7 gap-1 px-1.5 text-xs text-muted-foreground hover:text-foreground"
      onClick={openCreate}
      title="沿用原账户、机房、配置和周期新建抢购任务"
    >
      <RotateCcw className="h-3 w-3" />
      再抢一单
    </Button>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>购买历史 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <HistoryPage />
    </AppLayout>
  </>
);

export default Page;
