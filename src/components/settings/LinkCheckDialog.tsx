import { useTranslation } from "react-i18next";
import { fmtDateTime } from "@/i18n/format";
import {
  Activity,
  AlertTriangle,
  Fingerprint,
  Network,
  RefreshCw,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { Skeleton } from "@/components/common/Skeleton";
import { LoadFailedBanner } from "@/components/common/LoadFailed";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import {
  type OVHAccount,
  type ProxyProbeTarget,
  type LatencyLevel,
  useProxyCheck,
  useLastProxyCheck,
  worstMinMs,
  gradeLatency,
  isJittery,
  accountChipColor,
  proxyCheckTime,
} from "@/hooks/use-accounts";

const latencyText: Record<LatencyLevel, string> = {
  fast: "text-success",
  slow: "text-warning",
  bad: "text-destructive",
};

const latencyBox: Record<LatencyLevel, string> = {
  fast: "border-success/40 bg-success/5",
  slow: "border-warning/40 bg-warning/5",
  bad: "border-destructive/40 bg-destructive/5",
};

const latencyLabel: Record<LatencyLevel, string> = {
  fast: "settings.accounts.latency.fast",
  slow: "settings.accounts.latency.slow",
  bad: "settings.accounts.latency.bad",
};

function ProbeRow({ t }: { t: ProxyProbeTarget }) {
  const { t: tt } = useTranslation();
  const g = t.ok && typeof t.minMs === "number" ? gradeLatency(t.minMs) : null;
  return (
    <div className="space-y-0.5">
      <div className="flex items-center gap-2">
        <StatusDot tone={t.ok ? "success" : "danger"} />
        <span className="text-[12px] font-medium truncate" title={t.url}>
          {t.name}
        </span>
        <span className="flex-1 min-w-[8px] border-b border-dashed border-border" />
        {t.ok ? (
          <span className="text-[11px] font-mono whitespace-nowrap">
            {tt("settings.accounts.probeMin")} <b className={g ? latencyText[g.level] : undefined}>{t.minMs}ms</b>
            <span className="text-muted-foreground">{tt("settings.accounts.probeAvg")}{t.avgMs}ms</span>
          </span>
        ) : (
          <span className="text-[11px] text-destructive whitespace-nowrap">{tt("settings.accounts.probeNoResponse")}</span>
        )}
        {t.status ? (
          <Chip className="whitespace-nowrap">HTTP {t.status}</Chip>
        ) : null}
      </div>
      {!t.ok && t.error && <p className="pl-4 text-[11px] text-destructive break-all">{t.error}</p>}
      {t.ok && t.error && (
        <p className="pl-4 text-[11px] text-warning break-all">
          {tt("settings.accounts.probeJitter", { error: t.error })}
        </p>
      )}
    </div>
  );
}

export function LinkCheckDialog({
  acc,
  open,
  onOpenChange,
  check,
}: {
  acc: OVHAccount;
  open: boolean;
  onOpenChange: (v: boolean) => void;
  check: ReturnType<typeof useProxyCheck>;
}) {
  const { t } = useTranslation();
  const record = useLastProxyCheck(acc.id).data;
  const done = record && record.success ? record : null;
  const run = () => check.mutate(acc.id);

  const targets = done?.targets || [];
  const down = targets.filter((tg) => !tg.ok);
  const worst = worstMinMs(targets);
  const grade = worst === undefined ? null : gradeLatency(worst);
  const jittery = targets.filter(isJittery);
  const staleCfg = !!done && done.proxy !== (acc.proxyUrl || "");

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Activity className="w-4 h-4" />
            {t("settings.accounts.linkCheckTitle", { name: acc.name })}
          </DialogTitle>
          <DialogDescription>
            {t("settings.accounts.linkCheckDescPre")}<b>{t("settings.accounts.linkCheckDescBold")}</b>{t("settings.accounts.linkCheckDescPost")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 py-1 max-h-[65vh] overflow-y-auto -mx-6 px-6">
          <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1.5">
            <div className="flex items-center gap-1.5 flex-wrap">
              <span className="font-semibold text-[13px]">{done ? done.accountName : acc.name}</span>
              <span
                className={cn(
                  "inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium",
                  accountChipColor(done ? done.region : acc.zone)
                )}
              >
                {done ? t("settings.accounts.regionChip", { region: done.region }) : acc.zone}
              </span>
              {(done ? done.usingProxy : !!acc.proxyUrl) ? (
                <Chip tone="info">
                  <Network className="w-3 h-3" />
                  <span className="font-mono">{(done ? done.proxy : acc.proxyUrl) || t("settings.accounts.proxyFallback")}</span>
                </Chip>
              ) : (
                <Chip>{t("settings.accounts.direct")}</Chip>
              )}
              <Chip>
                <Fingerprint className="w-3 h-3" />
                {done ? done.fingerprint : acc.fingerprint || "default"}
              </Chip>
            </div>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {done
                ? t("settings.accounts.regionScopeNote", { region: done.region })
                : t("settings.accounts.savedRouteNote")}
            </p>
            {staleCfg && (
              <p className="text-[11px] text-warning">
                {t("settings.accounts.staleCfgPre")}
                <span className="font-mono">{done?.proxy || t("settings.accounts.direct")}</span>
                {t("settings.accounts.staleCfgPost")}
              </p>
            )}
          </div>

          {check.isError && (
            <LoadFailedBanner
              title={t("settings.accounts.linkCheckReqFailedTitle")}
              error={check.error}
              onRetry={run}
            />
          )}

          {check.isPending ? (
            <div className="space-y-2">
              <p className="text-[11px] text-muted-foreground flex items-center gap-1.5">
                <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                {t("settings.accounts.linkCheckingNote")}
              </p>
              <Skeleton className="h-20 rounded-2xl" />
              <Skeleton className="h-24 rounded-2xl" />
            </div>
          ) : record && !record.success ? (
            <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2.5 space-y-1 text-[11px]">
              <p className="font-semibold text-destructive flex items-center gap-1.5">
                <AlertTriangle className="w-3.5 h-3.5" />
                {t("settings.accounts.backendCheckFailedTitle")}
              </p>
              <p className="text-muted-foreground break-all">{record.error}</p>
              <p className="text-muted-foreground">
                {t("settings.accounts.backendCheckFailedNote")}
              </p>
            </div>
          ) : done ? (
            <>
              <div
                className={cn(
                  "rounded-xl border px-3 py-2.5 space-y-1",
                  done.egressIP ? "border-success/40 bg-success/5" : "border-destructive/40 bg-destructive/5"
                )}
              >
                <p className="text-[11px] text-muted-foreground">{t("settings.accounts.egressIpLabel")}</p>
                {done.egressIP ? (
                  <>
                    <p className="text-2xl font-mono font-semibold tracking-tight break-all">{done.egressIP}</p>
                    <p className="text-[11px] text-muted-foreground">
                      {t("settings.accounts.compareIpHintFull")}
                    </p>
                  </>
                ) : (
                  <>
                    <p className="text-[13px] font-semibold text-destructive flex items-center gap-1.5">
                      <AlertTriangle className="w-3.5 h-3.5" />
                      {t("settings.accounts.noEgressIp")}
                    </p>
                    <p className="text-[11px] text-destructive break-all">{done.egressError || t("settings.accounts.egressNoReason")}</p>
                    <p className="text-[11px] text-muted-foreground">
                      {t("settings.accounts.egressIpProbeNote")}
                    </p>
                  </>
                )}
                {done.warning && <p className="text-[11px] text-warning">⚠ {done.warning}</p>}
              </div>

              <div className="space-y-2">
                <p className="text-[12px] font-semibold">{t("settings.accounts.targetsTitle")}</p>
                {targets.length === 0 ? (
                  <p className="text-[11px] text-muted-foreground">
                    {t("settings.accounts.noTargetsNote")}
                  </p>
                ) : (
                  <div className="space-y-1.5">
                    {targets.map((tg) => (
                      <ProbeRow key={tg.name + tg.url} t={tg} />
                    ))}
                  </div>
                )}

                {down.length > 0 && (
                  <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2 text-[11px] space-y-0.5">
                    <p className="font-semibold text-destructive">
                      {t("settings.accounts.downTitle", { count: down.length })}
                    </p>
                    <p className="text-muted-foreground">
                      {done.usingProxy
                        ? t("settings.accounts.downProxiedNote")
                        : t("settings.accounts.downDirectNote")}
                    </p>
                  </div>
                )}

                {grade && (
                  <div className={cn("rounded-xl border px-3 py-2 text-[11px] space-y-0.5", latencyBox[grade.level])}>
                    <p className={cn("font-semibold", latencyText[grade.level])}>
                      {t("settings.accounts.slowestLine", { ms: worst, label: t(latencyLabel[grade.level]) })}
                    </p>
                    <p className="text-muted-foreground">{grade.note}</p>
                  </div>
                )}

                {jittery.length > 0 && (
                  <div className="rounded-xl border border-warning/40 bg-warning/5 px-3 py-2 text-[11px] space-y-0.5">
                    <p className="font-semibold text-warning">
                      {t("settings.accounts.jitterTitle")}{jittery.map((tg) => tg.name).join(t("settings.accounts.listSeparator"))}
                    </p>
                    <p className="text-muted-foreground">
                      {t("settings.accounts.jitterNote")}
                    </p>
                  </div>
                )}
              </div>

              <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1 text-[11px] leading-relaxed text-muted-foreground">
                <p className="font-semibold text-foreground">{t("settings.accounts.howToReadTitle")}</p>
                <p>
                  {t("settings.accounts.howToReadPre")}<b>{t("settings.accounts.howToReadBold1")}</b>{t("settings.accounts.howToReadMid")}
                  <b>{t("settings.accounts.howToReadBold2")}</b>{t("settings.accounts.howToReadPost")}
                </p>
                <p>
                  {t("settings.accounts.howToReadP2")}
                </p>
              </div>
            </>
          ) : !check.isError ? (
            <div className="rounded-xl border border-border px-3 py-5 text-center space-y-1">
              <p className="text-[12px] font-medium">{t("settings.accounts.neverChecked")}</p>
              <p className="text-[11px] text-muted-foreground leading-relaxed">
                {t("settings.accounts.neverCheckedNote")}
              </p>
            </div>
          ) : null}
        </div>

        <DialogFooter className="flex-wrap gap-2 space-x-0 sm:justify-between items-center">
          <span className="text-[11px] text-muted-foreground">
            {check.isPending
              ? t("settings.checking")
              : record
                ? t("settings.accounts.lastCheckAt", { date: fmtDateTime(proxyCheckTime(record)) })
                : t("settings.accounts.neverCheckedShort")}
          </span>
          <div className="flex items-center gap-2">
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t("common.close")}
            </Button>
            <Button onClick={run} disabled={check.isPending}>
              <RefreshCw className={cn("w-3.5 h-3.5", check.isPending && "animate-spin")} />
              {check.isPending ? t("settings.checking") : record ? t("settings.accounts.recheck") : t("settings.accounts.startCheck")}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
