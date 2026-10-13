import { useState } from "react";
import { useTranslation } from "react-i18next";
import { fmtDateTime } from "@/i18n/format";
import {
  AlertTriangle,
  Radar,
  RotateCw,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
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
import { Field } from "./Field";
import { OVH_SUBSIDIARIES, subsidiaryLabel } from "@/lib/ovh-subsidiaries";
import { apiBaseUrlForEndpoint } from "@/lib/ovh-regions";
import { OvhTokenGuide } from "@/components/common/OvhTokenGuide";
import {
  type OVHAccount,
  type AccountInput,
  type ProxyTestRecord,
  useCreateAccount,
  useUpdateAccount,
  useProxyTest,
  useProxyStatus,
  useLastProxyTest,
  useVerifyAccount,
} from "@/hooks/use-accounts";

export function endpointForZone(zone: string): string {
  return OVH_SUBSIDIARIES.find((s) => s.code === zone)?.endpoint || "ovh-eu";
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
      return res.account.id;
    }
    const res = await create.mutateAsync(payload as AccountInput);
    applySaved(res.account);
    return res.account.id;
  };

  const verify = useVerifyAccount();
  const busy = create.isPending || update.isPending || test.isPending || verify.isPending;

  const saveAndTest = async () => {
    const id = await save();
    if (id) {
      test.mutate(id);
    }
  };

  const submit = async () => {
    const id = await save();
    if (!id) return;
    try {
      const vRes = await verify.mutateAsync(id);
      if (!vRes.valid) {
        return;
      }
    } catch {
      return;
    }
    onClose();
  };

  return (
    <Dialog open onOpenChange={onClose}>
      <DialogContent className="w-[95vw] sm:w-full sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("settings.accounts.editTitle") : t("settings.accounts.addTitle")}</DialogTitle>
          <DialogDescription>
            {isEdit ? t("settings.accounts.editDesc") : t("settings.accounts.addDesc")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-3 py-1 max-h-[65vh] overflow-y-auto -mx-6 px-6">
          <Field label={t("settings.accounts.nameLabel")} required>
            <Input
              placeholder={t("settings.accounts.namePlaceholder")}
              value={form.name}
              onChange={(e) => set("name", e.target.value)}
            />
          </Field>
          <Field label={t("settings.accounts.zoneLabel")} required>
            <Select value={form.zone} onValueChange={(v) => set("zone", v)}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {OVH_SUBSIDIARIES.map((s) => (
                  <SelectItem key={s.code} value={s.code}>
                    {subsidiaryLabel(s.code)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          {!isEdit && (
            <div className="flex items-center justify-between p-2 rounded-lg bg-muted/60 text-xs text-muted-foreground">
              <span>Endpoint: <code className="font-mono text-foreground">{endpointForZone(form.zone)}</code></span>
              <a
                href={`${apiBaseUrlForEndpoint(endpointForZone(form.zone))}/createToken/`}
                target="_blank"
                rel="noreferrer"
                className="text-primary hover:underline font-medium"
              >
                {t("settings.accounts.createTokenLink")} ↗
              </a>
            </div>
          )}

          {!isEdit && <OvhTokenGuide endpoint={endpointForZone(form.zone)} />}

          <Field
            label={t("settings.accounts.appKeyLabel")}
            required={!isEdit}
            hint={isEdit ? t("settings.accounts.leaveBlankKeep") : undefined}
          >
            <Input
              type="password"
              placeholder={isEdit ? t("settings.accounts.leaveBlankKeep") : "Application Key"}
              value={form.appKey}
              onChange={(e) => set("appKey", e.target.value)}
            />
          </Field>
          <Field
            label={t("settings.accounts.appSecretLabel")}
            required={!isEdit}
            hint={isEdit ? t("settings.accounts.leaveBlankKeep") : undefined}
          >
            <Input
              type="password"
              placeholder={isEdit ? t("settings.accounts.leaveBlankKeep") : "Application Secret"}
              value={form.appSecret}
              onChange={(e) => set("appSecret", e.target.value)}
            />
          </Field>
          <Field
            label={t("settings.accounts.consumerKeyLabel")}
            required={!isEdit}
            hint={isEdit ? t("settings.accounts.leaveBlankKeep") : undefined}
          >
            <Input
              type="password"
              placeholder={isEdit ? t("settings.accounts.leaveBlankKeep") : "Consumer Key"}
              value={form.consumerKey}
              onChange={(e) => set("consumerKey", e.target.value)}
            />
          </Field>

          {/* 出口网络与伪装配置 */}
          <div className="rounded-xl border border-border p-3 space-y-3 bg-secondary/20">
            <div className="flex items-center justify-between">
              <span className="text-[12px] font-semibold">{t("settings.accounts.outboundTitle")}</span>
              <span className="text-[11px] text-muted-foreground">{t("settings.accounts.outboundOptional")}</span>
            </div>

            <Field
              label={t("settings.accounts.proxyUrlLabel")}
              hint={t("settings.accounts.proxyUrlHint")}
            >
              <div className="space-y-1.5">
                <Input
                  type="text"
                  placeholder={
                    saved?.proxyUrl && !clearProxy
                      ? t("settings.accounts.savedProxyPlaceholder")
                      : "socks5://user:pass@1.2.3.4:1080"
                  }
                  value={form.proxyUrl}
                  disabled={clearProxy}
                  onChange={(e) => set("proxyUrl", e.target.value)}
                />
                {saved?.proxyUrl && (
                  <div className="flex items-center justify-between text-[11px]">
                    <span className="text-muted-foreground truncate font-mono" title={saved.proxyUrl}>
                      {clearProxy ? (
                        <span className="text-warning">{t("settings.accounts.willClearProxy")}</span>
                      ) : (
                        t("settings.accounts.savedProxyShort", { proxy: saved.proxyUrl })
                      )}
                    </span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-6 text-[11px] px-2 text-muted-foreground hover:text-foreground"
                      onClick={() => {
                        setClearProxy((v) => !v);
                        if (!clearProxy) set("proxyUrl", "");
                      }}
                    >
                      <Trash2 className="w-3 h-3 mr-1" />
                      {clearProxy ? t("settings.accounts.keepProxy") : t("settings.accounts.clearProxy")}
                    </Button>
                  </div>
                )}
                {proxyErr && <p className="text-[11px] text-destructive">{proxyErr}</p>}
              </div>
            </Field>

            {/* 出口 IP 体检面板 */}
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-medium text-muted-foreground">{t("settings.accounts.egressCheckTitle")}</span>
                {saved && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-6 text-[11px] px-2"
                    disabled={test.isPending}
                    onClick={() => test.mutate(saved.id)}
                  >
                    <RotateCw className={`w-3 h-3 mr-1 ${test.isPending ? "animate-spin" : ""}`} />
                    {lastTest ? t("settings.accounts.retest") : t("settings.accounts.testNow")}
                  </Button>
                )}
              </div>
              {!saved && (
                <p className="text-[11px] text-muted-foreground/70">
                  {t("settings.accounts.saveFirstToTest")}
                </p>
              )}
              {saved && outboundDirty && (
                <p className="text-[11px] text-warning">
                  {t("settings.accounts.outboundDirtyNote")}
                </p>
              )}
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
