import { Clock, PauseCircle, PlayCircle, X, Pencil } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { AccountChip } from "@/components/common/AccountChip";
import { IntervalEditor } from "./IntervalEditor";
import { describeOptionCodes } from "@/lib/option-groups";
import type { QueueItem } from "@/hooks/use-queue";

interface QueueRowProps {
  item: QueueItem;
  onToggle: () => void;
  onDelete: () => void;
  onEdit?: () => void;
}

export function QueueRow({ item, onToggle, onDelete, onEdit }: QueueRowProps) {
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

  return (
    <Card className="surface-card rounded-xl border-border hover:border-border/80 transition-all duration-200 shadow-sm">
      <CardContent className="p-3.5 sm:p-5 flex flex-col sm:flex-row sm:items-center gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1.5 flex-wrap">
            <span className="font-mono font-semibold text-sm text-foreground tracking-tight">{item.planCode}</span>
            <AccountChip accountId={item.accountId} />
            {item.preorder && (
              <Chip tone="warning" title="预订抢购：等待官方目录收录后自动下单">预订抢购</Chip>
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
            {item.autoPay && (
              <Chip tone="default">自动扣款</Chip>
            )}
            {item.options && item.options.length > 0 && (
              <Chip tone="default" title={item.options.join("\n")}>
                {describeOptionCodes(item.options)}
              </Chip>
            )}
          </div>
          <div className="text-[11px] text-muted-foreground flex items-center gap-2 flex-wrap">
            <Clock className="w-3.5 h-3.5 text-muted-foreground/80" />
            {item.status === "failed" ? (
              <span>已停止重试（原因见抢购历史）</span>
            ) : item.status === "completed" ? (
              <span>已下单{item.orderId ? `（订单 #${item.orderId}）` : ""}</span>
            ) : (
              <span className="inline-flex items-center gap-1">
                下次尝试
                {item.retryCount > 0 ? (
                  <>
                    <IntervalEditor id={item.id} value={item.retryInterval} />
                    秒后（第 {item.retryCount + 1} 次）
                  </>
                ) : (
                  "即将开始"
                )}
              </span>
            )}
            <span>·</span>
            <span>{new Date(item.createdAt).toLocaleString()}</span>
          </div>
        </div>
        <div className="flex items-center justify-between sm:justify-end gap-2 flex-shrink-0 w-full sm:w-auto pt-2 sm:pt-0 border-t sm:border-t-0 border-border/40">
          {chip}
          <div className="flex items-center gap-1">
            {onEdit && (item.status === "paused" || item.status === "failed") && (
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
            {item.status !== "completed" && item.status !== "failed" && (
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-muted-foreground hover:text-foreground hover:bg-secondary rounded-lg"
                onClick={onToggle}
                aria-label={item.status === "running" ? "暂停" : "恢复"}
              >
                {item.status === "running" ? <PauseCircle className="w-4 h-4 text-amber-400" /> : <PlayCircle className="w-4 h-4 text-emerald-400" />}
              </Button>
            )}
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-destructive hover:bg-destructive/10 rounded-lg"
              onClick={onDelete}
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
