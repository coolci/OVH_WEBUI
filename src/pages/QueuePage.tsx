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
  AlertCircle,
  CheckCircle2,
  XCircle,
  MoreVertical
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
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

interface QueueItem {
  id: string;
  planCode: string;
  serverName: string;
  datacenter: string;
  options: string[];
  status: "pending" | "running" | "paused" | "completed" | "failed";
  createdAt: string;
  retryInterval: number;
  retryCount: number;
  lastCheckTime: number;
  errorMessage?: string;
}

const mockQueue: QueueItem[] = [
  {
    id: "q-001",
    planCode: "24ska01",
    serverName: "KS-A",
    datacenter: "gra",
    options: ["ram-32g-ecc-2400", "softraid-2"],
    status: "running",
    createdAt: "2024-12-29T08:00:00",
    retryInterval: 30,
    retryCount: 156,
    lastCheckTime: Date.now() - 15000,
  },
  {
    id: "q-002",
    planCode: "24sk30",
    serverName: "KS-30",
    datacenter: "rbx",
    options: ["ram-64g-ecc-2400"],
    status: "running",
    createdAt: "2024-12-29T09:30:00",
    retryInterval: 30,
    retryCount: 89,
    lastCheckTime: Date.now() - 5000,
  },
  {
    id: "q-003",
    planCode: "24rise01",
    serverName: "RISE-1",
    datacenter: "sbg",
    options: [],
    status: "pending",
    createdAt: "2024-12-29T10:00:00",
    retryInterval: 60,
    retryCount: 0,
    lastCheckTime: 0,
  },
  {
    id: "q-004",
    planCode: "24adv01",
    serverName: "ADV-1",
    datacenter: "gra",
    options: ["ram-128g-ecc-2933"],
    status: "paused",
    createdAt: "2024-12-28T15:00:00",
    retryInterval: 30,
    retryCount: 450,
    lastCheckTime: Date.now() - 3600000,
  },
  {
    id: "q-005",
    planCode: "24ska01",
    serverName: "KS-A",
    datacenter: "rbx",
    options: [],
    status: "completed",
    createdAt: "2024-12-27T12:00:00",
    retryInterval: 30,
    retryCount: 234,
    lastCheckTime: Date.now() - 86400000,
  },
  {
    id: "q-006",
    planCode: "24rise02",
    serverName: "RISE-2",
    datacenter: "gra",
    options: [],
    status: "failed",
    createdAt: "2024-12-26T18:00:00",
    retryInterval: 30,
    retryCount: 999,
    lastCheckTime: Date.now() - 172800000,
    errorMessage: "达到最大重试次数",
  },
];

const QueuePage = () => {
  const [queue, setQueue] = useState(mockQueue);
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false);

  const activeCount = queue.filter(q => q.status === "running" || q.status === "pending").length;
  const completedCount = queue.filter(q => q.status === "completed").length;
  const failedCount = queue.filter(q => q.status === "failed").length;

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleString("zh-CN", { 
      month: "2-digit", 
      day: "2-digit",
      hour: "2-digit", 
      minute: "2-digit" 
    });
  };

  const getLastCheckDisplay = (lastCheckTime: number) => {
    if (lastCheckTime === 0) return "从未";
    const diff = Date.now() - lastCheckTime;
    if (diff < 60000) return `${Math.floor(diff / 1000)}秒前`;
    if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
    return `${Math.floor(diff / 86400000)}天前`;
  };

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
              <Button variant="outline" size="sm">
                <Trash2 className="h-4 w-4 mr-2" />
                清空已完成
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
                      <Select>
                        <SelectTrigger>
                          <SelectValue placeholder="选择型号" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="24ska01">KS-A (24ska01)</SelectItem>
                          <SelectItem value="24sk30">KS-30 (24sk30)</SelectItem>
                          <SelectItem value="24rise01">RISE-1 (24rise01)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>目标机房</Label>
                      <Select>
                        <SelectTrigger>
                          <SelectValue placeholder="选择机房" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="gra">GRA (Gravelines)</SelectItem>
                          <SelectItem value="rbx">RBX (Roubaix)</SelectItem>
                          <SelectItem value="sbg">SBG (Strasbourg)</SelectItem>
                          <SelectItem value="bhs">BHS (Beauharnois)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>重试间隔 (秒)</Label>
                      <Input type="number" defaultValue={30} min={5} max={300} />
                    </div>
                  </div>
                  <DialogFooter>
                    <DialogClose asChild>
                      <Button variant="outline">取消</Button>
                    </DialogClose>
                    <Button>添加任务</Button>
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
                {queue.filter(q => q.status === "running").length}
              </p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 text-muted-foreground mb-1">
                <Clock className="h-4 w-4" />
                <span className="text-xs uppercase">等待中</span>
              </div>
              <p className="text-2xl font-bold">
                {queue.filter(q => q.status === "pending").length}
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
                  {queue.map((item, index) => (
                    <tr 
                      key={item.id}
                      className={cn(
                        "border-b border-border/50 transition-colors",
                        item.status === "running" && "bg-accent/5",
                        item.status === "failed" && "bg-destructive/5"
                      )}
                      style={{ animationDelay: `${index * 50}ms` }}
                    >
                      <td className="py-3 px-2">
                        <span className="font-mono text-primary">{item.id}</span>
                      </td>
                      <td className="py-3 px-2">
                        <div>
                          <p className="font-medium">{item.serverName}</p>
                          <p className="text-xs text-muted-foreground">{item.planCode}</p>
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
                        <StatusBadge status={item.status} size="sm" />
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
                              <DropdownMenuItem>
                                <Pause className="h-4 w-4 mr-2" />
                                暂停
                              </DropdownMenuItem>
                            )}
                            {(item.status === "paused" || item.status === "pending") && (
                              <DropdownMenuItem>
                                <Play className="h-4 w-4 mr-2" />
                                启动
                              </DropdownMenuItem>
                            )}
                            {item.status === "failed" && (
                              <DropdownMenuItem>
                                <RefreshCw className="h-4 w-4 mr-2" />
                                重试
                              </DropdownMenuItem>
                            )}
                            <DropdownMenuSeparator />
                            <DropdownMenuItem className="text-destructive">
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

            {queue.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <ListOrdered className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无队列任务</p>
              </div>
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default QueuePage;
