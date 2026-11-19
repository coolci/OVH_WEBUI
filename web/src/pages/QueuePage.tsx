import { useState, useMemo, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  ListOrdered, RefreshCw, Trash2, PauseCircle, PlayCircle, Plus, Search,
} from "lucide-react";
import { Helmet } from "react-helmet-async";
import { toast } from "sonner";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { LoadFailed } from "@/components/common/LoadFailed";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { QueueFilterBar } from "@/components/queue/QueueFilterBar";
import { QueueRow } from "@/components/queue/QueueRow";
import { CreateQueueDialog } from "@/components/queue/CreateQueueDialog";
import { EditQueueDialog } from "@/components/queue/EditQueueDialog";
import {
  useQueueList,
  useToggleQueueItem,
  useRemoveQueueItem,
  useClearQueue,
  type QueueItem,
} from "@/hooks/use-queue";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";
import { cn } from "@/lib/utils";
import {
  BulkCleanupDialog,
  BulkSelectAllControl,
  BulkSelectionDock,
} from "@/components/common/BulkCleanup";
import { useBulkCleanup } from "@/hooks/use-bulk-cleanup";
import { cleanupTargets, queueCleanupTarget } from "@/lib/bulk-cleanup";
import { DragSelectArea } from "@/components/common/DragSelectArea";

