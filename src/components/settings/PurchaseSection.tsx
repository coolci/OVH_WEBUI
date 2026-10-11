import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Section } from "./Section";
import { Field } from "./Field";
import { RETRY_INTERVAL, type SettingsConfig } from "@/hooks/use-settings";

interface PurchaseSectionProps {
  form: SettingsConfig;
  set: <K extends keyof SettingsConfig>(k: K, v: SettingsConfig[K]) => void;
}

/**
 * 抢购参数。
 * 统一设置默认重试间隔与快速下单间隔。
 */
export function PurchaseSection({ form, set }: PurchaseSectionProps) {
  const { t } = useTranslation();

  const numField = (k: "defaultRetryInterval" | "quickOrderRetryInterval", fallback: number) => (
    <Input
      type="text"
      inputMode="numeric"
      value={form[k] === undefined ? "" : String(form[k])}
      placeholder={t("settings.purchase.defaultPlaceholder", { value: fallback })}
      onChange={(e) => {
        const v = e.target.value;
        if (v === "") return set(k, undefined);
        if (/^\d+$/.test(v)) set(k, Number(v));
      }}
    />
  );

  const invalid = (v?: number) =>
    v !== undefined && (v < RETRY_INTERVAL.min || v > RETRY_INTERVAL.max);

  return (
    <Section title={t("settings.purchase.title")}>
      <Field
        label={t("settings.purchase.defaultRetryLabel")}
        hint={t("settings.purchase.defaultRetryHint", {
          default: RETRY_INTERVAL.defaultTask,
          min: RETRY_INTERVAL.min,
          max: RETRY_INTERVAL.max,
        })}
      >
        {numField("defaultRetryInterval", RETRY_INTERVAL.defaultTask)}
        {invalid(form.defaultRetryInterval) && (
          <p className="text-[11px] text-destructive mt-1">
            {t("settings.purchase.rangeInvalid", { min: RETRY_INTERVAL.min, max: RETRY_INTERVAL.max })}
          </p>
        )}
      </Field>

      <Field
        label={t("settings.purchase.quickRetryLabel")}
        hint={t("settings.purchase.quickRetryHint", { default: RETRY_INTERVAL.defaultQuick })}
      >
        {numField("quickOrderRetryInterval", RETRY_INTERVAL.defaultQuick)}
        {invalid(form.quickOrderRetryInterval) && (
          <p className="text-[11px] text-destructive mt-1">
            {t("settings.purchase.rangeInvalid", { min: RETRY_INTERVAL.min, max: RETRY_INTERVAL.max })}
          </p>
        )}
      </Field>

      <div className="rounded-2xl border border-border bg-secondary/30 px-4 py-3 text-[12px] text-muted-foreground">
        {t("settings.purchase.onlyNewPre")}<b className="text-foreground">{t("settings.purchase.onlyNewBold")}</b>{t("settings.purchase.onlyNewPost")}
      </div>
    </Section>
  );
}
