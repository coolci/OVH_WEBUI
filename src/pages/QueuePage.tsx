import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { 
  ListOrdered, 
  Plus, 
  Trash2, 
  Play,
  Pause,
  RefreshCw,
  Clock,
  CheckCircle2,
  XCircle,
  MoreVertical,
  Loader2,
  Power,
  Square
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import api from "@/lib/api";
import { useQueue, useServers } from "@/hooks/useApi";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const QueuePage = () => {
  const { data: queue, isLoading, refetch } = useQueue();
  const { data: servers } = useServers();
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);
  const [newTask, setNewTask] = useState({ planCode: "", datacenter: "", retryInterval: 30 });
  const [isAdding, setIsAdding] = useState(false);
  const [isProcessorRunning, setIsProcessorRunning] = useState(false);
  const [isTogglingProcessor, setIsTogglingProcessor] = useState(false);

  useEffect(() => {
    const interval = setInterval(refetch, 5000);
    return () => clearInterval(interval);
  }, [refetch]);

  // 检查处理器状态
  useEffect(() => {
    const checkProcessorStatus = async () => {
      try {
        const status = await api.getQueueProcessorStatus();
        setIsProcessorRunning(status.running);
      } catch {
        // fallback to stats
        try {
          const stats = await api.getStats();
          setIsProcessorRunning(stats.queueProcessorRunning || false);
        } catch {}
      }
    };
    checkProcessorStatus();
    const interval = setInterval(checkProcessorStatus, 10000);
    return () => clearInterval(interval);
  }, []);

  const handleToggleProcessor = async () => {
    setIsTogglingProcessor(true);
    try {
      if (isProcessorRunning) {
        await api.stopQueueProcessor();
        toast.success("队列处理器已停止");
      } else {
        await api.startQueueProcessor();
        toast.success("队列处理器已启动");
      }
      setIsProcessorRunning(!isProcessorRunning);
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsTogglingProcessor(false);
    }
  };

  const queueList = queue || [];

  const activeCount = queueList.filter(q => q.status === "running" || q.status === "pending").length;
  const completedCount = queueList.filter(q => q.status === "completed").length;
  const failedCount = queueList.filter(q => q.status === "failed").length;

  const formatTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleString("zh-CN", { 
        month: "2-digit", 
        day: "2-digit",
        hour: "2-digit", 
        minute: "2-digit" 
      });
    } catch {
      return "N/A";
    }
  };

  const getLastCheckDisplay = (lastCheckTime: number) => {
    if (!lastCheckTime || lastCheckTime === 0) return "从未";
    const diff = Date.now() - lastCheckTime * 1000;
    if (diff < 60000) return `${Math.floor(diff / 1000)}秒前`;
    if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
    return `${Math.floor(diff / 86400000)}天前`;
  };

  const handleAddTask = async () => {
    if (!newTask.planCode || !newTask.datacenter) {
      toast.error("请选择服务器型号和机房");
      return;
    }
    
    setIsAdding(true);
    try {
      await api.addQueueItem({
        planCode: newTask.planCode,
        datacenter: newTask.datacenter,
        retryInterval: newTask.retryInterval,
      });
      toast.success("任务已添加");
      setIsAddDialogOpen(false);
      setNewTask({ planCode: "", datacenter: "", retryInterval: 30 });
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsAdding(false);
    }
  };

  const handleDeleteTask = async (id: string) => {
    try {
      await api.removeQueueItem(id);
      toast.success("任务已删除");
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const handleUpdateStatus = async (id: string, status: string) => {
    try {
      await api.updateQueueStatus(id, status);
      toast.success(`任务状态已更新为 ${status}`);
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const handleClearQueue = async () => {
    try {
      const result = await api.clearQueue();
      toast.success(`已清空 ${result.count} 个任务`);
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const selectedServer = servers?.find(s => s.planCode === newTask.planCode);

  return (
    <>
      <Helmet>
        <title>抢购队列 | OVH Sniper</title>
        <meta name="description" content="管理服务器抢购任务队列" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                抢购队列
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                活跃: {activeCount} | 完成: {completedCount} | 失败: {failedCount}
              </p>
            </div>
            
            <div className="flex gap-2">
              {/* 处理器控制按钮 */}
              <Button 
                variant={isProcessorRunning ? "destructive" : "default"}
                size="sm"
                onClick={handleToggleProcessor}
                disabled={isTogglingProcessor}
              >
                {isTogglingProcessor ? (
                  <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                ) : isProcessorRunning ? (
                  <Square className="h-4 w-4 mr-2" />
                ) : (
                  <Power className="h-4 w-4 mr-2" />
                )}
                {isProcessorRunning ? "停止处理器" : "启动处理器"}
              </Button>
              
              <Button variant="outline" size="sm" onClick={handleClearQueue}>
                <Trash2 className="h-4 w-4 mr-2" />
                清空队列
              </Button>
              <Dialog open={isAddDialogOpen} onOpenChange={setIsAddDialogOpen}>
                <DialogTrigger asChild>
                  <Button size="sm">
                    <Plus className="h-4 w-4 mr-2" />
                    添加任务
                  </Button>
                </DialogTrigger>
                <DialogContent className="terminal-card border-primary/30">
                  <DialogHeader>
                    <DialogTitle className="text-primary">添加抢购任务</DialogTitle>
                    <DialogDescription>
                      配置服务器抢购参数
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>服务器型号</Label>
                      <Select 
                        value={newTask.planCode} 
                        onValueChange={(v) => setNewTask({...newTask, planCode: v, datacenter: ""})}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="选择型号" />
                        </SelectTrigger>
                        <SelectContent>
                          {servers?.slice(0, 50).map(server => (
                            <SelectItem key={server.planCode} value={server.planCode}>
                              {server.name || server.planCode} ({server.planCode})
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>目标机房</Label>
                      <Select 
                        value={newTask.datacenter} 
                        onValueChange={(v) => setNewTask({...newTask, datacenter: v})}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="选择机房" />
                        </SelectTrigger>
                        <SelectContent>
                          {selectedServer?.datacenters?.map(dc => (
                            <SelectItem key={dc.datacenter} value={dc.datacenter}>
                              {dc.datacenter.toUpperCase()} - {dc.availability}
                            </SelectItem>
                          )) || (
                            <>
                              <SelectItem value="gra">GRA (Gravelines)</SelectItem>
                              <SelectItem value="rbx">RBX (Roubaix)</SelectItem>
                              <SelectItem value="sbg">SBG (Strasbourg)</SelectItem>
                              <SelectItem value="bhs">BHS (Beauharnois)</SelectItem>
                            </>
                          )}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>重试间隔 (秒)</Label>
                      <Input 
                        type="number" 
                        value={newTask.retryInterval}
                        onChange={(e) => setNewTask({...newTask, retryInterval: parseInt(e.target.value) || 30})}
                        min={5} 
                        max={300} 
                      />
                    </div>
                  </div>
                  <DialogFooter>
                    <DialogClose asChild>
                      <Button variant="outline">取消</Button>
                    </DialogClose>
                    <Button onClick={handleAddTask} disabled={isAdding}>
                      {isAdding ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                      添加任务
                    </Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {/* Stats Cards */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="terminal-card p-4 border-accent/30">
              <div className="flex items-center gap-2 text-accent mb-1">
                <Play className="h-4 w-4" />
                <span className="text-xs uppercase">运行中</span>
              </div>
              <p className="text-2xl font-bold text-accent">
                {queueList.filter(q => q.status === "running").length}
              </p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 text-muted-foreground mb-1">
                <Clock className="h-4 w-4" />
                <span className="text-xs uppercase">等待中</span>
              </div>
              <p className="text-2xl font-bold">
                {queueList.filter(q => q.status === "pending").length}
              </p>
            </div>
            <div className="terminal-card p-4 border-primary/30">
              <div className="flex items-center gap-2 text-primary mb-1">
                <CheckCircle2 className="h-4 w-4" />
                <span className="text-xs uppercase">已完成</span>
              </div>
              <p className="text-2xl font-bold text-primary">{completedCount}</p>
            </div>
            <div className="terminal-card p-4 border-destructive/30">
              <div className="flex items-center gap-2 text-destructive mb-1">
                <XCircle className="h-4 w-4" />
                <span className="text-xs uppercase">失败</span>
              </div>
              <p className="text-2xl font-bold text-destructive">{failedCount}</p>
            </div>
          </div>

          {/* Queue Table */}
          <TerminalCard
            title="任务列表"
            icon={<ListOrdered className="h-4 w-4" />}
          >
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : queueList.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <ListOrdered className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无队列任务</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                      <th className="text-left py-3 px-2">任务ID</th>
                      <th className="text-left py-3 px-2">服务器</th>
                      <th className="text-left py-3 px-2">机房</th>
                      <th className="text-left py-3 px-2 hidden md:table-cell">创建时间</th>
                      <th className="text-center py-3 px-2">重试</th>
                      <th className="text-left py-3 px-2 hidden lg:table-cell">最后检查</th>
                      <th className="text-center py-3 px-2">状态</th>
                      <th className="text-right py-3 px-2">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {queueList.map((item, index) => (
                      <tr 
                        key={item.id}
                        className={cn(
                          "border-b border-border/50 transition-colors",
                          item.status === "running" && "bg-accent/5",
                          item.status === "failed" && "bg-destructive/5"
                        )}
                      >
                        <td className="py-3 px-2">
                          <span className="font-mono text-primary text-xs">{item.id.slice(0, 8)}</span>
                        </td>
                        <td className="py-3 px-2">
                          <div>
                            <p className="font-medium">{item.planCode}</p>
                          </div>
                        </td>
                        <td className="py-3 px-2">
                          <span className="uppercase font-mono text-xs bg-muted px-2 py-1 rounded-sm">
                            {item.datacenter}
                          </span>
                        </td>
                        <td className="py-3 px-2 hidden md:table-cell text-muted-foreground">
                          {formatTime(item.createdAt)}
                        </td>
                        <td className="py-3 px-2 text-center">
                          <span className={cn(
                            "font-mono",
                            item.retryCount > 500 && "text-warning",
                            item.retryCount > 900 && "text-destructive"
                          )}>
                            {item.retryCount}
                          </span>
                        </td>
                        <td className="py-3 px-2 hidden lg:table-cell text-muted-foreground text-xs">
                          {getLastCheckDisplay(item.lastCheckTime)}
                        </td>
                        <td className="py-3 px-2 text-center">
                          <StatusBadge status={item.status as any} size="sm" />
                        </td>
                        <td className="py-3 px-2 text-right">
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon" className="h-8 w-8">
                                <MoreVertical className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              {item.status === "running" && (
                                <DropdownMenuItem onClick={() => handleUpdateStatus(item.id, "paused")}>
                                  <Pause className="h-4 w-4 mr-2" />
                                  暂停
                                </DropdownMenuItem>
                              )}
                              {(item.status === "paused" || item.status === "pending") && (
                                <DropdownMenuItem onClick={() => handleUpdateStatus(item.id, "running")}>
                                  <Play className="h-4 w-4 mr-2" />
                                  启动
                                </DropdownMenuItem>
                              )}
                              {item.status === "failed" && (
                                <DropdownMenuItem onClick={() => handleUpdateStatus(item.id, "running")}>
                                  <RefreshCw className="h-4 w-4 mr-2" />
                                  重试
                                </DropdownMenuItem>
                              )}
                              <DropdownMenuSeparator />
                              <DropdownMenuItem 
                                className="text-destructive"
                                onClick={() => handleDeleteTask(item.id)}
                              >
                                <Trash2 className="h-4 w-4 mr-2" />
                                删除
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default QueuePage;
