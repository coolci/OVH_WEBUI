import { useState, useEffect, useMemo, useRef } from "react";
import { Loader2, Save } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
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
import { AccountChip } from "@/components/common/AccountChip";
import { OptionGroupSection } from "@/components/common/OptionGroupSection";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useServers } from "@/hooks/use-servers";
import { useUpdateQueueItem, type QueueItem } from "@/hooks/use-queue";
import { useOrderDurations } from "@/hooks/use-order-durations";
import { OrderDurationField } from "@/components/orders/OrderDurationField";
import { DEFAULT_ORDER_DURATION, normalizeDuration, supportsDuration } from "@/lib/order-duration";
import {
  useAvailability,
  buildVariantIndex,
  hasStockWithOption,
} from "@/hooks/use-availability";
import { useAccounts, findAccountByID } from "@/hooks/use-accounts";
import { OVH_DATACENTERS, apiDatacenterCode, displayDatacenterCode } from "@/lib/datacenters";
import { describeOptionCode, groupOptions, type OptionGroupKey } from "@/lib/option-groups";
import { parseOptionCodes } from "@/lib/order-inputs";
import { mergeStockPlans } from "@/lib/stock-plans";
import { preorderOptions, preorderVariantCoversAll } from "@/lib/preorder-options";
import { AutoPayCheck } from "@/components/orders/AutoPayCheck";

interface EditQueueDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  item: QueueItem | null;
}

