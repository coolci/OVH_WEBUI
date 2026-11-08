import { useState } from "react";
import { Link } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { ListChecks } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Chip } from "@/components/common/Chip";
import { useOrderFollowUp } from "@/hooks/use-ovh-extra";
import { apiMessage } from "@/lib/api-error";

const STATUS_TONE: Record<string, "success" | "info" | "danger" | "default"> = {
  DONE: "success",
  DOING: "info",
  ERROR: "danger",
  TODO: "default",
};

function tr(t: (k: string, o?: Record<string, unknown>) => string, key: string, code: string) {
  const v = t(`ovhExtra.followUp.${key}.${code}`);
  return v.startsWith("ovhExtra.") ? code : v;
}

/** 购买历史里的"进度"按钮:OVH 订单跟踪 + 交付后跳到服务器 / VPS 控制 */
export function OrderProgressButton({ accountId, orderId }: { accountId?: string; orderId?: string | null }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const q = useOrderFollowUp(accountId, orderId || undefined, open);
  if (!accountId || !orderId) return null;
  return (
    <>
      <Button variant="ghost" size="sm" className="h-7 px-2 text-[12px] gap-1" onClick={() => setOpen(true)}>
        <ListChecks className="w-3.5 h-3.5" />
        {t("ovhExtra.followUp.button")}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("ovhExtra.followUp.title", { orderId })}</DialogTitle>
            <DialogDescription>{t("ovhExtra.followUp.note")}</DialogDescription>
          </DialogHeader>
          {q.isPending ? (
            <p className="text-sm text-muted-foreground">{t("ovhExtra.followUp.loading")}</p>
          ) : q.isError ? (
            <p role="alert" className="text-sm text-destructive">{t("ovhExtra.followUp.failed", { reason: apiMessage(q.error) || "" })}</p>
          ) : (
            <div className="space-y-4">
              {q.data.steps.length === 0 ? (
                <p className="text-sm text-muted-foreground">{t("ovhExtra.followUp.empty")}</p>
              ) : (
                <ol className="space-y-3">
                  {q.data.steps.map((s, i) => (
                    <li key={`${s.step}-${i}`} className="rounded-xl border border-border p-3">
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-sm font-medium">{tr(t, "step", s.step)}</span>
                        <Chip tone={STATUS_TONE[s.status] || "default"}>{tr(t, "status", s.status)}</Chip>
                      </div>
                      {s.history.length > 0 && (
                        <ul className="mt-2 space-y-1">
                          {s.history.map((h, j) => (
                            <li
                              key={j}
                              className={`text-[12px] ${h.label === "FRAUD_DOCS_REQUESTED" || h.label === "FRAUD_REFUSED" ? "text-destructive font-medium" : "text-muted-foreground"}`}
                            >
                              {h.date ? new Date(h.date).toLocaleString() : ""} · {tr(t, "label", h.label)}
                              {h.description ? ` — ${h.description}` : ""}
                            </li>
                          ))}
                        </ul>
                      )}
                    </li>
                  ))}
                </ol>
              )}
              {q.data.services.length > 0 && (
                <div className="space-y-2">
                  <p className="text-[13px] font-medium">{t("ovhExtra.followUp.delivered")}</p>
                  {q.data.services.map((svc) => {
                    const isVps = /^vps-|\.vps\.ovh\./i.test(svc);
                    const to = `${isVps ? "/vps-control" : "/server-control"}?account=${encodeURIComponent(accountId)}&service=${encodeURIComponent(svc)}`;
                    return (
                      <div key={svc} className="flex items-center justify-between gap-2 rounded-lg border border-border px-3 py-2">
                        <code className="text-[12px] break-all">{svc}</code>
                        <Button asChild size="sm" variant="outline" className="h-7 text-[12px] shrink-0">
                          <Link to={to}>{isVps ? t("ovhExtra.followUp.openVps") : t("ovhExtra.followUp.openServer")}</Link>
                        </Button>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
