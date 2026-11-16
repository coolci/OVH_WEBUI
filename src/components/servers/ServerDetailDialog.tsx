import { useState, useEffect, useRef, useMemo } from "react";
import {
  Cpu, MemoryStick, HardDrive, Wifi, ShoppingCart, Bell, Loader2, Zap,
} from "lucide-react";
import { toast } from "sonner";
import { DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { AccountSelect } from "@/components/common/AccountSelect";
import { Chip } from "@/components/common/Chip";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { StatusDot } from "@/components/common/StatusDot";
import { OptionGroupSection } from "@/components/common/OptionGroupSection";
import { type ServerPlan } from "@/hooks/use-servers";
import { useCreateQueueItem } from "@/hooks/use-queue";
import { useAccounts, useDefaultAccount, findAccountByID } from "@/hooks/use-accounts";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import { useSettings, RETRY_INTERVAL } from "@/hooks/use-settings";
import { usePrewarmOrder } from "@/hooks/use-prewarm";
import { useOrderDurations } from "@/hooks/use-order-durations";
import { OrderDurationField } from "@/components/orders/OrderDurationField";
import { DEFAULT_ORDER_DURATION, supportsDuration } from "@/lib/order-duration";
import { preorderOptions, preorderVariantCoversAll } from "@/lib/preorder-options";
import { normalizeMonitorOptionsKey } from "@/lib/order-inputs";
import { accountSubsidiary } from "@/lib/ovh-subsidiaries";
import {
  type AvailabilityItem,
  type CatalogIndex,
  type PriceInfo,
  useAvailability,
  buildAvailabilityMap,
  variantsForPlan,
  useOvhCatalog,
  buildCatalogIndex,
  computePriceFromOptions,
  formatPrice,
  hasStockWithOption,
} from "@/hooks/use-availability";
import { groupOptions, describeOptionCodes, type OptionGroupKey } from "@/lib/option-groups";
import { selectionDcStatus, stockOnlyParts } from "@/lib/variant-stock";
import { OVH_DATACENTERS, isDcInStock, lookupDcStatus, mergeDcAvailability } from "@/lib/datacenters";
import { AutoPayCheck } from "@/components/orders/AutoPayCheck";
import { OSAvailabilityList } from "@/components/servers/OSAvailabilityList";
import type { MonitorSubscription } from "@/hooks/use-monitor";

interface ServerDetailDialogProps {
  server: ServerPlan;
  realtimeDcMap?: Record<string, string>;
  variants?: AvailabilityItem[];
  defaultPrice?: PriceInfo;
  catalogIdx: CatalogIndex;
  subsidiary: string;
  isMonitored?: boolean;
  subscriptions?: MonitorSubscription[];
  onClose: () => void;
  onMonitor: (datacenters: string[], options: string[]) => void;
}

/** 不出现在库存 FQN 里的选项组:不参与"这个配置有没有货"的判断 */
const NON_FQN_GROUPS = new Set<OptionGroupKey>(["bandwidth", "vrack", "cpu", "other"]);

export function ServerDetailDialog({
  server,
  realtimeDcMap,
  variants,
  defaultPrice,
  catalogIdx,
  subsidiary,
  isMonitored,
  subscriptions,
  onClose,
  onMonitor,
}: ServerDetailDialogProps) {
  const create = useCreateQueueItem();
  const { data: accounts } = useAccounts();
  const defaultAcc = useDefaultAccount();
  const [activeAccId] = useActiveServerControlAccount();

  const [accountId, setAccountId] = useState(() => activeAccId || defaultAcc?.id || "");
  useEffect(() => {
    if (!accountId && (activeAccId || defaultAcc?.id)) {
      setAccountId(activeAccId || defaultAcc?.id || "");
    }
  }, [activeAccId, defaultAcc, accountId]);

  const selectedAccount = useMemo(
    () => findAccountByID(accounts, accountId),
    [accounts, accountId]
  );

  // 弹窗内切换下单账户时，自动联动该账户所属子公司目录与区域实时可用性重算报价/库存
  const effectiveSubsidiary = useMemo(() => {
    if (selectedAccount && selectedAccount.id !== activeAccId) {
      return accountSubsidiary(selectedAccount.zone, selectedAccount.endpoint);
    }
    if (subsidiary) return subsidiary;
    if (selectedAccount) return accountSubsidiary(selectedAccount.zone, selectedAccount.endpoint);
    return "IE";
  }, [selectedAccount, activeAccId, subsidiary]);

  const accountCatalogQ = useOvhCatalog(effectiveSubsidiary);
  const effectiveCatalogIdx = useMemo(() => {
    if (accountCatalogQ.data) return buildCatalogIndex(accountCatalogQ.data);
    if (effectiveSubsidiary === subsidiary) return catalogIdx;
    return buildCatalogIndex(undefined);
  }, [accountCatalogQ.data, effectiveSubsidiary, subsidiary, catalogIdx]);

  const accountAvailQ = useAvailability(selectedAccount?.endpoint || undefined);
  const effectiveRealtimeDcMap = useMemo(() => {
    if (selectedAccount && selectedAccount.id !== activeAccId && accountAvailQ.data) {
      return buildAvailabilityMap(accountAvailQ.data)[server.planCode];
    }
    return realtimeDcMap;
  }, [selectedAccount, activeAccId, accountAvailQ.data, server.planCode, realtimeDcMap]);

  const effectiveVariants = useMemo(() => {
    if (selectedAccount && selectedAccount.id !== activeAccId && accountAvailQ.data) {
      return variantsForPlan(accountAvailQ.data, server.planCode);
    }
    return variants;
  }, [selectedAccount, activeAccId, accountAvailQ.data, server.planCode, variants]);

  const [selectedDCs, setSelectedDCs] = useState<string[]>([]);
  const [quantity, setQuantity] = useState("1");
  const [coupon, setCoupon] = useState("");
  const [autoPay, setAutoPay] = useState(false);
  const [duration, setDuration] = useState(DEFAULT_ORDER_DURATION);
  const durations = useOrderDurations(accountId, server.planCode);
  const durationValid = supportsDuration(durations.data, duration);
  const settingsQ = useSettings();
  const cfgDefault = settingsQ.data?.defaultRetryInterval || RETRY_INTERVAL.defaultTask;
  const [retryInterval, setRetryInterval] = useState("");
  const intervalTouchedRef = useRef(false);

  useEffect(() => {
    if (!intervalTouchedRef.current) setRetryInterval(String(cfgDefault));
  }, [cfgDefault]);

  const qty = Math.max(1, Number(quantity) || 1);
  const totalTasks = selectedDCs.length * qty;
  const dcMap = useMemo(
    () => mergeDcAvailability(server.datacenters, effectiveRealtimeDcMap),
    [server.datacenters, effectiveRealtimeDcMap]
  );

  const grouped = useMemo(() => groupOptions(server.availableOptions), [server.availableOptions]);
  const defaultValueSet = useMemo(
    () => new Set((server.defaultOptions || []).map((o) => o.value)),
    [server.defaultOptions]
  );

  const initialPicked = useMemo(() => {
    const out: Partial<Record<OptionGroupKey, string>> = {};
    (Object.keys(grouped) as OptionGroupKey[]).forEach((g) => {
      const list = grouped[g];
      if (list.length === 0) return;
      const def = list.find((o) => defaultValueSet.has(o.value));
      if (def) out[g] = def.value;
    });
    return out;
  }, [grouped, defaultValueSet]);

  const [picked, setPicked] = useState<Partial<Record<OptionGroupKey, string>>>(initialPicked);

  // 型号级:任一配置有货的机房数
  const planOk = OVH_DATACENTERS.filter((dc) => isDcInStock(lookupDcStatus(dcMap, dc))).length;

  const optionHasStock = (groupKey: OptionGroupKey, value: string): boolean => {
    if (NON_FQN_GROUPS.has(groupKey)) {
      return true;
    }
    return hasStockWithOption(effectiveVariants, picked as Record<string, string>, groupKey, value);
  };

  // 预订(只在库存出现、尚未进入销售目录):CPU 不参与匹配,不随任务提交
  const preorder = !!server.prerelease;

  // 机房红绿按"当前所选配置"判断,不是型号级。
  const hasVariants = !!effectiveVariants && effectiveVariants.length > 0;
  const stockPicks = useMemo(
    () =>
      (Object.entries(picked) as [OptionGroupKey, string | undefined][])
        .filter(([g, v]) => !!v && !NON_FQN_GROUPS.has(g))
        .map(([, v]) => v as string),
    [picked],
  );
  const selectionMap = useMemo(
    () => (hasVariants && !preorder ? selectionDcStatus(dcMap, effectiveVariants!, stockPicks) : dcMap),
    [hasVariants, preorder, dcMap, effectiveVariants, stockPicks],
  );
  const ok = OVH_DATACENTERS.filter((dc) => isDcInStock(lookupDcStatus(selectionMap, dc))).length;
  // 有货、但销售目录里选不到的配置段(如目录只卖 3×4T,有货的却是 2×4T)
  const offCatalog = useMemo(() => {
    if (!hasVariants || preorder) return [];
    const catalogValues = (server.availableOptions || []).map((o) => o.value);
    return stockOnlyParts(effectiveVariants!, catalogValues);
  }, [hasVariants, preorder, effectiveVariants, server.availableOptions]);
  const selectedValues = useMemo(() => {
    const options = Object.values(picked).filter(Boolean) as string[];
    return preorder ? preorderOptions(options, grouped.cpu?.map((o) => o.value)) : options;
  }, [picked, preorder, grouped]);
  const preorderMismatch = preorder && selectedValues.length > 0 && !!effectiveVariants?.length &&
    !effectiveVariants.some((v) => preorderVariantCoversAll(v.fqn, selectedValues));

  const price = useMemo(() => {
    const defaults = (server.defaultOptions || []).map((o) => o.value).filter(Boolean);
    const targetOpts = selectedValues.length > 0 ? selectedValues : defaults;
    const computed = computePriceFromOptions(server.planCode, targetOpts, effectiveCatalogIdx);
    if (computed) return computed;
    if (effectiveSubsidiary === subsidiary && selectedValues.length === 0) return defaultPrice;
    return null;
  }, [server.planCode, server.defaultOptions, selectedValues, effectiveCatalogIdx, effectiveSubsidiary, subsidiary, defaultPrice]);

  const currentOptionsKey = useMemo(
    () => normalizeMonitorOptionsKey(selectedValues),
    [selectedValues]
  );
  const isCurrentSelectionMonitored = useMemo(() => {
    if (subscriptions) {
      return subscriptions.some(
        (s) => s.planCode === server.planCode && normalizeMonitorOptionsKey(s.options) === currentOptionsKey
      );
    }
    return Boolean(isMonitored && !currentOptionsKey);
  }, [subscriptions, server.planCode, currentOptionsKey, isMonitored]);

  const prewarm = usePrewarmOrder({
    account_id: accountId,
    planCode: server.planCode,
    datacenters: selectedDCs,
    options: selectedValues,
    coupon: coupon.trim() || undefined,
    duration,
    // 预订开售前装不起车,预热只会白刷 OVH;下单时后端会逐项核对购物车
    enabled: selectedDCs.length > 0 && !!accountId && !preorder,
  });

  return (
    <>
      <DialogHeader className="space-y-1.5 pb-2 text-left">
        <div className="pr-8">
          <DialogTitle className="font-mono text-lg sm:text-xl font-bold tracking-tight text-foreground break-all sm:break-normal">
            {server.planCode}
          </DialogTitle>
        </div>

        <div className="flex items-center justify-between gap-2.5 pt-0.5">
          <DialogDescription className="text-xs sm:text-sm text-muted-foreground font-medium truncate flex-1 min-w-0" title={server.name}>
            {server.name}
          </DialogDescription>
          {ok > 0 ? (
            <Chip tone="success" className="flex-shrink-0">
              <StatusDot tone="success" size="xs" pulse />
              <span>{ok} 机房有货</span>
            </Chip>
          ) : planOk > 0 ? (
            <Chip tone="warning" className="flex-shrink-0">
              <StatusDot tone="warning" size="xs" />
              <span>所选配置缺货</span>
            </Chip>
          ) : (
            <Chip tone="default" className="flex-shrink-0 text-muted-foreground">
              <StatusDot tone="muted" size="xs" />
              <span>暂时缺货</span>
            </Chip>
          )}
        </div>
      </DialogHeader>

      <div className="overflow-y-auto -mx-6 px-6 space-y-6 flex-1">
        {preorder && (
          <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-xs flex items-start gap-2">
            <span className="font-semibold text-amber-700 dark:text-amber-400 shrink-0">预订：</span>
            <span className="text-muted-foreground">该型号只出现在库存中、尚未进入当前账户销售目录。预订按型号、内存、完整硬盘组合和机房严格匹配，CPU 不参与匹配；以 OVH 实际下单接口开放为准，不更换型号、内存频率或硬盘。开售前的等待不计失败次数，任务保存在本地，不会预留 OVH 库存。</span>
          </div>
        )}
        {/* 价格 Hero */}
        <div className="border border-border rounded-2xl p-4 bg-secondary/30 flex items-end justify-between gap-3 flex-wrap">
          <div>
            <div className="text-[11px] text-muted-foreground">
              月费 · {effectiveSubsidiary}
              <span className="ml-2 text-[10px]">
                {selectedValues.length > 0 ? "（随当前选配）" : "（默认配置）"}
              </span>
            </div>
            <div className="text-2xl font-bold tabular-nums mt-0.5">
              {price ? (
                formatPrice(price)
              ) : (
                <span className="text-muted-foreground font-normal text-base">
                  {accountCatalogQ.isPending ? "— · 价格加载中" : "—"}
                </span>
              )}
            </div>
          </div>
          {price && (
            <div className="text-right text-[11px] text-muted-foreground space-y-0.5 tabular-nums">
              {price.installPrice > 0 && (
                <div>安装费 {fmtMoney(price.installPrice, price.currency)}（一次性）</div>
              )}
              <div>币种 {price.currency}</div>
            </div>
          )}
        </div>

        {/* 规格 4 卡 */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 sm:gap-3">
          <SpecCard icon={<Cpu className="w-4 h-4" />} label="CPU" value={server.cpu} />
          <SpecCard icon={<MemoryStick className="w-4 h-4" />} label="内存" value={server.memory} />
          <SpecCard icon={<HardDrive className="w-4 h-4" />} label="硬盘" value={server.storage} />
          <SpecCard icon={<Wifi className="w-4 h-4" />} label="带宽" value={server.bandwidth} />
        </div>

        {/* 硬件配置选择 */}
        {(["cpu", "memory", "systemStorage", "storage", "bandwidth", "vrack", "other"] as OptionGroupKey[])
          .filter((g) => grouped[g].length > 0 && !(preorder && g === "cpu"))
          .map((g) => (
            <OptionGroupSection
              key={g}
              groupKey={g}
              options={grouped[g]}
              picked={picked[g] || ""}
              defaultValueSet={defaultValueSet}
              hasStock={effectiveVariants && effectiveVariants.length > 0 ? (value) => optionHasStock(g, value) : undefined}
              onPick={(value) => setPicked((p) => ({ ...p, [g]: p[g] === value ? "" : value }))}
            />
          ))}

        {preorderMismatch && (
          <p role="alert" className="text-xs text-destructive">库存记录中没有这套配置组合，请重新选择内存或存储。</p>
        )}

        {!preorder && ok === 0 && planOk > 0 && (
          <div role="status" className="rounded-xl border border-warning/30 bg-warning/5 p-3 text-xs space-y-1">
            <p className="font-medium">所选配置目前在所有机房都缺货（下面按所选配置显示）。</p>
            <p className="text-muted-foreground">
              这个型号有 {planOk} 个机房有货，但有货的是其他配置
              {offCatalog.length > 0
                ? `，其中「${describeOptionCodes(offCatalog)}」不在当前销售目录里，这里选不到`
                : "，换成带绿点的选项就能看到"}
              。按当前配置创建任务会等这套配置有货再下单。
            </p>
          </div>
        )}

        <DatacenterPicker value={selectedDCs} onChange={setSelectedDCs} availability={selectionMap} />

        {!preorder && <OSAvailabilityList planCode={server.planCode} accountId={accountId} />}

        {/* 抢购参数 */}
        <div className="border-t border-border pt-4">
          <h3 className="text-[13px] font-semibold mb-2.5 flex items-center gap-1.5">
            <ShoppingCart className="w-3.5 h-3.5 text-muted-foreground" />
            抢购参数
          </h3>
          <div className="space-y-3">
            <div>
              <label className="block text-[11px] text-muted-foreground mb-1">OVH 账户 *</label>
              <AccountSelect value={accountId} onChange={setAccountId} />
            </div>
            <OrderDurationField value={duration} onChange={setDuration} query={durations} />
            <div>
              <label className="block text-[11px] text-muted-foreground mb-1">优惠码（可选）</label>
              <Input
                value={coupon}
                onChange={(e) => setCoupon(e.target.value.toUpperCase())}
                placeholder="输入 OVH 优惠码（如 SAVE10），留空不使用"
                autoCapitalize="none"
                spellCheck={false}
                className="font-mono uppercase text-xs"
              />
              <p className="text-[11px] text-muted-foreground mt-1">
                展示价格未扣除此优惠码；实际优惠以 OVH 校验为准。优惠码被拒绝时取消使用该码继续下单，并在购买历史和通知中提示。
              </p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-[11px] text-muted-foreground mb-1">每个数据中心数量</label>
                <Input
                  type="number"
                  min={1}
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-[11px] text-muted-foreground mb-1">重试间隔（秒）</label>
                <Input
                  type="number"
                  min={RETRY_INTERVAL.min}
                  max={RETRY_INTERVAL.max}
                  value={retryInterval}
                  onChange={(e) => {
                    intervalTouchedRef.current = true;
                    setRetryInterval(e.target.value);
                  }}
                  placeholder={`默认 ${cfgDefault}`}
                />
              </div>
            </div>
            <label className="flex items-center gap-2 pt-1 cursor-pointer text-xs">
              <Checkbox checked={autoPay} onCheckedChange={(v) => setAutoPay(!!v)} />
              <span className="text-muted-foreground">抢到后自动付款（默认关闭；开启后使用 OVH 账户默认支付方式自动结算）</span>
            </label>
            <AutoPayCheck accountIds={[accountId]} enabled={autoPay} />
          </div>
        </div>
      </div>

      <DialogFooter className="flex flex-col-reverse sm:flex-row sm:items-center justify-between gap-2.5 pt-3 border-t border-border">
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>
            {selectedDCs.length > 0
              ? `已选 ${selectedDCs.length} 个机房 · 共计 ${totalTasks} 个任务`
              : "请选择至少一个数据中心"}
          </span>
          {prewarm.isReady && (
            <span className="inline-flex items-center gap-1 text-[11px] text-emerald-500 font-medium bg-emerald-500/10 px-2 py-0.5 rounded">
              <Zap className="w-3 h-3 fill-emerald-500 text-emerald-500" />
              预热就绪 (0ms秒杀)
            </span>
          )}
          {prewarm.isPrewarming && !prewarm.isReady && (
            <span className="inline-flex items-center gap-1 text-[11px] text-amber-500 font-medium bg-amber-500/10 px-2 py-0.5 rounded">
              <Loader2 className="w-3 h-3 animate-spin" />
              预热装车中...
            </span>
          )}
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <Button
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={create.isPending}
            className="flex-1 sm:flex-none h-9 rounded-lg text-xs border-border/80 hover:bg-secondary px-3"
          >
            关闭
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={create.isPending}
            onClick={() => onMonitor(selectedDCs, selectedValues)}
            className="flex-1 sm:flex-none h-9 rounded-lg text-xs gap-1.5 border-border/80 hover:bg-secondary px-3"
          >
            <Bell className="w-3.5 h-3.5" />
            {isCurrentSelectionMonitored ? "监控设置" : "加入监控"}
          </Button>
          <Button
            size="sm"
            disabled={selectedDCs.length === 0 || !accountId || create.isPending || preorderMismatch || !durationValid}
            className="flex-1 sm:flex-none h-9 rounded-lg text-xs font-medium gap-1.5 px-3.5"
            onClick={async () => {
              if (selectedDCs.length === 0) {
                toast.error("请至少选择一个数据中心");
                return;
              }
              if (!accountId) {
                toast.error("请选择 OVH 账户");
                return;
              }
              if (!durationValid) {
                toast.error("当前型号不支持所选购买时长，请重新选择");
                return;
              }
              try {
                const result = await create.mutateAsync({
                  account_id: accountId,
                  planCode: server.planCode,
                  datacenters: selectedDCs,
                  quantity: qty,
                  retryInterval: Number(retryInterval) || cfgDefault,
                  options: selectedValues,
                  autoPay,
                  coupon: coupon.trim() || undefined,
                  duration,
                  preorder,
                });
                prewarm.reset();
                if (result.success > 0) {
                  toast.success(`已创建 ${result.success}/${result.total} 个抢购任务`);
                  onClose();
                  return;
                }
                if (result.canForce) {
                  const ok = window.confirm(`型号【${server.planCode}】未在当前账户所在区域目录收录（${result.firstError || "可能为新品/跨区机型"}）。\n\n是否以强制模式（force=true）继续添加任务？`);
                  if (ok) {
                    const forceResult = await create.mutateAsync({
                      account_id: accountId,
                      planCode: server.planCode,
                      datacenters: selectedDCs,
                      quantity: qty,
                      retryInterval: Number(retryInterval) || cfgDefault,
                      options: selectedValues,
                      autoPay,
                      coupon: coupon.trim() || undefined,
                      duration,
                      preorder,
                      force: true,
                    });
                    if (forceResult.success > 0) {
                      toast.success(`已强制创建 ${forceResult.success}/${forceResult.total} 个抢购任务`);
                      onClose();
                      return;
                    }
                  }
                }
                if (result.failed > 0 && !result.firstError) {
                  toast.error(`${result.failed} 个任务创建失败`);
                }
              } catch {
                // useCreateQueueItem.onError 已弹出错误提示
              }
            }}
          >
            {create.isPending ? (
              <>
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                创建中…
              </>
            ) : (
              <>
                <ShoppingCart className="w-3.5 h-3.5" />
                {preorder
                  ? (selectedDCs.length > 0 ? `创建 ${totalTasks} 个预订任务` : "创建预订任务")
                  : (selectedDCs.length > 0 ? `创建 ${totalTasks} 个任务` : "创建抢购任务")}
              </>
            )}
          </Button>
        </div>
      </DialogFooter>
    </>
  );
}

function fmtMoney(v: number, currency: string): string {
  const sym = currency === "EUR" ? "€" : currency === "USD" ? "$" : currency === "GBP" ? "£" : currency === "CAD" ? "CA$" : `${currency} `;
  return `${sym}${v.toFixed(2)}`;
}

function SpecCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="surface-card border border-border rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0">
      <div className="w-9 h-9 rounded-lg bg-secondary border border-border/60 flex items-center justify-center text-primary flex-shrink-0">
        {icon}
      </div>
      <div className="min-w-0">
        <div className="text-[11px] text-muted-foreground">{label}</div>
        <div className="text-[13px] font-semibold truncate text-foreground" title={value}>{value}</div>
      </div>
    </div>
  );
}
