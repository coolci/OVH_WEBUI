import { useState, useEffect, useRef, useMemo } from "react";
import { Plus, Loader2 } from "lucide-react";
import { toast } from "sonner";
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
import { Chip } from "@/components/common/Chip";
import { AccountSelect } from "@/components/common/AccountSelect";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { PlanCodeCombobox } from "@/components/common/PlanCodeCombobox";
import { OptionGroupSection } from "@/components/common/OptionGroupSection";
import { useServers } from "@/hooks/use-servers";
import { useCreateQueueItem } from "@/hooks/use-queue";
import { useSettings, RETRY_INTERVAL } from "@/hooks/use-settings";
import {
  useAvailability,
  buildAvailabilityMap,
  buildVariantIndex,
  hasStockWithOption,
} from "@/hooks/use-availability";
import { mergeDcAvailability } from "@/lib/datacenters";
import { describeOptionCode, describeOptionCodes, groupOptions, type OptionGroupKey } from "@/lib/option-groups";

interface CreateQueueDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialPlanCode?: string;
  initialOptions?: string;
}

export function CreateQueueDialog({
  open,
  onOpenChange,
  initialPlanCode,
  initialOptions,
}: CreateQueueDialogProps) {
  const servers = useServers();
  const availQ = useAvailability();
  const variantIndex = useMemo(() => buildVariantIndex(availQ.data), [availQ.data]);
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);
  const create = useCreateQueueItem();
  const settingsQ = useSettings();
  const cfgDefault = settingsQ.data?.defaultRetryInterval || RETRY_INTERVAL.defaultTask;

  const [accountId, setAccountId] = useState("");
  const [planCode, setPlanCode] = useState(initialPlanCode || "");
  const [datacenters, setDatacenters] = useState<string[]>([]);
  const [quantity, setQuantity] = useState("1");
  const [retryInterval, setRetryInterval] = useState("");
  const intervalTouchedRef = useRef(false);

  useEffect(() => {
    if (!intervalTouchedRef.current) setRetryInterval(String(cfgDefault));
  }, [cfgDefault]);

  const [picked, setPicked] = useState<Partial<Record<OptionGroupKey, string>>>({});
  const [extraInput, setExtraInput] = useState("");
  const [autoPay, setAutoPay] = useState(false);

  useEffect(() => {
    if (open) {
      if (initialPlanCode) setPlanCode(initialPlanCode);
    }
  }, [initialPlanCode, open]);

  const matchedServer = useMemo(
    () => (servers.data || []).find((s) => s.planCode === planCode.trim()),
    [servers.data, planCode]
  );

  const prevPlanCodeRef = useRef("");
  useEffect(() => {
    const code = planCode.trim();
    if (code === prevPlanCodeRef.current) return;
    prevPlanCodeRef.current = code;
    if (initialOptions && code === (initialPlanCode || "").trim()) {
      const wantedList = initialOptions.split(",").map((v) => v.trim()).filter(Boolean);
      const consumed = new Set<string>();
      const next: Partial<Record<OptionGroupKey, string>> = {};
      const groupedMap = matchedServer ? groupOptions(matchedServer.availableOptions) : null;
      if (groupedMap) {
        for (const g of Object.keys(groupedMap) as OptionGroupKey[]) {
          const hit = groupedMap[g].find((o) => wantedList.includes(o.value));
          if (hit) {
            next[g] = hit.value;
            consumed.add(hit.value);
          }
        }
      }
      setPicked(next);
      const leftover = wantedList.filter((v) => !consumed.has(v));
      setExtraInput(leftover.join(", "));
    } else {
      setPicked({});
      setExtraInput("");
    }
  }, [planCode, initialOptions, initialPlanCode, matchedServer]);

  const grouped = useMemo(
    () => (matchedServer ? groupOptions(matchedServer.availableOptions) : null),
    [matchedServer]
  );
  const defaultValueSet = useMemo(
    () => new Set((matchedServer?.defaultOptions || []).map((o) => o.value)),
    [matchedServer]
  );

  const parsedOptions = useMemo(() => {
    if (matchedServer) {
      return Object.values(picked).filter(Boolean) as string[];
    }
    return extraInput
      .split(",")
      .map((v) => v.trim())
      .filter(Boolean);
  }, [matchedServer, picked, extraInput]);

  const variants = matchedServer ? variantIndex[matchedServer.planCode] : undefined;
  const optionHasStock = (groupKey: OptionGroupKey, value: string): boolean => {
    if (groupKey === "bandwidth" || groupKey === "vrack" || groupKey === "cpu" || groupKey === "other") {
      return true;
    }
    return hasStockWithOption(
      variants,
      picked as Record<string, string>,
      groupKey,
      value,
      datacenters.length > 0 ? datacenters : undefined
    );
  };

  const qty = Number(quantity) || 1;
  const totalTasks = datacenters.length * qty;
  const canSubmit = !!accountId && planCode.trim().length > 0 && datacenters.length > 0 && qty > 0;

  const reset = () => {
    setPlanCode("");
    setDatacenters([]);
    setQuantity("1");
    intervalTouchedRef.current = false;
    setRetryInterval(String(cfgDefault));
    setPicked({});
    setExtraInput("");
    setAutoPay(false);
    prevPlanCodeRef.current = "";
  };

  const handleClose = () => {
    if (create.isPending) return;
    onOpenChange(false);
  };

  const dcAvailability = useMemo(
    () =>
      mergeDcAvailability(
        matchedServer?.datacenters,
        matchedServer ? availMap[matchedServer.planCode] : undefined
      ),
    [matchedServer, availMap]
  );

  const handleSubmit = async () => {
    if (!canSubmit) {
      toast.error("请填写计划代码并至少选择一个数据中心");
      return;
    }
    const isCustom = Boolean(
      planCode.trim() && !servers.data?.some((s) => s.planCode.toLowerCase() === planCode.trim().toLowerCase())
    );
    const result = await create.mutateAsync({
      account_id: accountId,
      planCode: planCode.trim(),
      datacenters,
      quantity: qty,
      retryInterval: Number(retryInterval) || cfgDefault,
      options: parsedOptions,
      autoPay,
      force: isCustom,
    });
    if (result.success > 0) {
      toast.success(`已创建 ${result.success}/${result.total} 个抢购任务`);
    }
    if (result.failed > 0 && !result.firstError) {
      toast.error(`${result.failed} 个任务创建失败`);
    }
    if (result.success > 0) {
      reset();
      onOpenChange(false);
    }
  };

  const isCustomPlan = Boolean(
    planCode.trim() && !servers.data?.some((s) => s.planCode.toLowerCase() === planCode.trim().toLowerCase())
  );

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>新建抢购任务</DialogTitle>
          <DialogDescription>
            为每个数据中心创建指定数量的独立任务，每台服务器单独成单。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-2">
          <div>
            <label className="block text-[13px] font-medium mb-1.5">OVH 账户 *</label>
            <AccountSelect value={accountId} onChange={setAccountId} />
            <p className="text-[11px] text-muted-foreground mt-1">下单时用该账户的凭据,购物车 subsidiary 跟随账户 zone</p>
          </div>

          <div>
            <label className="block text-[13px] font-medium mb-1.5">服务器计划代码</label>
            <PlanCodeCombobox
              value={planCode}
              onChange={setPlanCode}
              servers={servers.data || []}
              placeholder="输入或搜索型号，例如 24ska01"
            />
            {matchedServer && (
              <p className="text-[11px] text-muted-foreground mt-1 truncate">
                {matchedServer.cpu} · {matchedServer.memory} · {matchedServer.storage}
              </p>
            )}
            {isCustomPlan && (
              <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-[12px] text-amber-200/90 flex items-start gap-2 mt-1.5">
                <span className="text-amber-400 font-bold shrink-0">自定义型号:</span>
                <span className="text-muted-foreground text-[11px] leading-relaxed">
                  当前型号未在已知官方目录中收录，已启用强制创建模式。
                </span>
              </div>
            )}
          </div>

          <DatacenterPicker
            value={datacenters}
            onChange={setDatacenters}
            availability={dcAvailability}
          />

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-[13px] font-medium mb-1.5">每个数据中心数量</label>
              <Input
                type="text"
                inputMode="numeric"
                value={quantity}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === "" || /^\d*$/.test(v)) setQuantity(v);
                }}
                placeholder="默认: 1"
              />
            </div>
            <div>
              <label className="block text-[13px] font-medium mb-1.5">重试间隔（秒）</label>
              <Input
                type="text"
                inputMode="numeric"
                value={retryInterval}
                onChange={(e) => {
                  const v = e.target.value;
                  intervalTouchedRef.current = true;
                  if (v === "" || /^\d*$/.test(v)) setRetryInterval(v);
                }}
                placeholder={`默认: ${cfgDefault}`}
              />
            </div>
          </div>

          <div>
            <label className="block text-[13px] font-medium mb-1.5">
              可选配置
              <span className="text-muted-foreground ml-2 font-normal">
                {grouped
                  ? "（点击 chip 选择,留空走 OVH 默认下单）"
                  : "（catalog 里没找到这个型号,需要手填 addon planCode）"}
              </span>
            </label>
            {grouped ? (
              <div className="space-y-4">
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
                      onPick={(value) =>
                        setPicked((p) => ({
                          ...p,
                          [g]: p[g] === value ? "" : value,
                        }))
                      }
                    />
                  ))}
              </div>
            ) : (
              <Input
                placeholder="addon planCode,逗号分隔。例如:ram-64g-ecc-2400, softraid-2x450nvme-24sk50"
                value={extraInput}
                onChange={(e) => setExtraInput(e.target.value)}
              />
            )}

            {parsedOptions.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mt-3 pt-3 border-t border-border">
                <span className="text-[11px] text-muted-foreground">已选:</span>
                {parsedOptions.map((opt, i) => (
                  <Chip key={`${opt}-${i}`} tone="default" title={opt}>
                    {describeOptionCode(opt)}
                  </Chip>
                ))}
              </div>
            )}
          </div>

          <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
            <Checkbox checked={autoPay} onCheckedChange={(v) => setAutoPay(!!v)} />
            <div>
              <div className="text-sm">抢到后自动付款</div>
              <p className="text-[11px] text-muted-foreground">默认关闭。开启后用 OVH 账户默认支付方式扣款。</p>
            </div>
          </label>

          {datacenters.length > 0 && (
            <div className="border border-border rounded-2xl p-3 text-[12px] text-muted-foreground">
              将创建 <span className="font-semibold text-foreground">{totalTasks}</span> 个独立任务
              （{datacenters.length} 个数据中心 × {qty} 台
              {parsedOptions.length > 0 ? ` · ${describeOptionCodes(parsedOptions)}` : ""}）
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={create.isPending}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || create.isPending}>
            {create.isPending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                创建中...
              </>
            ) : (
              <>
                <Plus className="w-4 h-4" />
                {datacenters.length > 0 ? `创建 ${totalTasks} 个任务` : "创建任务"}
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
