import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { RefreshCw, CheckCircle2, AlertTriangle, Send } from "lucide-react";
import { LoadFailedBanner } from "@/components/common/LoadFailed";
import { Section } from "./Section";
import { Field } from "./Field";
import { useNotifyChannels, useTestNotification } from "@/hooks/use-notify-channels";
import { cn } from "@/lib/utils";
import type { SettingsConfig } from "@/hooks/use-settings";

interface NotifySectionProps {
  form: SettingsConfig;
  set: (k: keyof SettingsConfig, v: string) => void;
}

export function NotifySection({ form, set }: NotifySectionProps) {
  const { t } = useTranslation();
  const channels = useNotifyChannels(true);
  const test = useTestNotification();

  return (
    <Section title={t("settings.notify.title")}>
      <div className="rounded-2xl border border-border p-4 space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-[13px] font-medium">{t("settings.notify.currentStatus")}</h3>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => channels.refetch()}
            disabled={channels.isFetching}
          >
            <RefreshCw className={cn("w-3.5 h-3.5", channels.isFetching && "animate-spin")} />
            {t("settings.notify.recheck")}
          </Button>
        </div>
        {channels.isPending ? (
          <p className="text-[12px] text-muted-foreground">{t("settings.checking")}</p>
        ) : channels.isError ? (
          <LoadFailedBanner
            title={t("settings.notify.detectFailedTitle")}
            error={channels.error}
            onRetry={() => channels.refetch()}
          />
        ) : (
          <div className="space-y-1.5">
            {(channels.data?.channels || []).map((c) => (
              <div key={c.name} className="flex items-start gap-2 text-[12px]">
                {!c.configured ? (
                  <span className="mt-1.5 w-1.5 h-1.5 rounded-full bg-muted-foreground/40 flex-shrink-0" />
                ) : c.ok ? (
                  <CheckCircle2 className="w-3.5 h-3.5 text-success flex-shrink-0 mt-0.5" />
                ) : (
                  <AlertTriangle className="w-3.5 h-3.5 text-warning flex-shrink-0 mt-0.5" />
                )}
                <span className="font-medium w-20 flex-shrink-0">{c.name}</span>
                <span className="text-muted-foreground break-all">
                  {!c.configured ? t("settings.notify.notConfigured") : c.ok ? t("settings.notify.available") : c.detail || t("settings.notify.unavailable")}
                </span>
              </div>
            ))}
            {channels.data && !channels.data.anyAvailable && (
              <p className="text-[11px] text-warning pt-1">
                {t("settings.notify.noneAvailable")}
              </p>
            )}
          </div>
        )}
      </div>

      <Field label={t("settings.notify.webhookLabel")}>
        <Input
          value={form.notifyWebhookUrl || ""}
          onChange={(e) => set("notifyWebhookUrl", e.target.value)}
          placeholder={t("settings.notify.webhookPlaceholder")}
        />
        <p className="text-[11px] text-muted-foreground mt-1">
          {t("settings.notify.webhookHintPre")}<b>{t("settings.notify.webhookHintBold")}</b>{t("settings.notify.webhookHintMid")}
          <code className="mx-1 px-1 rounded bg-muted">text</code>
          <code className="mr-1 px-1 rounded bg-muted">message</code>
          <code className="mr-1 px-1 rounded bg-muted">text_content.text</code>
          {t("settings.notify.webhookHintPost")}
        </p>
      </Field>

      <div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => test.mutate()}
          disabled={test.isPending}
        >
          <Send className={cn("w-3.5 h-3.5", test.isPending && "animate-pulse")} />
          {test.isPending ? t("settings.notify.sending") : t("settings.notify.sendTest")}
        </Button>
        <p className="text-[11px] text-muted-foreground mt-1.5">
          {t("settings.notify.testHint")}
        </p>
      </div>
    </Section>
  );
}