function QueuePage() {
  const qc = useQueryClient();
  const queue = useQueueList();
  const toggle = useToggleQueueItem();
  const remove = useRemoveQueueItem();
  const clear = useClearQueue();
  const [clearingStatus, setClearingStatus] = useState(false);
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const createPlanCode = searchParams.get("create") || undefined;
  const createOptions = searchParams.get("options") || undefined;
  const createAccount = searchParams.get("account") || undefined;
  const createDatacenter = searchParams.get("dc") || undefined;
  const createDuration = searchParams.get("duration") || undefined;
  const [clearScope, setClearScope] = useState<"all" | "completed" | "failed" | null>(null);
  const [confirmResumeItem, setConfirmResumeItem] = useState<QueueItem | null>(null);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingItem, setEditingItem] = useState<QueueItem | null>(null);
  const [prefillPlanCode, setPrefillPlanCode] = useState<string>("");
  const [prefillOptions, setPrefillOptions] = useState<string>("");
  const [prefillAccountId, setPrefillAccountId] = useState<string>("");
  const [prefillDatacenter, setPrefillDatacenter] = useState<string>("");
  const [prefillDuration, setPrefillDuration] = useState<string>("");

  useEffect(() => {
    if (createPlanCode) {
      setPrefillPlanCode(createPlanCode);
      setPrefillOptions(createOptions || "");
      setPrefillAccountId(createAccount || "");
      setPrefillDatacenter(createDatacenter || "");
      setPrefillDuration(createDuration || "");
      setShowCreateDialog(true);
    }
  }, [createPlanCode, createOptions, createAccount, createDatacenter, createDuration]);

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  const rawQueue = queue.data;
  const items = useMemo(() => rawQueue || [], [rawQueue]);
  const runningCount = items.filter((it) => it.status === "running").length;
  const pausedCount = items.filter((it) => it.status === "paused").length;
  const failedCount = items.filter((it) => it.status === "failed").length;
  // 已下单任务现在保留在队列里作为只读记录(后端不会再调度它们)
  const completedCount = items.filter((it) => it.status === "completed").length;

  const handlePauseAll = async () => {
    const runningItems = items.filter((it) => it.status === "running");
    if (runningItems.length === 0) return;
    let okCount = 0;
    for (const it of runningItems) {
      try {
        await toggle.mutateAsync({ id: it.id, action: "pause" });
        okCount++;
      } catch {
        // 单个失败由 useToggleQueueItem.onError 提示,继续暂停其余任务
      }
    }
    if (okCount > 0) {
      toast.success(`已暂停 ${okCount} 个任务`);
    }
  };

  const handleResumeAll = async () => {
    const pausedItems = items.filter((it) => it.status === "paused");
    if (pausedItems.length === 0) return;
    const safeItems = pausedItems.filter((it) => !it.uncertainCheckout && it.stopReason !== "checkoutUncertain");
    const skippedUncertain = pausedItems.length - safeItems.length;
    let okCount = 0;
    for (const it of safeItems) {
      try {
        await toggle.mutateAsync({ id: it.id, action: "resume" });
        okCount++;
      } catch {
        // 单个失败由 useToggleQueueItem.onError 提示,继续恢复其余任务
      }
    }
    if (okCount > 0) {
      toast.success(`已恢复 ${okCount} 个任务`);
    }
    if (skippedUncertain > 0) {
      toast.warning(`已跳过 ${skippedUncertain} 个「结账结果待核对」的任务，请核对 OVH 订单后逐条手动恢复`);
    }
  };

  const handleToggleItem = (q: QueueItem) => {
    if (q.status === "running" || q.status === "pending") {
      toggle.mutate({ id: q.id, action: "pause" });
      return;
    }
    if (q.uncertainCheckout || q.stopReason === "checkoutUncertain") {
      setConfirmResumeItem(q);
      return;
    }
    toggle.mutate({ id: q.id, action: "resume" });
  };

  const handleClearByStatus = async (status: "completed" | "failed") => {
    const targets = items.filter((it) => it.status === status).map(queueCleanupTarget);
    if (targets.length === 0) return;
    setClearingStatus(true);
    try {
      const { succeeded, failed } = await cleanupTargets(targets, (target) => api.delete(target.path));
      if (succeeded.length > 0) {
        toast.success(`已清理 ${succeeded.length} 个${status === "completed" ? "已下单" : "失败"}任务`);
      }
      if (failed.length > 0) {
        toast.error(`${failed.length} 个任务清理失败，请重试`);
      }
    } finally {
      setClearingStatus(false);
      qc.invalidateQueries({ queryKey: qk.queue.list() });
      qc.invalidateQueries({ queryKey: qk.stats() });
    }
  };

  const filteredItems = useMemo(() => {
    let out = items;
    const s = search.trim().toLowerCase();
    if (s) {
      out = out.filter(
        (it) =>
          it.planCode.toLowerCase().includes(s) ||
          it.datacenter.toLowerCase().includes(s) ||
          it.accountId.toLowerCase().includes(s)
      );
    }
    if (statusFilter !== "all") {
      out = out.filter((it) => it.status === statusFilter);
    }
    return out;
  }, [items, search, statusFilter]);
  const cleanup = useBulkCleanup(filteredItems, queueCleanupTarget, "queue");

  const selectionDisabled = cleanup.locked || queue.isError || queue.isPending || clear.isPending || clearingStatus;

  return (
    <div className="space-y-5">
      <PageHeader
        icon={ListOrdered}
        title="抢购队列"
        description="管理自动抢购服务器的队列"
        action={
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="h-8 text-xs font-medium gap-1.5 rounded-lg border-border/80 hover:bg-secondary px-3"
              onClick={() => queue.refetch()}
              disabled={queue.isFetching}
              title="刷新队列数据"
            >
              <RefreshCw className={cn("w-3.5 h-3.5", queue.isFetching && "animate-spin")} />
              <span>刷新</span>
            </Button>
            <Button
              onClick={() => setShowCreateDialog(true)}
              size="sm"
              className="h-8 text-xs font-medium gap-1.5 px-3 rounded-lg shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>新建抢购</span>
            </Button>
          </div>
        }
      />

      {items.length > 0 && (
        <QueueFilterBar
          search={search}
          onSearchChange={setSearch}
          statusFilter={statusFilter}
          onStatusFilterChange={setStatusFilter}
          totalCount={items.length}
          runningCount={runningCount}
          pausedCount={pausedCount}
          failedCount={failedCount}
          completedCount={completedCount}
        />
      )}

      {items.length > 0 && (
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs px-0.5">
          <div className="flex items-center gap-3 flex-wrap">
            {filteredItems.length > 0 && (
              <BulkSelectAllControl
                totalCount={cleanup.totalCount}
                selectedCount={cleanup.selectedCount}
                allSelected={cleanup.allSelected}
                someSelected={cleanup.someSelected}
                disabled={selectionDisabled}
                onToggleAll={cleanup.toggleAll}
              />
            )}
            <div className="text-muted-foreground font-medium flex items-center gap-1.5">
              <span>队列任务</span>
              <span className="text-[11px] px-2 py-0.5 leading-none rounded-full bg-secondary text-foreground font-mono tabular-nums font-medium">
                {filteredItems.length}/{items.length}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-1.5 flex-wrap">
            {runningCount > 0 && (
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1.5 px-2.5 rounded-lg border-border/80 hover:bg-secondary font-medium"
                onClick={handlePauseAll}
                disabled={toggle.isPending}
                title="暂停所有运行中的任务"
              >
                <PauseCircle className="w-3.5 h-3.5 text-warning" />
                <span>全部暂停 ({runningCount})</span>
              </Button>
            )}
            {pausedCount > 0 && (
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1.5 px-2.5 rounded-lg border-border/80 hover:bg-secondary font-medium"
                onClick={handleResumeAll}
                disabled={toggle.isPending}
                title="恢复所有已暂停的任务"
              >
                <PlayCircle className="w-3.5 h-3.5 text-primary" />
                <span>全部恢复 ({pausedCount})</span>
              </Button>
            )}
            {(runningCount > 0 || pausedCount > 0) && (
              <span className="hidden sm:block h-4 w-px bg-border/70 mx-0.5" aria-hidden />
            )}
            {completedCount > 0 && (
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1 px-2.5 rounded-lg border-border/80 hover:bg-secondary font-medium"
                onClick={() => setClearScope("completed")}
                disabled={selectionDisabled || remove.isPending}
                title="仅清空已下单的任务，不影响运行中与已暂停的任务"
              >
                <Trash2 className="w-3.5 h-3.5 text-muted-foreground" />
                <span>清理已下单 ({completedCount})</span>
              </Button>
            )}
            {failedCount > 0 && (
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1 px-2.5 rounded-lg border-border/80 hover:bg-secondary font-medium"
                onClick={() => setClearScope("failed")}
                disabled={selectionDisabled || remove.isPending}
                title="仅清空失败任务，不影响运行中与已暂停的任务"
              >
                <Trash2 className="w-3.5 h-3.5 text-muted-foreground" />
                <span>清理失败 ({failedCount})</span>
              </Button>
            )}
            <Button
              variant="soft-destructive"
              size="sm"
              className="h-7 text-xs gap-1 px-2.5 rounded-lg"
              onClick={() => setClearScope("all")}
              disabled={selectionDisabled}
              title="清空全部队列任务（含运行中与已暂停）"
            >
              <Trash2 className="w-3.5 h-3.5" />
              <span>清空全部</span>
            </Button>
          </div>
        </div>
      )}

      {queue.isPending ? (
        <div className="space-y-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-20 rounded-xl" />
          ))}
        </div>
      ) : queue.isError ? (
        <Card className="surface-card rounded-xl border-border">
          <LoadFailed
            icon={ListOrdered}
            title="抢购队列读取失败"
            error={queue.error}
            onRetry={() => queue.refetch()}
          />
        </Card>
      ) : items.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState
            icon={ListOrdered}
            title="暂无任务"
            description="点击右上角“新建抢购任务”开始抢购"
          />
        </Card>
      ) : filteredItems.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState
            icon={Search}
            title="未找到匹配的任务"
            description="尝试调整搜索关键词或状态筛选"
          />
        </Card>
      ) : (
        <DragSelectArea
          className="space-y-3"
          selectedKeys={cleanup.selectedKeys}
          onSelectionChange={cleanup.selectKeys}
          onToggleKey={cleanup.toggleKey}
          onSelectAll={cleanup.selectAll}
          onDeleteSelected={cleanup.requestCleanup}
          disabled={selectionDisabled}
        >
          {filteredItems.map((q) => (
            <QueueRow
              key={q.id}
              item={q}
              selected={cleanup.isSelected(q)}
              onSelect={(shiftKey) => cleanup.toggle(q, shiftKey)}
              selectionDisabled={selectionDisabled}
              onToggle={() => handleToggleItem(q)}
              onDelete={() => { if (!cleanup.locked) remove.mutate(q.id); }}
              onEdit={() => setEditingItem(q)}
            />
          ))}
        </DragSelectArea>
      )}

      <BulkSelectionDock
        totalCount={cleanup.totalCount}
        selectedCount={cleanup.selectedCount}
        allSelected={cleanup.allSelected}
        disabled={selectionDisabled}
        onToggleAll={cleanup.toggleAll}
        onInvertSelection={cleanup.invertSelection}
        onClearSelection={cleanup.clearSelection}
        onCleanup={cleanup.requestCleanup}
      />
      <BulkCleanupDialog {...cleanup.dialogProps} />
      <Dialog open={clearScope !== null} onOpenChange={(v) => !v && setClearScope(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {clearScope === "completed"
                ? `确认清理 ${completedCount} 个已下单任务？`
                : clearScope === "failed"
                  ? `确认清理 ${failedCount} 个失败任务？`
                  : "确认清空全部队列任务？"}
            </DialogTitle>
            <DialogDescription>
              {clearScope === "all"
                ? "此操作会删除包括正在运行与已暂停在内的全部任务，且不可撤销。"
                : "仅删除对应状态的记录，不会影响正在运行或已暂停的抢购任务，且不会取消 OVH 已有订单。"}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setClearScope(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                const scope = clearScope;
                setClearScope(null);
                if (scope === "all") clear.mutate();
                else if (scope === "completed" || scope === "failed") void handleClearByStatus(scope);
              }}
            >
              {clearScope === "all" ? "确认清空全部" : "确认清理"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!confirmResumeItem} onOpenChange={(v) => !v && setConfirmResumeItem(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认恢复「结账结果待核对」的任务？</DialogTitle>
            <DialogDescription>
              该任务（{confirmResumeItem?.planCode} · {confirmResumeItem?.datacenter?.toUpperCase()}）上次结账响应超时，无法确认订单是否已经创建。请确认您已前往 OVH 订单页核对无重复订单后再恢复运行。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmResumeItem(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                const target = confirmResumeItem;
                setConfirmResumeItem(null);
                if (target) toggle.mutate({ id: target.id, action: "resume" });
              }}
            >
              我已核对 OVH 订单，确认恢复
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <CreateQueueDialog
        open={showCreateDialog}
        onOpenChange={(v) => {
          setShowCreateDialog(v);
          if (!v) {
            setPrefillPlanCode("");
            setPrefillOptions("");
            setPrefillAccountId("");
            setPrefillDatacenter("");
            setPrefillDuration("");
            navigate("/queue", { replace: true });
          }
        }}
        initialPlanCode={prefillPlanCode}
        initialOptions={prefillOptions}
        initialAccountId={prefillAccountId}
        initialDatacenter={prefillDatacenter}
        initialDuration={prefillDuration}
      />

      <EditQueueDialog
        open={!!editingItem}
        onOpenChange={(v) => !v && setEditingItem(null)}
        item={editingItem}
      />
    </div>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>抢购队列 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <QueuePage />
    </AppLayout>
  </>
);

export default Page;
