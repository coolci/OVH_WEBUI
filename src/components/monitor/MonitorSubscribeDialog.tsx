import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { useOrderDurations } from "@/hooks/use-order-durations";
import { OrderDurationField } from "@/components/orders/OrderDurationField";
import { DEFAULT_ORDER_DURATION, normalizeDuration } from "@/lib/order-duration";
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
import { useNotifyGate } from "@/hooks/ovh/use-notify-channels";
import { useServers } from "@/hooks/use-servers";
import { useAvailability, buildAvailabilityMap } from "@/hooks/use-availability";
import {
  DEFAULT_MONITOR_OPTIONS,
  MonitorOptionsFields,
  type MonitorNotifyOptions,
} from "./MonitorOptionsFields";
import { OptionGroupSection } from "@/components/common/OptionGroupSection";
import { groupOptions, type OptionGroupKey } from "@/lib/option-groups";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { useAccounts } from "@/hooks/use-accounts";
import { accountsWhenEnablingAutoOrder, isSinglePlanCode, monitorAccountIds, parseOptionCodes } from "@/lib/order-inputs";

const ALL_DC_CODES = OVH_DATACENTERS.map((dc) => dc.code);
const HW_GROUPS: OptionGroupKey[] = ["memory", "systemStorage", "storage", "bandwidth", "vrack"];

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
    autoOrderAccountIds: monitorAccountIds(sub),
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
  const [notifyBlocked, notifyReason, notifyPending] = useNotifyGate();
  const serversQ = useServers();
  const accountsQ = useAccounts();
  const accountsInitialized = useRef(false);
  const availQ = useAvailability();
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);

  const [planCode, setPlanCode] = useState("");
  const [selectedDCs, setSelectedDCs] = useState<string[]>([...ALL_DC_CODES]);
  const [opts, setOpts] = useState<MonitorNotifyOptions>(DEFAULT_MONITOR_OPTIONS);
  const [restrictHw, setRestrictHw] = useState(false);
  const [optionsInput, setOptionsInput] = useState("");
  const [duration, setDuration] = useState(DEFAULT_ORDER_DURATION);
  const { t } = useTranslation();
  // 周期按第一个下单账户查;多账户同区下单时 OVH 给的周期一致,下单前后端仍逐单核验
  const durations = useOrderDurations(opts.autoOrderAccountIds[0], planCode, open && opts.autoOrder);
  // 新建时换了型号就回到 1 个月;编辑时型号锁定,不会触发
  const lastPlanRef = useRef("");
  useEffect(() => {
    const code = planCode.trim();
    if (lastPlanRef.current && code !== lastPlanRef.current) setDuration(DEFAULT_ORDER_DURATION);
    lastPlanRef.current = code;
  }, [planCode]);

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
  const grouped = useMemo(() => groupOptions(matchedServer?.availableOptions), [matchedServer]);
  const selectedOptions = useMemo(() => parseOptionCodes(optionsInput), [optionsInput]);

  useEffect(() => {
    if (!open) return;
    setPlanCode((initial?.planCode || planCodeProp || "").trim());
    setSelectedDCs(parseDatacenters(initial?.datacenters));
    setOpts(optionsFromSub(initial));
    // Existing subscriptions keep their saved accounts; only new subscriptions default to all.
    accountsInitialized.current = mode === "edit" || monitorAccountIds(initial).length > 0;
    setRestrictHw(!!(initial?.options && initial.options.length > 0));
    setOptionsInput((initial?.options || []).join(", "));
    setDuration(normalizeDuration(initial?.duration));
    // 打开时的型号就是基准,不算"换型号",否则会把刚载入的已保存时长冲掉
    lastPlanRef.current = (initial?.planCode || planCodeProp || "").trim();
    // 只在打开瞬间快照，避免父组件每次 render 把正在改的表单冲掉
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (!open || accountsInitialized.current || !accountsQ.data) return;
    accountsInitialized.current = true;
    setOpts((current) => ({ ...current, autoOrderAccountIds: accountsQ.data.map((a) => a.id) }));
  }, [open, accountsQ.data]);

  const close = () => onOpenChange(false);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const code = planCode.trim();
    if (!isSinglePlanCode(code)) {
      toast.error("请输入一个服务器 planCode，不含空格或逗号");
      return;
    }
    if (opts.autoOrder && !opts.autoOrderAccountIds.length) {
      toast.error("没有下单账户，请添加 OVH 账户或勾选要一起下单的账户");
      return;
    }

    const datacenters =
      selectedDCs.length === 0 || selectedDCs.length === ALL_DC_CODES.length ? [] : selectedDCs;

    const hwOptions = restrictHw ? selectedOptions : [];
    if (restrictHw && hwOptions.length === 0) {
      toast.error("请选择至少一项硬件配置，或关闭「只盯指定配置」");
      return;
    }

    const initialOptionsKey = [...(initial?.options || [])].sort().join(",");

    const payload = {
      planCode: code,
      optionsKey: mode === "edit" ? initialOptionsKey : undefined,
      datacenters,
      options: hwOptions,
      notifyAvailable: opts.notifyAvailable,
      notifyUnavailable: opts.notifyUnavailable,
      autoOrder: opts.autoOrder,
      quantity: opts.autoOrder ? opts.quantity : undefined,
      autoOrderAccountIds: opts.autoOrder ? opts.autoOrderAccountIds : [],
      autoOrderAccountId: opts.autoOrder ? opts.autoOrderAccountIds[0] || "" : "",
      autoPay: opts.autoOrder ? opts.autoPay : false,
      // 只在自动下单时提交;关掉自动下单不冲掉已保存的时长
      duration: opts.autoOrder ? duration : undefined,
    };

    const pending = mode === "edit" ? update : create;
    pending.mutate(payload, {
      onSuccess: () => close(),
    });
  };

  const pending = create.isPending || update.isPending;
  const blockCreate = mode === "create" && (notifyBlocked || notifyPending);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="sm:max-w-2xl max-h-[90dvh] flex flex-col overflow-hidden"
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

        <form onSubmit={submit} className="flex-1 flex flex-col min-h-0 overflow-hidden">
          <div className="flex-1 overflow-y-auto -mx-6 px-6 space-y-4 py-1">
          {mode === "create" && notifyBlocked && (
            <div className="flex items-start gap-2.5 rounded-lg border border-amber-500/40 bg-amber-500/10 px-3.5 py-2.5">
              <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-600" />
              <div className="min-w-0 flex-1 text-xs">
                <div className="font-medium text-amber-900 dark:text-amber-200">
                  通知通道未配置或无效
                </div>
                <div className="mt-0.5 break-words text-amber-800/80 dark:text-amber-200/80">
                  {notifyReason || "请先在设置页配置可用的 Telegram 或 Webhook 通知"}
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
              onChange={(code) => {
                setPlanCode(code);
                setOptionsInput("");
                setRestrictHw(false);
              }}
              servers={serversQ.data || []}
              placeholder="输入或搜索型号，例如 24sk102"
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

          {planCode.trim() && (
            <div className="space-y-3">
              <label className="flex cursor-pointer items-start gap-2.5 rounded-xl border border-border px-3.5 py-2.5 transition-colors hover:bg-muted/40">
                <Checkbox
                  className="mt-0.5"
                  checked={restrictHw}
                  onCheckedChange={(v) => setRestrictHw(!!v)}
                />
                <span>
                  <span className="block text-sm">只盯指定硬件配置</span>
                  <span className="mt-0.5 block text-[11px] text-muted-foreground">
                    关闭时监控该型号下全部内存/硬盘组合。打开后只对选中的配置发通知和自动下单。
                  </span>
                </span>
              </label>
              {restrictHw &&
                HW_GROUPS.filter((g) => grouped[g].length > 0).map((g) => (
                  <OptionGroupSection
                    key={g}
                    groupKey={g}
                    options={grouped[g]}
                    picked={grouped[g].find((o) => selectedOptions.includes(o.value))?.value || ""}
                    defaultValueSet={new Set()}
                    onPick={(value) => {
                      const next = selectedOptions.filter((code) => !grouped[g].some((o) => o.value === code));
                      if (value) next.push(value);
                      setOptionsInput(next.join(", "));
                    }}
                  />
                ))}
              {restrictHw && (
                <div>
                  <label htmlFor="monitor-options" className="mb-1.5 block text-xs font-medium text-muted-foreground">
                    配置 planCode（可手动输入，逗号分隔）
                  </label>
                  <Input id="monitor-options" value={optionsInput} onChange={(e) => setOptionsInput(e.target.value)}
                    placeholder="例如 ram-64g-ecc-2400, softraid-2x450nvme-24sk50" />
                  <p className="mt-1 text-[11px] text-muted-foreground">目录未收录的型号也可指定配置；已有配置会完整保留。</p>
                </div>
              )}
            </div>
          )}

          <MonitorOptionsFields value={opts} onChange={(next) => {
            if (!opts.autoOrder && next.autoOrder && next.autoOrderAccountIds.length === 0 && !accountsQ.data) {
              accountsInitialized.current = false;
            }
            setOpts((current) => ({
              ...next,
              autoOrderAccountIds: accountsWhenEnablingAutoOrder(current.autoOrder, next.autoOrder,
                next.autoOrderAccountIds, (accountsQ.data || []).map((account) => account.id)),
            }));
          }} />
          {opts.autoOrder && (
            <OrderDurationField value={duration} onChange={setDuration} query={durations} hint={t("orderDuration.monitorHint")} />
          )}
          </div>

          <DialogFooter className="pt-3">
            <Button type="button" variant="outline" onClick={close} disabled={pending}>
              取消
            </Button>
            <Button
              type="submit"
              disabled={pending || blockCreate || (opts.autoOrder && accountsQ.isPending)}
              title={mode === "create" && notifyBlocked ? "通知通道无效,无法添加订阅" : undefined}
            >
              {pending
                ? "提交中…"
                : mode === "create" && notifyPending
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
