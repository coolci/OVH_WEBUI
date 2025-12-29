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
  FileText
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

interface AccountInfo {
  email: string;
  firstname: string;
  name: string;
  nichandle: string;
  organisation: string;
  country: string;
  city: string;
  zip: string;
  address: string;
  currency: string;
  ovhSubsidiary: string;
  state: string;
}

interface Refund {
  id: string;
  date: string;
  amount: number;
  currency: string;
  status: string;
}

interface EmailHistory {
  id: string;
  date: string;
  subject: string;
  body: string;
}

const mockAccountInfo: AccountInfo = {
  email: "user@example.com",
  firstname: "John",
  name: "Doe",
  nichandle: "xx12345-ovh",
  organisation: "Example Corp",
  country: "IE",
  city: "Dublin",
  zip: "D01 AB12",
  address: "123 Example Street",
  currency: "EUR",
  ovhSubsidiary: "IE",
  state: "active",
};

const mockRefunds: Refund[] = [
  { id: "RF-001", date: "2024-12-15", amount: 29.99, currency: "EUR", status: "completed" },
  { id: "RF-002", date: "2024-11-20", amount: 49.99, currency: "EUR", status: "completed" },
  { id: "RF-003", date: "2024-10-05", amount: 89.99, currency: "EUR", status: "pending" },
];

const mockEmails: EmailHistory[] = [
  { id: "EM-001", date: "2024-12-28", subject: "订单确认 #123456789", body: "您的订单已确认..." },
  { id: "EM-002", date: "2024-12-25", subject: "付款成功通知", body: "您的付款已处理成功..." },
  { id: "EM-003", date: "2024-12-20", subject: "服务开通通知", body: "您的服务器已开通..." },
];

const AccountPage = () => {
  const [accountInfo] = useState(mockAccountInfo);
  const [refunds] = useState(mockRefunds);
  const [emails] = useState(mockEmails);
  const [isLoading, setIsLoading] = useState(false);

  const handleRefresh = () => {
    setIsLoading(true);
    setTimeout(() => setIsLoading(false), 1500);
  };

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
            
            <Button variant="terminal" onClick={handleRefresh} disabled={isLoading}>
              <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
              刷新数据
            </Button>
          </div>

          <Tabs defaultValue="info" className="space-y-6">
            <TabsList className="bg-muted/50 border border-border">
              <TabsTrigger value="info" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <User className="h-4 w-4 mr-2" />
                账户信息
              </TabsTrigger>
              <TabsTrigger value="refunds" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <CreditCard className="h-4 w-4 mr-2" />
                退款记录
              </TabsTrigger>
              <TabsTrigger value="emails" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Mail className="h-4 w-4 mr-2" />
                邮件历史
              </TabsTrigger>
            </TabsList>

            {/* Account Info */}
            <TabsContent value="info">
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
                        <p className="font-bold text-lg">{accountInfo.firstname} {accountInfo.name}</p>
                        <p className="text-sm text-muted-foreground font-mono">{accountInfo.nichandle}</p>
                      </div>
                    </div>

                    <div className="grid grid-cols-2 gap-4 text-sm">
                      <div className="space-y-1">
                        <p className="text-muted-foreground flex items-center gap-1">
                          <Mail className="h-3 w-3" /> 邮箱
                        </p>
                        <p className="font-mono">{accountInfo.email}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground flex items-center gap-1">
                          <Building className="h-3 w-3" /> 组织
                        </p>
                        <p>{accountInfo.organisation || "-"}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground flex items-center gap-1">
                          <DollarSign className="h-3 w-3" /> 货币
                        </p>
                        <p>{accountInfo.currency}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground">区域</p>
                        <p>{accountInfo.ovhSubsidiary}</p>
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
                      <p>{accountInfo.address}</p>
                    </div>
                    <div className="grid grid-cols-2 gap-4">
                      <div className="space-y-1">
                        <p className="text-muted-foreground">城市</p>
                        <p>{accountInfo.city}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground">邮编</p>
                        <p>{accountInfo.zip}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground">国家</p>
                        <p>{accountInfo.country}</p>
                      </div>
                      <div className="space-y-1">
                        <p className="text-muted-foreground">账户状态</p>
                        <span className={cn(
                          "px-2 py-0.5 rounded-sm text-xs",
                          accountInfo.state === "active" 
                            ? "bg-primary/20 text-primary" 
                            : "bg-destructive/20 text-destructive"
                        )}>
                          {accountInfo.state === "active" ? "活跃" : "非活跃"}
                        </span>
                      </div>
                    </div>
                    
                    <Button variant="outline" size="sm" className="mt-4">
                      <ExternalLink className="h-4 w-4 mr-2" />
                      在 OVH 控制台编辑
                    </Button>
                  </div>
                </TerminalCard>
              </div>
            </TabsContent>

            {/* Refunds */}
            <TabsContent value="refunds">
              <TerminalCard
                title="退款记录"
                icon={<CreditCard className="h-4 w-4" />}
              >
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                        <th className="text-left py-3 px-2">退款ID</th>
                        <th className="text-left py-3 px-2">日期</th>
                        <th className="text-right py-3 px-2">金额</th>
                        <th className="text-center py-3 px-2">状态</th>
                      </tr>
                    </thead>
                    <tbody>
                      {refunds.map((refund, index) => (
                        <tr 
                          key={refund.id}
                          className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                          style={{ animationDelay: `${index * 50}ms` }}
                        >
                          <td className="py-3 px-2 font-mono text-primary">{refund.id}</td>
                          <td className="py-3 px-2 text-muted-foreground">
                            {new Date(refund.date).toLocaleDateString("zh-CN")}
                          </td>
                          <td className="py-3 px-2 text-right font-mono text-accent">
                            €{refund.amount.toFixed(2)}
                          </td>
                          <td className="py-3 px-2 text-center">
                            <span className={cn(
                              "px-2 py-0.5 rounded-sm text-xs",
                              refund.status === "completed" 
                                ? "bg-primary/20 text-primary" 
                                : "bg-warning/20 text-warning"
                            )}>
                              {refund.status === "completed" ? "已完成" : "处理中"}
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {refunds.length === 0 && (
                  <div className="text-center py-12 text-muted-foreground">
                    <CreditCard className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无退款记录</p>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Email History */}
            <TabsContent value="emails">
              <TerminalCard
                title="邮件历史"
                icon={<Mail className="h-4 w-4" />}
              >
                <div className="space-y-3">
                  {emails.map((email, index) => (
                    <div 
                      key={email.id}
                      className="p-4 rounded-sm border border-border hover:border-primary/30 transition-colors cursor-pointer"
                      style={{ animationDelay: `${index * 50}ms` }}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2 mb-1">
                            <FileText className="h-4 w-4 text-primary flex-shrink-0" />
                            <p className="font-medium truncate">{email.subject}</p>
                          </div>
                          <p className="text-sm text-muted-foreground line-clamp-2">{email.body}</p>
                        </div>
                        <div className="text-xs text-muted-foreground flex items-center gap-1">
                          <Calendar className="h-3 w-3" />
                          {new Date(email.date).toLocaleDateString("zh-CN")}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>

                {emails.length === 0 && (
                  <div className="text-center py-12 text-muted-foreground">
                    <Mail className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>暂无邮件记录</p>
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
