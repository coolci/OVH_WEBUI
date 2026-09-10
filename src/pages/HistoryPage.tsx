import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { Clock, RefreshCw, Trash2, Search, ExternalLink, AlertCircle, Hourglass } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
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
import { cn } from "@/lib/utils";

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
 * 成交价 + 币种展示。
 * 币种缺失时只显示金额并在 title 里说明，绝不猜 "EUR"
 */
function HistoryPrice({ item, strike }: { item: PurchaseHistory; strike: boolean }) {
  const value = item.price?.withTax;
  if (value == null) return <span className="text-muted-foreground">—</span>;
  const currency = (item.price?.currencyCode || "").trim();
  return (
    <span
      className={`font-mono font-medium text-success ${strike ? "line-through" : ""}`}
      title={currency ? undefined : "币种未知"}
    >
      {value}
      {currency ? ` ${currency}` : <span className="text-muted-foreground"> (币种未知)</span>}
    </span>
  );
}

/** 订单有效期 15 天，未提供 expirationTime 时用 purchaseTime + 15d 兜底 */
const ORDER_VALIDITY_MS = 15 * 24 * 60 * 60 * 1000;

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

function getExpirationMs(item: PurchaseHistory): number {
  if (item.expirationTime) return new Date(item.expirationTime).getTime();
  return new Date(item.purchaseTime).getTime() + ORDER_VALIDITY_MS;
}

function HistoryPage() {
  const list = useHistory();
  const clear = useClearHistory();
  const refreshStatus = useRefreshOrderStatus();
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "success" | "failed">("all");
  const [confirmClear, setConfirmClear] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  // 每分钟刷新一次 now，让所有行的倒计时同步推进
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);

  const items = list.data || [];
  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    return items.filter((i) => {
      if (statusFilter !== "all" && i.status !== statusFilter) return false;
      if (s && !`${i.planCode} ${i.datacenter} ${i.orderId || ""}`.toLowerCase().includes(s)) return false;
      return true;
    });
  }, [items, search, statusFilter]);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Clock}
        title="抢购历史"
        description="查看服务器购买历史记录"
        action={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs gap-1.5 border-border/80 hover:bg-secondary px-2.5 sm:px-3 rounded-lg"
              onClick={() => refreshStatus.mutate()}
              disabled={list.isFetching || refreshStatus.isPending}
              title="向 OVH 查询未到终态订单的支付状态，然后重载列表"
            >
              <RefreshCw
                className={`w-3.5 h-3.5 ${list.isFetching || refreshStatus.isPending ? "animate-spin" : ""}`}
              />
              <span className="hidden sm:inline">刷新状态</span>
            </Button>
            {items.length > 0 && (
              <Button
                variant="outline"
                size="sm"
                className="h-8 text-xs gap-1.5 border-destructive/30 text-destructive hover:bg-destructive/10 px-2.5 sm:px-3 rounded-lg"
                onClick={() => setConfirmClear(true)}
                title="清空所有历史"
              >
                <Trash2 className="w-3.5 h-3.5" />
                <span className="hidden sm:inline">清空</span>
              </Button>
            )}
          </div>
        }
      />

      <Card className="surface-card rounded-xl border-border">
        <CardContent className="p-3.5 sm:p-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="relative">
              <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
              <Input
                placeholder="搜索型号 / 机房 / 订单号..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9 rounded-lg h-8 text-xs bg-background/50 border-border/80 focus-visible:ring-1 focus-visible:ring-primary/40"
              />
            </div>
            <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v as any)}>
              <SelectTrigger className="rounded-lg h-8 text-xs bg-background/50 border-border/80">
                <SelectValue placeholder="所有状态" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">所有状态</SelectItem>
                <SelectItem value="success">成功</SelectItem>
                <SelectItem value="failed">失败</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {list.isPending ? (
        <Card className="surface-card rounded-xl border-border">
          <CardContent className="p-4 space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
          </CardContent>
        </Card>
      ) : filtered.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState icon={Clock} title="没有匹配的订单" />
        </Card>
      ) : (
        <>
          {/* 桌面 / 平板:横向表格 */}
          <Card className="surface-card rounded-xl border-border overflow-hidden shadow-sm hidden md:block">
            <div className="table-scroll">
              <table className="w-full min-w-[760px]">
                <thead>
                  <tr className="text-left text-[11px] font-medium text-muted-foreground/80 border-b border-border/60 bg-muted/30">
                    <th className="px-4 py-3">型号</th>
                    <th className="px-4 py-3">机房</th>
                    <th className="px-4 py-3">配置</th>
                    <th className="px-4 py-3">价格</th>
                    <th className="px-4 py-3">状态</th>
                    <th className="px-4 py-3">时间</th>
                    <th className="px-4 py-3" title="付款窗口:下单不会自动扣款,倒计时结束前未付款订单作废">
                      付款剩余
                    </th>
                    <th className="px-4 py-3">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/40">
                  {filtered.map((item) => (
                    <HistoryRow key={item.id} item={item} now={now} />
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          {/* 手机:卡片堆叠,每条订单一张卡 */}
          <div className="md:hidden space-y-3">
            {filtered.map((item) => (
              <HistoryCard key={item.id} item={item} now={now} />
            ))}
          </div>
        </>
      )}

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
    </div>
  );
}

