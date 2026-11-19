import { Clock, PauseCircle, PlayCircle, X, Pencil } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { AccountChip } from "@/components/common/AccountChip";
import { IntervalEditor } from "./IntervalEditor";
import { describeOptionCodes } from "@/lib/option-groups";
import type { QueueItem } from "@/hooks/use-queue";
import { OrderDurationBadge } from "@/components/orders/OrderDurationField";
import { SelectionCheckbox } from "@/components/common/BulkCleanup";

interface QueueRowProps {
  item: QueueItem;
  onToggle: () => void;
  onDelete: () => void;
  onEdit?: () => void;
  selected?: boolean;
  selectionDisabled?: boolean;
  onSelect?: (shiftKey: boolean) => void;
}

export function QueueRow({ item, onToggle, onDelete, onEdit, selected, selectionDisabled, onSelect }: QueueRowProps) {
  const chip = (() => {
    if (item.status === "running")
      return (
        <Chip tone="success">
          <StatusDot tone="success" pulse size="xs" />运行中
        </Chip>
      );
    if (item.status === "pending")
      return (
        <Chip tone="warning">
          <StatusDot tone="warning" size="xs" />等待中
        </Chip>
      );
    if (item.status === "paused")
      return (
        <Chip tone="default">
          <StatusDot tone="muted" size="xs" />已暂停
        </Chip>
      );
    if (item.status === "completed")
      return (
        <Chip tone="info">
          <StatusDot tone="info" size="xs" />已完成
        </Chip>
      );
    return (
      <Chip tone="danger">
        <StatusDot tone="danger" size="xs" />失败
      </Chip>
    );
  })();

  const hasUncertain = Boolean(item.uncertainCheckout || item.stopReason === "checkoutUncertain");
  const hasSecondaryTags = Boolean(
    hasUncertain ||
    (item.status === "paused" && (item.stopReason === "monitorPaused" || item.stopReason === "proxyFailed")) ||
    item.preorder ||
    item.coupon ||
    item.fromTelegram ||
    item.quickOrder ||
    (item.duration && item.duration !== "P1M") ||
    (item.options && item.options.length > 0)
  );

  return (
    <Card
      data-selection-key={item.id}
      className={`surface-card rounded-xl border-border hover:border-border/90 transition-colors ${selected ? "border-primary/70 bg-primary/5 ring-1 ring-primary/25" : ""}`}
    >
      <CardContent className="p-3.5 sm:p-4 flex flex-col sm:flex-row sm:items-center gap-3">
        <div className="flex-1 min-w-0 space-y-1.5">
          {/* 第一层：核心标识（复选框 + 型号 + 机房 + 名称 + 账户） */}
          <div className="flex items-center gap-2 flex-wrap">
            {onSelect && (
              <SelectionCheckbox
                checked={Boolean(selected)}
                disabled={selectionDisabled}
                onToggle={onSelect}
                ariaLabel={`选择队列任务 ${item.planCode} ${item.id}`}
              />
            )}
            <span className="font-mono font-bold text-sm text-foreground tracking-tight">{item.planCode}</span>
            {item.datacenter && (
              <span className="px-1.5 py-0.5 rounded-md bg-primary/10 text-primary border border-primary/20 font-mono text-[10.5px] font-semibold uppercase leading-none">
                {item.datacenter}
              </span>
            )}
            {item.planName && item.planName !== item.planCode && (
              <span className="text-xs text-muted-foreground truncate max-w-[220px]" title={item.planName}>
                {item.planName}
              </span>
            )}
            <AccountChip accountId={item.accountId} />
          </div>

          {/* 第二层：配置与状态标记（有内容才渲染，避免与型号挤成一团） */}
          {hasSecondaryTags && (
            <div className="flex items-center gap-1.5 flex-wrap">
              {hasUncertain && (
                <Chip tone="danger" title="结账超时，无法确认订单是否已创建">结果待核对</Chip>
              )}
              {item.status === "paused" && item.stopReason === "monitorPaused" && (
                <Chip tone="warning" title="因对应监控订阅暂停而被系统连带暂停">监控已暂停</Chip>
              )}
              {item.status === "paused" && item.stopReason === "proxyFailed" && (
                <Chip tone="danger" title="因账户出站代理连续出错而被系统自动暂停">代理故障暂停</Chip>
              )}
              {item.preorder && (
                <Chip tone="warning" title="预订抢购：按型号、内存、完整硬盘和机房严格匹配，OVH 下单接口开放后自动下单；等待不计失败次数">预订抢购</Chip>
              )}
              <OrderDurationBadge value={item.duration} />
              {item.options && item.options.length > 0 && (
                <Chip tone="default" title={item.options.join("\n")}>
                  {describeOptionCodes(item.options)}
                </Chip>
              )}
              {item.coupon && (
                <Chip tone="default" title={`优惠码: ${item.coupon}`}>优惠码 {item.coupon}</Chip>
              )}
              {item.fromTelegram && (
                <Chip tone="default">TG 任务</Chip>
              )}
              {item.quickOrder && (
                <Chip tone="default">极速抢购</Chip>
              )}
              {/* 自动付款已停用：{item.autoPay && (
                <Chip tone="default">自动扣款</Chip>
              )} */}
            </div>
          )}

          {/* 第三层：执行节奏与创建时间 */}
          <div className="text-[11px] text-muted-foreground flex items-center gap-2 flex-wrap tabular-nums">
            <Clock className="w-3.5 h-3.5 text-muted-foreground/75 shrink-0" />
            {item.status === "failed" ? (
              <span>已停止重试（原因见抢购历史）</span>
            ) : item.status === "completed" ? (
              <span>已下单{item.orderId ? `（订单 #${item.orderId}）` : ""}</span>
            ) : (
              <span className="inline-flex items-center gap-1">
                下次尝试
                {item.retryCount > 0 ? (
                  <span data-no-drag-select className="inline-flex items-center gap-1">
                    <IntervalEditor id={item.id} value={item.retryInterval} />
                    秒后（第 {item.retryCount + 1} 次）
                  </span>
                ) : (
                  "即将开始"
                )}
              </span>
            )}
            {item.preorder && item.status === "running" && item.retryInterval > 15 && (
              <span>（预订最长每 15 秒检查一次）</span>
            )}
            <span className="text-border">·</span>
            <span className="font-mono">{new Date(item.createdAt).toLocaleString()}</span>
          </div>

          {hasUncertain && (
            <p
              role="alert"
              data-no-drag-select
              className="mt-1.5 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[11px] leading-relaxed text-destructive"
            >
              上次结账超时，无法确认订单是否已创建
              {item.uncertainTime ? `（${new Date(item.uncertainTime).toLocaleString()}）` : ""}。
              请先到 OVH 订单页查看是否已有未付订单：有就不用恢复；没有再恢复运行——恢复时会先自动核对订单，不会直接再下一单。
            </p>
          )}
        </div>
        <div className="flex items-center justify-between sm:justify-end gap-2.5 flex-shrink-0 w-full sm:w-auto pt-2.5 sm:pt-0 border-t sm:border-t-0 border-border/50">
          {chip}
          <div className="flex items-center gap-1">
            {onEdit && item.status !== "completed" && (
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-secondary rounded-lg"
                onClick={onEdit}
                title="编辑任务"
                aria-label="编辑任务"
              >
                <Pencil className="w-3.5 h-3.5" />
              </Button>
            )}
            {item.status !== "completed" && (
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-secondary rounded-lg"
                onClick={onToggle}
                title={item.status === "running" ? "暂停" : item.status === "failed" ? "重新开始" : "恢复"}
                aria-label={item.status === "running" ? "暂停" : item.status === "failed" ? "重新开始" : "恢复"}
              >
                {item.status === "running" ? <PauseCircle className="w-4 h-4 text-amber-400" /> : <PlayCircle className="w-4 h-4 text-emerald-400" />}
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-lg"
              onClick={onDelete}
              disabled={selectionDisabled}
              aria-label="删除"
            >
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
