import { useRef, useState } from "react";
import { fmtMoney } from "@/i18n/format";
import { useTranslation } from "react-i18next";
import { CreditCard, Loader2, RefreshCw, Wallet } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { LoadFailed } from "@/components/common/LoadFailed";
import { errorMessage } from "@/lib/api-error";
import { safeHttpsUrl } from "@/lib/utils";
import { useOrderPaymentInfo, usePayAccountOrder } from "@/hooks/use-account";

/** 付款窗口只展示只读预检结果；明确确认后才调用绑定方式或账户余额付款 mutation。 */
export function OrderPaymentButton({
  accountId,
  orderId,
  managerUrl,
  showButton = true,
  className,
}: {
  accountId: string;
  orderId: number;
  managerUrl?: string;
  showButton?: boolean;
  className?: string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState("");
  const [confirmedQuote, setConfirmedQuote] = useState("");
  // 同步闸门：连点两下时 isPending 尚未重渲染，保证单次确认只发一次付款请求。
  const submittingRef = useRef(false);
  const info = useOrderPaymentInfo(accountId, orderId, open);
  const pay = usePayAccountOrder(orderId);
  const data = info.data;
  const methods = data?.methods || [];
  const balances = data?.balances || [];
  const methodKey = (m: (typeof methods)[number]) => `m:${m.type}:${m.id}`;
  const balanceKey = (b: (typeof balances)[number]) => `b:${b.name}`;
  type Choice =
    | { kind: "method"; key: string; m: (typeof methods)[number] }
    | { kind: "balance"; key: string; b: (typeof balances)[number] };
  const choices: Choice[] = [
    ...methods.map((m) => ({ kind: "method" as const, key: methodKey(m), m })),
    ...balances.filter((b) => b.enough).map((b) => ({ kind: "balance" as const, key: balanceKey(b), b })),
  ];
  const choice = selected
    ? choices.find((c) => c.key === selected)
    : choices.find((c) => c.kind === "method" && c.m.default) || choices[0];
  const balanceTypeLabel = (type: string) => t(`account.payment.balanceType.${type}`, { defaultValue: type });
  // 确认绑定实际账户、订单、金额、币种及所选方式/余额；后台刷新改变任一项都需重新勾选。
  const quote =
    data && choice
      ? JSON.stringify([
          data.accountId,
          data.orderId,
          data.amount,
          data.currency,
          choice.kind === "method" ? [choice.m.id, choice.m.type] : [choice.b.name, choice.b.amount, choice.b.currency],
        ])
      : "";
  const confirmed = !!quote && confirmedQuote === quote;
  const displayAmount =
    data?.amount != null && data.currency
      ? fmtMoney(data.amount, data.currency)
      : data?.priceText || "—";
  const paymentUrl = safeHttpsUrl(managerUrl) || (data?.accountId === accountId ? safeHttpsUrl(data.managerUrl) : "");
  const ready =
    !!data &&
    !info.isError &&
    data.status === "notPaid" &&
    !data.attempted &&
    !!choice &&
    data.amount != null &&
    Number.isFinite(data.amount) &&
    data.amount >= 0 &&
    !!data.currency &&
    data.accountId === accountId &&
    data.orderId === orderId &&
    !info.isFetching &&
    !pay.isPending &&
    !pay.isSuccess;

  /** 关闭后清除确认，下一次打开必须重新核对。 */
  const changeOpen = (value: boolean) => {
    if (pay.isPending || submittingRef.current) return;
    setOpen(value);
    setSelected("");
    setConfirmedQuote("");
    submittingRef.current = false;
    pay.reset();
  };
  /** 捕获付款结果供窗口呈现，不会自动重发。 */
  const submit = async () => {
    if (!ready || !confirmed || !choice || !data || data.amount == null || submittingRef.current) return;
    submittingRef.current = true;
    try {
      await pay.mutateAsync(
        choice.kind === "balance"
          ? {
              kind: "balance",
              accountId: data.accountId,
              balanceName: choice.b.name,
              amount: data.amount,
              currency: data.currency,
              confirm: true,
            }
          : {
              kind: "method",
              accountId: data.accountId,
              paymentMethodId: choice.m.id,
              paymentType: choice.m.type,
              amount: data.amount,
              currency: data.currency,
              confirm: true,
            }
      );
      submittingRef.current = false;
    } catch {
      setConfirmedQuote("");
      submittingRef.current = false;
    }
  };
  // 已付款时隐藏历史行上的按钮，但保留正在打开的结果窗口。
  if (!showButton && !open) return null;
  return (
    <>
      {showButton && (
        <Button variant="outline" size="sm" className={className} onClick={() => changeOpen(true)}>
          <CreditCard className="w-3.5 h-3.5" />
          {t("account.payment.open")}
        </Button>
      )}
      <Dialog open={open} onOpenChange={changeOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="pr-6 leading-7">
              {t("account.payment.title", { id: orderId })}
            </DialogTitle>
            <DialogDescription>
              {t("account.payment.description")}
            </DialogDescription>
          </DialogHeader>
          {info.isPending ? (
            <p role="status" className="text-sm text-muted-foreground">
              {t("account.payment.loading")}
            </p>
          ) : info.isError ? (
            <LoadFailed
              title={t("account.payment.loadFailed")}
              error={info.error}
              onRetry={() => info.refetch()}
              compact
            />
          ) : (
            data && (
              <div className="space-y-4">
                <dl className="space-y-4 border-b border-border pb-4 text-sm">
                  <div className="flex justify-between gap-4">
                    <dt className="text-muted-foreground">
                      {t("account.payment.account")}
                    </dt>
                    <dd className="break-all">
                      {data.accountName || data.accountId}
                    </dd>
                  </div>
                  <div className="flex justify-between gap-4">
                    <dt className="text-muted-foreground">
                      {t("account.payment.amount")}
                    </dt>
                    <dd className="text-xl font-semibold tabular-nums">
                      {displayAmount}
                    </dd>
                  </div>
                </dl>
                <Button
                  className="h-9"
                  variant="outline"
                  onClick={() => info.refetch()}
                  disabled={info.isFetching || pay.isPending}
                >
                  <RefreshCw
                    aria-hidden="true"
                    className={`w-3.5 h-3.5 ${
                      info.isFetching
                        ? "animate-spin motion-reduce:animate-none"
                        : ""
                    }`}
                  />
                  {t("common.refresh")}
                </Button>
                {data.status !== "notPaid" ? (
                  <p role="status" className="text-sm">
                    {t(`account.payment.status.${data.status}`, { defaultValue: t("account.payment.notPayable") })}
                  </p>
                ) : data.attempted ? (
                  <p role="status" className="text-sm text-warning">
                    {t("account.payment.attempted")}
                  </p>
                ) : !choices.length ? (
                  <p role="status" className="text-sm text-warning">
                    {t("account.payment.noMethods")}
                  </p>
                ) : (
                  <div className="space-y-3">
                    <div className="space-y-2">
                      <div id={`payment-method-${orderId}`} className="text-sm font-medium">
                        {t("account.payment.method")}
                      </div>
                      <div
                        role="radiogroup"
                        aria-labelledby={`payment-method-${orderId}`}
                        className="space-y-2 max-h-[min(45vh,360px)] overflow-y-auto overscroll-contain -mx-1 px-1 py-0.5"
                      >
                        {methods.length > 0 && balances.length > 0 && (
                          <div className="text-[11.5px] text-muted-foreground">
                            {t("account.payment.groupMethods")}
                          </div>
                        )}
                        {methods.map((m) => (
                          <PayOption
                            key={methodKey(m)}
                            icon={<CreditCard className="w-4 h-4" />}
                            title={m.label}
                            sub={`#${m.id}${m.default ? ` · ${t("account.payment.default")}` : ""}`}
                            selected={choice?.key === methodKey(m)}
                            disabled={pay.isPending || pay.isSuccess}
                            onSelect={() => {
                              setSelected(methodKey(m));
                              setConfirmedQuote("");
                            }}
                          />
                        ))}
                        {balances.length > 0 && (
                          <div className={`text-[11.5px] text-muted-foreground ${methods.length ? "pt-1" : ""}`}>
                            {t("account.payment.groupBalances")}
                          </div>
                        )}
                        {balances.map((b) => {
                          const amountText = Number.isFinite(b.amount) && b.currency ? fmtMoney(b.amount, b.currency) : b.text;
                          const leftText =
                            b.enough && data.amount != null && b.currency === data.currency
                              ? fmtMoney(Math.max(0, b.amount - data.amount), b.currency)
                              : "";
                          return (
                            <PayOption
                              key={balanceKey(b)}
                              icon={<Wallet className="w-4 h-4" />}
                              tone="balance"
                              title={balanceTypeLabel(b.type)}
                              sub={
                                b.enough
                                  ? t("account.payment.balanceLeft", { amount: amountText, left: leftText })
                                  : `${amountText} · ${b.currency !== data.currency ? t("account.payment.balanceCurrency") : t("account.payment.balanceShort")}`
                              }
                              selected={choice?.key === balanceKey(b)}
                              disabled={!b.enough || pay.isPending || pay.isSuccess}
                              onSelect={() => {
                                setSelected(balanceKey(b));
                                setConfirmedQuote("");
                              }}
                            />
                          );
                        })}
                      </div>
                    </div>
                    {choice?.kind === "balance" && (
                      <p className="text-xs leading-5 text-muted-foreground">
                        {t("account.payment.balanceNote")}
                      </p>
                    )}
                    <label
                      className={`flex cursor-pointer items-start gap-3 rounded-xl border p-3.5 text-sm leading-5 transition-colors ${
                        confirmed ? "border-primary/50 bg-primary/[.05]" : "border-border hover:bg-muted/40"
                      }`}
                    >
                      <Checkbox
                        className="mt-0.5"
                        checked={confirmed}
                        onCheckedChange={(v) => setConfirmedQuote(v === true ? quote : "")}
                        disabled={!ready}
                      />
                      {choice?.kind === "balance"
                        ? t("account.payment.balanceConfirm", { name: balanceTypeLabel(choice.b.type) })
                        : t("account.payment.confirm")}
                    </label>
                  </div>
                )}
                {data.status === "notPaid" && !data.attempted && data.balanceError && (
                  <p className="text-xs leading-5 text-muted-foreground">
                    {t("account.payment.balanceError", { detail: data.balanceError })}
                  </p>
                )}
              </div>
            )
          )}
          {pay.isError && (
            <p role="alert" className="text-sm text-destructive break-words">
              {errorMessage(pay.error)}
            </p>
          )}
          {pay.isSuccess && (
            <p role="status" className="text-sm text-success">
              {pay.variables?.kind === "balance"
                ? pay.data?.status === "paid"
                  ? t("account.payment.balancePaid", { status: pay.data.orderStatus || "" })
                  : t("account.payment.balanceSubmitted")
                : t("account.payment.submitted")}
            </p>
          )}
          {paymentUrl && (
            <Button asChild variant="outline" className="h-10">
              <a href={paymentUrl} target="_blank" rel="noopener noreferrer">
                {t("account.payment.payOnOvh")}
              </a>
            </Button>
          )}
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => changeOpen(false)}
              disabled={pay.isPending}
            >
              {t("common.close")}
            </Button>
            <Button
              className="h-10"
              onClick={submit}
              disabled={!ready || !confirmed}
            >
              {pay.isPending && (
                <Loader2 className="w-4 h-4 animate-spin motion-reduce:animate-none" />
              )}
              {pay.isPending
                ? t("account.payment.submitting")
                : t("account.payment.pay", { amount: displayAmount })}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

/** 付款选项卡片：图标 + 名称 + 说明 + 右侧单选圆点；支持键盘导航（role=radio） */
function PayOption({
  icon,
  title,
  sub,
  selected,
  disabled,
  onSelect,
  tone,
}: {
  icon: React.ReactNode;
  title: string;
  sub: string;
  selected: boolean;
  disabled?: boolean;
  onSelect: () => void;
  tone?: "balance";
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      disabled={disabled}
      onClick={onSelect}
      className={`w-full flex items-center gap-3 rounded-xl border px-3.5 py-2.5 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed ${
        selected
          ? "border-primary/60 bg-primary/[.05] ring-1 ring-primary/25"
          : "border-border hover:bg-muted/40 disabled:hover:bg-transparent"
      } ${disabled && !selected ? "opacity-55" : ""}`}
    >
      <span
        className={`w-9 h-9 rounded-lg flex items-center justify-center shrink-0 ${
          tone === "balance" ? "bg-success/10 text-success" : "bg-primary/10 text-primary"
        }`}
        aria-hidden
      >
        {icon}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-[13.5px] font-medium truncate">{title}</span>
        <span className="block text-[12px] text-muted-foreground truncate tabular-nums">{sub}</span>
      </span>
      <span
        className={`w-4 h-4 rounded-full border-2 shrink-0 flex items-center justify-center ${
          selected ? "border-primary" : "border-muted-foreground/40"
        }`}
        aria-hidden
      >
        {selected && <span className="w-2 h-2 rounded-full bg-primary" />}
      </span>
    </button>
  );
}
