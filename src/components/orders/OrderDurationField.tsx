import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  durationMonths,
  FALLBACK_ORDER_DURATIONS,
  formatDuration,
  isDefaultDuration,
  normalizeDuration,
  supportsDuration,
} from "@/lib/order-duration";
import type { OrderDurationsQuery } from "@/hooks/use-order-durations";

/** 列表/详情里展示购买时长;月付(默认)时不占位,只有非默认才显示 */
export function OrderDurationBadge({ value, showDefault = false }: { value?: string | null; showDefault?: boolean }) {
  const { t } = useTranslation();
  if (!showDefault && isDefaultDuration(value)) return null;
  return (
    <span className="inline-flex items-center rounded border border-amber-500/40 bg-amber-500/10 px-1.5 py-0.5 text-[11px] font-medium text-amber-700 dark:text-amber-300">
      {formatDuration(t, value)}
    </span>
  );
}

/** 购买时长选择框。可选项来自 OVH 实际报价,查不到时退回常见周期并提示下单前会核验。 */
export function OrderDurationField({
  value,
  onChange,
  query,
  hint,
}: {
  value: string;
  onChange: (value: string) => void;
  query: OrderDurationsQuery;
  hint?: string;
}) {
  const { t } = useTranslation();
  const id = useId();
  const current = normalizeDuration(value);
  const offered = query.data;
  const valid = supportsDuration(offered, current);
  const choices = offered?.length ? offered.map(normalizeDuration) : FALLBACK_ORDER_DURATIONS;
  const options = [...new Set([...choices, current])].sort((a, b) => durationMonths(a) - durationMonths(b));
  const status = !valid
    ? t("orderDuration.unsupported")
    : query.isFetching
      ? t("orderDuration.loading")
      : query.isError || !offered?.length
        ? t("orderDuration.unverified")
        : hint || t("orderDuration.hint");
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="block text-[13px] font-medium">
        {t("orderDuration.label")}
      </label>
      <Select value={current} onValueChange={onChange}>
        <SelectTrigger id={id} aria-describedby={`${id}-hint`} aria-invalid={!valid} className="h-9">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option} value={option} disabled={!supportsDuration(offered, option)}>
              {formatDuration(t, option)}
              {isDefaultDuration(option) ? ` · ${t("orderDuration.default")}` : ""}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p
        id={`${id}-hint`}
        role={!valid ? "alert" : "status"}
        className={`text-xs leading-relaxed ${!valid ? "text-destructive" : "text-muted-foreground"}`}
      >
        {status}
      </p>
      {durationMonths(current) > 1 && (
        <p className="text-xs leading-relaxed text-amber-700 dark:text-amber-300">{t("orderDuration.prepaid")}</p>
      )}
    </div>
  );
}
