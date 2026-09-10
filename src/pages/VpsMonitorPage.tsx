import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { Link } from "react-router-dom";
import {
  Cloud,
  Bell,
  BellOff,
  RefreshCw,
  Trash2,
  X,
  History as HistoryIcon,
  ChevronUp,
  Plus,
  Pencil,
  AlertTriangle,
} from "lucide-react";
import { useEffect, useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { AccountSelect } from "@/components/common/AccountSelect";
import { AccountChip } from "@/components/common/AccountChip";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { EmptyState } from "@/components/common/EmptyState";
import { Skeleton } from "@/components/common/Skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useVPSMonitorList,
  useVPSMonitorStatus,
  useToggleVPSMonitor,
  useRemoveVPSSubscription,
  useClearVPSMonitor,
  useCreateVPSMonitorSubscription,
  useUpdateVPSSubscription,
  useVPSMonitorHistory,
  useVPSModels,
  type VPSSubscription,
} from "@/hooks/use-vps-monitor";
import { useTelegramVerify } from "@/hooks/use-telegram";

/** VPS 补货通知 */

const SUBSIDIARIES = [
  { value: "IE", label: "IE 爱尔兰" },
  { value: "FR", label: "FR 法国" },
  { value: "GB", label: "GB 英国" },
  { value: "DE", label: "DE 德国" },
  { value: "ES", label: "ES 西班牙" },
  { value: "IT", label: "IT 意大利" },
  { value: "PL", label: "PL 波兰" },
  { value: "CA", label: "CA 加拿大" },
  { value: "US", label: "US 美国" },
];

