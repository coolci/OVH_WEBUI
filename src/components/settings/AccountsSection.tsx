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
  Network,
  Fingerprint,
  ShieldAlert,
  Activity,
  AlertTriangle,
} from "lucide-react";
import { LoadFailed, LoadFailedBanner } from "@/components/common/LoadFailed";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { Chip } from "@/components/common/Chip";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { Section } from "./Section";
import { cn } from "@/lib/utils";
import { AccountDialog, endpointForZone } from "./AccountDialog";
import { LinkCheckDialog } from "./LinkCheckDialog";
import {
  useAccounts,
  useDeleteAccount,
  useSetDefaultAccount,
  useVerifyAccount,
  useProxyStatus,
  useLastProxyTests,
  useProxyCheck,
  useLastProxyTest,
  accountChipColor,
  type OVHAccount,
  type AccountProxyStatus,
} from "@/hooks/use-accounts";

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

export { AccountDialog, endpointForZone };
