import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { 
  MonitorDot, 
  Plus, 
  Trash2,
  Play,
  Square,
  Bell,
  Settings2,
  RefreshCw,
  Loader2,
  History as HistoryIcon,
  Clock
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { useVpsSubscriptions, useVpsMonitorStatus } from "@/hooks/useApi";
import { api } from "@/lib/api";
import { toast } from "sonner";

interface VpsSubscription {
  id: string;
  planCode: string;
  displayName?: string;
  ovhSubsidiary?: string;
  datacenters: string[];
  monitorLinux?: boolean;
  monitorWindows?: boolean;
  notifyAvailable: boolean;
  notifyUnavailable: boolean;
  lastStatus: Record<string, { linux?: string; windows?: string } | string>;
  autoOrder?: boolean;
}

const VpsMonitorPage = () => {
  const { data: subscriptions, isLoading, refetch } = useVpsSubscriptions();
  const { data: monitorStatus, refetch: refetchStatus } = useVpsMonitorStatus();
  const [isRunning, setIsRunning] = useState(false);
  const [isToggling, setIsToggling] = useState(false);
  const [isAdding, setIsAdding] = useState(false);
  const [isDeleting, setIsDeleting] = useState<string | null>(null);
  const [isClearing, setIsClearing] = useState(false);
  const [isBatchAdding, setIsBatchAdding] = useState(false);
  
  // 历史记录状态
  const [selectedSubscription, setSelectedSubscription] = useState<VpsSubscription | null>(null);
  const [subscriptionHistory, setSubscriptionHistory] = useState<any[]>([]);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);
  
  // 间隔设置状态
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [newInterval, setNewInterval] = useState(60);
  const [isSavingInterval, setIsSavingInterval] = useState(false);
  
  // 添加订阅表单状态
  const [newPlanCode, setNewPlanCode] = useState("");
  const [selectedDatacenters, setSelectedDatacenters] = useState<string[]>([]);
  const [notifyAvailable, setNotifyAvailable] = useState(true);
  const [notifyUnavailable, setNotifyUnavailable] = useState(false);
  const [autoOrder, setAutoOrder] = useState(false);

  useEffect(() => {
    if (monitorStatus) {
      setIsRunning(monitorStatus.running);
      setNewInterval(monitorStatus.checkInterval || 60);
    }
  }, [monitorStatus]);

  // 自动刷新
  useEffect(() => {
    const interval = setInterval(() => {
      refetch();
      refetchStatus();
    }, 10000);
    return () => clearInterval(interval);
  }, [refetch, refetchStatus]);

  const handleToggleMonitor = async () => {
    setIsToggling(true);
    try {
      if (isRunning) {
        await api.stopVpsMonitor();
        toast.success("VPS监控已停止");
      } else {
        await api.startVpsMonitor();
        toast.success("VPS监控已启动");
      }
      setIsRunning(!isRunning);
      refetchStatus();
    } catch (error: any) {
      toast.error(`操作失败: ${error.message}`);
    } finally {
      setIsToggling(false);
    }
  };

  const handleAddSubscription = async () => {
    if (!newPlanCode) {
      toast.error("请选择VPS型号");
      return;
    }
    setIsAdding(true);
    try {
      await api.addVpsSubscription({
        planCode: newPlanCode,
        datacenters: selectedDatacenters.length > 0 ? selectedDatacenters : undefined,
        notifyAvailable,
        notifyUnavailable,
        autoOrder,
      });
      toast.success("VPS订阅添加成功");
      refetch();
      // 重置表单
      setNewPlanCode("");
      setSelectedDatacenters([]);
      setNotifyAvailable(true);
      setNotifyUnavailable(false);
      setAutoOrder(false);
    } catch (error: any) {
      toast.error(`添加失败: ${error.message}`);
    } finally {
      setIsAdding(false);
    }
  };

  const handleRemoveSubscription = async (id: string) => {
    setIsDeleting(id);
    try {
      await api.removeVpsSubscription(id);
      toast.success("VPS订阅已删除");
      refetch();
    } catch (error: any) {
      toast.error(`删除失败: ${error.message}`);
    } finally {
      setIsDeleting(null);
    }
  };

  const handleClearSubscriptions = async () => {
    setIsClearing(true);
    try {
      const result = await api.clearVpsSubscriptions();
      toast.success(`已清空 ${result.count} 个VPS订阅`);
      refetch();
    } catch (error: any) {
      toast.error(`清空失败: ${error.message}`);
    } finally {
      setIsClearing(false);
    }
  };

  const handleBatchAddAll = async () => {
    setIsBatchAdding(true);
    try {
      const result = await api.batchAddAllVps({
        notifyAvailable: true,
        notifyUnavailable: false,
        autoOrder: false,
      });
      toast.success(`批量添加完成: 添加 ${result.added} 个, 跳过 ${result.skipped} 个`);
      refetch();
    } catch (error: any) {
      toast.error(`批量添加失败: ${error.message}`);
    } finally {
      setIsBatchAdding(false);
    }
  };

  const handleViewHistory = async (sub: VpsSubscription) => {
    setSelectedSubscription(sub);
    setIsLoadingHistory(true);
    try {
      const result = await api.getVpsSubscriptionHistory(sub.id);
      setSubscriptionHistory(result.history || []);
    } catch (error: any) {
      console.error('Failed to load history:', error);
      setSubscriptionHistory([]);
      toast.error("加载历史记录失败");
    } finally {
      setIsLoadingHistory(false);
    }
  };

  const handleSaveInterval = async () => {
    if (newInterval < 5 || newInterval > 3600) {
      toast.error("间隔必须在 5-3600 秒之间");
      return;
    }
    setIsSavingInterval(true);
    try {
      await api.updateVpsMonitorInterval(newInterval);
      toast.success(`检查间隔已更新为 ${newInterval} 秒`);
      setIsSettingsOpen(false);
      refetchStatus();
    } catch (error: any) {
      toast.error(`更新失败: ${error.message}`);
    } finally {
      setIsSavingInterval(false);
    }
  };

  const formatHistoryTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleString("zh-CN", { 
        month: "2-digit", 
        day: "2-digit",
        hour: "2-digit", 
        minute: "2-digit",
        second: "2-digit"
      });
    } catch {
      return "N/A";
    }
  };

  const subscriptionList = subscriptions || [];
  const checkInterval = monitorStatus?.checkInterval || 60;

  const getStatusColor = (status: string) => {
    if (status === "available") return "text-primary bg-primary/10";
    if (status === "unavailable") return "text-destructive bg-destructive/10";
    return "text-muted-foreground bg-muted";
  };

  const datacenterOptions = ["gra", "rbx", "sbg", "bhs", "waw", "lon"];
  const vpsPlans = [
    { value: "vps-starter", label: "VPS Starter" },
    { value: "vps-value", label: "VPS Value" },
    { value: "vps-essential", label: "VPS Essential" },
    { value: "vps-comfort", label: "VPS Comfort" },
    { value: "vps-elite", label: "VPS Elite" },
  ];

  return (
    <>
      <Helmet>
        <title>VPS 监控 | OVH Sniper</title>
        <meta name="description" content="监控OVH VPS库存变化" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                VPS 监控
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                监控 {subscriptionList.length} 款 VPS，检查间隔 {checkInterval} 秒
              </p>
            </div>
            
            <div className="flex gap-2">
              {/* 间隔设置 */}
              <Dialog open={isSettingsOpen} onOpenChange={setIsSettingsOpen}>
                <DialogTrigger asChild>
                  <Button variant="outline" size="sm">
                    <Settings2 className="h-4 w-4 mr-2" />
                    设置
                  </Button>
                </DialogTrigger>
                <DialogContent className="terminal-card border-primary/30">
                  <DialogHeader>
                    <DialogTitle className="text-primary">监控设置</DialogTitle>
                    <DialogDescription>
                      配置 VPS 监控参数
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>检查间隔 (秒)</Label>
                      <div className="flex items-center gap-2">
                        <input 
                          type="number" 
                          min="5" 
                          max="3600"
                          value={newInterval}
                          onChange={(e) => setNewInterval(parseInt(e.target.value) || 60)}
                          className="w-full px-3 py-2 bg-background border border-border rounded-sm text-sm"
                        />
                        <span className="text-sm text-muted-foreground">秒</span>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        建议值: 30-120秒，过低可能触发频率限制
                      </p>
                    </div>
                  </div>
                  <DialogFooter>
                    <DialogClose asChild>
                      <Button variant="outline">取消</Button>
                    </DialogClose>
                    <Button onClick={handleSaveInterval} disabled={isSavingInterval}>
                      {isSavingInterval && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                      保存设置
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
              <Button 
                variant="outline" 
                size="sm"
                onClick={() => { refetch(); refetchStatus(); }}
                disabled={isLoading}
              >
                <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
                刷新
              </Button>
              
              <Button 
                variant={isRunning ? "destructive" : "default"}
                size="sm"
                onClick={handleToggleMonitor}
                disabled={isToggling}
              >
                {isToggling ? (
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                ) : isRunning ? (
                  <Square className="h-4 w-4 mr-2" />
                ) : (
                  <Play className="h-4 w-4 mr-2" />
                )}
                {isRunning ? "停止监控" : "启动监控"}
              </Button>
              
              <Dialog>
                <DialogTrigger asChild>
                  <Button size="sm">
                    <Plus className="h-4 w-4 mr-2" />
                    添加订阅
                  </Button>
                </DialogTrigger>
                <DialogContent className="terminal-card border-primary/30 max-w-lg">
                  <DialogHeader>
                    <DialogTitle className="text-primary">添加 VPS 监控</DialogTitle>
                    <DialogDescription>
                      配置需要监控的 VPS 型号
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>VPS 型号</Label>
                      <Select value={newPlanCode} onValueChange={setNewPlanCode}>
                        <SelectTrigger>
                          <SelectValue placeholder="选择型号" />
                        </SelectTrigger>
                        <SelectContent>
                          {vpsPlans.map(plan => (
                            <SelectItem key={plan.value} value={plan.value}>
                              {plan.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>监控机房</Label>
                      <div className="grid grid-cols-3 gap-2">
                        {datacenterOptions.map(dc => (
                          <div key={dc} className="flex items-center space-x-2">
                            <Checkbox 
                              id={`vps-${dc}`}
                              checked={selectedDatacenters.includes(dc)}
                              onCheckedChange={(checked) => {
                                if (checked) {
                                  setSelectedDatacenters([...selectedDatacenters, dc]);
                                } else {
                                  setSelectedDatacenters(selectedDatacenters.filter(d => d !== dc));
                                }
                              }}
                            />
                            <label htmlFor={`vps-${dc}`} className="text-sm uppercase">{dc}</label>
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="space-y-3 pt-2">
                      <div className="flex items-center justify-between">
                        <Label>有货通知</Label>
                        <Switch checked={notifyAvailable} onCheckedChange={setNotifyAvailable} />
                      </div>
                      <div className="flex items-center justify-between">
                        <Label>无货通知</Label>
                        <Switch checked={notifyUnavailable} onCheckedChange={setNotifyUnavailable} />
                      </div>
                      <div className="flex items-center justify-between">
                        <Label>自动下单</Label>
                        <Switch checked={autoOrder} onCheckedChange={setAutoOrder} />
                      </div>
                    </div>
                  </div>
                  <DialogFooter>
                    <DialogClose asChild>
                      <Button variant="outline">取消</Button>
                    </DialogClose>
                    <Button onClick={handleAddSubscription} disabled={isAdding}>
                      {isAdding && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                      添加订阅
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {/* Batch Actions */}
          <div className="flex gap-2">
            <Button 
              variant="outline" 
              size="sm" 
              onClick={handleBatchAddAll}
              disabled={isBatchAdding}
            >
              {isBatchAdding ? (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <Plus className="h-4 w-4 mr-2" />
              )}
              批量添加全部VPS
            </Button>
            <Button 
              variant="outline" 
              size="sm" 
              className="text-destructive hover:text-destructive"
              onClick={handleClearSubscriptions}
              disabled={isClearing || subscriptionList.length === 0}
            >
              {isClearing ? (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <Trash2 className="h-4 w-4 mr-2" />
              )}
              清空所有订阅
            </Button>
          </div>

          {/* Status Cards */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className={cn(
              "terminal-card p-4",
              isRunning ? "border-primary/30" : "border-destructive/30"
            )}>
              <div className="flex items-center gap-2 mb-1">
                <MonitorDot className={cn(
                  "h-4 w-4",
                  isRunning ? "text-primary animate-pulse" : "text-destructive"
                )} />
                <span className="text-xs uppercase text-muted-foreground">监控状态</span>
              </div>
              <p className={cn(
                "text-lg font-bold",
                isRunning ? "text-primary" : "text-destructive"
              )}>
                {isRunning ? "运行中" : "已停止"}
              </p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 mb-1 text-muted-foreground">
                <span className="text-xs uppercase">订阅数量</span>
              </div>
              <p className="text-lg font-bold">{subscriptionList.length}</p>
            </div>
            <div className="terminal-card p-4 border-accent/30">
              <div className="flex items-center gap-2 mb-1 text-accent">
                <Bell className="h-4 w-4" />
                <span className="text-xs uppercase">检查间隔</span>
              </div>
              <p className="text-lg font-bold text-accent">{checkInterval}s</p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 mb-1 text-muted-foreground">
                <span className="text-xs uppercase">有货 VPS</span>
              </div>
              <p className="text-lg font-bold">
                {subscriptionList.filter((s: VpsSubscription) => {
                  const statuses = Object.values(s.lastStatus || {});
                  return statuses.some(v => {
                    if (typeof v === 'string') return v === 'available';
                    return v?.linux === 'available' || v?.windows === 'available';
                  });
                }).length}
              </p>
            </div>
          </div>

          {/* Subscriptions List */}
          <TerminalCard
            title="VPS 订阅列表"
            icon={<MonitorDot className="h-4 w-4" />}
          >
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : (
              <div className="space-y-4">
                {subscriptionList.map((sub: VpsSubscription, index: number) => (
                  <div 
                    key={sub.id}
                    className="p-4 rounded-sm border border-border hover:border-primary/30 transition-all"
                  >
                    <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4">
                      {/* Subscription Info */}
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-3">
                          <span className="font-bold text-lg text-foreground">{sub.displayName || sub.planCode}</span>
                          <span className="text-xs text-muted-foreground font-mono">({sub.planCode})</span>
                          {sub.ovhSubsidiary && (
                            <span className="text-xs px-2 py-0.5 bg-muted rounded-sm">{sub.ovhSubsidiary}</span>
                          )}
                        </div>
                        
                        {/* Datacenter Status Grid */}
                        <div className="overflow-x-auto">
                          <table className="w-full text-xs">
                            <thead>
                              <tr className="text-muted-foreground uppercase">
                                <th className="text-left py-1 pr-4">机房</th>
                                <th className="text-center py-1 px-2">状态</th>
                              </tr>
                            </thead>
                            <tbody>
                              {(sub.datacenters || []).map(dc => {
                                const status = sub.lastStatus?.[dc];
                                const statusStr = typeof status === 'string' 
                                  ? status 
                                  : (status?.linux || status?.windows || 'unknown');
                                return (
                                  <tr key={dc} className="border-t border-border/50">
                                    <td className="py-2 pr-4 uppercase font-mono">{dc}</td>
                                    <td className="py-2 px-2 text-center">
                                      <span className={cn(
                                        "px-2 py-0.5 rounded-sm",
                                        getStatusColor(statusStr)
                                      )}>
                                        {statusStr === "available" ? "有货" : "无货"}
                                      </span>
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        </div>
                      </div>

                      {/* Actions */}
                      <div className="flex gap-2">
                        <Button 
                          variant="ghost" 
                          size="sm"
                          onClick={() => handleViewHistory(sub)}
                        >
                          <HistoryIcon className="h-4 w-4" />
                        </Button>
                        <Button 
                          variant="ghost" 
                          size="sm" 
                          className="text-destructive hover:text-destructive"
                          onClick={() => handleRemoveSubscription(sub.id)}
                          disabled={isDeleting === sub.id}
                        >
                          {isDeleting === sub.id ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <Trash2 className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {!isLoading && subscriptionList.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <MonitorDot className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无 VPS 监控订阅</p>
              </div>
            )}
          </TerminalCard>

          {/* History Dialog */}
          <Dialog open={!!selectedSubscription} onOpenChange={(open) => !open && setSelectedSubscription(null)}>
            <DialogContent className="terminal-card border-primary/30 max-w-2xl max-h-[80vh] overflow-hidden">
              <DialogHeader>
                <DialogTitle className="text-primary flex items-center gap-2">
                  <HistoryIcon className="h-5 w-5" />
                  历史记录 - {selectedSubscription?.displayName || selectedSubscription?.planCode}
                </DialogTitle>
                <DialogDescription>
                  库存状态变化历史
                </DialogDescription>
              </DialogHeader>
              <div className="py-4 max-h-[50vh] overflow-y-auto">
                {isLoadingHistory ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                  </div>
                ) : subscriptionHistory.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <HistoryIcon className="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>暂无历史记录</p>
                  </div>
                ) : (
                  <div className="space-y-2">
                    {subscriptionHistory.map((item, index) => (
                      <div 
                        key={index}
                        className={cn(
                          "flex items-center justify-between p-3 rounded-sm border",
                          item.changeType === "available" || item.status === "available"
                            ? "border-primary/30 bg-primary/5"
                            : "border-border bg-muted/30"
                        )}
                      >
                        <div className="flex items-center gap-3">
                          <Clock className="h-4 w-4 text-muted-foreground" />
                          <span className="text-sm text-muted-foreground">
                            {formatHistoryTime(item.timestamp)}
                          </span>
                          <span className="text-xs uppercase font-mono bg-muted px-2 py-0.5 rounded-sm">
                            {item.datacenter}
                          </span>
                        </div>
                        <div className="flex items-center gap-2">
                          <span className={cn(
                            "text-xs px-2 py-1 rounded-sm",
                            item.status === "available" || item.changeType === "available"
                              ? "bg-primary/10 text-primary"
                              : "bg-destructive/10 text-destructive"
                          )}>
                            {item.changeType === "available" || item.status === "available" ? "有货" : "无货"}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">关闭</Button>
                </DialogClose>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </AppLayout>
    </>
  );
};

export default VpsMonitorPage;