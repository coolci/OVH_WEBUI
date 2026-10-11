import { useState } from "react";
import { useTranslation } from "react-i18next";
import { fmtDate, fmtDateTime } from "@/i18n/format";
import {
  Globe,
  Plus,
  Star,
  RotateCw,
  Trash2,
  Pencil,
  RefreshCw,
  Network,
  Fingerprint,
  ShieldAlert,
  Radar,
  Ban,
  Activity,
  AlertTriangle,
} from "lucide-react";
import { LoadFailed, LoadFailedBanner } from "@/components/common/LoadFailed";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/common/Skeleton";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Section } from "./Section";
import { Field } from "./Field";
import { cn } from "@/lib/utils";
import { OVH_SUBSIDIARIES, subsidiaryLabel } from "@/lib/ovh-subsidiaries";
import { apiBaseUrlForEndpoint } from "@/lib/ovh-regions";
import { OvhTokenGuide } from "@/components/common/OvhTokenGuide";
import {
  useAccounts,
  useCreateAccount,
  useUpdateAccount,
  useDeleteAccount,
  useSetDefaultAccount,
  useVerifyAccount,
  useProxyStatus,
  useProxyTest,
  useLastProxyTest,
  useLastProxyTests,
  useProxyCheck,
  useLastProxyCheck,
  accountChipColor,
  gradeLatency,
  isJittery,
  worstMinMs,
  proxyCheckTime,
  type OVHAccount,
  type AccountInput,
  type AccountProxyStatus,
  type ProxyTestRecord,
  type ProxyProbeTarget,
  type LatencyLevel,
} from "@/hooks/use-accounts";

/** 根据 zone 推 endpoint */
export function endpointForZone(zone: string): string {
  return OVH_SUBSIDIARIES.find((s) => s.code === zone)?.endpoint || "ovh-eu";
}

// ─── 账户管理 ───────────────────────────────────────────────────────────────

export function AccountsSection() {
  const { t } = useTranslation();
  const accounts = useAccounts();
  const health = useProxyStatus();
  const [showAdd, setShowAdd] = useState(false);
  const [editAcc, setEditAcc] = useState<OVHAccount | null>(null);
  const list = accounts.data || [];
  const healthByID = new Map((health.data?.accounts || []).map((h) => [h.id, h]));

  const tests = useLastProxyTests(list.map((a) => a.id));
  const byIP = new Map<string, { name: string; hasProxy: boolean }[]>();
  list.forEach((a, i) => {
    const r = tests[i]?.data;
    if (!r || !r.success) return;
    byIP.set(r.egressIP, [...(byIP.get(r.egressIP) || []), { name: a.name, hasProxy: !!a.proxyUrl }]);
  });
  const collisions = Array.from(byIP.entries())
    .filter(([, xs]) => xs.length > 1)
    .map(([ip, xs]) => ({ ip, xs, proxied: xs.some((x) => x.hasProxy) }));
  const proxiedCollision = collisions.some((c) => c.proxied);

  return (
    <Section title={t("settings.accounts.title")}>
      <div className="flex items-start justify-between gap-3">
        <p className="text-[12px] text-muted-foreground">
          {t("settings.accounts.intro")}
        </p>
        <Button onClick={() => setShowAdd(true)} size="sm" className="flex-shrink-0">
          <Plus className="w-4 h-4" />
          {t("settings.accounts.addAccount")}
        </Button>
      </div>

      <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1.5 text-[11px] leading-relaxed">
        <p className="font-semibold flex items-center gap-1.5">
          <Network className="w-3.5 h-3.5" />
          {t("settings.accounts.proxyWhyTitle")}
        </p>
        <p className="text-muted-foreground">
          {t("settings.accounts.proxyWhyP1")}
        </p>
        <p className="text-muted-foreground">
          {t("settings.accounts.proxyWhyP2Pre")}<b className="text-warning">{t("settings.accounts.proxyWhyP2Bold")}</b>{t("settings.accounts.proxyWhyP2Post")}
        </p>
      </div>

      {health.isError && (
        <LoadFailedBanner
          title={t("settings.accounts.healthFailedTitle")}
          error={health.error}
          onRetry={() => health.refetch()}
        />
      )}

      {collisions.length > 0 && (
        <div
          className={cn(
            "rounded-xl border px-3 py-2.5 text-[11px] space-y-1",
            proxiedCollision ? "border-destructive/40 bg-destructive/5" : "border-warning/40 bg-warning/5"
          )}
        >
          <p className={cn("font-semibold flex items-center gap-1.5", proxiedCollision ? "text-destructive" : "text-warning")}>
            <AlertTriangle className="w-3.5 h-3.5" />
            {proxiedCollision
              ? t("settings.accounts.collisionProxiedTitle")
              : t("settings.accounts.collisionSharedTitle")}
          </p>
          {collisions.map((c) => (
            <p key={c.ip} className="text-muted-foreground">
              <span className="font-mono font-semibold text-foreground">{c.ip}</span> ←{" "}
              {c.xs.map((x) => `${x.name}${x.hasProxy ? t("settings.accounts.proxiedSuffix") : t("settings.accounts.directSuffix")}`).join(t("settings.accounts.listSeparator"))}
            </p>
          ))}
          <p className="text-muted-foreground">
            {t("settings.accounts.collisionNote")}
          </p>
        </div>
      )}

      {accounts.isError ? (
        <LoadFailed
          icon={Globe}
          title={t("settings.accounts.loadFailedTitle")}
          error={accounts.error}
          onRetry={() => accounts.refetch()}
          compact
        />
      ) : accounts.isPending ? (
        <div className="space-y-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-24 rounded-2xl" />
          ))}
        </div>
      ) : list.length === 0 ? (
        <Card>
          <CardContent className="p-8 text-center text-sm text-muted-foreground">
            {t("settings.accounts.emptyHint")}
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {list.map((a) => (
            <AccountCard
              key={a.id}
              acc={a}
              onEdit={() => setEditAcc(a)}
              health={healthByID.get(a.id)}
              healthUnknown={health.isError || health.isPending}
            />
          ))}
        </div>
      )}

      {showAdd && <AccountDialog onClose={() => setShowAdd(false)} />}
      {editAcc && <AccountDialog acc={editAcc} onClose={() => setEditAcc(null)} />}
    </Section>
  );
}

