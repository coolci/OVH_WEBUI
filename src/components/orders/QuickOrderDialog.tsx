import { useEffect, useMemo, useState } from "react";
import { Bell, Loader2, ShoppingCart, Zap } from "lucide-react";
import { toast } from "sonner";

import { cn } from "@/lib/utils";
import { AccountSelect } from "@/components/common/AccountSelect";
import { PlanCodeCombobox } from "@/components/common/PlanCodeCombobox";
import { OptionGroupSection } from "@/components/common/OptionGroupSection";
import { StatusDot } from "@/components/common/StatusDot";
import { Chip } from "@/components/common/Chip";
import { groupOptions, type OptionGroupKey } from "@/lib/option-groups";
import { OVH_DATACENTERS, isDcInStock, lookupDcStatus, mergeDcAvailability } from "@/lib/datacenters";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { useServers } from "@/hooks/use-servers";
import { useMonitorList } from "@/hooks/use-monitor";
import { MonitorSubscribeDialog } from "@/components/monitor/MonitorSubscribeDialog";
import { useCreateQueueItem } from "@/hooks/use-queue";
import { useDefaultAccount, useAccounts } from "@/hooks/use-accounts";
import {
  useAvailability,
  buildAvailabilityMap,
  buildVariantIndex,
  hasStockWithOption,
  useOvhCatalog,
  buildCatalogIndex,
  computePriceFromOptions,
  formatPrice,
} from "@/hooks/use-availability";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";

type QuickOrderDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

const DEFAULT_RETRY_INTERVAL = 60;
const OPTION_GROUP_ORDER: OptionGroupKey[] = [
  "cpu",
  "memory",
  "systemStorage",
  "storage",
  "bandwidth",
  "vrack",
  "other",
];

