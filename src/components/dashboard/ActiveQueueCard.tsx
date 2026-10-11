import { Link } from "react-router-dom";
import { ClipboardList, ChevronRight, Calendar, Plus, Clock } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import type { QueueItem } from "@/hooks/use-queue";

export function QueueStatusChip({ status }: { status: string }) {
  if (status === "running")
    return (
      <Chip tone="success">
        <StatusDot tone="success" pulse size="xs" />
        运行中
      </Chip>
    );
  if (status === "pending")
    return (
      <Chip tone="warning">
        <StatusDot tone="warning" size="xs" />
        等待中
      </Chip>
    );
  return (
    <Chip tone="default">
      <StatusDot tone="muted" size="xs" />
      已暂停
    </Chip>
  );
}

interface ActiveQueueCardProps {
  activeQueue: QueueItem[];
  totalQueueCount: number;
  isPending: boolean;
}

export function ActiveQueueCard({
  activeQueue,
  totalQueueCount,
  isPending,
}: ActiveQueueCardProps) {
  return (
    <Card className="surface-card rounded-2xl overflow-hidden lg:col-span-2 border-border/80 shadow-sm">
      <CardContent className="p-3.5 sm:p-5">
        <div className="flex items-center justify-between mb-3.5">
          <div className="flex items-center gap-2">
            <ClipboardList className="w-4 h-4 text-primary" />
            <h2 className="text-[14px] sm:text-[15px] font-semibold text-foreground">活跃抢购队列</h2>
          </div>
          <Link
            to="/queue"
            className="text-xs text-muted-foreground hover:text-primary transition-colors inline-flex items-center gap-0.5 font-medium"
          >
            查看全部 ({totalQueueCount})
            <ChevronRight className="w-3.5 h-3.5" />
          </Link>
        </div>
        {isPending ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-xl" />
            ))}
          </div>
        ) : activeQueue.length === 0 ? (
          <EmptyState
            icon={Calendar}
            title="暂无活跃抢购任务"
            action={
              <Button asChild size="sm" className="h-8 text-xs font-medium rounded-lg gap-1.5">
                <Link to="/queue">
                  <Plus className="w-3.5 h-3.5" />
                  创建抢购任务
                </Link>
              </Button>
            }
          />
        ) : (
          <div className="space-y-2.5">
            {activeQueue.map((q) => (
              <div
                key={q.id}
                className="flex items-center justify-between gap-3 rounded-xl px-3.5 py-2.5 bg-secondary/30 border border-border/70 hover:border-primary/40 transition-colors"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-xs sm:text-sm font-mono text-foreground">{q.planCode}</span>
                    <span className="text-[10px] font-mono uppercase bg-primary/10 text-primary border border-primary/20 px-1.5 py-0.5 rounded leading-none">
                      {q.datacenter}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 text-[11px] text-muted-foreground mt-1">
                    <span className="inline-flex items-center gap-1">
                      <Clock className="w-3 h-3 text-muted-foreground/70" />
                      已尝试 {q.retryCount + 1} 次
                    </span>
                    {q.retryInterval && (
                      <>
                        <span className="text-muted-foreground/40">·</span>
                        <span>间隔 {q.retryInterval}s</span>
                      </>
                    )}
                  </div>
                </div>
                <QueueStatusChip status={q.status} />
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
