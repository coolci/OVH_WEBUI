import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { 
  User, 
  Mail,
  CreditCard,
  RefreshCw,
  ExternalLink,
  Building,
  MapPin,
  Calendar,
  DollarSign,
  FileText,
  Loader2,
  Receipt,
  Wallet,
  Link as LinkIcon,
  Undo2,
  Coins,
  Users
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useOvhAccount, useOvhBalance, useOvhOrders, useOvhBills, useOvhEmails, useOvhRefunds, useOvhCreditBalance, useOvhSubAccounts } from "@/hooks/useApi";
import { toast } from "sonner";

interface AccountInfo {
  nichandle?: string;
  email?: string;
  firstname?: string;
  firstName?: string;
  name?: string;
  country?: string;
  city?: string;
  zip?: string;
  address?: string;
  organisation?: string;
  currency?: string;
  ovhSubsidiary?: string;
  state?: string;
}

const AccountPage = () => {
  const { data: accountInfo, isLoading: isLoadingAccount, refetch: refetchAccount } = useOvhAccount();
  const { data: balanceData, refetch: refetchBalance } = useOvhBalance();
  const { data: creditBalanceData, isLoading: isLoadingCreditBalance, refetch: refetchCreditBalance } = useOvhCreditBalance();
  const { data: subAccountsData, isLoading: isLoadingSubAccounts, refetch: refetchSubAccounts } = useOvhSubAccounts();
  const { data: ordersData, isLoading: isLoadingOrders, refetch: refetchOrders } = useOvhOrders(20);
  const { data: billsData, isLoading: isLoadingBills, refetch: refetchBills } = useOvhBills(20);
  const { data: emailsData, isLoading: isLoadingEmails, refetch: refetchEmails } = useOvhEmails(50);
  const { data: refundsData, isLoading: isLoadingRefunds, refetch: refetchRefunds } = useOvhRefunds(20);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await Promise.all([
        refetchAccount(),
        refetchBalance(),
        refetchCreditBalance(),
        refetchSubAccounts(),
        refetchOrders(),
        refetchBills(),
        refetchEmails(),
        refetchRefunds(),
      ]);
      toast.success("数据刷新成功");
    } catch (error: any) {
      toast.error(`刷新失败: ${error.message}`);
    } finally {
      setIsRefreshing(false);
    }
  };

  // Helper to extract links from email body
  const extractLinks = (body: string): string[] => {
    const urlRegex = /(https?:\/\/[^\s<>"]+)/g;
    return body?.match(urlRegex) || [];
  };

  const account: AccountInfo = (accountInfo?.account || {}) as AccountInfo;
  const balance = balanceData?.balance;
  const creditBalances = creditBalanceData?.data || [];
  const subAccounts = subAccountsData?.data || [];
  const orders = ordersData?.orders || [];
  const bills = billsData?.bills || [];
  const emails = emailsData?.emails || [];
  const refunds = refundsData?.refunds || [];

  return (
    <>
      <Helmet>
        <title>账户管理 | OVH Sniper</title>
        <meta name="description" content="管理OVH账户信息" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                账户管理
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                查看和管理 OVH 账户信息
              </p>
            </div>
            
            <Button variant="terminal" onClick={handleRefresh} disabled={isRefreshing}>
              <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
              刷新数据
            </Button>
          </div>

          {/* Balance Cards */}
          {(balance || creditBalances.length > 0) ? (
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              {balance && (
                <div className="terminal-card p-4 border-primary/30">
                  <div className="flex items-center gap-2 mb-2 text-primary">
                    <Wallet className="h-4 w-4" />
                    <span className="text-xs uppercase">账户余额</span>
                  </div>
                  <p className="text-2xl font-bold font-mono">
                    {balance.value?.toFixed(2) || '0.00'} {balance.currencyCode || 'EUR'}
                  </p>
                </div>
              )}
              {creditBalances.map((credit: any, index: number) => (
                <div key={index} className="terminal-card p-4 border-accent/30">
                  <div className="flex items-center gap-2 mb-2 text-accent">
                    <Coins className="h-4 w-4" />
                    <span className="text-xs uppercase">{credit.balanceName || credit.type || '信用余额'}</span>
                  </div>
                  <p className="text-2xl font-bold font-mono">
                    {credit.balance?.value?.toFixed(2) || '0.00'} {credit.balance?.currencyCode || 'EUR'}
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">{credit.type}</p>
                </div>
              ))}
            </div>
          ) : null}

          <Tabs defaultValue="info" className="space-y-6">
            <TabsList className="bg-muted/50 border border-border flex-wrap h-auto gap-1 p-1">
              <TabsTrigger value="info" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <User className="h-4 w-4 mr-2" />
                账户信息
              </TabsTrigger>
              <TabsTrigger value="emails" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Mail className="h-4 w-4 mr-2" />
                邮件历史
              </TabsTrigger>
              <TabsTrigger value="refunds" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Undo2 className="h-4 w-4 mr-2" />
                退款记录
              </TabsTrigger>
              <TabsTrigger value="orders" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Receipt className="h-4 w-4 mr-2" />
                订单记录
              </TabsTrigger>
              <TabsTrigger value="bills" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <CreditCard className="h-4 w-4 mr-2" />
                账单记录
              </TabsTrigger>
              <TabsTrigger value="subaccounts" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Users className="h-4 w-4 mr-2" />
                子账户
              </TabsTrigger>
            </TabsList>

            {/* Account Info */}
            <TabsContent value="info">
              {isLoadingAccount ? (
                <div className="flex items-center justify-center py-12">
                  <Loader2 className="h-8 w-8 animate-spin text-primary" />
                </div>
              ) : !account.nichandle && !account.email ? (
                <div className="terminal-card p-8">
                  <div className="text-center py-8">
                    <User className="h-16 w-16 mx-auto mb-4 text-muted-foreground/30" />
                    <h3 className="text-lg font-medium text-muted-foreground mb-2">暂无账户信息</h3>
                    <p className="text-sm text-muted-foreground/70 mb-6 max-w-md mx-auto">
                      请先在设置页面配置 OVH API 凭证，然后刷新数据获取账户信息
                    </p>
                    <div className="flex justify-center gap-3">
                      <Button variant="outline" asChild>
                        <a href="/settings">
                          前往设置
                        </a>
                      </Button>
                      <Button variant="terminal" onClick={handleRefresh} disabled={isRefreshing}>
                        <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
                        重试
                      </Button>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                  <TerminalCard
                    title="基本信息"
                    icon={<User className="h-4 w-4" />}
                  >
                    <div className="space-y-4">
                      <div className="flex items-center gap-4 p-3 bg-primary/5 rounded-sm border border-primary/20">
                        <div className="h-12 w-12 rounded-sm bg-primary/20 flex items-center justify-center">
                          <User className="h-6 w-6 text-primary" />
                        </div>
                        <div>
                          <p className="font-bold text-lg">
                            {(account.firstname || account.firstName || '') + ' ' + (account.name || '')}
                          </p>
                          <p className="text-sm text-muted-foreground font-mono">{account.nichandle || '-'}</p>
                        </div>
                      </div>

                      <div className="grid grid-cols-2 gap-4 text-sm">
                        <div className="space-y-1">
                          <p className="text-muted-foreground flex items-center gap-1">
                            <Mail className="h-3 w-3" /> 邮箱
                          </p>
                          <p className="font-mono text-xs break-all">{account.email || '-'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-muted-foreground flex items-center gap-1">
                            <Building className="h-3 w-3" /> 组织
                          </p>
                          <p>{account.organisation || "-"}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-muted-foreground flex items-center gap-1">
                            <DollarSign className="h-3 w-3" /> 货币
                          </p>
                          <p>{account.currency || '-'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-muted-foreground">区域</p>
                          <p>{account.ovhSubsidiary || '-'}</p>
                        </div>
                      </div>
                    </div>
                  </TerminalCard>

                  <TerminalCard
                    title="地址信息"
                    icon={<MapPin className="h-4 w-4" />}
                  >
                    <div className="space-y-4 text-sm">
                      <div className="space-y-1">
                        <p className="text-muted-foreground">街道地址</p>
                        <p>{account.address || '-'}</p>
                      </div>
                      <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-1">
                          <p className="text-muted-foreground">城市</p>
                          <p>{account.city || '-'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-muted-foreground">邮编</p>
                          <p>{account.zip || '-'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-muted-foreground">国家</p>
                          <p>{account.country || '-'}</p>
                        </div>
                        <div className="space-y-1">
                          <p className="text-muted-foreground">账户状态</p>
                          {account.state ? (
                            <span className={cn(
                              "px-2 py-0.5 rounded-sm text-xs inline-block",
                              account.state === "complete" 
                                ? "bg-primary/20 text-primary" 
                                : "bg-warning/20 text-warning"
                            )}>
                              {account.state === "complete" ? "活跃" : account.state}
                            </span>
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </div>
                      </div>
                      
                      <Button variant="outline" size="sm" className="mt-4" asChild>
                        <a href="https://www.ovh.com/manager" target="_blank" rel="noopener noreferrer">
                          <ExternalLink className="h-4 w-4 mr-2" />
                          在 OVH 控制台编辑
                        </a>
                      </Button>
                    </div>
                  </TerminalCard>
                </div>
              )}
            </TabsContent>

            {/* Emails */}
            <TabsContent value="emails">
              <TerminalCard
                title="邮件历史 (最近50封)"
                icon={<Mail className="h-4 w-4" />}
              >
                {isLoadingEmails ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                  </div>
                ) : emails.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Mail className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无邮件记录</p>
                  </div>
                ) : (
                  <div className="space-y-4">
                    {emails.map((email: any) => {
                      const links = extractLinks(email.body || '');
                      return (
                        <div 
                          key={email.id}
                          className="p-4 rounded-sm border border-border hover:border-primary/30 transition-colors"
                        >
                          <div className="flex flex-col gap-2">
                            <div className="flex items-start justify-between gap-4">
                              <h4 className="font-medium text-foreground">{email.subject}</h4>
                              <span className="text-xs text-muted-foreground whitespace-nowrap">
                                {email.date ? new Date(email.date).toLocaleString("zh-CN") : '-'}
                              </span>
                            </div>
                            <p className="text-sm text-muted-foreground line-clamp-2">
                              {email.body?.replace(/<[^>]*>/g, '').substring(0, 200)}...
                            </p>
                            {links.length > 0 && (
                              <div className="flex flex-wrap gap-2 mt-2">
                                {links.slice(0, 3).map((link, idx) => (
                                  <Button key={idx} variant="outline" size="sm" asChild>
                                    <a href={link} target="_blank" rel="noopener noreferrer">
                                      <LinkIcon className="h-3 w-3 mr-1" />
                                      链接 {idx + 1}
                                    </a>
                                  </Button>
                                ))}
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Refunds */}
            <TabsContent value="refunds">
              <TerminalCard
                title="退款记录 (最近20条)"
                icon={<Undo2 className="h-4 w-4" />}
              >
                {isLoadingRefunds ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                  </div>
                ) : refunds.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Undo2 className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无退款记录</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                          <th className="text-left py-3 px-2">退款ID</th>
                          <th className="text-left py-3 px-2">关联订单</th>
                          <th className="text-left py-3 px-2">日期</th>
                          <th className="text-right py-3 px-2">金额</th>
                          <th className="text-center py-3 px-2">状态</th>
                        </tr>
                      </thead>
                      <tbody>
                        {refunds.map((refund: any) => (
                          <tr 
                            key={refund.refundId}
                            className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                          >
                            <td className="py-3 px-2 font-mono text-primary">{refund.refundId}</td>
                            <td className="py-3 px-2 font-mono text-muted-foreground">{refund.orderId || '-'}</td>
                            <td className="py-3 px-2 text-muted-foreground">
                              {refund.date ? new Date(refund.date).toLocaleDateString("zh-CN") : '-'}
                            </td>
                            <td className="py-3 px-2 text-right font-mono text-accent">
                              {refund.priceWithTax?.value?.toFixed(2) || '-'} {refund.priceWithTax?.currencyCode || '€'}
                            </td>
                            <td className="py-3 px-2 text-center">
                              <span className={cn(
                                "px-2 py-0.5 rounded-sm text-xs",
                                refund.status === 'done' ? "bg-primary/20 text-primary" : "bg-warning/20 text-warning"
                              )}>
                                {refund.status === 'done' ? "已完成" : refund.status || "处理中"}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Orders */}
            <TabsContent value="orders">
              <TerminalCard
                title="订单记录"
                icon={<Receipt className="h-4 w-4" />}
              >
                {isLoadingOrders ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                          <th className="text-left py-3 px-2">订单ID</th>
                          <th className="text-left py-3 px-2">日期</th>
                          <th className="text-right py-3 px-2">金额</th>
                          <th className="text-center py-3 px-2">状态</th>
                          <th className="text-center py-3 px-2">操作</th>
                        </tr>
                      </thead>
                      <tbody>
                        {orders.map((order: any, index: number) => (
                          <tr 
                            key={order.orderId}
                            className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                          >
                            <td className="py-3 px-2 font-mono text-primary">{order.orderId}</td>
                            <td className="py-3 px-2 text-muted-foreground">
                              {order.date ? new Date(order.date).toLocaleDateString("zh-CN") : '-'}
                            </td>
                            <td className="py-3 px-2 text-right font-mono text-accent">
                              {order.priceWithTax?.value?.toFixed(2) || order.priceWithTax || '-'} €
                            </td>
                            <td className="py-3 px-2 text-center">
                              <span className={cn(
                                "px-2 py-0.5 rounded-sm text-xs",
                                order.retractionDate ? "bg-primary/20 text-primary" : "bg-warning/20 text-warning"
                              )}>
                                {order.retractionDate ? "已完成" : "待付款"}
                              </span>
                            </td>
                            <td className="py-3 px-2 text-center">
                              {order.url && (
                                <Button variant="ghost" size="sm" asChild>
                                  <a href={order.url} target="_blank" rel="noopener noreferrer">
                                    <ExternalLink className="h-3 w-3" />
                                  </a>
                                </Button>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}

                {!isLoadingOrders && orders.length === 0 && (
                  <div className="text-center py-12 text-muted-foreground">
                    <Receipt className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无订单记录</p>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Bills */}
            <TabsContent value="bills">
              <TerminalCard
                title="账单记录"
                icon={<CreditCard className="h-4 w-4" />}
              >
                {isLoadingBills ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                          <th className="text-left py-3 px-2">账单ID</th>
                          <th className="text-left py-3 px-2">日期</th>
                          <th className="text-right py-3 px-2">金额</th>
                          <th className="text-center py-3 px-2">操作</th>
                        </tr>
                      </thead>
                      <tbody>
                        {bills.map((bill: any, index: number) => (
                          <tr 
                            key={bill.billId}
                            className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                          >
                            <td className="py-3 px-2 font-mono text-primary">{bill.billId}</td>
                            <td className="py-3 px-2 text-muted-foreground">
                              {bill.date ? new Date(bill.date).toLocaleDateString("zh-CN") : '-'}
                            </td>
                            <td className="py-3 px-2 text-right font-mono text-accent">
                              {bill.priceWithTax?.value?.toFixed(2) || bill.priceWithTax || '-'} €
                            </td>
                            <td className="py-3 px-2 text-center">
                              {bill.pdfUrl && (
                                <Button variant="ghost" size="sm" asChild>
                                  <a href={bill.pdfUrl} target="_blank" rel="noopener noreferrer">
                                    <FileText className="h-3 w-3" />
                                  </a>
                                </Button>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}

                {!isLoadingBills && bills.length === 0 && (
                  <div className="text-center py-12 text-muted-foreground">
                    <CreditCard className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无账单记录</p>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Sub Accounts */}
            <TabsContent value="subaccounts">
              <TerminalCard
                title="子账户管理"
                icon={<Users className="h-4 w-4" />}
              >
                {isLoadingSubAccounts ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                  </div>
                ) : subAccounts.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Users className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无子账户</p>
                    <p className="text-xs mt-2">子账户功能允许您创建和管理关联账户</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                          <th className="text-left py-3 px-2">ID</th>
                          <th className="text-left py-3 px-2">NIC Handle</th>
                          <th className="text-left py-3 px-2">描述</th>
                        </tr>
                      </thead>
                      <tbody>
                        {subAccounts.map((sub: any) => (
                          <tr 
                            key={sub.id}
                            className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                          >
                            <td className="py-3 px-2 font-mono text-primary">{sub.id}</td>
                            <td className="py-3 px-2 font-mono">{sub.nichandle}</td>
                            <td className="py-3 px-2 text-muted-foreground">{sub.description || '-'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>
          </Tabs>
        </div>
      </AppLayout>
    </>
  );
};

export default AccountPage;