function HistoryRow({ item, now }: { item: PurchaseHistory; now: number }) {
  const st = orderStatusView(item);
  // 倒计时是"付款窗口":付了、取消了、交付了都不再显示
  const showCountdown = item.status === "success" && !st.paid && !st.closed;
  const remainingMs = showCountdown ? getExpirationMs(item) - now : 0;
  const isExpired = showCountdown && remainingMs <= 0;
  // 24 小时内进入告警色
  const isUrgent = showCountdown && !isExpired && remainingMs < 24 * 60 * 60 * 1000;

  return (
    <tr className={`text-[13px] hover:bg-muted ${isExpired ? "opacity-60" : ""}`}>
      <td className={`px-4 py-3 font-mono font-semibold ${isExpired ? "line-through" : ""}`}>
        <div className="flex items-center gap-2 flex-wrap">
          {item.planCode}
          <AccountChip accountId={item.accountId} />
          <TimingChip totalMs={item.totalMs} phases={item.timing} />
        </div>
      </td>
      <td className={`px-4 py-3 ${isExpired ? "line-through" : ""}`}>{item.datacenter.toUpperCase()}</td>
      <td className={`px-4 py-3 text-muted-foreground max-w-[200px] truncate ${isExpired ? "line-through" : ""}`}>
        {item.options && item.options.length > 0 ? item.options.join(", ") : "默认配置"}
      </td>
      <td className="px-4 py-3">
        <HistoryPrice item={item} strike={isExpired} />
      </td>
      <td className="px-4 py-3">
        {item.status === "success" ? (
          <Chip tone={st.tone} title={st.title}>
            {st.label}
          </Chip>
        ) : (
          <Chip tone="danger">失败</Chip>
        )}
      </td>
      <td className="px-4 py-3 text-[11px] text-muted-foreground font-mono whitespace-nowrap">
        {new Date(item.purchaseTime).toLocaleString("zh-CN", {
          month: "2-digit",
          day: "2-digit",
          hour: "2-digit",
          minute: "2-digit",
        })}
      </td>
      <td className="px-4 py-3 whitespace-nowrap">
        {showCountdown ? (
          <Chip
            tone={isExpired ? "danger" : isUrgent ? "warning" : "info"}
            title={`到期截止时间: ${new Date(getExpirationMs(item)).toLocaleString("zh-CN")}`}
          >
            <Hourglass className="w-3 h-3" />
            {formatCountdown(remainingMs)}
          </Chip>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </td>
      <td className="px-4 py-3">
        <div className="flex items-center gap-2">
          {item.status === "success" && item.orderUrl ? (
            <a
              href={item.orderUrl}
              target="_blank"
              rel="noopener noreferrer"
              aria-disabled={isExpired}
              className={`inline-flex items-center gap-1 text-foreground hover:underline text-[12px] ${
                isExpired ? "pointer-events-none opacity-50" : ""
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
              错误
            </button>
          ) : (
            "—"
          )}
        </div>
      </td>
    </tr>
  );
}

/** 手机端的订单卡片渲染。跟 HistoryRow 字段一一对应,但堆叠成卡片。 */
function HistoryCard({ item, now }: { item: PurchaseHistory; now: number }) {
  const st = orderStatusView(item);
  const showCountdown = item.status === "success" && !st.paid && !st.closed;
  const remainingMs = showCountdown ? getExpirationMs(item) - now : 0;
  const isExpired = showCountdown && remainingMs <= 0;
  const isUrgent = showCountdown && !isExpired && remainingMs < 24 * 60 * 60 * 1000;

  return (
    <Card className={cn("surface-card rounded-xl border-border hover:border-border/80 transition-all duration-200 shadow-sm", isExpired && "opacity-60")}>
      <CardContent className="p-3.5 sm:p-4 space-y-2.5">
        <div className="flex items-start justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-2 flex-wrap min-w-0">
            <span className={`font-mono font-semibold text-[13px] ${isExpired ? "line-through" : ""}`}>
              {item.planCode}
            </span>
            <AccountChip accountId={item.accountId} />
            <Chip tone="default" className="text-[10px]">
              {item.datacenter.toUpperCase()}
            </Chip>
            <TimingChip totalMs={item.totalMs} phases={item.timing} />
          </div>
          {item.status === "success" ? (
            <Chip tone={st.tone} title={st.title}>
              {st.label}
            </Chip>
          ) : (
            <Chip tone="danger">失败</Chip>
          )}
        </div>
        <div className={`text-[11px] text-muted-foreground break-all ${isExpired ? "line-through" : ""}`}>
          {item.options && item.options.length > 0 ? item.options.join(", ") : "默认配置"}
        </div>
        <div className="flex items-center justify-between gap-2 text-[11px]">
          <span className="text-muted-foreground font-mono">
            {new Date(item.purchaseTime).toLocaleString("zh-CN", {
              month: "2-digit",
              day: "2-digit",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </span>
          {item.price?.withTax != null ? <HistoryPrice item={item} strike={isExpired} /> : null}
        </div>
        <div className="flex items-center justify-between gap-2 flex-wrap">
          {showCountdown ? (
            <Chip
              tone={isExpired ? "danger" : isUrgent ? "warning" : "info"}
              title={`到期截止时间: ${new Date(getExpirationMs(item)).toLocaleString("zh-CN")}`}
            >
              <Hourglass className="w-3 h-3" />
              {formatCountdown(remainingMs)}
            </Chip>
          ) : (
            <span />
          )}
          <div className="flex items-center gap-2">
            {item.status === "success" && item.orderUrl ? (
              <a
                href={item.orderUrl}
                target="_blank"
                rel="noopener noreferrer"
                className={`inline-flex items-center gap-1 text-foreground hover:underline text-[12px] ${
                  isExpired ? "pointer-events-none opacity-50" : ""
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
