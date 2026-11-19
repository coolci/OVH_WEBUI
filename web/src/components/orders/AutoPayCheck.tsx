import { useTranslation } from "react-i18next";
import { useDefaultPayment } from "@/hooks/use-ovh-extra";
import { useAccounts, findAccountByID } from "@/hooks/use-accounts";
import { apiMessage } from "@/lib/api-error";

function OneAccount({ accountId }: { accountId: string }) {
  const { t } = useTranslation();
  const { data: accounts } = useAccounts();
  const name = findAccountByID(accounts, accountId)?.name || accountId;
  const q = useDefaultPayment(accountId, true);
  if (q.isPending) return <p className="text-[11px] text-muted-foreground">{t("ovhExtra.payment.checking")}</p>;
  if (q.isError) {
    return <p className="text-[11px] text-muted-foreground">{t("ovhExtra.payment.unknown", { account: name, reason: apiMessage(q.error) || "" })}</p>;
  }
  const d = q.data;
  if (!d?.hasDefault) return <p role="alert" className="text-[11px] text-destructive">{t("ovhExtra.payment.none", { account: name })}</p>;
  if (!d.valid) return <p role="alert" className="text-[11px] text-destructive">{t("ovhExtra.payment.invalid", { account: name, reason: d.reason || d.status || "" })}</p>;
  return <p className="text-[11px] text-muted-foreground">{t("ovhExtra.payment.ok", { account: name, label: d.label || d.type || "" })}</p>;
}

/** 勾选"自动付款"时,检查下单账户有没有可用的默认支付方式 */
export function AutoPayCheck({ accountIds, enabled }: { accountIds: (string | undefined)[]; enabled: boolean }) {
  const ids = [...new Set(accountIds.filter((id): id is string => !!id))];
  if (!enabled || ids.length === 0) return null;
  return (
    <div className="space-y-1 pl-1">
      {ids.map((id) => <OneAccount key={id} accountId={id} />)}
    </div>
  );
}
