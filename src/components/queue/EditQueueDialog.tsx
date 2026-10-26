import { useState, useEffect, useMemo } from "react";
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
import {
  useAvailability,
  buildAvailabilityMap,
  buildVariantIndex,
  hasStockWithOption,
} from "@/hooks/use-availability";
import { useAccounts, findAccountByID } from "@/hooks/use-accounts";
import { OVH_DATACENTERS, apiDatacenterCode } from "@/lib/datacenters";
import { describeOptionCode, describeOptionCodes, groupOptions, type OptionGroupKey } from "@/lib/option-groups";

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
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);

  const [datacenter, setDatacenter] = useState("");
  const [retryInterval, setRetryInterval] = useState("");
  const [autoPay, setAutoPay] = useState(false);
  const [coupon, setCoupon] = useState("");
  const [picked, setPicked] = useState<Partial<Record<OptionGroupKey, string>>>({});
  const [extraInput, setExtraInput] = useState("");

  const matchedServer = useMemo(
    () => (servers.data || []).find((s) => s.planCode === (item?.planCode || "").trim()),
    [servers.data, item?.planCode]
  );

  useEffect(() => {
    if (open && item) {
      setDatacenter(item.datacenter.toLowerCase());
      setRetryInterval(String(item.retryInterval || 60));
      setAutoPay(!!item.autoPay);
      setCoupon(item.coupon || "");

      const wantedList = item.options || [];
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
    }
  }, [open, item, matchedServer]);

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
      return [...new Set([...(Object.values(picked).filter(Boolean) as string[]), ...extraInput.split(",").map((v) => v.trim()).filter(Boolean)])];
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
    const intervalNum = Number(retryInterval) || 60;
    try {
      await update.mutateAsync({
        id: item.id,
        datacenter: apiDatacenterCode(datacenter),
        options: parsedOptions,
        retryInterval: intervalNum,
        autoPay,
        coupon: coupon.trim(),
      });
      onOpenChange(false);
    } catch {
      // Toast handled by mutation
    }
  };

  if (!item) return null;

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
              {item.preorder && (
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
                placeholder={t("queue.extraOptionsPlaceholder")}
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
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={handleClose} disabled={update.isPending}>
            {t("queue.cancel")}
          </Button>
          <Button onClick={handleSubmit} disabled={update.isPending || !datacenter}>
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
