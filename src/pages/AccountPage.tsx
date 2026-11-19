import { LoadFailed, LoadFailedBanner } from "@/components/common/LoadFailed";
import { PartialNotice } from "@/components/common/PartialNotice";
import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { User, Mail, RefreshCw, FileText, Inbox, ShieldCheck, ShoppingCart, ExternalLink, CalendarClock, Undo2, Wallet, type LucideIcon } from "lucide-react";
import { formatDate, formatDateTime } from "@/lib/format-os";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { fmtMoney } from "@/i18n/format";
import { cn, safeHttpsUrl } from "@/lib/utils";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { OrderPaymentButton } from "@/components/account/OrderPaymentButton";
import { WalletActions } from "@/components/account/WalletActions";
import { OfficialChat } from "@/components/support/OfficialChat";
import { AccountSecurityPanel } from "@/components/account/AccountSecurityPanel";
import {
  useAccountInfo,
  useCreditBalances,
  useRefunds,
  useEmails,
  useOrders,
  useAccountScope,
  type EmailHistoryEntry,
} from "@/hooks/use-account";

/** 账户管理：顶部 KPI + 钱包余额卡 + Tabs (邮件 / 订单 / 退款 / 客服 / 安全) */
function AccountPage() {
  const info = useAccountInfo();
  const balances = useCreditBalances();
  const accountId = useAccountScope();
  const nicSub =
    info.data?.nichandle && info.data.nichandle.toLowerCase() !== (info.data.email || "").toLowerCase()
      ? info.data.nichandle
      : undefined;
  return (
    <div className="space-y-5">
      <PageHeader
        icon={User}
        title="账户管理"
        description="查看和管理您的 OVH 账户信息"
        action={
          <Button
            variant="outline"
            size="sm"
            className="h-8 text-xs font-medium gap-1.5 rounded-lg border-border/80 hover:bg-secondary px-3"
            onClick={() => {
              void info.refetch();
              void balances.refetch();
            }}
            disabled={info.isFetching || balances.isFetching}
            title="刷新账户与余额信息"
          >
            <RefreshCw className={cn("w-3.5 h-3.5", (info.isFetching || balances.isFetching) && "animate-spin")} />
            <span>刷新</span>
          </Button>
        }
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3.5 items-stretch">
        <KpiCard
          icon={User}
          label="客户代码"
          value={info.data?.customerCode}
          sub={nicSub}
          loading={info.isPending}
          badge={
            info.data && (
              <Chip tone={info.data.kycValidated ? "success" : "warning"}>
                <ShieldCheck className="w-3 h-3" />
                {info.data.kycValidated ? "已验证" : "未验证"}
              </Chip>
            )
          }
        />
        <KpiCard icon={Mail} label="邮箱" value={info.data?.email} loading={info.isPending} />
        <KpiCard
          icon={User}
          label="账户持有人"
          value={info.data ? `${info.data.firstname ?? ""} ${info.data.name ?? ""}`.trim() : undefined}
          sub={info.data?.city && info.data?.country ? `${info.data.city}, ${info.data.country}` : undefined}
          loading={info.isPending}
        />
      </div>

      <WalletBalanceCard accountId={accountId} balances={balances} />

      <Tabs defaultValue="emails" className="space-y-4">
        <TabsList className="flex overflow-x-auto no-scrollbar sm:inline-flex w-full sm:w-auto justify-start h-auto gap-1 p-1 bg-muted/50 rounded-lg border border-border/60">
          <TabsTrigger value="emails" className="shrink-0 whitespace-nowrap text-xs px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">邮件历史</TabsTrigger>
          <TabsTrigger value="orders" className="shrink-0 whitespace-nowrap text-xs px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">订单记录</TabsTrigger>
          <TabsTrigger value="refunds" className="shrink-0 whitespace-nowrap text-xs px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">退款记录</TabsTrigger>
          <TabsTrigger value="chat" className="shrink-0 whitespace-nowrap text-xs px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">在线客服</TabsTrigger>
          <TabsTrigger value="security" className="shrink-0 whitespace-nowrap text-xs px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">API 安全</TabsTrigger>
        </TabsList>
        <TabsContent value="emails">
          <EmailsTab />
        </TabsContent>
        <TabsContent value="orders">
          <OrdersTab />
        </TabsContent>
        <TabsContent value="chat">
          <OfficialChat />
        </TabsContent>
        <TabsContent value="security">
          <SecurityTab />
        </TabsContent>
        <TabsContent value="refunds">
          <RefundsTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}

function KpiCard({
  icon: Icon,
  label,
  value,
  sub,
  loading,
  badge,
}: {
  icon: LucideIcon;
  label: string;
  value?: string;
  sub?: string;
  loading?: boolean;
  badge?: React.ReactNode;
}) {
  return (
    <Card className="surface-card rounded-xl border-border hover:border-border/80 transition-all duration-200 shadow-sm">
      <CardContent className="p-4 sm:p-5 flex items-start gap-3.5">
        <div className="w-9 h-9 rounded-lg bg-secondary border border-border/60 text-foreground flex items-center justify-center flex-shrink-0">
          <Icon className="w-4 h-4" strokeWidth={1.75} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <p className="text-[12px] text-muted-foreground">{label}</p>
            {badge}
          </div>
          {loading ? (
            <Skeleton className="h-6 w-32 mt-1" />
          ) : (
            <p className="text-lg font-bold truncate text-foreground" title={value}>{value || "—"}</p>
          )}
          {sub && <p className="text-[11px] text-muted-foreground mt-0.5 truncate">{sub}</p>}
        </div>
      </CardContent>
    </Card>
  );
}

/** 账户钱包卡：汇总可用信用余额（按币种）+ 各项余额详情 + 即将过期提醒 + 充值/兑换券码入口 */
function WalletBalanceCard({
  accountId,
  balances,
}: {
  accountId: string;
  balances: ReturnType<typeof useCreditBalances>;
}) {
  const { t } = useTranslation();
  const list = balances.data?.list || [];
  const totals = new Map<string, number>();
  for (const b of list) {
    if (!b.usable) continue;
    const v = b.amount?.value;
    const cur = b.amount?.currencyCode || "";
    if (typeof v === "number" && Number.isFinite(v) && v > 0 && cur) {
      totals.set(cur, (totals.get(cur) || 0) + v);
    }
  }
  const totalSummary = [...totals.entries()].map(([cur, v]) => fmtMoney(v, cur)).join(" + ");

  return (
    <Card className="surface-card rounded-xl border-primary/25 bg-gradient-to-br from-primary/[.05] via-card to-card">
      <CardContent className="p-4 sm:p-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3.5">
          <div className="flex items-start gap-3 min-w-0">
            <div className="w-9 h-9 rounded-lg bg-primary/10 border border-primary/20 text-primary flex items-center justify-center shrink-0 mt-0.5">
              <Wallet className="w-4 h-4" />
            </div>
            <div className="min-w-0 space-y-0.5">
              <div className="flex items-center gap-2 flex-wrap text-xs">
                <span className="font-semibold text-foreground">{t("account.balance.title")}</span>
                <span className="text-muted-foreground">{t("account.balance.hint")}</span>
              </div>
              {balances.isPending ? (
                <Skeleton className="h-7 w-40 mt-1" />
              ) : balances.isError ? (
                <div className="pt-1">
                  <LoadFailed
                    title={t("account.balance.loadFailed")}
                    error={balances.error}
                    onRetry={() => void balances.refetch()}
                    compact
                  />
                </div>
              ) : (
                <p className="text-2xl font-bold font-mono tabular-nums tracking-tight text-foreground pt-0.5">
                  {totalSummary || t("account.balance.empty")}
                </p>
              )}
            </div>
          </div>
          {accountId && (
            <div className="shrink-0 self-start sm:self-center">
              <WalletActions accountId={accountId} />
            </div>
          )}
        </div>
        {!balances.isPending && !balances.isError && list.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2.5 pt-3.5 mt-3.5 border-t border-border/50">
            {list.map((b) => {
              const label = t(`account.payment.balanceType.${b.type}`, { defaultValue: b.type || b.balanceName });
              const amt =
                b.amount?.value != null && b.amount.currencyCode
                  ? fmtMoney(b.amount.value, b.amount.currencyCode)
                  : b.amount?.text || "—";
              const exp = (b.expiring || []).find((e) => e.expirationDate && e.amount?.value);
              return (
                <div
                  key={b.balanceName}
                  className={cn(
                    "rounded-lg border px-3 py-2.5 text-xs bg-background/80 flex flex-col justify-between gap-1",
                    b.usable ? "border-border/80" : "border-border/50 opacity-60"
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium text-muted-foreground truncate">{label}</span>
                    {!b.usable && (
                      <Chip tone="default" className="text-[10px] shrink-0">
                        {t("account.balance.unusable")}
                      </Chip>
                    )}
                  </div>
                  <div className="font-mono text-sm font-semibold tabular-nums text-foreground">{amt}</div>
                  {exp?.expirationDate && exp.amount?.value != null && exp.amount.currencyCode && (
                    <div className="text-[10.5px] text-warning tabular-nums">
                      {t("account.balance.expiring", {
                        amount: fmtMoney(exp.amount.value, exp.amount.currencyCode),
                        date: formatDate(exp.expirationDate),
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
        {(balances.data?.failed || 0) > 0 && (
          <p className="text-[11px] text-warning mt-2.5">
            {t("account.balance.partial", { n: balances.data?.failed })}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

function EmailsTab() {
  const accountId = useAccountScope();
  const emails = useEmails();
  const [selected, setSelected] = useState<EmailHistoryEntry | null>(null);

  useEffect(() => {
    setSelected(null);
  }, [accountId]);

  const activeEmail =
    selected && (emails.data?.items || []).some((item) => item.id === selected.id)
      ? selected
      : null;

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[360px_1fr] gap-4">
      <Card className="surface-card rounded-xl border-border overflow-hidden">
        <div className="px-4 py-3 border-b border-border/60 flex items-center justify-between">
          <span className="text-sm font-semibold">邮件列表</span>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs font-medium gap-1.5 rounded-lg border-border/80 hover:bg-secondary px-2.5"
            onClick={() => emails.refetch()}
            disabled={emails.isFetching}
          >
            <RefreshCw className={cn("w-3.5 h-3.5", emails.isFetching && "animate-spin")} />
            <span>刷新</span>
          </Button>
        </div>
        {emails.isError && emails.data && <LoadFailedBanner title="刷新失败，显示上次读取的记录" error={emails.error} onRetry={() => emails.refetch()} />}
        <PartialNotice what="邮件" failedCount={emails.data?.failed || 0} />
        {emails.isError && !emails.data ? (
          <LoadFailed error={emails.error} onRetry={() => emails.refetch()} />
        ) : emails.isPending ? (
          <div className="p-4 space-y-2">
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-14 rounded-lg" />)}
          </div>
        ) : (emails.data?.items || []).length === 0 ? (
          <EmptyState icon={Inbox} title="暂无邮件" />
        ) : (
          <div className="divide-y divide-border/40 max-h-[500px] overflow-y-auto">
            {(emails.data?.items || []).map((e) => (
              <button
                key={e.id}
                type="button"
                onClick={() => setSelected(e)}
                className={
                  "w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors " +
                  (activeEmail?.id === e.id ? "bg-muted border-l-2 border-primary" : "")
                }
              >
                <div className="flex items-center gap-2 mb-1">
                  <Mail className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
                  <p className="text-[13px] font-medium truncate">{e.subject}</p>
                </div>
                <p className="text-[11px] text-muted-foreground">{new Date(e.date).toLocaleString("zh-CN")}</p>
              </button>
            ))}
          </div>
        )}
      </Card>

      <Card className="surface-card rounded-xl border-border shadow-sm">
        <CardContent className="p-4 sm:p-5">
          <div className="flex items-center gap-2 mb-4">
            <FileText className="w-4 h-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">邮件详情</h3>
          </div>
          {activeEmail ? (
            <>
              <p className="text-[15px] font-semibold mb-1">{activeEmail.subject}</p>
              <p className="text-[12px] text-muted-foreground mb-4">{new Date(activeEmail.date).toLocaleString("zh-CN")}</p>
              <pre className="text-[12px] font-mono whitespace-pre-wrap text-foreground bg-muted/40 border border-border/50 rounded-lg p-3.5 max-h-[400px] overflow-y-auto">
                {activeEmail.body}
              </pre>
            </>
          ) : (
            <EmptyState icon={Mail} title="请选择一封邮件" />
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function SecurityTab() {
  const accountId = useAccountScope();
  return <AccountSecurityPanel accountId={accountId} />;
}

function OrdersTab() {
  const orders = useOrders(30);
  const accountId = useAccountScope();
  return (
    <Card className="surface-card rounded-xl border-border shadow-sm">
      <CardContent className="p-4 sm:p-5">
        <div className="flex items-center justify-between mb-4 gap-2">
          <h3 className="text-sm font-semibold flex items-center gap-2">
            <ShoppingCart className="w-4 h-4 text-foreground" />
            最近订单
          </h3>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs font-medium gap-1.5 rounded-lg border-border/80 hover:bg-secondary px-2.5"
            onClick={() => orders.refetch()}
            disabled={orders.isFetching}
          >
            <RefreshCw className={cn("w-3.5 h-3.5", orders.isFetching && "animate-spin")} />
            <span>刷新</span>
          </Button>
        </div>
        {orders.isError && orders.data && <LoadFailedBanner title="刷新失败，显示上次读取的记录" error={orders.error} onRetry={() => orders.refetch()} />}
        <PartialNotice what="订单" failedCount={orders.data?.failed || 0} />
        {orders.isError && !orders.data ? (
          <LoadFailed error={orders.error} onRetry={() => orders.refetch()} />
        ) : orders.isPending ? (
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
          </div>
        ) : (orders.data?.items || []).length === 0 ? (
          <EmptyState icon={ShoppingCart} title="暂无订单记录" />
        ) : (
          <div className="divide-y divide-border/40">
            {(orders.data?.items || []).map((o, idx) => {
              const id = o.orderId != null ? o.orderId : idx;
              // 主显应付含税总价 priceWithTax，与快捷支付窗口实际扣款金额保持严格一致
              const hasWithTax = o.priceWithTax?.value != null || Boolean(o.priceWithTax?.text);
              const priceObj = hasWithTax ? o.priceWithTax : o.priceWithoutTax;
              const priceText =
                priceObj?.text ||
                (priceObj?.value != null
                  ? `${priceObj.value} ${priceObj.currencyCode || ""}`.trim()
                  : "—");
              const showExTaxNote =
                hasWithTax &&
                o.priceWithoutTax?.value != null &&
                o.priceWithTax?.value != null &&
                o.priceWithoutTax.value !== o.priceWithTax.value;
              const exTaxText = showExTaxNote
                ? o.priceWithoutTax?.text ||
                  `${o.priceWithoutTax?.value} ${o.priceWithoutTax?.currencyCode || ""}`.trim()
                : "";
              const dateStr = formatDateTime(o.date);
              return (
                <div
                  key={String(id)}
                  className="py-3 flex flex-col sm:flex-row sm:items-center justify-between gap-2 sm:gap-4"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap mb-1">
                      <span className="font-mono text-sm font-semibold">#{id}</span>
                      {o.expirationDate && (
                        <Chip tone="default" className="font-mono text-[11px] gap-1">
                          <CalendarClock className="w-3 h-3 text-muted-foreground flex-shrink-0" />
                          <span>到期 {formatDate(o.expirationDate)}</span>
                        </Chip>
                      )}
                      {Boolean(o.retractionDate) && (
                        <Chip tone="warning" className="font-mono text-[11px] gap-1">
                          <Undo2 className="w-3 h-3 text-warning flex-shrink-0" />
                          <span>撤销截止 {formatDate(String(o.retractionDate))}</span>
                        </Chip>
                      )}
                    </div>
                    <p className="text-[11px] text-muted-foreground font-mono">{dateStr}</p>
                  </div>
                  <div className="flex flex-wrap items-center gap-3 sm:text-right flex-shrink-0">
                    <div>
                      <p className="text-base sm:text-lg font-bold font-mono text-foreground">{priceText}</p>
                      {exTaxText && (
                        <p className="text-[11px] font-mono text-muted-foreground">
                          未税 {exTaxText}
                        </p>
                      )}
                    </div>
                    {o.orderStatus === "notPaid" && typeof o.orderId === "number" && (
                      <OrderPaymentButton
                        accountId={o.accountId || accountId}
                        orderId={o.orderId}
                        managerUrl={typeof o.url === "string" ? safeHttpsUrl(o.url) : undefined}
                      />
                    )}
                    {o.paymentStatusError && (
                      <span className="text-[11px] text-warning" title={o.paymentStatusError}>
                        付款状态未能读取，请刷新核对
                      </span>
                    )}
                    {safeHttpsUrl(typeof o.url === "string" ? o.url : undefined) && (
                      <a
                        href={safeHttpsUrl(String(o.url))}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-[11px] text-foreground hover:underline min-h-8"
                      >
                        <ExternalLink className="w-3.5 h-3.5" />
                        打开
                      </a>
                    )}
                    {safeHttpsUrl(typeof o.pdfUrl === "string" ? o.pdfUrl : undefined) && (
                      <a
                        href={safeHttpsUrl(String(o.pdfUrl))}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-[11px] text-muted-foreground hover:underline min-h-8 inline-flex items-center"
                      >
                        PDF
                      </a>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function RefundsTab() {
  const refunds = useRefunds();
  return (
    <Card className="surface-card rounded-xl border-border shadow-sm">
      <CardContent className="p-4 sm:p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-sm font-semibold">退款记录</h3>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs font-medium gap-1.5 rounded-lg border-border/80 hover:bg-secondary px-2.5"
            onClick={() => refunds.refetch()}
            disabled={refunds.isFetching}
          >
            <RefreshCw className={cn("w-3.5 h-3.5", refunds.isFetching && "animate-spin")} />
            <span>刷新</span>
          </Button>
        </div>
        {refunds.isError && refunds.data && <LoadFailedBanner title="刷新失败，显示上次读取的记录" error={refunds.error} onRetry={() => refunds.refetch()} />}
        <PartialNotice what="退款" failedCount={refunds.data?.failed || 0} />
        {refunds.isError && !refunds.data ? (
          <LoadFailed error={refunds.error} onRetry={() => refunds.refetch()} />
        ) : refunds.isPending ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-16 rounded-lg" />)}
          </div>
        ) : (refunds.data?.items || []).length === 0 ? (
          <EmptyState icon={Inbox} title="暂无退款记录" />
        ) : (
          <div className="divide-y divide-border">
            {(refunds.data?.items || []).map((r) => {
              const refundPriceText =
                r.priceWithTax?.text ||
                (r.priceWithTax?.value != null
                  ? `${r.priceWithTax.value} ${r.priceWithTax.currencyCode || ""}`.trim()
                  : "—");
              return (
                <div key={r.refundId} className="py-3 flex items-center justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-mono text-sm font-semibold">#{r.refundId}</span>
                      <Chip tone="default">订单 {r.orderId}</Chip>
                    </div>
                    <p className="text-[11px] text-muted-foreground">{r.date ? new Date(r.date).toLocaleString("zh-CN") : "—"}</p>
                  </div>
                  <div className="text-right flex-shrink-0">
                    <p className="text-lg font-bold text-success">{refundPriceText}</p>
                    {safeHttpsUrl(r.pdfUrl) && (
                      <a href={safeHttpsUrl(r.pdfUrl)} target="_blank" rel="noopener noreferrer" className="text-[11px] text-foreground hover:underline">
                        下载 PDF
                      </a>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}


const Page = () => (
  <>
    <Helmet>
      <title>账户管理 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <AccountPage />
    </AppLayout>
  </>
);

export default Page;