/** 侧栏快速下单：与服务器列表「抢购 / 监控」同一套机房、选配、入队逻辑。 */
export function QuickOrderDialog({ open, onOpenChange }: QuickOrderDialogProps) {
  const serversQ = useServers();
  const availQ = useAvailability();
  const create = useCreateQueueItem();
  const monitorList = useMonitorList();
  const defaultAcc = useDefaultAccount();
  const { data: accounts } = useAccounts();
  const [monitorOpen, setMonitorOpen] = useState(false);

  const [accountId, setAccountId] = useState("");
  const [planCode, setPlanCode] = useState("");
  const [selectedDCs, setSelectedDCs] = useState<string[]>([]);
  const [quantity, setQuantity] = useState("1");
  const [retryInterval, setRetryInterval] = useState(String(DEFAULT_RETRY_INTERVAL));
  const [picked, setPicked] = useState<Partial<Record<OptionGroupKey, string>>>({});
  const [autoPay, setAutoPay] = useState(false);

  const servers = serversQ.data || [];
  const server = useMemo(
    () => servers.find((s) => s.planCode === planCode) || null,
    [servers, planCode]
  );

  const selectedAccount = accounts?.find((a) => a.id === accountId);
  const subsidiary = selectedAccount?.zone || defaultAcc?.zone || "IE";
  const catalogQ = useOvhCatalog(subsidiary);
  const catalogIdx = useMemo(() => buildCatalogIndex(catalogQ.data), [catalogQ.data]);

  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);
  const variantIndex = useMemo(() => buildVariantIndex(availQ.data), [availQ.data]);
  const variants = server ? variantIndex[server.planCode] : undefined;

  const grouped = useMemo(
    () => (server ? groupOptions(server.availableOptions) : null),
    [server]
  );
  const defaultValueSet = useMemo(
    () => new Set((server?.defaultOptions || []).map((o) => o.value)),
    [server]
  );

  const dcMap = useMemo(
    () => mergeDcAvailability(server?.datacenters, server ? availMap[server.planCode] : undefined),
    [server, availMap]
  );

  const okCount = useMemo(
    () => OVH_DATACENTERS.filter((dc) => isDcInStock(lookupDcStatus(dcMap, dc))).length,
    [dcMap]
  );

  const selectedValues = useMemo(
    () => (Object.values(picked).filter(Boolean) as string[]),
    [picked]
  );

  const price = useMemo(() => {
    if (!server) return null;
    return computePriceFromOptions(server.planCode, selectedValues, catalogIdx);
  }, [server, selectedValues, catalogIdx]);

  useEffect(() => {
    if (!open) return;
    setPlanCode("");
    setSelectedDCs([]);
    setQuantity("1");
    setRetryInterval(String(DEFAULT_RETRY_INTERVAL));
    setPicked({});
    setAutoPay(false);
    setAccountId(defaultAcc?.id || "");
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!server) {
      setPicked({});
      setSelectedDCs([]);
      return;
    }
    const next: Partial<Record<OptionGroupKey, string>> = {};
    if (grouped) {
      for (const g of OPTION_GROUP_ORDER) {
        const list = grouped[g];
        if (!list?.length) continue;
        const def = list.find((o) => defaultValueSet.has(o.value));
        if (def) next[g] = def.value;
      }
    }
    setPicked(next);
    setSelectedDCs([]);
  }, [server?.planCode]); // eslint-disable-line react-hooks/exhaustive-deps

  const optionHasStock = (groupKey: OptionGroupKey, value: string): boolean => {
    if (groupKey === "bandwidth" || groupKey === "vrack" || groupKey === "cpu" || groupKey === "other") {
      return true;
    }
    return hasStockWithOption(variants, picked as Record<string, string>, groupKey, value);
  };

  const qty = Math.max(1, Number(quantity) || 1);
  const totalTasks = selectedDCs.length * qty;
  const busy = create.isPending;
  const canSubmit = Boolean(accountId && planCode.trim() && selectedDCs.length > 0) && !busy;
  const existingMonitor = planCode
    ? monitorList.data?.find((s) => s.planCode === planCode)
    : undefined;

  const handleClose = (next: boolean) => {
    if (busy && !next) return;
    onOpenChange(next);
  };

  const handleCreate = async () => {
    if (!accountId) {
      toast.error("请选择 OVH 账户");
      return;
    }
    if (!planCode.trim()) {
      toast.error("请输入服务器型号");
      return;
    }
    if (selectedDCs.length === 0) {
      toast.error("请至少选择一个数据中心");
      return;
    }
    const isCustomPlan = Boolean(
      planCode.trim() && !servers.some((s) => s.planCode.toLowerCase() === planCode.trim().toLowerCase())
    );
    const result = await create.mutateAsync({
      account_id: accountId,
      planCode: planCode.trim(),
      datacenters: selectedDCs,
      quantity: qty,
      retryInterval: Number(retryInterval) || DEFAULT_RETRY_INTERVAL,
      options: selectedValues,
      autoPay,
      force: isCustomPlan,
    });
    if (result.success > 0) {
      toast.success(`已创建 ${result.success}/${result.total} 个抢购任务`);
      onOpenChange(false);
    }
    if (result.failed > 0) {
      toast.error(result.error || `${result.failed} 个任务创建失败`);
    }
  };

  const handleMonitor = () => {
    if (!planCode.trim()) {
      toast.error("请先输入服务器型号");
      return;
    }
    setMonitorOpen(true);
    onOpenChange(false);
  };

  const isCustomPlan = Boolean(
    planCode.trim() && !servers.some((s) => s.planCode.toLowerCase() === planCode.trim().toLowerCase())
  );

  return (
    <>
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent
        className={cn(
          "!flex h-[min(92dvh,860px)] max-h-[min(92dvh,860px)] !flex-col gap-0 !overflow-hidden p-0",
          "w-[calc(100vw-1.25rem)] max-w-3xl"
        )}
      >
        <DialogHeader className="shrink-0 space-y-1.5 border-b border-border/60 px-4 pb-3 pt-4 pr-12 sm:px-6 sm:pt-5">
          <DialogTitle className="flex min-w-0 items-center gap-2">
            <Zap className="h-5 w-5 shrink-0 text-primary" />
            <span className="truncate">快速下单</span>
          </DialogTitle>
          <DialogDescription className="text-left text-sm leading-snug">
            与服务器列表相同：选型号、机房（缺货也可抢）、选配后加入抢购队列，或订阅监控。
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 space-y-5 overflow-y-auto overscroll-contain px-4 py-4 sm:px-6">
          <div className="space-y-1.5">
            <label className="block text-[13px] font-medium">OVH 账户 *</label>
            <AccountSelect value={accountId} onChange={setAccountId} placeholder="选择 OVH 账户" />
            <p className="text-[11px] text-muted-foreground">
              下单走该账户凭据，价格地区跟随账户 {subsidiary}
            </p>
          </div>

          <div className="space-y-1.5">
            <label className="block text-[13px] font-medium">服务器型号 *</label>
            <PlanCodeCombobox
              value={planCode}
              onChange={setPlanCode}
              servers={servers}
              placeholder={serversQ.isPending ? "型号加载中…" : "输入或搜索型号，例如 24ska01"}
            />
            {server && (
              <p className="truncate text-[11px] text-muted-foreground">
                {[server.cpu, server.memory, server.storage, server.bandwidth].filter(Boolean).join(" · ")}
              </p>
            )}
            {isCustomPlan && (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-[12px] text-amber-200/90 flex items-start gap-2">
                <span className="text-amber-400 font-bold shrink-0">自定义型号:</span>
                <span className="text-muted-foreground text-[11px] leading-relaxed">
                  当前型号未在已知官方目录中收录，已启用强制创建模式。若为您自拟的新款或未公开型号，有货后会自动提交；若为拼写错误（如想买 24sk20），建议更正型号。
                </span>
              </div>
            )}
          </div>

          {server && (
            <div className="flex flex-wrap items-end justify-between gap-3 rounded-2xl border border-border bg-secondary/30 p-4">
              <div>
                <div className="text-[11px] text-muted-foreground">
                  月费 · {subsidiary}
                  {selectedValues.length > 0 ? "（随当前选配）" : "（默认配置）"}
                </div>
                <div className="mt-0.5 text-2xl font-bold tabular-nums">
                  {price ? (
                    formatPrice(price)
                  ) : (
                    <span className="text-base font-normal text-muted-foreground">
                      {catalogQ.isPending ? "价格加载中" : "—"}
                    </span>
                  )}
                </div>
              </div>
              {okCount > 0 ? (
                <Chip tone="success">
                  <StatusDot tone="success" pulse size="xs" />
                  {okCount}/{OVH_DATACENTERS.length} 可用
                </Chip>
              ) : (
                <Chip tone="danger">
                  <StatusDot tone="danger" size="xs" />
                  暂时缺货 · 仍可抢购
                </Chip>
              )}
            </div>
          )}

          <DatacenterPicker
            value={selectedDCs}
            onChange={setSelectedDCs}
            availability={dcMap}
            disabled={!planCode.trim()}
          />

          {grouped &&
            OPTION_GROUP_ORDER.filter((g) => grouped[g].length > 0).map((g) => (
              <OptionGroupSection
                key={g}
                groupKey={g}
                options={grouped[g]}
                picked={picked[g] || ""}
                defaultValueSet={defaultValueSet}
                hasStock={variants && variants.length > 0 ? (value) => optionHasStock(g, value) : undefined}
                onPick={(value) => setPicked((p) => ({ ...p, [g]: value }))}
              />
            ))}

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-[11px] text-muted-foreground">每个数据中心数量</label>
              <Input
                type="number"
                min={1}
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
              />
            </div>
            <div>
              <label className="mb-1 block text-[11px] text-muted-foreground">重试间隔（秒）</label>
              <Input
                type="number"
                min={10}
                value={retryInterval}
                onChange={(e) => setRetryInterval(e.target.value)}
              />
            </div>
          </div>

          <label className="flex cursor-pointer items-center gap-2.5 rounded-xl border border-border px-3.5 py-2.5 transition-colors hover:bg-muted/40">
            <Checkbox checked={autoPay} onCheckedChange={(v) => setAutoPay(!!v)} />
            <div>
              <div className="text-sm">抢到后自动付款</div>
              <p className="text-[11px] text-muted-foreground">默认关闭。开启后用 OVH 账户默认支付方式扣款。</p>
            </div>
          </label>
        </div>

        <div className="relative z-10 shrink-0 space-y-2 border-t border-border/60 bg-card px-4 py-3 sm:px-6">
          <div className="text-[12px] text-muted-foreground">
            {selectedDCs.length > 0
              ? `将创建 ${totalTasks} 个任务（${selectedDCs.length} DC × ${qty}）${
                  selectedValues.length > 0 ? ` · ${selectedValues.length} 项选配` : ""
                }`
              : "请选数据中心后再创建抢购任务"}
          </div>
          <DialogFooter className="flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" onClick={() => handleClose(false)} disabled={busy}>
              取消
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={!planCode || busy}
              onClick={handleMonitor}
            >
              <Bell className="h-4 w-4" />
              {existingMonitor ? "监控设置" : "加入监控"}
            </Button>
            <Button type="button" disabled={!canSubmit} onClick={() => void handleCreate()}>
              {create.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <ShoppingCart className="h-4 w-4" />
              )}
              {selectedDCs.length > 0 ? `创建 ${totalTasks} 个任务` : "创建抢购任务"}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
    <MonitorSubscribeDialog
      open={monitorOpen}
      onOpenChange={setMonitorOpen}
      mode={existingMonitor ? "edit" : "create"}
      lockPlanCode
      planCode={planCode.trim()}
      serverName={server?.name}
      initial={
        existingMonitor ?? {
          planCode: planCode.trim(),
          datacenters:
            selectedDCs.length > 0 ? selectedDCs : OVH_DATACENTERS.map((dc) => dc.code),
        }
      }
    />
    </>
  );
}
