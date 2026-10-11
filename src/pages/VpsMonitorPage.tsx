import { useState } from "react";
import { Helmet } from "react-helmet-async";
import { MonitorDot, Cloud, RefreshCw, Trash2, Plus } from "lucide-react";
import { toast } from "sonner";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/common/EmptyState";
import { Skeleton } from "@/components/common/Skeleton";
import { LoadFailed } from "@/components/common/LoadFailed";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import {
  useVPSMonitorList,
  useVPSMonitorStatus,
  useToggleVPSMonitor,
  useRemoveVPSSubscription,
  useClearVPSMonitor,
  type VPSSubscription,
} from "@/hooks/use-vps-monitor";
import { VPSMonitorHeaderCard } from "@/components/vps-monitor/VPSMonitorHeaderCard";
import { VPSRow } from "@/components/vps-monitor/VPSRow";
import { AddVPSDialog } from "@/components/vps-monitor/AddVPSDialog";
import { modelLabel } from "@/components/vps-monitor/types";

/** VPS 补货通知 */
function VPSMonitorPage() {
  const list = useVPSMonitorList();
  const status = useVPSMonitorStatus();
  const toggle = useToggleVPSMonitor();
  const remove = useRemoveVPSSubscription();
  const clear = useClearVPSMonitor();

  const [confirmClear, setConfirmClear] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState<VPSSubscription | null>(null);
  const [openAdd, setOpenAdd] = useState(false);
  const [editSub, setEditSub] = useState<VPSSubscription | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await Promise.all([list.refetch(), status.refetch()]);
      toast.success("已刷新 VPS 监控与订阅状态");
    } finally {
      setTimeout(() => setIsRefreshing(false), 450);
    }
  };

  const subs = list.data || [];
  const running = !!status.data?.running;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={MonitorDot}
        title="VPS 补货通知"
        description="选择 VPS 型号，自动监控所有数据中心的库存变化"
        action={
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              className="h-8 text-xs gap-1.5 px-3 font-medium rounded-lg shadow-sm"
              onClick={() => setOpenAdd(true)}
            >
              <Plus className="w-3.5 h-3.5" />
              <span>添加订阅</span>
            </Button>
            <Button
              size="sm"
              className="h-8 text-xs gap-1.5 font-medium rounded-lg shadow-sm px-3"
              onClick={handleRefresh}
              disabled={isRefreshing || list.isFetching || status.isFetching}
              title="刷新订阅与库存状态"
            >
              <RefreshCw
                className={cn(
                  "w-3.5 h-3.5",
                  (isRefreshing || list.isFetching || status.isFetching) && "animate-spin"
                )}
              />
              <span>刷新</span>
            </Button>
          </div>
        }
      />

      {/* 监控状态卡片：集成启停控制与运行统计 */}
      <VPSMonitorHeaderCard
        running={running}
        subscriptionsCount={status.data?.subscriptions_count ?? 0}
        checkInterval={status.data?.check_interval ?? 0}
        onToggle={(curr) => toggle.mutate(curr)}
        isToggling={toggle.isPending}
      />

      {list.isPending ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-xl" />
          ))}
        </div>
      ) : list.isError ? (
        <Card className="surface-card rounded-xl border-border">
          <LoadFailed
            icon={Cloud}
            title="VPS 监控订阅列表读取失败"
            error={list.error}
            onRetry={() => list.refetch()}
          />
        </Card>
      ) : subs.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState
            icon={Cloud}
            title="暂无 VPS 订阅"
            description='点击"添加订阅"按钮，选择 VPS 型号开始监控'
          />
        </Card>
      ) : (
        <div className="space-y-3">
          {/* 订阅列表头部栏 */}
          <div className="flex items-center justify-between text-xs px-1">
            <div className="text-muted-foreground font-medium flex items-center gap-1.5">
              <span>已订阅型号</span>
              <span className="text-[11px] px-1.5 py-0.2 rounded-full bg-secondary text-foreground font-mono font-medium">
                {subs.length}
              </span>
            </div>
            <Button
              variant="soft-destructive"
              size="sm"
              className="h-7 text-xs gap-1 px-2.5 rounded-lg"
              onClick={() => setConfirmClear(true)}
              title="清空全部 VPS 订阅"
            >
              <Trash2 className="w-3.5 h-3.5" />
              <span>清空所有订阅</span>
            </Button>
          </div>

          {subs.map((s) => (
            <VPSRow
              key={s.id}
              sub={s}
              expanded={expanded === s.id}
              onToggleExpand={() => setExpanded((c) => (c === s.id ? null : s.id))}
              onEdit={() => setEditSub(s)}
              onDelete={() => setConfirmRemove(s)}
            />
          ))}
        </div>
      )}

      <AddVPSDialog open={openAdd} onOpenChange={setOpenAdd} />
      <AddVPSDialog
        open={!!editSub}
        onOpenChange={(v) => !v && setEditSub(null)}
        initial={editSub}
      />

      {/* 删除确认 */}
      <Dialog open={!!confirmRemove} onOpenChange={(v) => !v && setConfirmRemove(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>取消订阅</DialogTitle>
            <DialogDescription>
              确定要取消订阅{" "}
              <span className="font-mono">{confirmRemove && modelLabel(confirmRemove.planCode)}</span>{" "}
              吗？
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmRemove(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (confirmRemove) remove.mutate(confirmRemove.id);
                setConfirmRemove(null);
              }}
            >
              确定
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 清空确认 */}
      <Dialog open={confirmClear} onOpenChange={setConfirmClear}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认清空所有 VPS 订阅？</DialogTitle>
            <DialogDescription>此操作不可撤销。</DialogDescription>
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

const Page = () => (
  <>
    <Helmet>
      <title>VPS 监控 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <VPSMonitorPage />
    </AppLayout>
  </>
);

export default Page;