export function EditQueueDialog({ open, onOpenChange, item }: EditQueueDialogProps) {
  const { t } = useTranslation();
  const update = useUpdateQueueItem();
  const { data: accounts } = useAccounts();

  const accountId = item?.accountId || "";
  const activeAcc = findAccountByID(accounts, accountId);
  const servers = useServers(true, accountId);
  const availQ = useAvailability(activeAcc?.endpoint || undefined);
  const variantIndex = useMemo(() => buildVariantIndex(availQ.data), [availQ.data]);

  const [datacenter, setDatacenter] = useState("");
  const [retryInterval, setRetryInterval] = useState("");
  const [autoPay, setAutoPay] = useState(false);
  const [coupon, setCoupon] = useState("");
  const [duration, setDuration] = useState(DEFAULT_ORDER_DURATION);
  const durations = useOrderDurations(accountId, item?.planCode, open);
  const [picked, setPicked] = useState<Partial<Record<OptionGroupKey, string>>>({});
  const [extraInput, setExtraInput] = useState("");

  const searchServers = useMemo(
    () => mergeStockPlans(servers.data || [], availQ.data),
    [servers.data, availQ.data]
  );

  const matchedServer = useMemo(
    () => searchServers.find((s) => s.planCode === (item?.planCode || "").trim()),
    [searchServers, item?.planCode]
  );

  const preorder = Boolean(item?.preorder || matchedServer?.prerelease);

  const initKeyRef = useRef<string>("");
  useEffect(() => {
    if (!open || !item) {
      initKeyRef.current = "";
      return;
    }
    const nextKey = `${item.id}:${matchedServer ? "matched" : "raw"}:${preorder ? "pre" : "std"}`;
    if (initKeyRef.current === nextKey) return;
    const isFirstInitForItem = !initKeyRef.current.startsWith(`${item.id}:`);
    initKeyRef.current = nextKey;

    if (isFirstInitForItem) {
      // 将后端存储的 API 码（如 ynm、ca-east-tor-a、eu-west-par-a）还原为显示短码，避免下拉框空白
      setDatacenter(displayDatacenterCode(item.datacenter));
      setRetryInterval(String(item.retryInterval || 60));
      setAutoPay(!!item.autoPay);
      setCoupon(item.coupon || "");
      setDuration(normalizeDuration(item.duration));
    }

    const groupedMap = matchedServer ? groupOptions(matchedServer.availableOptions) : null;
    const rawWanted = item.options || [];
    const wantedList = preorder
      ? preorderOptions(rawWanted, groupedMap?.cpu?.map((o) => o.value))
      : rawWanted;
    const consumed = new Set<string>();
    const next: Partial<Record<OptionGroupKey, string>> = {};
    if (groupedMap) {
      for (const g of Object.keys(groupedMap) as OptionGroupKey[]) {
        if (preorder && g === "cpu") continue;
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
  }, [open, item, matchedServer, preorder]);

  const grouped = useMemo(
    () => (matchedServer ? groupOptions(matchedServer.availableOptions) : null),
    [matchedServer]
  );
  const defaultValueSet = useMemo(
    () => new Set((matchedServer?.defaultOptions || []).map((o) => o.value)),
    [matchedServer]
  );

  const parsedOptions = useMemo(() => {
    const merged = matchedServer
      ? [...new Set([...(Object.values(picked).filter(Boolean) as string[]), ...parseOptionCodes(extraInput)])]
      : parseOptionCodes(extraInput);
    return preorder ? preorderOptions(merged, grouped?.cpu?.map((o) => o.value)) : merged;
  }, [matchedServer, picked, extraInput, preorder, grouped]);

  const variants = matchedServer ? variantIndex[matchedServer.planCode] : undefined;
  const preorderMismatch =
    preorder &&
    parsedOptions.length > 0 &&
    !!variants?.length &&
    !variants.some((v) => preorderVariantCoversAll(v.fqn, parsedOptions));
  const durationValid = supportsDuration(durations.data, duration);

  const optionHasStock = (groupKey: OptionGroupKey, value: string): boolean => {
    if (groupKey === "bandwidth" || groupKey === "vrack" || groupKey === "cpu" || groupKey === "other") {
      return true;
    }
    return hasStockWithOption(
      variants,
      picked as Record<string, string>,
      groupKey,
      value,
      datacenter ? [datacenter] : undefined
    );
  };

  const handleClose = () => {
    if (update.isPending) return;
    onOpenChange(false);
  };

  const handleSubmit = async () => {
    if (!item) return;
    if (!datacenter) {
      toast.error("请选择数据中心");
      return;
    }
    if (preorderMismatch) {
      toast.error("库存记录中没有这套配置组合，请重新选择内存或存储");
      return;
    }
    if (!durationValid) {
      toast.error(t("orderDuration.unsupported"));
      return;
    }
    const intervalNum = Number(retryInterval) || 60;
    try {
      await update.mutateAsync({
        id: item.id,
        datacenter: apiDatacenterCode(datacenter),
        options: parsedOptions,
        retryInterval: intervalNum,
        autoPay,
        coupon: coupon.trim(),
        duration,
      });
      onOpenChange(false);
    } catch {
      // Toast handled by mutation
    }
  };

  if (!item) return null;

  const isCustomDatacenter =
    Boolean(datacenter) && !OVH_DATACENTERS.some((dc) => dc.code === datacenter);

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("queue.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("queue.editDesc")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-5 py-2">
          {/* Identity: locked info */}
          <div className="rounded-xl border border-border/60 bg-muted/30 p-3 flex flex-col gap-2">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="font-mono font-semibold text-sm text-foreground">{item.planCode}</span>
              <AccountChip accountId={item.accountId} />
              {preorder && (
                <Chip tone="warning">预订抢购</Chip>
              )}
            </div>
            <p className="text-[11px] text-muted-foreground">
              {t("queue.editIdentity")}
            </p>
          </div>

          {/* Datacenter selector */}
          <div>
            <label className="block text-[13px] font-medium mb-1.5">
              数据中心
            </label>
            <Select value={datacenter} onValueChange={setDatacenter}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder="选择数据中心" />
              </SelectTrigger>
              <SelectContent>
                {OVH_DATACENTERS.map((dc) => (
                  <SelectItem key={dc.code} value={dc.code}>
                    <span className="font-medium">{dc.code.toUpperCase()}</span>
                    <span className="text-muted-foreground ml-2">({dc.name} · {dc.region})</span>
                  </SelectItem>
                ))}
                {isCustomDatacenter && (
                  <SelectItem value={datacenter}>
                    <span className="font-medium">{datacenter.toUpperCase()}</span>
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
          </div>

          {/* Retry Interval */}
          <div>
            <label className="block text-[13px] font-medium mb-1.5">
              {t("queue.retryInterval")}
            </label>
            <Input
              type="text"
              inputMode="numeric"
              value={retryInterval}
              onChange={(e) => {
                const v = e.target.value;
                if (v === "" || /^\d*$/.test(v)) setRetryInterval(v);
              }}
              placeholder="默认: 60"
            />
            <p className="text-[11px] text-muted-foreground mt-1">
              {t("queue.retryHint")}
            </p>
          </div>

          {/* Options */}
          <div>
            <label className="block text-[13px] font-medium mb-1.5">
              {t("queue.optionsLabel")}
              <span className="text-muted-foreground ml-2 font-normal">
                {grouped
                  ? t("queue.optionsChipHint")
                  : t("queue.optionsNotFound")}
              </span>
            </label>
            {grouped ? (
              <div className="space-y-4">
                {(["cpu", "memory", "systemStorage", "storage", "bandwidth", "vrack", "other"] as OptionGroupKey[])
                  .filter((g) => grouped[g].length > 0 && !(preorder && g === "cpu"))
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
            ) : null}

            {/* 始终保留自定义选配输入框：确保不在当前标准目录中的存量自定义 addon 可见、可编辑、可清空 */}
            <div className={grouped ? "mt-3" : ""}>
              {grouped && (
                <label htmlFor="edit-queue-custom-options" className="mb-1 block text-[11px] text-muted-foreground">
                  自定义配置 planCode（可选）
                </label>
              )}
              <Input
                id="edit-queue-custom-options"
                placeholder={t("queue.extraOptionsPlaceholder")}
                value={extraInput}
                onChange={(e) => setExtraInput(e.target.value)}
              />
            </div>

            {preorderMismatch && (
              <p role="alert" className="text-[11px] text-destructive mt-1.5">
                库存记录中没有这套配置组合，请重新选择内存或存储。
              </p>
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

          <OrderDurationField value={duration} onChange={setDuration} query={durations} />

          {/* Coupon */}
          <div>
            <label className="block text-[13px] font-medium mb-1.5">{t("queue.couponLabel")}</label>
            <Input
              value={coupon}
              onChange={(e) => setCoupon(e.target.value.toUpperCase())}
              placeholder={t("queue.couponPlaceholder")}
            />
            <p className="text-[11px] text-muted-foreground mt-1">
              {t("queue.couponHint")}
            </p>
          </div>

          {/* AutoPay */}
          <label className="flex items-center gap-2.5 cursor-pointer rounded-xl border border-border px-3.5 py-2.5 hover:bg-muted/40 transition-colors">
            <Checkbox checked={autoPay} onCheckedChange={(v) => setAutoPay(!!v)} />
            <div>
              <div className="text-sm">{t("queue.autoPay")}</div>
              <p className="text-[11px] text-muted-foreground">
                {autoPay ? t("queue.autoPayOn") : t("queue.autoPayOff")}
              </p>
            </div>
          </label>
          <AutoPayCheck accountIds={[item?.accountId]} enabled={autoPay} />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={update.isPending}>
            {t("queue.cancel")}
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={update.isPending || !datacenter || preorderMismatch || !durationValid}
          >
            {update.isPending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                {t("queue.saving")}
              </>
            ) : (
              <>
                <Save className="w-4 h-4" />
                {t("queue.save")}
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
