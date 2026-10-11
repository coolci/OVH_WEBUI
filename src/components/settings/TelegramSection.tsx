import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/common/Skeleton";
import { Chip } from "@/components/common/Chip";
import { Radio, RefreshCw, CheckCircle2, AlertTriangle } from "lucide-react";
import { LoadFailedBanner } from "@/components/common/LoadFailed";
import { Section } from "./Section";
import { Field } from "./Field";
import { useTelegramPoller, type SettingsConfig } from "@/hooks/use-settings";
import { fmtDateTime } from "@/i18n/format";
import { cn } from "@/lib/utils";

interface TelegramSectionProps {
  form: SettingsConfig;
  set: (k: keyof SettingsConfig, v: string) => void;
}

function isPollConflict(err: string): boolean {
  return err.includes("Conflict") || err.includes("409");
}

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex justify-between items-start gap-3">
      <span className="text-muted-foreground flex-shrink-0">{label}</span>
      <span className="font-medium text-right min-w-0">{value}</span>
    </div>
  );
}

export function TelegramSection({ form, set }: TelegramSectionProps) {
  const { t } = useTranslation();
  const poll = useTelegramPoller();

  const poller = poll.data?.poller;
  const hasToken = poll.data?.hasToken === true;

  return (
    <Section title={t("settings.telegram.title")}>
      <div className="rounded-2xl border border-border p-4 space-y-2.5 text-[13px]">
        <div className="flex items-center justify-between">
          <h3 className="text-[13px] font-medium flex items-center gap-1.5">
            <Radio className="w-3.5 h-3.5 text-muted-foreground" />
            {t("settings.telegram.pollerTitle")}
          </h3>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => poll.refetch()}
            disabled={poll.isFetching}
          >
            <RefreshCw className={cn("w-3.5 h-3.5", poll.isFetching && "animate-spin")} />
            {t("common.refresh")}
          </Button>
        </div>
        <p className="text-[11px] text-muted-foreground">
          {t("settings.telegram.pollerDescPre")}<b>{t("settings.telegram.pollerDescBold")}</b>{t("settings.telegram.pollerDescPost")}
        </p>

        {poll.isPending ? (
          <Skeleton className="h-20 rounded-xl" />
        ) : poll.isError ? (
          <LoadFailedBanner
            title={t("settings.telegram.statusFailedTitle")}
            error={poll.error}
            onRetry={() => poll.refetch()}
          />
        ) : !hasToken ? (
          <p className="text-[12px] text-warning">
            {t("settings.telegram.noTokenWarning")}
          </p>
        ) : !poller ? (
          <p className="text-[12px] text-muted-foreground">
            {t("settings.telegram.noPollerInfo")}
          </p>
        ) : (
          <>
            <InfoRow
              label={t("settings.telegram.runningState")}
              value={
                poller.running ? (
                  <Chip tone="success">
                    <CheckCircle2 className="w-3 h-3" />
                    {t("settings.telegram.running")}
                  </Chip>
                ) : (
                  <Chip tone="danger">
                    <AlertTriangle className="w-3 h-3" />
                    {t("settings.telegram.stopped")}
                  </Chip>
                )
              }
            />
            <InfoRow
              label={t("settings.telegram.lastPoll")}
              value={
                poller.lastPollAt ? (
                  <span className="font-mono text-[12px]">
                    {fmtDateTime(poller.lastPollAt)}
                  </span>
                ) : (
                  <span className="text-muted-foreground">{t("settings.telegram.neverPolled")}</span>
                )
              }
            />
            <InfoRow
              label={t("settings.telegram.confirmedOffset")}
              value={<span className="font-mono text-[12px]">{poller.offset}</span>}
            />
            {poller.lastError ? (
              <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2 text-[12px] flex items-start gap-2">
                <AlertTriangle className="w-4 h-4 text-destructive flex-shrink-0 mt-0.5" />
                <div className="min-w-0">
                  <p className="font-semibold text-destructive">{t("settings.telegram.lastErrorTitle")}</p>
                  <p className="mt-0.5 break-words">{poller.lastError}</p>
                  {isPollConflict(poller.lastError) && (
                    <p className="mt-1.5 text-destructive">
                      {t("settings.telegram.conflictPre")}<b>{t("settings.telegram.conflictBold")}</b>{t("settings.telegram.conflictPost")}
                    </p>
                  )}
                </div>
              </div>
            ) : (
              <InfoRow
                label={t("settings.telegram.errorState")}
                value={
                  <Chip tone="success">
                    <CheckCircle2 className="w-3 h-3" />
                    {t("settings.telegram.normal")}
                  </Chip>
                }
              />
            )}
            {!poller.running && (
              <p className="text-[11px] text-destructive">
                {t("settings.telegram.notRunningWarning")}
              </p>
            )}
          </>
        )}
      </div>

      <Field label="Bot Token">
        <Input
          type="password"
          value={form.tgToken || ""}
          onChange={(e) => set("tgToken", e.target.value)}
          placeholder="123456:ABCdef..."
        />
      </Field>
      <Field label="Chat ID">
        <Input
          value={form.tgChatId || ""}
          onChange={(e) => set("tgChatId", e.target.value)}
          placeholder="-1001234567890"
        />
      </Field>
    </Section>
  );
}
