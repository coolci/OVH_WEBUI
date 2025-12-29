import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { 
  History, 
  ExternalLink, 
  Trash2,
  CheckCircle2,
  XCircle,
  Clock,
  DollarSign,
  AlertTriangle
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";

interface PurchaseHistory {
  id: string;
  planCode: string;
  serverName: string;
  datacenter: string;
  options: string[];
  status: "success" | "failed";
  orderId?: string;
  orderUrl?: string;
  errorMessage?: string;
  purchaseTime: string;
  expirationTime?: string;
  price?: {
    withTax: number;
    withoutTax: number;
    tax: number;
    currencyCode: string;
  };
}

const mockHistory: PurchaseHistory[] = [
  {
    id: "h-001",
    planCode: "24ska01",
    serverName: "KS-A",
    datacenter: "gra",
    options: ["ram-32g-ecc-2400"],
    status: "success",
    orderId: "123456789",
    orderUrl: "https://ovh.com/order/123456789",
    purchaseTime: "2024-12-29T10:30:00",
    expirationTime: "2025-01-13T10:30:00",
    price: { withTax: 29.99, withoutTax: 24.99, tax: 5.00, currencyCode: "EUR" }
  },
  {
    id: "h-002",
    planCode: "24sk30",
    serverName: "KS-30",
    datacenter: "rbx",
    options: [],
    status: "success",
    orderId: "123456788",
    orderUrl: "https://ovh.com/order/123456788",
    purchaseTime: "2024-12-28T15:20:00",
    expirationTime: "2025-01-12T15:20:00",
    price: { withTax: 39.99, withoutTax: 33.32, tax: 6.67, currencyCode: "EUR" }
  },
  {
    id: "h-003",
    planCode: "24rise01",
    serverName: "RISE-1",
    datacenter: "sbg",
    options: ["ram-64g-ecc-2400"],
    status: "failed",
    errorMessage: "库存不足",
    purchaseTime: "2024-12-27T18:45:00",
  },
  {
    id: "h-004",
    planCode: "24adv01",
    serverName: "ADV-1",
    datacenter: "gra",
    options: [],
    status: "success",
    orderId: "123456787",
    orderUrl: "https://ovh.com/order/123456787",
    purchaseTime: "2024-12-25T09:00:00",
    expirationTime: "2025-01-09T09:00:00",
    price: { withTax: 89.99, withoutTax: 74.99, tax: 15.00, currencyCode: "EUR" }
  },
  {
    id: "h-005",
    planCode: "24ska01",
    serverName: "KS-A",
    datacenter: "rbx",
    options: [],
    status: "failed",
    errorMessage: "API请求超时",
    purchaseTime: "2024-12-24T14:30:00",
  },
];

const HistoryPage = () => {
  const [history] = useState(mockHistory);
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  const successCount = history.filter(h => h.status === "success").length;
  const failedCount = history.filter(h => h.status === "failed").length;
  const totalSpent = history
    .filter(h => h.status === "success" && h.price)
    .reduce((sum, h) => sum + (h.price?.withTax || 0), 0);

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString("zh-CN");
  };

  const getTimeRemaining = (expirationTime: string) => {
    const expiry = new Date(expirationTime).getTime();
    const diff = expiry - now;
    
    if (diff <= 0) return { text: "已过期", isExpired: true, isUrgent: true };
    
    const days = Math.floor(diff / 86400000);
    const hours = Math.floor((diff % 86400000) / 3600000);
    const minutes = Math.floor((diff % 3600000) / 60000);
    
    if (days > 0) {
      return { text: `${days}天 ${hours}小时`, isExpired: false, isUrgent: days < 3 };
    }
    if (hours > 0) {
      return { text: `${hours}小时 ${minutes}分钟`, isExpired: false, isUrgent: true };
    }
    return { text: `${minutes}分钟`, isExpired: false, isUrgent: true };
  };

  return (
    <>
      <Helmet>
        <title>购买历史 | OVH Sniper</title>
        <meta name="description" content="查看服务器购买记录和订单状态" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                购买历史
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                共 {history.length} 条记录
              </p>
            </div>
            
            <Button variant="outline" size="sm">
              <Trash2 className="h-4 w-4 mr-2" />
              清空历史
            </Button>
          </div>

          {/* Stats Cards */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="terminal-card p-4 border-primary/30">
              <div className="flex items-center gap-2 text-primary mb-1">
                <CheckCircle2 className="h-4 w-4" />
                <span className="text-xs uppercase">成功</span>
              </div>
              <p className="text-2xl font-bold text-primary">{successCount}</p>
            </div>
            <div className="terminal-card p-4 border-destructive/30">
              <div className="flex items-center gap-2 text-destructive mb-1">
                <XCircle className="h-4 w-4" />
                <span className="text-xs uppercase">失败</span>
              </div>
              <p className="text-2xl font-bold text-destructive">{failedCount}</p>
            </div>
            <div className="terminal-card p-4 border-accent/30">
              <div className="flex items-center gap-2 text-accent mb-1">
                <DollarSign className="h-4 w-4" />
                <span className="text-xs uppercase">总消费</span>
              </div>
              <p className="text-2xl font-bold text-accent">€{totalSpent.toFixed(2)}</p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 text-muted-foreground mb-1">
                <Clock className="h-4 w-4" />
                <span className="text-xs uppercase">待支付</span>
              </div>
              <p className="text-2xl font-bold">
                {history.filter(h => h.status === "success" && h.expirationTime && getTimeRemaining(h.expirationTime).isExpired === false).length}
              </p>
            </div>
          </div>

          {/* History List */}
          <TerminalCard
            title="订单记录"
            icon={<History className="h-4 w-4" />}
          >
            <div className="space-y-4">
              {history.map((item, index) => {
                const timeRemaining = item.expirationTime ? getTimeRemaining(item.expirationTime) : null;
                
                return (
                  <div 
                    key={item.id}
                    className={cn(
                      "p-4 rounded-sm border transition-all",
                      item.status === "success" 
                        ? "border-primary/20 bg-primary/5" 
                        : "border-destructive/20 bg-destructive/5"
                    )}
                    style={{ animationDelay: `${index * 50}ms` }}
                  >
                    <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                      {/* Order Info */}
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-2">
                          <StatusBadge status={item.status === "success" ? "completed" : "failed"} />
                          <span className="font-bold text-foreground">{item.serverName}</span>
                          <span className="text-xs text-muted-foreground font-mono">({item.planCode})</span>
                          <span className="text-xs text-muted-foreground uppercase bg-muted px-2 py-0.5 rounded-sm">
                            {item.datacenter}
                          </span>
                        </div>
                        
                        <div className="flex flex-wrap items-center gap-4 text-sm text-muted-foreground">
                          <span>下单时间: {formatTime(item.purchaseTime)}</span>
                          {item.orderId && (
                            <span>订单号: <span className="text-accent font-mono">#{item.orderId}</span></span>
                          )}
                          {item.errorMessage && (
                            <span className="text-destructive">错误: {item.errorMessage}</span>
                          )}
                        </div>

                        {item.options.length > 0 && (
                          <div className="flex flex-wrap gap-1 mt-2">
                            {item.options.map(opt => (
                              <span 
                                key={opt}
                                className="text-xs bg-muted px-2 py-0.5 rounded-sm text-muted-foreground"
                              >
                                {opt}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>

                      {/* Price & Expiry */}
                      {item.status === "success" && (
                        <div className="flex items-center gap-6">
                          {item.price && (
                            <div className="text-right">
                              <p className="text-lg font-bold text-primary">
                                €{item.price.withTax.toFixed(2)}
                              </p>
                              <p className="text-xs text-muted-foreground">
                                含税 (税额 €{item.price.tax.toFixed(2)})
                              </p>
                            </div>
                          )}
                          
                          {timeRemaining && (
                            <div className={cn(
                              "text-right px-3 py-2 rounded-sm border",
                              timeRemaining.isExpired 
                                ? "border-destructive/30 bg-destructive/10" 
                                : timeRemaining.isUrgent 
                                  ? "border-warning/30 bg-warning/10"
                                  : "border-border"
                            )}>
                              <div className="flex items-center gap-1 mb-1">
                                {timeRemaining.isUrgent && !timeRemaining.isExpired && (
                                  <AlertTriangle className="h-3 w-3 text-warning" />
                                )}
                                <p className="text-xs text-muted-foreground">剩余时间</p>
                              </div>
                              <p className={cn(
                                "font-mono font-bold",
                                timeRemaining.isExpired && "text-destructive",
                                timeRemaining.isUrgent && !timeRemaining.isExpired && "text-warning"
                              )}>
                                {timeRemaining.text}
                              </p>
                            </div>
                          )}
                          
                          {item.orderUrl && (
                            <Button variant="terminal" size="sm" asChild>
                              <a href={item.orderUrl} target="_blank" rel="noopener noreferrer">
                                <ExternalLink className="h-4 w-4 mr-1" />
                                支付订单
                              </a>
                            </Button>
                          )}
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>

            {history.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <History className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无购买记录</p>
              </div>
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default HistoryPage;
