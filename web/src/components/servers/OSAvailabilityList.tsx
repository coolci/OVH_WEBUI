import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, ChevronUp } from "lucide-react";
import { useOSAvailability } from "@/hooks/use-ovh-extra";
import { apiMessage } from "@/lib/api-error";

/** "debian12_64" → "debian12";"alma9-cpanel-latest_64" → "alma9-cpanel-latest" */
function osLabel(code: string) {
  return code.replace(/_64$/, "");
}

/** 服务器详情里的"可安装系统":展开时才向 OVH 查询 */
export function OSAvailabilityList({ planCode, accountId }: { planCode: string; accountId?: string }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const q = useOSAvailability(planCode, accountId, open);
  return (
    <div className="border-t border-border pt-4">
      <button type="button" className="flex items-center gap-1.5 text-[13px] font-semibold" onClick={() => setOpen((v) => !v)}>
        {open ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
        {open ? t("ovhExtra.os.title") : t("ovhExtra.os.show")}
        {open && q.data ? <span className="text-[11px] font-normal text-muted-foreground">· {t("ovhExtra.os.count", { count: q.data.length })}</span> : null}
      </button>
      {open && (
        <div className="mt-2">
          {q.isPending ? (
            <p className="text-[12px] text-muted-foreground">{t("ovhExtra.os.loading")}</p>
          ) : q.isError ? (
            <p className="text-[12px] text-destructive">{t("ovhExtra.os.failed", { reason: apiMessage(q.error) || "" })}</p>
          ) : q.data.length === 0 ? (
            <p className="text-[12px] text-muted-foreground">{t("ovhExtra.os.empty")}</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {q.data.map((os) => (
                <span key={os} className="rounded-md border border-border bg-secondary/40 px-1.5 py-0.5 font-mono text-[11px]">
                  {osLabel(os)}
                </span>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
