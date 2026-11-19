import { useState } from "react";
import { useTranslation } from "react-i18next";
import { KeyRound, Activity, RefreshCw } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useApiCredentials, useApiLogs, useRevokeApiCredential, type ApiCredential } from "@/hooks/use-ovh-extra";
import { apiMessage } from "@/lib/api-error";

const STATUS_TONE: Record<string, "success" | "info" | "danger" | "default"> = {
  validated: "success",
  pendingValidation: "info",
  expired: "danger",
  refused: "danger",
};

function fmt(d?: string) {
  if (!d) return "";
  const t = new Date(d);
  return Number.isNaN(t.getTime()) ? d : t.toLocaleString();
}

/** 账户页"API 安全":OVH API 凭据列表(可吊销)+ 最近 API 调用记录 */
export function AccountSecurityPanel({ accountId }: { accountId?: string }) {
  const { t } = useTranslation();
  const creds = useApiCredentials(accountId);
  const logs = useApiLogs(accountId, true);
  const revoke = useRevokeApiCredential(accountId);
  const [pending, setPending] = useState<ApiCredential | null>(null);
  // 关闭动画期间仍显示被确认的那条,不清空 pending,只切换 open
  const [confirmOpen, setConfirmOpen] = useState(false);

  return (
    <div className="space-y-4">
      <Card className="surface-card rounded-xl border-border shadow-sm">
        <CardContent className="p-4 sm:p-5 space-y-3">
          <div className="flex items-start justify-between gap-2">
            <div>
              <h3 className="text-sm font-semibold flex items-center gap-2">
                <KeyRound className="w-4 h-4" />
                {t("ovhExtra.security.credTitle")}
              </h3>
              <p className="text-[12px] text-muted-foreground mt-1">{t("ovhExtra.security.credHint")}</p>
            </div>
            <Button size="sm" variant="ghost" className="h-7 px-2 shrink-0" onClick={() => creds.refetch()} disabled={creds.isFetching} aria-label="refresh">
              <RefreshCw className={`w-3.5 h-3.5 ${creds.isFetching ? "animate-spin" : ""}`} />
            </Button>
          </div>
          {creds.isPending ? (
            <Skeleton className="h-24 w-full" />
          ) : creds.isError ? (
            <p role="alert" className="text-sm text-destructive">{t("ovhExtra.security.failed", { reason: apiMessage(creds.error) || "" })}</p>
          ) : creds.data.credentials.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("ovhExtra.security.empty")}</p>
          ) : (
            <div className="space-y-2">
              {creds.data.truncated && <p className="text-[12px] text-muted-foreground">{t("ovhExtra.security.truncated")}</p>}
              {creds.data.credentials.map((c) => (
                <div key={c.credentialId} className="rounded-lg border border-border p-3 space-y-1.5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2 min-w-0">
                      <span className="text-sm font-medium">#{c.credentialId}</span>
                      <span className="text-sm truncate">{c.appName || `app ${c.applicationId}`}</span>
                      <Chip tone={STATUS_TONE[c.status] || "default"}>{c.status}</Chip>
                      {c.current && <Chip tone="info">{t("ovhExtra.security.current")}</Chip>}
                    </div>
                    {!c.current && c.status !== "unknown" && (
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-[12px] text-destructive"
                        disabled={revoke.isPending}
                        onClick={() => {
                          setPending(c);
                          setConfirmOpen(true);
                        }}
                      >
                        {revoke.isPending && revoke.variables === c.credentialId ? t("ovhExtra.security.revoking") : t("ovhExtra.security.revoke")}
                      </Button>
                    )}
                  </div>
                  {c.appDescription && <p className="text-[12px] text-muted-foreground">{c.appDescription}</p>}
                  <dl className="grid grid-cols-1 sm:grid-cols-3 gap-x-4 gap-y-0.5 text-[12px]">
                    <div><dt className="inline text-muted-foreground">{t("ovhExtra.security.created")}: </dt><dd className="inline">{fmt(c.creation) || "—"}</dd></div>
                    <div><dt className="inline text-muted-foreground">{t("ovhExtra.security.lastUse")}: </dt><dd className="inline">{fmt(c.lastUse) || t("ovhExtra.security.never")}</dd></div>
                    <div><dt className="inline text-muted-foreground">{t("ovhExtra.security.expires")}: </dt><dd className="inline">{fmt(c.expiration) || t("ovhExtra.security.never")}</dd></div>
                  </dl>
                  {c.allowedIPs && c.allowedIPs.length > 0 && (
                    <p className="text-[12px]"><span className="text-muted-foreground">{t("ovhExtra.security.ips")}: </span><code className="break-all">{c.allowedIPs.join(", ")}</code></p>
                  )}
                  {c.rules && c.rules.length > 0 && (
                    <p className="text-[12px]">
                      <span className="text-muted-foreground">{t("ovhExtra.security.rules")}: </span>
                      {c.rules.some((r) => r.path === "/*" && r.method === "GET") && c.rules.some((r) => r.path === "/*" && r.method === "DELETE") ? (
                        <span>{t("ovhExtra.security.rulesAll")}</span>
                      ) : (
                        <code className="break-all">{c.rules.map((r) => `${r.method} ${r.path}`).join(", ")}</code>
                      )}
                    </p>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card className="surface-card rounded-xl border-border shadow-sm">
        <CardContent className="p-4 sm:p-5 space-y-3">
          <div>
            <h3 className="text-sm font-semibold flex items-center gap-2">
              <Activity className="w-4 h-4" />
              {t("ovhExtra.security.logsTitle")}
            </h3>
            <p className="text-[12px] text-muted-foreground mt-1">{t("ovhExtra.security.logsHint")}</p>
          </div>
          {logs.isPending ? (
            <Skeleton className="h-16 w-full" />
          ) : logs.isError ? (
            <p role="alert" className="text-sm text-destructive">{t("ovhExtra.security.failed", { reason: apiMessage(logs.error) || "" })}</p>
          ) : logs.data.unsupported ? (
            <p className="text-sm text-muted-foreground">{t("ovhExtra.security.logsUnsupported")}</p>
          ) : logs.data.logs.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("ovhExtra.security.empty")}</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[12px]">
                <tbody>
                  {logs.data.logs.map((l) => (
                    <tr key={l.logId} className="border-b border-border/60 last:border-0">
                      <td className="py-1.5 pr-3 whitespace-nowrap text-muted-foreground">{fmt(l.date)}</td>
                      <td className="py-1.5 pr-3 whitespace-nowrap"><code>{l.ip}</code></td>
                      <td className="py-1.5 pr-3 whitespace-nowrap font-medium">{l.method}</td>
                      <td className="py-1.5 break-all"><code>{l.path}</code></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("ovhExtra.security.revoke")}</AlertDialogTitle>
            <AlertDialogDescription>
              {pending ? t("ovhExtra.security.confirm", { id: pending.credentialId, app: pending.appName || pending.applicationId }) : ""}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (pending) revoke.mutate(pending.credentialId);
              }}
            >
              {t("ovhExtra.security.revoke")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
