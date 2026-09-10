import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { User, Mail, RefreshCw, FileText, Inbox, ShieldCheck, ShoppingCart, ExternalLink, type LucideIcon } from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import {
  useAccountInfo,
  useRefunds,
  useEmails,
  useOrders,
  type EmailHistoryEntry,
} from "@/hooks/use-account";

/** 账户管理：顶部 3 张 KPI + Tabs (邮件 / 订单 / 退款) */
function AccountPage() {
  const info = useAccountInfo();
  return (
    <div className="space-y-6">
      <PageHeader icon={User} title="账户管理" description="查看和管理您的 OVH 账户信息" />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <KpiCard
          icon={User}
          label="客户代码"
          value={info.data?.customerCode}
          sub={info.data?.nichandle}
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

      <Tabs defaultValue="emails">
        <TabsList className="grid grid-cols-3 sm:flex h-auto gap-1 p-1 bg-muted/50 rounded-lg border border-border/60">
          <TabsTrigger value="emails" className="text-[12px] sm:text-xs px-2.5 sm:px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">邮件历史</TabsTrigger>
          <TabsTrigger value="orders" className="text-[12px] sm:text-xs px-2.5 sm:px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">订单记录</TabsTrigger>
          <TabsTrigger value="refunds" className="text-[12px] sm:text-xs px-2.5 sm:px-3.5 py-1.5 rounded-md data-[state=active]:bg-background data-[state=active]:text-foreground data-[state=active]:shadow-sm font-medium">退款记录</TabsTrigger>
        </TabsList>
        <TabsContent value="emails">
          <EmailsTab />
        </TabsContent>
        <TabsContent value="orders">
          <OrdersTab />
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

function EmailsTab() {
  const emails = useEmails();
  const [selected, setSelected] = useState<EmailHistoryEntry | null>(null);

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <Card className="surface-card rounded-xl border-border overflow-hidden shadow-sm">
        <div className="px-4 py-3 border-b border-border/60 flex items-center justify-between">
          <span className="text-sm font-semibold">邮件列表</span>
          <Button variant="outline" size="sm" className="h-8 text-xs gap-1.5 border-border/80 hover:bg-secondary" onClick={() => emails.refetch()} disabled={emails.isFetching}>
            <RefreshCw className={`w-3.5 h-3.5 ${emails.isFetching ? "animate-spin" : ""}`} />
            <span className="hidden sm:inline">刷新</span>
          </Button>
        </div>
        {emails.isPending ? (
          <div className="p-4 space-y-2">
            {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-14 rounded-lg" />)}
          </div>
        ) : (emails.data || []).length === 0 ? (
          <EmptyState icon={Inbox} title="暂无邮件" />
        ) : (
          <div className="divide-y divide-border/40 max-h-[500px] overflow-y-auto">
            {(emails.data || []).map((e) => (
              <button
                key={e.id}
                type="button"
                onClick={() => setSelected(e)}
                className={
                  "w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors " +
                  (selected?.id === e.id ? "bg-muted border-l-2 border-primary" : "")
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
          {selected ? (
            <>
              <p className="text-[15px] font-semibold mb-1">{selected.subject}</p>
              <p className="text-[12px] text-muted-foreground mb-4">{new Date(selected.date).toLocaleString("zh-CN")}</p>
              <pre className="text-[12px] font-mono whitespace-pre-wrap text-foreground bg-muted/40 border border-border/50 rounded-lg p-3.5 max-h-[400px] overflow-y-auto">
                {selected.body}
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

function OrdersTab() {
  const orders = useOrders(30);
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
            className="h-8 text-xs gap-1.5 border-border/80 hover:bg-secondary"
            onClick={() => orders.refetch()}
            disabled={orders.isFetching}
          >
            <RefreshCw className={`w-3.5 h-3.5 ${orders.isFetching ? "animate-spin" : ""}`} />
            <span className="hidden sm:inline">刷新</span>
          </Button>
        </div>
        {orders.isPending ? (
          <div className="space-y-2">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-16 rounded-lg" />
            ))}
          </div>
        ) : (orders.data || []).length === 0 ? (
          <EmptyState icon={ShoppingCart} title="暂无订单记录" />
        ) : (
          <div className="divide-y divide-border/40">
            {(orders.data || []).map((o, idx) => {
              const id = o.orderId != null ? o.orderId : idx;
              const priceText =
                o.priceWithTax?.text ||
                (o.priceWithTax?.value != null
                  ? `${o.priceWithTax.value} ${o.priceWithTax.currencyCode || ""}`
                  : "—");
              const dateStr = o.date ? new Date(String(o.date)).toLocaleString("zh-CN") : "—";
              return (
                <div
                  key={String(id)}
                  className="py-3 flex flex-col sm:flex-row sm:items-center justify-between gap-2 sm:gap-4"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap mb-1">
                      <span className="font-mono text-sm font-semibold">#{id}</span>
                      {o.expirationDate && (
                        <Chip tone="default">
                          到期 {new Date(String(o.expirationDate)).toLocaleDateString("zh-CN")}
                        </Chip>
                      )}
                    </div>
                    <p className="text-[11px] text-muted-foreground">{dateStr}</p>
                  </div>
                  <div className="flex items-center gap-3 sm:text-right flex-shrink-0">
                    <p className="text-base sm:text-lg font-bold font-mono text-foreground">{priceText}</p>
                    {o.url && (
                      <a
                        href={String(o.url)}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-[11px] text-foreground hover:underline min-h-8"
                      >
                        <ExternalLink className="w-3.5 h-3.5" />
                        打开
                      </a>
                    )}
                    {o.pdfUrl && (
                      <a
                        href={String(o.pdfUrl)}
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
          <Button variant="outline" size="sm" className="h-8 text-xs gap-1.5 border-border/80 hover:bg-secondary" onClick={() => refunds.refetch()} disabled={refunds.isFetching}>
            <RefreshCw className={`w-3.5 h-3.5 ${refunds.isFetching ? "animate-spin" : ""}`} />
            <span className="hidden sm:inline">刷新</span>
          </Button>
        </div>
        {refunds.isPending ? (
          <div className="space-y-2">
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-16 rounded-lg" />)}
          </div>
        ) : (refunds.data || []).length === 0 ? (
          <EmptyState icon={Inbox} title="暂无退款记录" />
        ) : (
          <div className="divide-y divide-border">
            {(refunds.data || []).map((r) => (
              <div key={r.refundId} className="py-3 flex items-center justify-between gap-4">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-mono text-sm font-semibold">#{r.refundId}</span>
                    <Chip tone="default">订单 {r.orderId}</Chip>
                  </div>
                  <p className="text-[11px] text-muted-foreground">{new Date(r.date).toLocaleString("zh-CN")}</p>
                </div>
                <div className="text-right flex-shrink-0">
                  <p className="text-lg font-bold text-success">{r.priceWithTax.text}</p>
                  {r.pdfUrl && (
                    <a href={r.pdfUrl} target="_blank" rel="noopener noreferrer" className="text-[11px] text-foreground hover:underline">
                      下载 PDF
                    </a>
                  )}
                </div>
              </div>
            ))}
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
