import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
  import { 
    Activity, 
    Plus, 
    Trash2,
    Bell,
    BellOff,
    ShoppingCart,
    History as HistoryIcon,
    Settings2,
    Eye,
    RefreshCw,
    Loader2,
    Edit
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
import { useSubscriptions, useMonitorStatus, useServers } from "@/hooks/useApi";
import { toast } from "sonner";

interface Subscription {
  planCode: string;
  serverName: string;
  datacenters: string[];
  notifyAvailable: boolean;
  notifyUnavailable: boolean;
  autoOrder: boolean;
  quantity: number;
  lastStatus: Record<string, string>;
  createdAt?: string;
  history: { timestamp: string; datacenter: string; status: string; changeType: string }[];
}

const MonitorPage = () => {
  const { data: subscriptions, isLoading, refetch } = useSubscriptions();
  const { data: monitorStatus, refetch: refetchStatus } = useMonitorStatus();
  const { data: servers } = useServers();
  const [selectedSubscription, setSelectedSubscription] = useState<Subscription | null>(null);
  const [subscriptionHistory, setSubscriptionHistory] = useState<any[]>([]);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);
  const [isAdding, setIsAdding] = useState(false);
  const [isDeleting, setIsDeleting] = useState<string | null>(null);
  const [isClearing, setIsClearing] = useState(false);
  
  // 间隔设置状态
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [newInterval, setNewInterval] = useState(5);
  const [isSavingInterval, setIsSavingInterval] = useState(false);
  
  // 添加订阅表单状态
  const [newPlanCode, setNewPlanCode] = useState("");
  const [selectedDatacenters, setSelectedDatacenters] = useState<string[]>([]);
  const [notifyAvailable, setNotifyAvailable] = useState(true);
  const [notifyUnavailable, setNotifyUnavailable] = useState(false);
  const [autoOrder, setAutoOrder] = useState(false);
  const [quantity, setQuantity] = useState(1);
  
  // 编辑订阅状态
  const [editingSubscription, setEditingSubscription] = useState<Subscription | null>(null);
  const [editNotifyAvailable, setEditNotifyAvailable] = useState(true);
  const [editNotifyUnavailable, setEditNotifyUnavailable] = useState(false);
  const [editAutoOrder, setEditAutoOrder] = useState(false);
  const [editQuantity, setEditQuantity] = useState(1);
  const [isUpdating, setIsUpdating] = useState(false);
  const [isBatchAdding, setIsBatchAdding] = useState(false);

  useEffect(() => {
    if (monitorStatus) {
      setNewInterval(monitorStatus.checkInterval || 5);
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

  const handleAddSubscription = async () => {
    if (!newPlanCode) {
      toast.error("请选择服务器型号");
      return;
    }
    setIsAdding(true);
    try {
      await api.addSubscription({
        planCode: newPlanCode,
        datacenters: selectedDatacenters.length > 0 ? selectedDatacenters : undefined,
        notifyAvailable,
        notifyUnavailable,
        autoOrder,
        quantity,
      });
      toast.success("订阅添加成功");
      refetch();
      // 重置表单
      setNewPlanCode("");
      setSelectedDatacenters([]);
      setNotifyAvailable(true);
      setNotifyUnavailable(false);
      setAutoOrder(false);
      setQuantity(1);
    } catch (error: any) {
      toast.error(`添加失败: ${error.message}`);
    } finally {
      setIsAdding(false);
    }
  };

  const handleRemoveSubscription = async (planCode: string) => {
    setIsDeleting(planCode);
    try {
      await api.removeSubscription(planCode);
      toast.success("订阅已删除");
      refetch();
    } catch (error: any) {
      toast.error(`删除失败: ${error.message}`);
    } finally {
      setIsDeleting(null);
    }
  };

  const handleBatchAddAll = async () => {
    setIsBatchAdding(true);
    try {
      const result = await api.batchAddAllServers({
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

  const handleClearSubscriptions = async () => {
    setIsClearing(true);
    try {
      const result = await api.clearSubscriptions();
      toast.success(`已清空 ${result.count} 个订阅`);
      refetch();
    } catch (error: any) {
      toast.error(`清空失败: ${error.message}`);
    } finally {
      setIsClearing(false);
    }
  };

  const handleEditSubscription = (sub: Subscription) => {
    setEditingSubscription(sub);
    setEditNotifyAvailable(sub.notifyAvailable);
    setEditNotifyUnavailable(sub.notifyUnavailable);
    setEditAutoOrder(sub.autoOrder);
    setEditQuantity(sub.quantity || 1);
  };

  const handleUpdateSubscription = async () => {
    if (!editingSubscription) return;
    setIsUpdating(true);
    try {
      await api.updateSubscription(editingSubscription.planCode, {
        notifyAvailable: editNotifyAvailable,
        notifyUnavailable: editNotifyUnavailable,
        autoOrder: editAutoOrder,
        quantity: editQuantity,
      });
      toast.success("订阅配置已更新");
      setEditingSubscription(null);
      refetch();
    } catch (error: any) {
      toast.error(`更新失败: ${error.message}`);
    } finally {
      setIsUpdating(false);
    }
  };

  const handleTestNotification = async () => {
    try {
      await api.testNotification();
      toast.success("测试通知已发送");
    } catch (error: any) {
      toast.error(`发送失败: ${error.message}`);
    }
  };

  const handleViewHistory = async (sub: Subscription) => {
    setSelectedSubscription(sub);
    setIsLoadingHistory(true);
    try {
      const result = await api.getSubscriptionHistory(sub.planCode);
      setSubscriptionHistory(result.history || []);
    } catch (error: any) {
      console.error('Failed to load history:', error);
      setSubscriptionHistory(sub.history || []);
    } finally {
      setIsLoadingHistory(false);
    }
  };

  const handleSaveInterval = async () => {
    if (newInterval < 1 || newInterval > 300) {
      toast.error("间隔必须在 1-300 秒之间");
      return;
    }
    setIsSavingInterval(true);
    try {
      const result = await api.updateMonitorInterval(newInterval);
      if (result.status !== "success") {
        toast.info(result.message || "检查间隔已固定为 5 秒");
        setNewInterval(monitorStatus?.checkInterval || 5);
        setIsSettingsOpen(false);
        return;
      }
      toast.success(`检查间隔已更新为 ${newInterval} 秒`);
      setIsSettingsOpen(false);
      refetchStatus();
    } catch (error: any) {
      toast.error(`更新失败: ${error.message}`);
    } finally {
      setIsSavingInterval(false);
    }
  };

  const subscriptionList = subscriptions || [];
  const checkInterval = monitorStatus?.checkInterval || 5;

  const datacenterOptions = ["gra", "rbx", "sbg", "bhs", "waw", "lon"];

  return (
    <>
      <Helmet>
        <title>独服监控 | OVH Sniper</title>
        <meta name="description" content="监控OVH独立服务器库存变化" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                独服监控
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                监控 {subscriptionList.length} 款服务器，检查间隔 {checkInterval} 秒
              </p>
            </div>
            
            <div className="flex gap-2 flex-wrap">
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
                      配置独服监控参数
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>检查间隔 (秒)</Label>
                      <div className="flex items-center gap-2">
                        <input 
                          type="number" 
                          min="1" 
                          max="300"
                          value={newInterval}
                          onChange={(e) => setNewInterval(parseInt(e.target.value) || 5)}
                          className="w-full px-3 py-2 bg-background border border-border rounded-sm text-sm"
                        />
                        <span className="text-sm text-muted-foreground">秒</span>
                      </div>
                      <p className="text-xs text-muted-foreground">
                        建议值: 3-10秒，过低可能触发频率限制
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
                variant="outline" 
                size="sm"
                onClick={handleTestNotification}
              >
                <Bell className="h-4 w-4 mr-2" />
                测试通知
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
                    <DialogTitle className="text-primary">添加监控订阅</DialogTitle>
                    <DialogDescription>
                      配置需要监控的服务器和机房
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>服务器型号</Label>
                      <Select value={newPlanCode} onValueChange={setNewPlanCode}>
                        <SelectTrigger>
                          <SelectValue placeholder="选择型号" />
                        </SelectTrigger>
                        <SelectContent>
                          {servers?.map(server => (
                            <SelectItem key={server.planCode} value={server.planCode}>
                              {server.name} ({server.planCode})
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>监控机房 (留空监控全部)</Label>
                      <div className="grid grid-cols-2 gap-2">
                        {datacenterOptions.map(dc => (
                          <div key={dc} className="flex items-center space-x-2">
                            <Checkbox 
                              id={dc}
                              checked={selectedDatacenters.includes(dc)}
                              onCheckedChange={(checked) => {
                                if (checked) {
                                  setSelectedDatacenters([...selectedDatacenters, dc]);
                                } else {
                                  setSelectedDatacenters(selectedDatacenters.filter(d => d !== dc));
                                }
                              }}
                            />
                            <label htmlFor={dc} className="text-sm uppercase">{dc}</label>
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="space-y-3">
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
                      {autoOrder && (
                        <div className="space-y-2 pt-2 border-t border-border">
                          <Label>自动下单数量</Label>
                          <input 
                            type="number" 
                            min="1" 
                            max="10"
                            value={quantity}
                            onChange={(e) => setQuantity(parseInt(e.target.value) || 1)}
                            className="w-full px-3 py-2 bg-background border border-border rounded-sm text-sm"
                          />
                          <p className="text-xs text-muted-foreground">
                            💡 智能触发: 仅在"无货→有货"状态变化时自动下单
                          </p>
                        </div>
                      )}
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

          {/* Monitor Status */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div className="terminal-card p-4 border-primary/30">
                <div className="flex items-center gap-2 mb-1">
                  <Activity className="h-4 w-4 text-primary animate-pulse" />
                  <span className="text-xs uppercase text-muted-foreground">监控状态</span>
                </div>
                <p className="text-lg font-bold text-primary">运行中</p>
              </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 mb-1 text-muted-foreground">
                <Eye className="h-4 w-4" />
                <span className="text-xs uppercase">订阅数量</span>
              </div>
              <p className="text-lg font-bold">{subscriptionList.length}</p>
            </div>
            <div className="terminal-card p-4 border-accent/30">
              <div className="flex items-center gap-2 mb-1 text-accent">
                <Bell className="h-4 w-4" />
                <span className="text-xs uppercase">有货服务器</span>
              </div>
              <p className="text-lg font-bold text-accent">
                {subscriptionList.filter((s: Subscription) => 
                  Object.values(s.lastStatus || {}).some(v => v === "available")
                ).length}
              </p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 mb-1 text-muted-foreground">
                <ShoppingCart className="h-4 w-4" />
                <span className="text-xs uppercase">自动下单</span>
              </div>
              <p className="text-lg font-bold">
                {subscriptionList.filter((s: Subscription) => s.autoOrder).length}
              </p>
            </div>
          </div>

          {/* Batch Actions */}
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={handleBatchAddAll} disabled={isBatchAdding}>
              {isBatchAdding ? (
                <Loader2 className="h-4 w-4 mr-2 animate-spin" />
              ) : (
                <Plus className="h-4 w-4 mr-2" />
              )}
              批量添加全部服务器
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

          {/* Subscriptions List */}
          <TerminalCard
            title="订阅列表"
            icon={<Activity className="h-4 w-4" />}
          >
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : (
              <div className="space-y-4">
                {subscriptionList.map((sub: Subscription, index: number) => {
                  const hasAvailable = Object.values(sub.lastStatus || {}).some(v => v === "available");
                  
                  return (
                    <div 
                      key={sub.planCode}
                      className={cn(
                        "p-4 rounded-sm border transition-all",
                        hasAvailable 
                          ? "border-primary/30 bg-primary/5" 
                          : "border-border"
                      )}
                    >
                      <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4">
                        {/* Subscription Info */}
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-3">
                            <span className="font-bold text-lg text-foreground">{sub.serverName || sub.planCode}</span>
                            <span className="text-xs text-muted-foreground font-mono">({sub.planCode})</span>
                            {sub.autoOrder && (
                              <span className="text-xs px-2 py-0.5 bg-accent/20 text-accent rounded-sm flex items-center gap-1">
                                <ShoppingCart className="h-3 w-3" />
                                自动下单 x{sub.quantity}
                              </span>
                            )}
                          </div>
                          
                          {/* Datacenter Status */}
                          <div className="flex flex-wrap gap-2 mb-3">
                            {(sub.datacenters || []).map(dc => {
                              const status = (sub.lastStatus || {})[dc] || "unknown";
                              return (
                                <div 
                                  key={dc}
                                  className={cn(
                                    "flex items-center gap-2 px-3 py-1.5 rounded-sm border",
                                    status === "available" 
                                      ? "border-primary/30 bg-primary/10 text-primary" 
                                      : "border-border bg-muted/30 text-muted-foreground"
                                  )}
                                >
                                  <span className={cn(
                                    "h-2 w-2 rounded-full",
                                    status === "available" ? "bg-primary animate-pulse" : "bg-muted-foreground"
                                  )} />
                                  <span className="uppercase font-mono text-xs">{dc}</span>
                                  <span className="text-xs">
                                    {status === "available" ? "有货" : "无货"}
                                  </span>
                                </div>
                              );
                            })}
                          </div>

                          {/* Notification Settings */}
                          <div className="flex items-center gap-4 text-xs text-muted-foreground">
                            <span className="flex items-center gap-1">
                              {sub.notifyAvailable ? (
                                <Bell className="h-3 w-3 text-primary" />
                              ) : (
                                <BellOff className="h-3 w-3" />
                              )}
                              有货通知: {sub.notifyAvailable ? "开" : "关"}
                            </span>
                            <span className="flex items-center gap-1">
                              {sub.notifyUnavailable ? (
                                <Bell className="h-3 w-3 text-warning" />
                              ) : (
                                <BellOff className="h-3 w-3" />
                              )}
                              无货通知: {sub.notifyUnavailable ? "开" : "关"}
                            </span>
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex gap-2">
                          <Button 
                            variant="ghost" 
                            size="sm"
                            onClick={() => handleEditSubscription(sub)}
                            title="编辑设置"
                          >
                            <Edit className="h-4 w-4" />
                          </Button>
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
                            onClick={() => handleRemoveSubscription(sub.planCode)}
                            disabled={isDeleting === sub.planCode}
                          >
                            {isDeleting === sub.planCode ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : (
                              <Trash2 className="h-4 w-4" />
                            )}
                          </Button>
                        </div>
                      </div>

                      {/* Recent Changes */}
                      {(sub.history || []).length > 0 && (
                        <div className="mt-3 pt-3 border-t border-border/50">
                          <p className="text-xs text-muted-foreground mb-2">最近变更:</p>
                          <div className="flex flex-wrap gap-2">
                            {sub.history.slice(0, 3).map((h, i) => (
                              <span 
                                key={i}
                                className={cn(
                                  "text-xs px-2 py-1 rounded-sm",
                                  h.status === "available" 
                                    ? "bg-primary/10 text-primary" 
                                    : "bg-destructive/10 text-destructive"
                                )}
                              >
                                {h.datacenter.toUpperCase()} {h.changeType} @ {new Date(h.timestamp).toLocaleTimeString("zh-CN")}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}

            {!isLoading && subscriptionList.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <Activity className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无监控订阅</p>
              </div>
            )}
          </TerminalCard>

          {/* History Dialog */}
          <Dialog open={!!selectedSubscription} onOpenChange={() => { setSelectedSubscription(null); setSubscriptionHistory([]); }}>
            <DialogContent className="terminal-card border-primary/30">
              <DialogHeader>
                <DialogTitle className="text-primary">
                  {selectedSubscription?.serverName || selectedSubscription?.planCode} 变更历史
                </DialogTitle>
              </DialogHeader>
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {isLoadingHistory ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                  </div>
                ) : subscriptionHistory.length === 0 ? (
                  <p className="text-muted-foreground text-center py-8">暂无变更记录</p>
                ) : (
                  subscriptionHistory.map((h, i) => (
                    <div 
                      key={i}
                      className={cn(
                        "p-3 rounded-sm border",
                        h.status === "available" 
                          ? "border-primary/30 bg-primary/5" 
                          : "border-destructive/30 bg-destructive/5"
                      )}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-mono uppercase">{h.datacenter}</span>
                        <StatusBadge 
                          status={h.status === "available" ? "available" : "unavailable"} 
                          size="sm" 
                        />
                      </div>
                      <p className="text-xs text-muted-foreground mt-1">
                        {new Date(h.timestamp).toLocaleString("zh-CN")}
                      </p>
                    </div>
                  ))
                )}
              </div>
            </DialogContent>
          </Dialog>

          {/* Edit Subscription Dialog */}
          <Dialog open={!!editingSubscription} onOpenChange={(open) => !open && setEditingSubscription(null)}>
            <DialogContent className="terminal-card border-primary/30">
              <DialogHeader>
                <DialogTitle className="text-primary flex items-center gap-2">
                  <Edit className="h-5 w-5" />
                  编辑订阅 - {editingSubscription?.serverName || editingSubscription?.planCode}
                </DialogTitle>
                <DialogDescription>
                  修改通知和自动下单设置
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4 py-4">
                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-2">
                    <Bell className="h-4 w-4 text-primary" />
                    有货通知
                  </Label>
                  <Switch checked={editNotifyAvailable} onCheckedChange={setEditNotifyAvailable} />
                </div>
                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-2">
                    <Bell className="h-4 w-4 text-warning" />
                    无货通知
                  </Label>
                  <Switch checked={editNotifyUnavailable} onCheckedChange={setEditNotifyUnavailable} />
                </div>
                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-2">
                    <ShoppingCart className="h-4 w-4 text-accent" />
                    自动下单
                  </Label>
                  <Switch checked={editAutoOrder} onCheckedChange={setEditAutoOrder} />
                </div>
                {editAutoOrder && (
                  <div className="space-y-2 pt-2 border-t border-border">
                    <Label>自动下单数量</Label>
                    <input 
                      type="number" 
                      min="1" 
                      max="10"
                      value={editQuantity}
                      onChange={(e) => setEditQuantity(parseInt(e.target.value) || 1)}
                      className="w-full px-3 py-2 bg-background border border-border rounded-sm text-sm"
                    />
                  </div>
                )}
              </div>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">取消</Button>
                </DialogClose>
                <Button onClick={handleUpdateSubscription} disabled={isUpdating}>
                  {isUpdating && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  保存设置
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </AppLayout>
    </>
  );
};

export default MonitorPage;
