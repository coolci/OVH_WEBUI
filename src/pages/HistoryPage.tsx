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
  AlertTriangle,
  Loader2,
  RefreshCw
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import api from "@/lib/api";
import { usePurchaseHistory } from "@/hooks/useApi";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";

const HistoryPage = () => {
  const { data: history, isLoading, refetch } = usePurchaseHistory();
  const [now, setNow] = useState(Date.now());
  const [isClearing, setIsClearing] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    const interval = setInterval(refetch, 30000);
    return () => clearInterval(interval);
  }, [refetch]);

  const historyList = history || [];

  const successCount = historyList.filter(h => h.status === "success").length;
  const failedCount = historyList.filter(h => h.status === "failed").length;
  const totalSpent = historyList
    .filter(h => h.status === "success" && h.price)
    .reduce((sum, h) => sum + (h.price?.withTax || 0), 0);

  const formatTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleString("zh-CN");
    } catch {
      return "N/A";
    }
  };

  const getTimeRemaining = (expirationTime: string) => {
    try {
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
    } catch {
      return { text: "N/A", isExpired: false, isUrgent: false };
    }
  };

  const handleClearHistory = async () => {
    setIsClearing(true);
    try {
      await api.clearPurchaseHistory();
      toast.success("购买历史已清空");
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsClearing(false);
    }
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await refetch();
      toast.success("数据已刷新");
    } finally {
      setIsRefreshing(false);
    }
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
                共 {historyList.length} 条记录
              </p>
            </div>
            
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleRefresh} disabled={isRefreshing}>
                <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
                刷新
              </Button>
              
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button 
                    variant="outline" 
                    size="sm" 
                    className="text-destructive hover:text-destructive"
                    disabled={historyList.length === 0}
                  >
                    <Trash2 className="h-4 w-4 mr-2" />
                    清空历史
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent className="terminal-card border-destructive/30">
                  <AlertDialogHeader>
                    <AlertDialogTitle className="text-destructive">确认清空历史记录？</AlertDialogTitle>
                    <AlertDialogDescription>
                      此操作将删除所有 {historyList.length} 条购买记录，且无法撤销。
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>取消</AlertDialogCancel>
                    <AlertDialogAction 
                      onClick={handleClearHistory}
                      className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                      disabled={isClearing}
                    >
                      {isClearing && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                      确认清空
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
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
                <span className="text-xs uppercase">总订单</span>
              </div>
              <p className="text-2xl font-bold">{historyList.length}</p>
            </div>
          </div>

          {/* History List */}
          <TerminalCard
            title="订单记录"
            icon={<History className="h-4 w-4" />}
          >
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : historyList.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <History className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无购买记录</p>
              </div>
            ) : (
              <div className="space-y-4">
                {historyList.map((item, index) => {
                  const timeRemaining = (item as any).expirationTime ? getTimeRemaining((item as any).expirationTime) : null;
                  
                  return (
                    <div 
                      key={item.id}
                      className={cn(
                        "p-4 rounded-sm border transition-all",
                        item.status === "success" 
                          ? "border-primary/20 bg-primary/5" 
                          : "border-destructive/20 bg-destructive/5"
                      )}
                    >
                      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                        {/* Order Info */}
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-2">
                            <StatusBadge status={item.status === "success" ? "completed" : "failed"} />
                            <span className="font-bold text-foreground">{item.planCode}</span>
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

                          {item.options && item.options.length > 0 && (
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
                                  €{(item.price as any).withTax?.toFixed(2) || 'N/A'}
                                </p>
                                <p className="text-xs text-muted-foreground">
                                  {item.price.currency || 'EUR'}
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
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default HistoryPage;
