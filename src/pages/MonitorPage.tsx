import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";
import { 
  Activity, 
  Plus, 
  Trash2,
  Play,
  Square,
  Bell,
  BellOff,
  ShoppingCart,
  History as HistoryIcon,
  Settings2,
  Eye
} from "lucide-react";
import { useState } from "react";
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

interface Subscription {
  planCode: string;
  serverName: string;
  datacenters: string[];
  notifyAvailable: boolean;
  notifyUnavailable: boolean;
  autoOrder: boolean;
  quantity: number;
  lastStatus: Record<string, string>;
  createdAt: string;
  history: { timestamp: string; datacenter: string; status: string; changeType: string }[];
}

const mockSubscriptions: Subscription[] = [
  {
    planCode: "24ska01",
    serverName: "KS-A",
    datacenters: ["gra", "rbx", "sbg"],
    notifyAvailable: true,
    notifyUnavailable: false,
    autoOrder: true,
    quantity: 1,
    lastStatus: { gra: "unavailable", rbx: "unavailable", sbg: "available" },
    createdAt: "2024-12-20T10:00:00",
    history: [
      { timestamp: "2024-12-29T10:30:00", datacenter: "sbg", status: "available", changeType: "有货" },
      { timestamp: "2024-12-29T08:15:00", datacenter: "gra", status: "unavailable", changeType: "无货" },
    ]
  },
  {
    planCode: "24sk30",
    serverName: "KS-30",
    datacenters: ["gra", "rbx"],
    notifyAvailable: true,
    notifyUnavailable: true,
    autoOrder: false,
    quantity: 1,
    lastStatus: { gra: "available", rbx: "unavailable" },
    createdAt: "2024-12-22T14:30:00",
    history: [
      { timestamp: "2024-12-29T09:00:00", datacenter: "gra", status: "available", changeType: "有货" },
    ]
  },
  {
    planCode: "24rise01",
    serverName: "RISE-1",
    datacenters: ["gra", "rbx", "sbg", "bhs"],
    notifyAvailable: true,
    notifyUnavailable: false,
    autoOrder: true,
    quantity: 2,
    lastStatus: { gra: "available", rbx: "available", sbg: "unavailable", bhs: "unavailable" },
    createdAt: "2024-12-25T09:00:00",
    history: []
  },
];

const MonitorPage = () => {
  const [subscriptions] = useState(mockSubscriptions);
  const [isRunning, setIsRunning] = useState(true);
  const [checkInterval] = useState(5);
  const [selectedSubscription, setSelectedSubscription] = useState<Subscription | null>(null);

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
                监控 {subscriptions.length} 款服务器，检查间隔 {checkInterval} 秒
              </p>
            </div>
            
            <div className="flex gap-2">
              <Button 
                variant={isRunning ? "destructive" : "default"}
                size="sm"
                onClick={() => setIsRunning(!isRunning)}
              >
                {isRunning ? (
                  <>
                    <Square className="h-4 w-4 mr-2" />
                    停止监控
                  </>
                ) : (
                  <>
                    <Play className="h-4 w-4 mr-2" />
                    启动监控
                  </>
                )}
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
                      <Label>监控机房</Label>
                      <div className="grid grid-cols-2 gap-2">
                        {["gra", "rbx", "sbg", "bhs", "waw", "lon"].map(dc => (
                          <div key={dc} className="flex items-center space-x-2">
                            <Checkbox id={dc} />
                            <label htmlFor={dc} className="text-sm uppercase">{dc}</label>
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="space-y-3">
                      <div className="flex items-center justify-between">
                        <Label>有货通知</Label>
                        <Switch defaultChecked />
                      </div>
                      <div className="flex items-center justify-between">
                        <Label>无货通知</Label>
                        <Switch />
                      </div>
                      <div className="flex items-center justify-between">
                        <Label>自动下单</Label>
                        <Switch />
                      </div>
                    </div>
                  </div>
                  <DialogFooter>
                    <DialogClose asChild>
                      <Button variant="outline">取消</Button>
                    </DialogClose>
                    <Button>添加订阅</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          {/* Monitor Status */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className={cn(
              "terminal-card p-4",
              isRunning ? "border-primary/30" : "border-destructive/30"
            )}>
              <div className="flex items-center gap-2 mb-1">
                <Activity className={cn(
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
                <Eye className="h-4 w-4" />
                <span className="text-xs uppercase">订阅数量</span>
              </div>
              <p className="text-lg font-bold">{subscriptions.length}</p>
            </div>
            <div className="terminal-card p-4 border-accent/30">
              <div className="flex items-center gap-2 mb-1 text-accent">
                <Bell className="h-4 w-4" />
                <span className="text-xs uppercase">有货服务器</span>
              </div>
              <p className="text-lg font-bold text-accent">
                {subscriptions.filter(s => 
                  Object.values(s.lastStatus).some(v => v === "available")
                ).length}
              </p>
            </div>
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 mb-1 text-muted-foreground">
                <ShoppingCart className="h-4 w-4" />
                <span className="text-xs uppercase">自动下单</span>
              </div>
              <p className="text-lg font-bold">
                {subscriptions.filter(s => s.autoOrder).length}
              </p>
            </div>
          </div>

          {/* Subscriptions List */}
          <TerminalCard
            title="订阅列表"
            icon={<Activity className="h-4 w-4" />}
          >
            <div className="space-y-4">
              {subscriptions.map((sub, index) => {
                const hasAvailable = Object.values(sub.lastStatus).some(v => v === "available");
                
                return (
                  <div 
                    key={sub.planCode}
                    className={cn(
                      "p-4 rounded-sm border transition-all",
                      hasAvailable 
                        ? "border-primary/30 bg-primary/5" 
                        : "border-border"
                    )}
                    style={{ animationDelay: `${index * 50}ms` }}
                  >
                    <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4">
                      {/* Subscription Info */}
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-3">
                          <span className="font-bold text-lg text-foreground">{sub.serverName}</span>
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
                          {sub.datacenters.map(dc => {
                            const status = sub.lastStatus[dc] || "unknown";
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
                          onClick={() => setSelectedSubscription(sub)}
                        >
                          <HistoryIcon className="h-4 w-4 mr-1" />
                          历史
                        </Button>
                        <Button variant="ghost" size="sm">
                          <Settings2 className="h-4 w-4 mr-1" />
                          设置
                        </Button>
                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>

                    {/* Recent Changes */}
                    {sub.history.length > 0 && (
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

            {subscriptions.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <Activity className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无监控订阅</p>
              </div>
            )}
          </TerminalCard>

          {/* History Dialog */}
          <Dialog open={!!selectedSubscription} onOpenChange={() => setSelectedSubscription(null)}>
            <DialogContent className="terminal-card border-primary/30">
              <DialogHeader>
                <DialogTitle className="text-primary">
                  {selectedSubscription?.serverName} 变更历史
                </DialogTitle>
              </DialogHeader>
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {selectedSubscription?.history.length === 0 ? (
                  <p className="text-muted-foreground text-center py-8">暂无变更记录</p>
                ) : (
                  selectedSubscription?.history.map((h, i) => (
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
        </div>
      </AppLayout>
    </>
  );
};

export default MonitorPage;
