import { useState, useEffect, useRef, useMemo } from "react";
import {
  Cpu, MemoryStick, HardDrive, Wifi, ShoppingCart, Bell, Loader2,
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
import { useDefaultAccount } from "@/hooks/use-accounts";
import { useSettings, RETRY_INTERVAL } from "@/hooks/use-settings";
import {
  type AvailabilityItem,
  type CatalogIndex,
  type PriceInfo,
  computePriceFromOptions,
  formatPrice,
  hasStockWithOption,
} from "@/hooks/use-availability";
import { groupOptions, type OptionGroupKey } from "@/lib/option-groups";
import { OVH_DATACENTERS, isDcInStock, lookupDcStatus, mergeDcAvailability } from "@/lib/datacenters";

interface ServerDetailDialogProps {
  server: ServerPlan;
  realtimeDcMap?: Record<string, string>;
  variants?: AvailabilityItem[];
  defaultPrice?: PriceInfo;
  catalogIdx: CatalogIndex;
  subsidiary: string;
  isMonitored?: boolean;
  onClose: () => void;
  onMonitor: (datacenters: string[]) => void;
}

export function ServerDetailDialog({
  server,
  realtimeDcMap,
  variants,
  defaultPrice,
  catalogIdx,
  subsidiary,
  isMonitored,
  onClose,
  onMonitor,
}: ServerDetailDialogProps) {
  const create = useCreateQueueItem();
  const defaultAcc = useDefaultAccount();

  const [accountId, setAccountId] = useState("");
  useEffect(() => {
    if (!accountId && defaultAcc) setAccountId(defaultAcc.id);
  }, [defaultAcc, accountId]);

  const [selectedDCs, setSelectedDCs] = useState<string[]>([]);
  const [quantity, setQuantity] = useState("1");
  const [coupon, setCoupon] = useState("");
  const [autoPay, setAutoPay] = useState(false);
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
    () => mergeDcAvailability(server.datacenters, realtimeDcMap),
    [server.datacenters, realtimeDcMap]
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

  const ok = OVH_DATACENTERS.filter((dc) => isDcInStock(lookupDcStatus(dcMap, dc))).length;

  const optionHasStock = (groupKey: OptionGroupKey, value: string): boolean => {
    if (groupKey === "bandwidth" || groupKey === "vrack" || groupKey === "cpu" || groupKey === "other") {
      return true;
    }
    return hasStockWithOption(variants, picked as Record<string, string>, groupKey, value);
  };

  const selectedValues = useMemo(
    () => (Object.values(picked).filter(Boolean) as string[]),
    [picked]
  );

  const price = useMemo(() => {
    if (selectedValues.length === 0) return defaultPrice;
    return computePriceFromOptions(server.planCode, selectedValues, catalogIdx) || defaultPrice;
  }, [server.planCode, selectedValues, catalogIdx, defaultPrice]);

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
          ) : (
            <Chip tone="default" className="flex-shrink-0 text-muted-foreground">
              <StatusDot tone="muted" size="xs" />
              <span>暂时缺货</span>
            </Chip>
          )}
        </div>
      </DialogHeader>

      <div className="overflow-y-auto -mx-6 px-6 space-y-6 flex-1">
        {server.prerelease && (
          <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-xs text-amber-200/90 flex items-start gap-2">
            <span className="font-semibold text-amber-400 shrink-0">预售机型提示：</span>
            <span>仅出现在库存流中，尚未进入当前账户销售目录。创建抢购将作为预订任务在后台挂机轮询，一旦销售目录上架该型号即刻自动下单，等待期间不计入失败重试。</span>
          </div>
        )}
        {/* 价格 Hero */}
        <div className="border border-border rounded-2xl p-4 bg-secondary/30 flex items-end justify-between gap-3 flex-wrap">
          <div>
            <div className="text-[11px] text-muted-foreground">
              月费 · {subsidiary}
              <span className="ml-2 text-[10px]">
                {selectedValues.length > 0 ? "（随当前选配）" : "（默认配置）"}
              </span>
            </div>
            <div className="text-2xl font-bold tabular-nums mt-0.5">
              {price ? formatPrice(price) : <span className="text-muted-foreground font-normal text-base">— · 价格加载中</span>}
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
          .filter((g) => grouped[g].length > 0)
          .map((g) => (
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

        <DatacenterPicker value={selectedDCs} onChange={setSelectedDCs} availability={dcMap} />

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
          </div>
        </div>
      </div>

      <DialogFooter className="flex flex-col-reverse sm:flex-row sm:items-center justify-between gap-2.5 pt-3 border-t border-border">
        <div className="text-xs text-muted-foreground">
          {selectedDCs.length > 0
            ? `已选 ${selectedDCs.length} 个机房 · 共计 ${totalTasks} 个任务`
            : "请选择至少一个数据中心"}
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
            onClick={() => onMonitor(selectedDCs)}
            className="flex-1 sm:flex-none h-9 rounded-lg text-xs gap-1.5 border-border/80 hover:bg-secondary px-3"
          >
            <Bell className="w-3.5 h-3.5" />
            {isMonitored ? "监控设置" : "加入监控"}
          </Button>
          <Button
            size="sm"
            disabled={selectedDCs.length === 0 || create.isPending}
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
              const result = await create.mutateAsync({
                account_id: accountId,
                planCode: server.planCode,
                datacenters: selectedDCs,
                quantity: qty,
                retryInterval: Number(retryInterval) || cfgDefault,
                options: selectedValues,
                autoPay,
                coupon: coupon.trim() || undefined,
                preorder: !!server.prerelease,
              });
              if (result.success > 0) {
                toast.success(`已创建 ${result.success}/${result.total} 个${server.prerelease ? "预订" : "抢购"}任务`);
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
                    preorder: !!server.prerelease,
                    force: true,
                  });
                  if (forceResult.success > 0) {
                    toast.success(`已强制创建 ${forceResult.success}/${forceResult.total} 个${server.prerelease ? "预订" : "抢购"}任务`);
                    onClose();
                    return;
                  }
                }
              }
              if (result.failed > 0 && !result.firstError) {
                toast.error(`${result.failed} 个任务创建失败`);
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
                {server.prerelease
                  ? (selectedDCs.length > 0 ? `创建预订任务（共 ${totalTasks} 个）` : "创建预订抢购任务")
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
