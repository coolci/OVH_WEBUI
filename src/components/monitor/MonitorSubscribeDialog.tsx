import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { AlertTriangle } from "lucide-react";
import { toast } from "sonner";

import { OVH_DATACENTERS, mergeDcAvailability } from "@/lib/datacenters";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { PlanCodeCombobox } from "@/components/common/PlanCodeCombobox";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useCreateMonitorSubscription,
  useUpdateMonitorSubscription,
  type MonitorSubscription,
} from "@/hooks/use-monitor";
import { useTelegramVerify } from "@/hooks/use-telegram";
import { useServers } from "@/hooks/use-servers";
import { useAvailability, buildAvailabilityMap } from "@/hooks/use-availability";
import {
  DEFAULT_MONITOR_OPTIONS,
  MonitorOptionsFields,
  type MonitorNotifyOptions,
} from "./MonitorOptionsFields";

const ALL_DC_CODES = OVH_DATACENTERS.map((dc) => dc.code);

function parseDatacenters(raw: string[] | undefined): string[] {
  if (!raw || raw.length === 0) return [...ALL_DC_CODES];
  const set = new Set(raw.map((d) => d.trim().toLowerCase()).filter(Boolean));
  return ALL_DC_CODES.filter((code) => {
    const dc = OVH_DATACENTERS.find((d) => d.code === code);
    return set.has(code) || (!!dc?.apiCode && set.has(dc.apiCode));
  });
}

function optionsFromSub(sub?: Partial<MonitorSubscription> | null): MonitorNotifyOptions {
  return {
    notifyAvailable: sub?.notifyAvailable ?? true,
    notifyUnavailable: sub?.notifyUnavailable ?? false,
    autoOrder: !!sub?.autoOrder,
    autoPay: !!sub?.autoPay,
    quantity: sub?.quantity && sub.quantity > 0 ? sub.quantity : 1,
    autoOrderAccountId: sub?.autoOrderAccountId || "",
  };
}

export type MonitorSubscribeDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** create 走 POST；edit 走 PUT，不重置 LastStatus */
  mode: "create" | "edit";
  planCode?: string;
  serverName?: string;
  /** 从服务器列表 / 快速下单进来时锁死型号 */
  lockPlanCode?: boolean;
  initial?: Partial<MonitorSubscription> | null;
};

export function MonitorSubscribeDialog({
  open,
  onOpenChange,
  mode,
  planCode: planCodeProp,
  serverName,
  lockPlanCode,
  initial,
}: MonitorSubscribeDialogProps) {
  const create = useCreateMonitorSubscription();
  const update = useUpdateMonitorSubscription();
  const tgVerify = useTelegramVerify();
  const tgBlocked = tgVerify.data ? !tgVerify.data.ok : false;
  const serversQ = useServers();
  const availQ = useAvailability();
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);

  const [planCode, setPlanCode] = useState("");
  const [selectedDCs, setSelectedDCs] = useState<string[]>([...ALL_DC_CODES]);
  const [opts, setOpts] = useState<MonitorNotifyOptions>(DEFAULT_MONITOR_OPTIONS);

  const planLocked = mode === "edit" || !!lockPlanCode;
  const matchedServer = useMemo(
    () => (serversQ.data || []).find((s) => s.planCode === planCode.trim()),
    [serversQ.data, planCode]
  );
  const dcAvailability = useMemo(
    () =>
      mergeDcAvailability(
        matchedServer?.datacenters,
        matchedServer ? availMap[matchedServer.planCode] : undefined
      ),
    [matchedServer, availMap]
  );

  useEffect(() => {
    if (!open) return;
    setPlanCode((initial?.planCode || planCodeProp || "").trim());
    setSelectedDCs(parseDatacenters(initial?.datacenters));
    setOpts(optionsFromSub(initial));
    // 只在打开瞬间快照，避免父组件每次 render 把正在改的表单冲掉
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const close = () => onOpenChange(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const code = planCode.trim();
    if (!code) {
      toast.error("请输入服务器型号");
      return;
    }
    if (opts.autoOrder && !opts.autoOrderAccountId) {
      toast.error("开启自动下单时必须选择 OVH 账户(否则只通知不下单)");
      return;
    }

    const datacenters =
      selectedDCs.length === 0 || selectedDCs.length === ALL_DC_CODES.length ? [] : selectedDCs;

    const payload = {
      planCode: code,
      datacenters,
      notifyAvailable: opts.notifyAvailable,
      notifyUnavailable: opts.notifyUnavailable,
      autoOrder: opts.autoOrder,
      quantity: opts.autoOrder ? opts.quantity : undefined,
      autoOrderAccountId: opts.autoOrder ? opts.autoOrderAccountId : "",
      autoPay: opts.autoOrder ? opts.autoPay : false,
    };

    const pending = mode === "edit" ? update : create;
    pending.mutate(payload, {
      onSuccess: () => close(),
    });
  };

  const pending = create.isPending || update.isPending;
  const blockCreate = mode === "create" && (tgBlocked || tgVerify.isPending);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="sm:max-w-2xl"
        onOpenAutoFocus={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>
            {mode === "edit" ? "编辑监控订阅" : planLocked ? "加入监控" : "添加订阅"}
          </DialogTitle>
          <DialogDescription>
            {mode === "edit"
              ? "修改提醒与下单方式，不会重置当前库存状态，也不会误触发补货通知。"
              : "选择提醒方式、是否自动下单，以及抢到后是否自动付款。"}
            {serverName ? ` ${serverName}` : ""}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-4">
          {mode === "create" && tgBlocked && (
            <div className="flex items-start gap-2.5 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3.5 py-2.5">
              <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-600" />
              <div className="min-w-0 flex-1 text-xs">
                <div className="font-medium text-amber-900 dark:text-amber-200">
                  Telegram 通知未配置或无效
                </div>
                <div className="mt-0.5 break-words text-amber-800/80 dark:text-amber-200/80">
                  {tgVerify.data?.reason || "请先在设置页配置可用的 Telegram Bot Token 和 Chat ID"}
                </div>
                <Link
                  to="/settings"
                  className="mt-1 inline-block text-amber-900 underline underline-offset-2 dark:text-amber-200"
                >
                  去配置 →
                </Link>
              </div>
            </div>
          )}

          <div>
            <label className="mb-1.5 block text-xs font-medium text-muted-foreground">
              服务器型号 <span className="text-destructive">*</span>
            </label>
            <PlanCodeCombobox
              value={planCode}
              onChange={setPlanCode}
              servers={serversQ.data || []}
              placeholder="输入或搜索型号，例如 24ska01"
              disabled={planLocked}
            />
          </div>

          <div>
            <DatacenterPicker
              value={selectedDCs}
              onChange={setSelectedDCs}
              availability={dcAvailability}
            />
            <p className="mt-1.5 text-[11px] text-muted-foreground">
              {selectedDCs.length === 0 || selectedDCs.length === ALL_DC_CODES.length
                ? "未选或全选 = 监控全部机房。缺货机房也可监控。"
                : `将监控 ${selectedDCs.length} 个机房`}
            </p>
          </div>

          <MonitorOptionsFields value={opts} onChange={setOpts} />

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close} disabled={pending}>
              取消
            </Button>
            <Button
              type="submit"
              disabled={pending || blockCreate}
              title={mode === "create" && tgBlocked ? "Telegram 通知无效,无法添加订阅" : undefined}
            >
              {pending
                ? "提交中…"
                : mode === "create" && tgVerify.isPending
                  ? "校验通知…"
                  : mode === "edit"
                    ? "保存设置"
                    : "确认添加"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