function modelLabel(code: string, models?: { planCode: string; name: string }[]): string {
  return models?.find((m) => m.planCode === code)?.name || code;
}

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

  const subs = list.data || [];
  const running = !!status.data?.running;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Cloud}
        title="VPS 补货通知"
        description="选择 VPS 型号，自动监控所有数据中心的库存变化"
        action={
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              className="h-8 text-xs gap-1.5 px-3 font-medium rounded-lg"
              onClick={() => setOpenAdd(true)}
            >
              <Plus className="w-3.5 h-3.5" />
              <span>添加订阅</span>
            </Button>
            <Button
              variant="outline"
              size="sm"
              className="h-8 w-8 p-0 rounded-lg border-border/80 hover:bg-secondary flex-shrink-0"
              onClick={() => list.refetch()}
              disabled={list.isFetching}
              title="刷新订阅与库存状态"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${list.isFetching ? "animate-spin" : ""}`} />
            </Button>
          </div>
        }
      />

      {/* 监控状态卡片：集成启停控制与运行统计 */}
      <Card className="surface-card rounded-xl border-border">
        <CardContent className="p-4 sm:p-5 space-y-3.5 sm:space-y-0 sm:flex sm:items-center sm:justify-between sm:gap-4">
          <div className="flex items-center justify-between sm:justify-start gap-3">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-lg bg-secondary border border-border/60 flex items-center justify-center text-foreground flex-shrink-0">
                {running ? (
                  <Bell className="w-4 h-4 text-emerald-400" />
                ) : (
                  <BellOff className="w-4 h-4 text-muted-foreground" />
                )}
              </div>
              <div>
                <div className="text-sm font-semibold text-foreground">VPS 监控状态</div>
                <div className="text-xs text-muted-foreground inline-flex items-center gap-1.5 mt-0.5">
                  <StatusDot tone={running ? "success" : "muted"} size="xs" pulse={running} />
                  <span>{running ? "轮询监控中" : "已停止"}</span>
                </div>
              </div>
            </div>

            {/* 移动端: 启停按钮置于卡片顶部右侧 */}
            <Button
              variant="outline"
              size="sm"
              className={cn(
                "sm:hidden h-8 text-xs gap-1.5 px-3 rounded-lg font-medium transition-all flex-shrink-0",
                running
                  ? "border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20"
                  : "border-border/80 hover:bg-secondary text-foreground"
              )}
              onClick={() => toggle.mutate(running)}
              disabled={toggle.isPending}
            >
              {running ? <BellOff className="w-3.5 h-3.5" /> : <Bell className="w-3.5 h-3.5" />}
              <span>{running ? "停止监控" : "启动监控"}</span>
            </Button>
          </div>

          {/* 指标与桌面端按钮 */}
          <div className="flex items-center justify-between sm:justify-end gap-6 text-sm border-t border-border/40 sm:border-t-0 pt-2.5 sm:pt-0">
            <div className="flex gap-6 text-sm">
              <Stat label="订阅数" value={status.data?.subscriptions_count ?? 0} />
              <Stat label="检查间隔" value={`${status.data?.check_interval ?? 0}s`} />
            </div>

            {/* 桌面端: 启停按钮置于卡片右侧 */}
            <Button
              variant="outline"
              size="sm"
              className={cn(
                "hidden sm:inline-flex h-8 text-xs gap-1.5 px-3 rounded-lg font-medium transition-all flex-shrink-0",
                running
                  ? "border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20"
                  : "border-border/80 hover:bg-secondary text-foreground"
              )}
              onClick={() => toggle.mutate(running)}
              disabled={toggle.isPending}
            >
              {running ? <BellOff className="w-3.5 h-3.5" /> : <Bell className="w-3.5 h-3.5" />}
              <span>{running ? "停止监控" : "启动监控"}</span>
            </Button>
          </div>
        </CardContent>
      </Card>

      {list.isPending ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-xl" />
          ))}
        </div>
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
              variant="ghost"
              size="sm"
              className="h-7 text-xs text-muted-foreground hover:text-destructive gap-1 px-2 rounded-lg"
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

/* -------------------------------- 行 / 历史 -------------------------------- */

function VPSRow({
  sub,
  expanded,
  onToggleExpand,
  onEdit,
  onDelete,
}: {
  sub: VPSSubscription;
  expanded: boolean;
  onToggleExpand: () => void;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <Card className="surface-card rounded-xl border-border hover:border-border/80 transition-all duration-200 shadow-sm">
      <CardContent className="p-4 sm:p-5">
        <div className="flex flex-col sm:flex-row sm:items-center gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1 flex-wrap">
              <span className="font-semibold text-sm">{modelLabel(sub.planCode)}</span>
              {sub.retired && <Chip tone="warning">已停售</Chip>}
              <span className="font-mono text-[11px] text-muted-foreground">{sub.planCode}</span>
              <Chip tone="default">{sub.ovhSubsidiary}</Chip>
            </div>
            <p className="text-xs text-muted-foreground mb-1.5">
              {sub.datacenters.length > 0
                ? `监控数据中心: ${sub.datacenters.join(", ")}`
                : "监控所有数据中心"}
            </p>
            <div className="flex gap-1.5 flex-wrap items-center">
              {sub.monitorLinux && <Chip tone="info">Linux</Chip>}
              {sub.monitorWindows && <Chip tone="info">Windows</Chip>}
              {sub.notifyAvailable && <Chip tone="success">有货提醒</Chip>}
              {sub.notifyUnavailable && <Chip tone="warning">无货提醒</Chip>}
              {sub.autoOrder && sub.autoOrderAccountId ? (
                <>
                  <Chip tone="solid">
                    自动下单
                    {sub.quantity && sub.quantity > 1 ? ` ×${sub.quantity}` : ""}
                  </Chip>
                  <span className="text-[11px] text-muted-foreground">→</span>
                  <AccountChip accountId={sub.autoOrderAccountId} />
                </>
              ) : sub.autoOrder ? (
                <Chip tone="warning">已勾自动下单但未选账户(只通知)</Chip>
              ) : null}
              {sub.autoPay && <Chip tone="success">自动付款</Chip>}
            </div>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
            <Button variant="ghost" size="icon" onClick={onEdit} aria-label="编辑订阅设置">
              <Pencil className="w-4 h-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              onClick={onToggleExpand}
              aria-label="查看历史"
            >
              {expanded ? (
                <ChevronUp className="w-4 h-4" />
              ) : (
                <HistoryIcon className="w-4 h-4" />
              )}
            </Button>
            <Button variant="ghost" size="icon" onClick={onDelete} aria-label="删除">
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>

        {expanded && (
          <div className="mt-4 pt-4 border-t border-border">
            <VPSHistoryPanel id={sub.id} />
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function VPSHistoryPanel({ id }: { id: string }) {
  const history = useVPSMonitorHistory(id);

  if (history.isPending) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-10 rounded-xl" />
        ))}
      </div>
    );
  }

  const entries = history.data || [];

  return (
    <div>
      <div className="flex items-center gap-2 mb-3">
        <HistoryIcon className="w-4 h-4 text-muted-foreground" />
        <span className="text-sm font-medium">变化历史</span>
      </div>
      {entries.length === 0 ? (
        <p className="text-xs text-muted-foreground text-center py-4">暂无历史记录</p>
      ) : (
        <div className="space-y-2 max-h-64 overflow-y-auto">
          {entries.map((e, i) => (
            <div
              key={i}
              className="flex items-start gap-3 p-2.5 bg-muted/40 rounded-xl text-xs"
            >
              <StatusDot
                tone={e.changeType === "available" ? "success" : "danger"}
                size="sm"
                className="mt-1"
              />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-medium">{e.datacenter}</span>
                  <Chip tone={e.changeType === "available" ? "success" : "danger"}>
                    {e.changeType === "available" ? "有货" : "无货"}
                  </Chip>
                </div>
                <p className="text-muted-foreground mt-1">{formatTime(e.timestamp)}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function formatTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(
    d.getHours()
  )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

/* ---------------------------- 添加 VPS Dialog ---------------------------- */

function AddVPSDialog({
  open,
  onOpenChange,
  initial,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  initial?: VPSSubscription | null;
}) {
  const create = useCreateVPSMonitorSubscription();
  const update = useUpdateVPSSubscription();
  const isEdit = !!initial;
  const tgVerify = useTelegramVerify();
  const tgBlocked = tgVerify.data ? !tgVerify.data.ok : false;
  const [ovhSubsidiary, setOvhSubsidiary] = useState("IE");
  const modelsQ = useVPSModels(ovhSubsidiary);
  const models = modelsQ.data?.models || [];
  const [vpsModel, setVpsModel] = useState("");
  const [datacenters, setDatacenters] = useState("");
  const [monitorLinux, setMonitorLinux] = useState(true);
  const [monitorWindows, setMonitorWindows] = useState(true);
  const [notifyAvailable, setNotifyAvailable] = useState(true);
  const [notifyUnavailable, setNotifyUnavailable] = useState(false);
  const [autoOrder, setAutoOrder] = useState(false);
  const [quantity, setQuantity] = useState(1);
  const [autoOrderAccountId, setAutoOrderAccountId] = useState("");
  const [autoPay, setAutoPay] = useState(false);

  useEffect(() => {
    if (isEdit) return;
    if (models.length === 0) return;
    if (!vpsModel || !models.some((m) => m.planCode === vpsModel)) {
      setVpsModel(models[0].planCode);
    }
  }, [models, vpsModel, isEdit]);

  const reset = () => {
    setVpsModel(models[0]?.planCode || "");
    setOvhSubsidiary("IE");
    setDatacenters("");
    setMonitorLinux(true);
    setMonitorWindows(true);
    setNotifyAvailable(true);
    setNotifyUnavailable(false);
    setAutoOrder(false);
    setQuantity(1);
    setAutoOrderAccountId("");
    setAutoPay(false);
  };

  useEffect(() => {
    if (!open) return;
    if (initial) {
      setVpsModel(initial.planCode);
      setOvhSubsidiary(initial.ovhSubsidiary || "IE");
      setDatacenters((initial.datacenters || []).join(","));
      setMonitorLinux(initial.monitorLinux);
      setMonitorWindows(initial.monitorWindows);
      setNotifyAvailable(initial.notifyAvailable);
      setNotifyUnavailable(initial.notifyUnavailable);
      setAutoOrder(!!initial.autoOrder);
      setQuantity(initial.quantity && initial.quantity > 0 ? initial.quantity : 1);
      setAutoOrderAccountId(initial.autoOrderAccountId || "");
      setAutoPay(!!initial.autoPay);
      return;
    }
    reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, initial?.id]);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const dcs = datacenters
      .split(",")
      .map((d) => d.trim())
      .filter(Boolean);

    if (autoOrder && !autoOrderAccountId) {
      toast.error("开启自动下单时必须选 OVH 账户");
      return;
    }
    const payload = {
      ovhSubsidiary,
      datacenters: dcs,
      monitorLinux,
      monitorWindows,
      notifyAvailable,
      notifyUnavailable,
      autoOrder,
      quantity: autoOrder ? quantity : undefined,
      autoOrderAccountId: autoOrder ? autoOrderAccountId : "",
      autoPay: autoOrder ? autoPay : false,
    };
    const onSuccess = () => {
      reset();
      onOpenChange(false);
    };
    if (isEdit && initial) {
      update.mutate({ id: initial.id, ...payload }, { onSuccess });
      return;
    }
    create.mutate({ planCode: vpsModel, ...payload }, { onSuccess });
  };

  const pending = create.isPending || update.isPending;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) reset();
        onOpenChange(v);
      }}
    >
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? "编辑 VPS 订阅" : "添加 VPS 订阅"}</DialogTitle>
          <DialogDescription>
            {isEdit ? "修改提醒与下单方式，不会重置当前库存状态。" : "选择 VPS 型号与可选条件"}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-4">
          {!isEdit && tgBlocked && (
            <div className="flex items-start gap-2.5 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3.5 py-2.5">
              <AlertTriangle className="w-4 h-4 text-amber-600 mt-0.5 flex-shrink-0" />
              <div className="text-xs flex-1 min-w-0">
                <div className="font-medium text-amber-900 dark:text-amber-200">
                  Telegram 通知未配置或无效
                </div>
                <div className="text-amber-800/80 dark:text-amber-200/80 mt-0.5 break-words">
                  {tgVerify.data?.reason || "请先在设置页配置可用的 Telegram Bot Token 和 Chat ID"}
                </div>
                <Link
                  to="/settings"
                  className="inline-block mt-1 text-amber-900 dark:text-amber-200 underline underline-offset-2"
                >
                  去配置 →
                </Link>
              </div>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                VPS 型号 <span className="text-destructive">*</span>
              </label>
              <Select
                value={vpsModel}
                onValueChange={setVpsModel}
                disabled={isEdit || modelsQ.isPending || models.length === 0}
              >
                <SelectTrigger>
                  <SelectValue placeholder={modelsQ.isPending ? "加载型号…" : "选择在售型号"} />
                </SelectTrigger>
                <SelectContent>
                  {isEdit && vpsModel && !models.some((m) => m.planCode === vpsModel) && (
                    <SelectItem value={vpsModel}>{vpsModel}（当前订阅）</SelectItem>
                  )}
                  {models.map((m) => (
                    <SelectItem key={m.planCode} value={m.planCode}>
                      {m.name} ({m.planCode})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {modelsQ.isError && (
                <p className="text-[11px] text-destructive mt-1">实时目录拉取失败，请稍后重试</p>
              )}
            </div>
            <div>
              <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                OVH 子公司
              </label>
              <Select value={ovhSubsidiary} onValueChange={setOvhSubsidiary}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SUBSIDIARIES.map((s) => (
                    <SelectItem key={s.value} value={s.value}>
                      {s.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-muted-foreground mb-1.5">
              数据中心代码（可选，多个用逗号分隔）
            </label>
            <Input
              value={datacenters}
              onChange={(e) => setDatacenters(e.target.value)}
              placeholder="例如: eu-west-gra,ca-east-bhs 或留空监控所有"
            />
          </div>

          <div>
            <p className="text-xs font-medium text-muted-foreground mb-2">监控系统</p>
            <div className="grid grid-cols-2 gap-3">
              <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
                <Checkbox
                  checked={monitorLinux}
                  onCheckedChange={(v) => setMonitorLinux(!!v)}
                />
                <span className="text-sm">Linux</span>
              </label>
              <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
                <Checkbox
                  checked={monitorWindows}
                  onCheckedChange={(v) => setMonitorWindows(!!v)}
                />
                <span className="text-sm">Windows</span>
              </label>
            </div>
          </div>

          <div>
            <p className="text-xs font-medium text-muted-foreground mb-2">通知与下单</p>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
                <Checkbox
                  checked={notifyAvailable}
                  onCheckedChange={(v) => setNotifyAvailable(!!v)}
                />
                <span className="text-sm">有货时提醒</span>
              </label>
              <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
                <Checkbox
                  checked={notifyUnavailable}
                  onCheckedChange={(v) => setNotifyUnavailable(!!v)}
                />
                <span className="text-sm">无货时提醒</span>
              </label>
              <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors sm:col-span-2">
                <Checkbox checked={autoOrder} onCheckedChange={(v) => setAutoOrder(!!v)} />
                <span className="text-sm">有货时自动下单</span>
              </label>
            </div>
          </div>

          {autoOrder && (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                  下单账户 <span className="text-destructive">*</span>
                </label>
                <AccountSelect value={autoOrderAccountId} onChange={setAutoOrderAccountId} />
                <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors mt-3">
                  <Checkbox checked={autoPay} onCheckedChange={(v) => setAutoPay(!!v)} />
                  <span className="text-sm">抢到后自动付款</span>
                </label>
                <p className="text-[11px] text-muted-foreground mt-1">不选账户 = 只通知不下单</p>
              </div>
              <div>
                <label className="block text-xs font-medium text-muted-foreground mb-1.5">
                  下单数量
                </label>
                <Input
                  type="number"
                  min={1}
                  max={100}
                  value={quantity}
                  onChange={(e) => {
                    const v = Number(e.target.value);
                    if (Number.isFinite(v)) {
                      setQuantity(Math.max(1, Math.min(100, Math.floor(v))));
                    }
                  }}
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                reset();
                onOpenChange(false);
              }}
            >
              取消
            </Button>
            <Button
              type="submit"
              disabled={pending || (!isEdit && (tgBlocked || tgVerify.isPending))}
              title={!isEdit && tgBlocked ? "Telegram 通知无效,无法添加订阅" : undefined}
            >
              {pending
                ? "提交中…"
                : !isEdit && tgVerify.isPending
                  ? "校验通知…"
                  : isEdit
                    ? "保存设置"
                    : "确认添加"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function Stat({ label, value }: { label: string; value: number | string }) {
  return (
    <div>
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="text-lg font-semibold">{value}</div>
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
