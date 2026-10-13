import { useEffect, useState, useMemo } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useCreateVPSMonitorSubscription,
  useUpdateVPSSubscription,
  useVPSModels,
  type VPSSubscription,
} from "@/hooks/use-vps-monitor";
import { useTelegramVerify } from "@/hooks/use-telegram";
import { SUBSIDIARIES } from "./types";

interface AddVPSDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initial?: VPSSubscription | null;
}

export function AddVPSDialog({ open, onOpenChange, initial }: AddVPSDialogProps) {
  const create = useCreateVPSMonitorSubscription();
  const update = useUpdateVPSSubscription();
  const isEdit = !!initial;
  const tgVerify = useTelegramVerify();
  const tgBlocked = tgVerify.data ? !tgVerify.data.ok : false;
  const [ovhSubsidiary, setOvhSubsidiary] = useState("IE");
  const modelsQ = useVPSModels(ovhSubsidiary);
  const rawModels = modelsQ.data?.models;
  const models = useMemo(() => rawModels || [], [rawModels]);
  const [vpsModel, setVpsModel] = useState("");
  const [datacenters, setDatacenters] = useState("");
  const [monitorLinux, setMonitorLinux] = useState(true);
  const [monitorWindows, setMonitorWindows] = useState(true);
  const [notifyAvailable, setNotifyAvailable] = useState(true);
  const [notifyUnavailable, setNotifyUnavailable] = useState(false);

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

    const payload = {
      ovhSubsidiary,
      datacenters: dcs,
      monitorLinux,
      monitorWindows,
      notifyAvailable,
      notifyUnavailable,
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
            <p className="text-xs font-medium text-muted-foreground mb-2">通知提醒</p>
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
            </div>
          </div>

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
