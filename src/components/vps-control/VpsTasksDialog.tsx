import { ListTodo, RefreshCw } from "lucide-react";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { Chip } from "@/components/common/Chip";
import { useVpsTasks, type VpsTask } from "@/hooks/use-vps-control";

/** VPS 任务管理:显示最近 10 个任务(reboot/start/stop/reinstall/createSnapshot/revert 等)+ 状态 + 进度。
 *  打开时每 5 秒轮询一次,关闭时停止 —— 进行中的任务能实时看到进度。 */
export function VpsTasksDialog({
  serviceName,
  open,
  onOpenChange,
}: {
  serviceName: string;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const q = useVpsTasks(open ? serviceName : null);
  // refetchInterval 在 q 选项里设不上,简单做法:用户点刷新

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-2xl max-h-[90vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ListTodo className="w-5 h-5" />
            任务历史
          </DialogTitle>
          <DialogDescription>最近 10 个任务(重启 / 装系统 / 快照 / 改密 等)+ 实时状态</DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto -mx-6 px-6">
          {q.isPending ? (
            <Skeleton className="h-40 rounded-2xl" />
          ) : (q.data || []).length === 0 ? (
            <EmptyState icon={ListTodo} title="暂无任务历史" />
          ) : (
            <div className="space-y-2 py-1">
              {(q.data || []).slice().reverse().map((t) => (
                <TaskRow key={t.id} task={t} />
              ))}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" size="sm" onClick={() => q.refetch()} disabled={q.isFetching}>
            <RefreshCw className={"w-3.5 h-3.5 mr-1" + (q.isFetching ? " animate-spin" : "")} />
            刷新
          </Button>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            关闭
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TaskRow({ task }: { task: VpsTask }) {
  const tone = taskTone(task.state);
  return (
    <div className="border border-border rounded-xl p-3 space-y-1.5">
      <div className="flex items-center gap-2 flex-wrap">
        <code className="text-[11px] font-mono text-muted-foreground">#{task.id}</code>
        <span className="text-[13px] font-semibold">{translateTaskType(task.type)}</span>
        <Chip tone={tone}>{translateTaskState(task.state)}</Chip>
        {task.progress > 0 && task.progress < 100 && (
          <span className="text-[11px] text-muted-foreground">{task.progress}%</span>
        )}
        <span className="ml-auto text-[11px] text-muted-foreground">
          {task.date ? new Date(task.date).toLocaleString("zh-CN") : "—"}
        </span>
      </div>
      {/* 进度条 */}
      {task.state === "doing" && (
        <div className="h-1 bg-secondary rounded overflow-hidden">
          <div
            className="h-full bg-primary transition-all"
            style={{ width: `${Math.max(5, task.progress)}%` }}
          />
        </div>
      )}
    </div>
  );
}

function taskTone(state: string): "default" | "success" | "warning" | "danger" | "info" {
  switch (state.toLowerCase()) {
    case "done":
      return "success";
    case "doing":
    case "todo":
    case "waitingack":
      return "warning";
    case "cancelled":
    case "error":
    case "blocked":
      return "danger";
    case "paused":
      return "info";
    default:
      return "default";
  }
}

/** OVH vps.TaskStateEnum: blocked / cancelled / doing / done / error / paused / todo / waitingAck */
function translateTaskState(s: string): string {
  return ({
    blocked: "已阻塞",
    cancelled: "已取消",
    doing: "进行中",
    done: "完成",
    error: "失败",
    paused: "已暂停",
    todo: "排队中",
    waitingack: "待确认",
  } as Record<string, string>)[s.toLowerCase()] || s;
}

/** OVH vps.TaskTypeEnum 全集 —— 注意 OVH 命名大多带 Vm 后缀(rebootVm 不是 reboot) */
function translateTaskType(t: string): string {
  return ({
    addVeeamBackupJob: "添加 Veeam 备份",
    changeRootPassword: "重置 root 密码",
    createSnapshot: "创建快照",
    deleteSnapshot: "删除快照",
    deliverVm: "交付 VM",
    getConsoleUrl: "生成控制台链接",
    internalTask: "内部任务",
    migrate: "迁移",
    openConsoleAccess: "打开控制台",
    provisioningAdditionalIp: "分配额外 IP",
    reOpenVm: "重新开机",
    rebootVm: "重启",
    reinstallVm: "重装系统",
    removeVeeamBackup: "移除 Veeam 备份",
    rescheduleAutoBackup: "调整自动备份",
    restoreFullVeeamBackup: "Veeam 完整还原",
    restoreVeeamBackup: "Veeam 还原",
    restoreVm: "还原 VM",
    revertSnapshot: "回滚快照",
    setMonitoring: "设置监控",
    setNetboot: "设置网络启动",
    startVm: "启动",
    stopVm: "关机",
    upgradeVm: "升级 VM",
  } as Record<string, string>)[t] || t;
}
