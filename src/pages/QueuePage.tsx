import { useState, useMemo, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
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
import { cn } from "@/lib/utils";

function QueuePage() {
  const queue = useQueueList();
  const toggle = useToggleQueueItem();
  const remove = useRemoveQueueItem();
  const clear = useClearQueue();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const createPlanCode = searchParams.get("create") || undefined;
  const createOptions = searchParams.get("options") || undefined;
  const [showClearDialog, setShowClearDialog] = useState(false);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingItem, setEditingItem] = useState<QueueItem | null>(null);
  const [prefillPlanCode, setPrefillPlanCode] = useState<string>("");
  const [prefillOptions, setPrefillOptions] = useState<string>("");

  useEffect(() => {
    if (createPlanCode) {
      setPrefillPlanCode(createPlanCode);
      setPrefillOptions(createOptions || "");
      setShowCreateDialog(true);
    }
  }, [createPlanCode, createOptions]);

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  const rawQueue = queue.data;
  const items = useMemo(() => rawQueue || [], [rawQueue]);
  const runningCount = items.filter((it) => it.status === "running").length;
  const pausedCount = items.filter((it) => it.status === "paused").length;
  const failedCount = items.filter((it) => it.status === "failed").length;

  const handlePauseAll = async () => {
    const runningItems = items.filter((it) => it.status === "running");
    if (runningItems.length === 0) return;
    for (const it of runningItems) {
      await toggle.mutateAsync({ id: it.id, action: "pause" });
    }
    toast.success(`已暂停 ${runningItems.length} 个任务`);
  };

  const handleResumeAll = async () => {
    const pausedItems = items.filter((it) => it.status === "paused");
    if (pausedItems.length === 0) return;
    for (const it of pausedItems) {
      await toggle.mutateAsync({ id: it.id, action: "resume" });
    }
    toast.success(`已恢复 ${pausedItems.length} 个任务`);
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

  return (
    <div className="space-y-6">
      <PageHeader
        icon={ListOrdered}
        title="抢购队列"
        description="管理自动抢购服务器的队列"
        action={
          <div className="flex items-center gap-2">
            <Button
              onClick={() => setShowCreateDialog(true)}
              size="sm"
              className="h-8 text-xs font-medium gap-1.5 px-3 rounded-lg shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>新建抢购</span>
            </Button>
            <Button
              size="sm"
              className="h-8 text-xs font-medium gap-1.5 rounded-lg shadow-sm px-3"
              onClick={() => queue.refetch()}
              disabled={queue.isFetching}
              title="刷新队列数据"
            >
              <RefreshCw className={cn("w-3.5 h-3.5", queue.isFetching && "animate-spin")} />
              <span>刷新</span>
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
        />
      )}

      {items.length > 0 && (
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs px-1">
          <div className="text-muted-foreground font-medium flex items-center gap-1.5">
            <span>队列任务</span>
            <span className="text-[11px] px-1.5 py-0.2 rounded-full bg-secondary text-foreground font-mono font-medium">
              {filteredItems.length}/{items.length}
            </span>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
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
            <Button
              variant="soft-destructive"
              size="sm"
              className="h-7 text-xs gap-1 px-2.5 rounded-lg"
              onClick={() => setShowClearDialog(true)}
              title="清空所有队列任务"
            >
              <Trash2 className="w-3.5 h-3.5" />
              <span>清空队列</span>
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
        <div className="space-y-3">
          {filteredItems.map((q) => (
            <QueueRow
              key={q.id}
              item={q}
              onToggle={() =>
                toggle.mutate({
                  id: q.id,
                  action: q.status === "running" ? "pause" : "resume",
                })
              }
              onDelete={() => remove.mutate(q.id)}
              onEdit={() => setEditingItem(q)}
            />
          ))}
        </div>
      )}

      <Dialog open={showClearDialog} onOpenChange={setShowClearDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确认清空队列？</DialogTitle>
            <DialogDescription>所有任务将被删除，此操作不可撤销。</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowClearDialog(false)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                clear.mutate();
                setShowClearDialog(false);
              }}
            >
              确认清空
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
            navigate("/queue", { replace: true });
          }
        }}
        initialPlanCode={prefillPlanCode}
        initialOptions={prefillOptions}
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