function AccountCard({
  acc,
  onEdit,
  health,
  healthUnknown,
}: {
  acc: OVHAccount;
  onEdit: () => void;
  health?: AccountProxyStatus;
  healthUnknown?: boolean;
}) {
  const { t } = useTranslation();
  const setDefault = useSetDefaultAccount();
  const del = useDeleteAccount();
  const verify = useVerifyAccount();
  const check = useProxyCheck();
  const [confirming, setConfirming] = useState(false);
  const [checking, setChecking] = useState(false);
  const lastTest = useLastProxyTest(acc.id).data;
  const subsidiaryWarning = verify.data?.subsidiaryWarning;

  return (
    <div className="border border-border rounded-2xl p-4 flex flex-col gap-3">
      <div className="flex flex-col sm:flex-row sm:items-center gap-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1 flex-wrap">
          <span className="font-semibold text-sm">{acc.name}</span>
          <span className={cn("inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium", accountChipColor(acc.zone))}>
            {acc.zone}
          </span>
          {acc.isDefault && (
            <Chip tone="success">
              <Star className="w-3 h-3" />
              {t("settings.accounts.defaultChip")}
            </Chip>
          )}
        </div>
        <div className="text-[11px] text-muted-foreground flex items-center gap-2 flex-wrap font-mono">
          <span>{acc.endpoint}</span>
          <span>·</span>
          <span>{acc.iam}</span>
          <span>·</span>
          <span>{t("settings.accounts.builtAt", { date: fmtDate(acc.createdAt) })}</span>
        </div>
        <div className="flex items-center gap-1.5 flex-wrap mt-1.5">
          {acc.proxyUrl ? (
            <Chip tone="info">
              <Network className="w-3 h-3" />
              <span className="font-mono">{acc.proxyUrl}</span>
            </Chip>
          ) : (
            <Chip>{t("settings.accounts.direct")}</Chip>
          )}
          <Chip>
            <Fingerprint className="w-3 h-3" />
            {acc.fingerprint || "default"}
          </Chip>
          {lastTest && lastTest.success ? (
            <Chip tone="success" title={t("settings.accounts.testedAtTitle", { date: fmtDateTime(lastTest.testedAt) })}>
              {t("settings.accounts.egressLabel")} <span className="font-mono font-semibold">{lastTest.egressIP}</span>
            </Chip>
          ) : lastTest ? (
            <Chip tone="danger" title={lastTest.error}>{t("settings.accounts.egressFailedVia", { via: lastTest.via })}</Chip>
          ) : (
            <span className="text-[11px] text-muted-foreground">{t("settings.accounts.egressUntested")}</span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2 flex-shrink-0">
        <Button
          variant="outline"
          size="sm"
          onClick={() => setChecking(true)}
          title={t("settings.accounts.chainCheckTitle")}
        >
          <Activity className={cn("w-3.5 h-3.5", check.isPending && "animate-pulse")} />
          {check.isPending ? t("settings.checking") : t("settings.accounts.chainCheck")}
        </Button>
        <Button variant="ghost" size="icon" onClick={() => verify.mutate(acc.id)} disabled={verify.isPending} title={t("settings.accounts.reverifyTitle")}>
          <RotateCw className={cn("w-4 h-4", verify.isPending && "animate-spin")} />
        </Button>
        {!acc.isDefault && (
          <Button variant="ghost" size="icon" onClick={() => setDefault.mutate(acc.id)} disabled={setDefault.isPending} title={t("settings.accounts.setDefaultTitle")}>
            <Star className="w-4 h-4" />
          </Button>
        )}
        <Button variant="ghost" size="icon" onClick={onEdit} title={t("common.edit")}>
          <Pencil className="w-4 h-4" />
        </Button>
        <Button variant="ghost" size="icon" onClick={() => setConfirming(true)} title={t("common.delete")} className="text-destructive hover:text-destructive">
          <Trash2 className="w-4 h-4" />
        </Button>
      </div>
      </div>

      {health?.tripped ? (
        <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2.5 text-[11px] space-y-1">
          <p className="font-semibold text-destructive flex items-center gap-1.5">
            <ShieldAlert className="w-3.5 h-3.5" />
            {t("settings.accounts.trippedTitle", { count: health.fails })}
          </p>
          <p className="text-muted-foreground">
            {t("settings.accounts.trippedSubsNote")}
            {health.trippedAt ? t("settings.accounts.trippedAtNote", { date: fmtDateTime(health.trippedAt) }) : ""}
          </p>
          <p className="text-muted-foreground">
            {t("settings.accounts.trippedRecoverPre")}<b>{t("settings.accounts.trippedRecoverBold")}</b>{t("settings.accounts.trippedRecoverPost")}
          </p>
        </div>
      ) : health && health.fails > 0 ? (
        <p className="text-[11px] text-warning border border-warning/40 bg-warning/5 rounded-xl px-3 py-2">
          {t("settings.accounts.proxyFailsWarning", {
            count: health.fails,
            lastFail: health.lastFailAt
              ? t("settings.accounts.lastFailNote", { time: fmtDateTime(health.lastFailAt) })
              : "",
          })}
        </p>
      ) : healthUnknown && acc.proxyUrl ? (
        <p className="text-[11px] text-muted-foreground">
          {t("settings.accounts.healthUnknownNote")}
        </p>
      ) : null}

      {subsidiaryWarning && (
        <p className="text-[11px] text-warning border border-warning/40 bg-warning/5 rounded-xl px-3 py-2">
          {t("settings.accounts.subsidiaryWarning", { detail: subsidiaryWarning })}
        </p>
      )}

      <LinkCheckDialog acc={acc} open={checking} onOpenChange={setChecking} check={check} />

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="w-[95vw] sm:w-full sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("settings.accounts.deleteConfirmTitle", { name: acc.name })}</DialogTitle>
            <DialogDescription className="text-destructive">
              {t("settings.accounts.deleteConfirmDesc")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirming(false)}>{t("common.cancel")}</Button>
            <Button
              variant="destructive"
              onClick={async () => {
                await del.mutateAsync(acc.id);
                setConfirming(false);
              }}
              disabled={del.isPending}
            >
              {t("settings.accounts.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

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

function LinkCheckDialog({
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

function proxyInputError(raw: string, t: (key: string, vars?: Record<string, string | number>) => string): string {
  const v = raw.trim();
  if (!v) return "";
  let u: URL;
  try {
    u = new URL(v);
  } catch {
    return t("settings.accounts.proxyErr.parse");
  }
  const scheme = u.protocol.replace(":", "").toLowerCase();
  if (!["http", "https", "socks5", "socks5h"].includes(scheme)) {
    return t("settings.accounts.proxyErr.scheme", { scheme });
  }
  if (!u.hostname) return t("settings.accounts.proxyErr.host");
  const authorityMatch = v.match(/^[^:]+:\/\/([^/?#]+)/);
  const authorityPart = authorityMatch ? authorityMatch[1] : "";
  const hasExplicitPort = Boolean(u.port) || /:\d+$/.test(authorityPart);
  if (!hasExplicitPort) {
    return t("settings.accounts.proxyErr.port");
  }
  return "";
}

function EgressPanel({
  record,
  pending,
  requestError,
  onRetry,
  expectProxy,
}: {
  record?: ProxyTestRecord | null;
  pending: boolean;
  requestError?: unknown;
  onRetry: () => void;
  expectProxy: boolean;
}) {
  const { t } = useTranslation();
  if (pending) return <Skeleton className="h-24 rounded-2xl" />;
  if (requestError) {
    return (
      <LoadFailedBanner
        title={t("settings.accounts.egressReqFailedTitle")}
        error={requestError}
        onRetry={onRetry}
      />
    );
  }
  if (!record) return null;
  const at = fmtDateTime(record.testedAt);

  if (!record.success) {
    return (
      <div className="rounded-xl border border-destructive/40 bg-destructive/5 px-3 py-2.5 space-y-1 text-[11px]">
        <p className="font-semibold text-destructive flex items-center gap-1.5">
          <AlertTriangle className="w-3.5 h-3.5" />
          {t("settings.accounts.egressFailedVia", { via: record.via })}
        </p>
        <p className="text-muted-foreground break-all">{record.error}</p>
        <p className="text-muted-foreground">
          {record.usingProxy
            ? t("settings.accounts.egressFailedProxyNote")
            : t("settings.accounts.egressFailedDirectNote")}
        </p>
        <p className="text-muted-foreground/70">{t("settings.accounts.testedAtShort", { date: at })}</p>
      </div>
    );
  }

  return (
    <div className="rounded-xl border border-success/40 bg-success/5 px-3 py-2.5 space-y-1.5">
      <p className="text-[11px] text-muted-foreground">{t("settings.accounts.egressIpLabel")}</p>
      <p className="text-2xl font-mono font-semibold tracking-tight break-all">{record.egressIP}</p>
      <p className="text-[11px] text-muted-foreground">
        {t("settings.accounts.viaLabel")} {record.usingProxy ? <span className="font-mono">{record.proxy || t("settings.accounts.proxyFallback")}</span> : t("settings.accounts.direct")}
        {" · "}{t("settings.accounts.fingerprintShort")} {record.fingerprint}
        {" · "}{t("settings.accounts.testedAtShort", { date: at })}
      </p>
      {expectProxy && !record.usingProxy && (
        <p className="text-[11px] text-destructive border border-destructive/40 bg-destructive/5 rounded-lg px-2 py-1.5">
          {t("settings.accounts.proxyNotAppliedPre")}<b>{t("settings.accounts.proxyNotAppliedBold")}</b>{t("settings.accounts.proxyNotAppliedPost")}
        </p>
      )}
      {!record.usingProxy && !expectProxy && (
        <p className="text-[11px] text-muted-foreground">
          {t("settings.accounts.directEgressNote")}
        </p>
      )}
      {record.warning && <p className="text-[11px] text-warning">⚠ {record.warning}</p>}
      <p className="text-[11px] text-muted-foreground">
        {t("settings.accounts.compareIpHintShort")}
      </p>
    </div>
  );
}

export function AccountDialog({ acc, onClose }: { acc?: OVHAccount; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateAccount();
  const update = useUpdateAccount();
  const test = useProxyTest();
  const proxyStatus = useProxyStatus();
  const isEdit = !!acc;

  const [saved, setSaved] = useState<{ id: string; proxyUrl: string; fingerprint: string } | null>(
    acc ? { id: acc.id, proxyUrl: acc.proxyUrl || "", fingerprint: acc.fingerprint || "default" } : null
  );

  const [form, setForm] = useState({
    name: acc?.name || "",
    appKey: "",
    appSecret: "",
    consumerKey: "",
    zone: acc?.zone || "IE",
    proxyUrl: "",
    fingerprint: acc?.fingerprint || "default",
  });
  const [clearProxy, setClearProxy] = useState(false);
  const set = (k: keyof typeof form, v: string) => setForm((p) => ({ ...p, [k]: v }));

  const lastTest = useLastProxyTest(saved?.id || "").data;
  const proxyErr = clearProxy ? "" : proxyInputError(form.proxyUrl, t);
  const canSubmit =
    !proxyErr &&
    (saved
      ? !!form.name.trim()
      : !!(form.name.trim() && form.appKey.trim() && form.appSecret.trim() && form.consumerKey.trim()));

  const outboundDirty = clearProxy || !!form.proxyUrl.trim() || form.fingerprint !== (saved?.fingerprint || "default");
  const profiles = proxyStatus.data?.profiles || [];
  const fpOptions = Array.from(
    new Set([...profiles, form.fingerprint, saved?.fingerprint || "default"].filter(Boolean))
  );
  const fpLocked = profiles.length === 0;

  const applySaved = (a: OVHAccount) => {
    setSaved({ id: a.id, proxyUrl: a.proxyUrl || "", fingerprint: a.fingerprint || "default" });
    setForm((p) => ({ ...p, proxyUrl: "", fingerprint: a.fingerprint || "default" }));
    setClearProxy(false);
  };

  const buildPayload = (): Partial<AccountInput> => {
    const payload: Partial<AccountInput> = {
      name: form.name.trim(),
      appKey: form.appKey.trim(),
      appSecret: form.appSecret.trim(),
      consumerKey: form.consumerKey.trim(),
      zone: form.zone,
      endpoint: endpointForZone(form.zone),
    };
    if (!saved) {
      payload.proxyUrl = clearProxy ? "" : form.proxyUrl.trim();
      payload.fingerprint = form.fingerprint;
      return payload;
    }
    if (clearProxy) {
      payload.proxyUrl = "";
    } else if (form.proxyUrl.trim()) {
      payload.proxyUrl = form.proxyUrl.trim();
    }
    if (form.fingerprint !== saved.fingerprint) {
      payload.fingerprint = form.fingerprint;
    }
    return payload;
  };

  const save = async (): Promise<string | null> => {
    if (!canSubmit) return null;
    const payload = buildPayload();
    if (saved) {
      const res = await update.mutateAsync({ id: saved.id, input: payload });
      applySaved(res.account);
      return saved.id;
    }
    const res = await create.mutateAsync(payload as AccountInput);
    applySaved(res.account);
    return res.account.id;
  };

  const submit = async () => {
    try {
      const id = await save();
      if (id) onClose();
    } catch {
      // Handled in onError
    }
  };

  const saveAndTest = async () => {
    try {
      const id = await save();
      if (id) await test.mutateAsync(id);
    } catch {
      // Handled in onError
    }
  };

  const busy = create.isPending || update.isPending || test.isPending;

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("settings.accounts.editAccountTitle", { name: acc!.name }) : t("settings.accounts.addAccountTitle")}</DialogTitle>
          <DialogDescription>{t("settings.accounts.dialogDesc")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2 max-h-[65vh] overflow-y-auto -mx-6 px-6">
          <Field label={t("settings.accounts.nameLabel")}>
            <Input value={form.name} onChange={(e) => set("name", e.target.value)} placeholder={t("settings.accounts.namePlaceholder")} autoFocus />
            {isEdit && (
              <p className="text-[11px] text-muted-foreground mt-1">
                {t("settings.accounts.keepCredentialsHint")}
              </p>
            )}
          </Field>
          <Field
            label={t("settings.accounts.zoneLabel")}
            hint={t("settings.accounts.zoneHint", {
              endpoint: endpointForZone(form.zone),
              iam: `go-ovh-${form.zone.toLowerCase()}`,
            })}
          >
            <Select value={form.zone} onValueChange={(v) => set("zone", v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {OVH_SUBSIDIARIES.map((s) => (
                  <SelectItem key={s.code} value={s.code}>
                    {s.code} · {subsidiaryLabel(s.code)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          {!isEdit && <OvhTokenGuide endpoint={endpointForZone(form.zone)} />}

          <Field label="APP KEY *" hint={isEdit ? undefined : t("settings.accounts.appKeyHint")}>
            <Input type="password" value={form.appKey} onChange={(e) => set("appKey", e.target.value)}
              placeholder={isEdit ? (acc?.appKey || t("settings.accounts.keepUnchangedPlaceholder")) : t("settings.accounts.copyFromOvhPlaceholder")} />
          </Field>
          <Field label="APP SECRET *" hint={isEdit ? undefined : t("settings.accounts.appSecretHint")}>
            <Input type="password" value={form.appSecret} onChange={(e) => set("appSecret", e.target.value)}
              placeholder={isEdit ? (acc?.appSecret || t("settings.accounts.keepUnchangedPlaceholder")) : t("settings.accounts.copyFromOvhPlaceholder")} />
          </Field>
          <Field label="CONSUMER KEY *" hint={isEdit ? undefined : t("settings.accounts.consumerKeyHint")}>
            <Input type="password" value={form.consumerKey} onChange={(e) => set("consumerKey", e.target.value)}
              placeholder={isEdit ? (acc?.consumerKey || t("settings.accounts.keepUnchangedPlaceholder")) : t("settings.accounts.copyFromOvhPlaceholder")} />
          </Field>

          <div className="border-t border-border pt-4 space-y-4">
            <div>
              <p className="text-[13px] font-semibold flex items-center gap-1.5">
                <Network className="w-3.5 h-3.5" />
                {t("settings.accounts.outboundTitle")}
              </p>
              <p className="text-[11px] text-muted-foreground mt-1 leading-relaxed">
                {t("settings.accounts.outboundDesc")}
              </p>
            </div>

            <Field label={t("settings.accounts.proxyAddrLabel")}>
              <Input
                value={form.proxyUrl}
                onChange={(e) => set("proxyUrl", e.target.value)}
                disabled={clearProxy}
                placeholder={
                  clearProxy
                    ? t("settings.accounts.proxyPlaceholderMarked")
                    : saved
                      ? t("settings.accounts.proxyPlaceholderKeep")
                      : t("settings.accounts.proxyPlaceholderNew")
                }
                className="font-mono"
              />
              {proxyErr && <p className="text-[11px] text-destructive mt-1">{t("settings.accounts.proxyInvalid", { error: proxyErr })}</p>}

              {saved && (
                <div className="mt-2 space-y-1.5">
                  <p className="text-[11px] text-muted-foreground">
                    {t("settings.accounts.currentLabel")}
                    {saved?.proxyUrl ? (
                      <code className="ml-1 font-mono">{saved.proxyUrl}</code>
                    ) : (
                      <span className="ml-1">{t("settings.accounts.directNoProxy")}</span>
                    )}
                    {saved?.proxyUrl ? t("settings.accounts.maskedNote") : ""}
                  </p>
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="text-[11px] text-muted-foreground">{t("settings.accounts.removeProxyHint")}</span>
                    {clearProxy ? (
                      <Button variant="outline" size="sm" onClick={() => setClearProxy(false)}>
                        {t("settings.accounts.undoDirect")}
                      </Button>
                    ) : (
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={!saved?.proxyUrl}
                        onClick={() => {
                          setClearProxy(true);
                          set("proxyUrl", "");
                        }}
                      >
                        <Ban className="w-3.5 h-3.5" />
                        {t("settings.accounts.backToDirect")}
                      </Button>
                    )}
                  </div>
                  {clearProxy && (
                    <p className="text-[11px] text-warning">
                      {t("settings.accounts.clearProxyWarning")}
                    </p>
                  )}
                </div>
              )}

              <div className="mt-2 space-y-1 text-[11px] leading-relaxed">
                <p className="text-muted-foreground">
                  {t("settings.accounts.proxyFormatPre")}<code className="font-mono">http://</code> <code className="font-mono">https://</code>{" "}
                  <code className="font-mono">socks5://</code> <code className="font-mono">socks5h://</code>{t("settings.accounts.proxyFormatComma")}
                  <b>{t("settings.accounts.proxyFormatMustPort")}</b>{t("settings.accounts.proxyFormatExample")}<code className="font-mono">socks5://user:pass@1.2.3.4:1080</code>{t("settings.accounts.proxyFormatEnd")}
                  {saved ? t("settings.accounts.keepSavedNote") : t("settings.accounts.directNewNote")}
                </p>
                <p className="text-warning">
                  {t("settings.accounts.proxyFailPre")}<b>{t("settings.accounts.proxyFailBold1")}</b>{t("settings.accounts.proxyFailMid")}
                  <b>{t("settings.accounts.proxyFailBold2")}</b>{t("settings.accounts.proxyFailPost")}
                </p>
              </div>
            </Field>

            <div className="space-y-2">
              <div className="flex items-center gap-2 flex-wrap">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => saved && test.mutate(saved.id)}
                  disabled={!saved || outboundDirty || busy}
                >
                  <Radar className={cn("w-3.5 h-3.5", test.isPending && "animate-pulse")} />
                  {test.isPending ? t("settings.accounts.testing") : t("settings.accounts.testEgress")}
                </Button>
                <span className="text-[11px] text-muted-foreground">
                  {!saved
                    ? t("settings.accounts.testHintNew")
                    : outboundDirty
                      ? t("settings.accounts.testHintDirty")
                      : t("settings.accounts.testHintSaved")}
                </span>
              </div>
              <EgressPanel
                record={lastTest}
                pending={test.isPending}
                requestError={test.isError ? test.error : undefined}
                onRetry={() => saved && test.mutate(saved.id)}
                expectProxy={!!saved?.proxyUrl}
              />
            </div>

            <Field label={t("settings.accounts.outboundFingerprintLabel")}>
              <Select value={form.fingerprint} onValueChange={(v) => set("fingerprint", v)} disabled={fpLocked}>
                <SelectTrigger><SelectValue placeholder="default" /></SelectTrigger>
                <SelectContent>
                  {fpOptions.map((p) => (
                    <SelectItem key={p} value={p}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {proxyStatus.isError && (
                <div className="mt-2">
                  <LoadFailedBanner
                    title={t("settings.accounts.fingerprintListFailedTitle")}
                    error={proxyStatus.error}
                    onRetry={() => proxyStatus.refetch()}
                  />
                </div>
              )}
              {proxyStatus.isPending && (
                <p className="text-[11px] text-muted-foreground mt-1">{t("settings.accounts.readingFingerprints")}</p>
              )}
              <p className="text-[11px] text-muted-foreground mt-2 leading-relaxed">
                <b>{t("settings.accounts.fingerprintNoteBold")}</b>{t("settings.accounts.fingerprintNoteMid")}<code className="font-mono">chrome-like</code>{" "}
                <b>{t("settings.accounts.fingerprintNoteNotEqual")}</b>{t("settings.accounts.fingerprintNotePost")}
              </p>
            </Field>
          </div>

          <div className="rounded-xl border border-border bg-secondary/30 px-3 py-2.5 space-y-1.5">
            <p className="text-[11px] font-semibold">{t("settings.accounts.noKeyTitle")}</p>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {t("settings.accounts.keyGuideGo")}
              <a
                href={`${apiBaseUrlForEndpoint(endpointForZone(form.zone))}/createToken/`}
                target="_blank"
                rel="noreferrer"
                className="underline mx-1 text-primary"
              >
                {apiBaseUrlForEndpoint(endpointForZone(form.zone)).replace("https://", "")}/createToken
              </a>
              {t("settings.accounts.keyGuideApply")}<b>{form.zone}</b>{t("settings.accounts.keyGuideBelongs")}
              <span className="text-warning">{t("settings.accounts.keyGuideOtherSite")}</span>{t("settings.accounts.keyGuideNotShared")}
            </p>
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              {t("settings.accounts.keyGuidePermsPre")}
              <code className="mx-1 px-1 py-0.5 rounded bg-background text-[10px]">GET POST PUT DELETE</code>
              {t("settings.accounts.keyGuidePermsMid")}<code className="px-1 py-0.5 rounded bg-background text-[10px]">/*</code>{t("settings.accounts.keyGuidePermsSep")}
              <b>Unlimited</b>{t("settings.accounts.keyGuideUnlimitedPost")}
            </p>
          </div>
        </div>
        <DialogFooter className="flex-wrap gap-2 space-x-0">
          <Button variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button variant="outline" onClick={saveAndTest} disabled={!canSubmit || busy}>
            <Radar className="w-3.5 h-3.5" />
            {busy ? t("settings.accounts.processing") : t("settings.accounts.saveAndTest")}
          </Button>
          <Button onClick={submit} disabled={!canSubmit || busy}>
            {(create.isPending || update.isPending) ? t("settings.accounts.savingAccount") : t("settings.accounts.saveAndVerify")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
