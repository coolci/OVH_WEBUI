import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
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
  Settings2
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

interface VpsSubscription {
  id: string;
  planCode: string;
  displayName: string;
  ovhSubsidiary: string;
  datacenters: string[];
  monitorLinux: boolean;
  monitorWindows: boolean;
  notifyAvailable: boolean;
  notifyUnavailable: boolean;
  lastStatus: Record<string, { linux: string; windows: string }>;
  createdAt: string;
}

const mockVpsSubscriptions: VpsSubscription[] = [
  {
    id: "vps-001",
    planCode: "vps-starter-1-2-20",
    displayName: "VPS Starter",
    ovhSubsidiary: "IE",
    datacenters: ["gra", "sbg", "bhs"],
    monitorLinux: true,
    monitorWindows: true,
    notifyAvailable: true,
    notifyUnavailable: false,
    lastStatus: {
      gra: { linux: "available", windows: "unavailable" },
      sbg: { linux: "available", windows: "available" },
      bhs: { linux: "unavailable", windows: "unavailable" },
    },
    createdAt: "2024-12-20T10:00:00",
  },
  {
    id: "vps-002",
    planCode: "vps-value-1-4-40",
    displayName: "VPS Value",
    ovhSubsidiary: "IE",
    datacenters: ["gra", "rbx"],
    monitorLinux: true,
    monitorWindows: false,
    notifyAvailable: true,
    notifyUnavailable: true,
    lastStatus: {
      gra: { linux: "available", windows: "unknown" },
      rbx: { linux: "unavailable", windows: "unknown" },
    },
    createdAt: "2024-12-22T14:30:00",
  },
  {
    id: "vps-003",
    planCode: "vps-essential-2-4-80",
    displayName: "VPS Essential",
    ovhSubsidiary: "IE",
    datacenters: ["gra", "sbg", "waw"],
    monitorLinux: true,
    monitorWindows: true,
    notifyAvailable: true,
    notifyUnavailable: false,
    lastStatus: {
      gra: { linux: "unavailable", windows: "unavailable" },
      sbg: { linux: "unavailable", windows: "unavailable" },
      waw: { linux: "available", windows: "unavailable" },
    },
    createdAt: "2024-12-25T09:00:00",
  },
];

const VpsMonitorPage = () => {
  const [subscriptions] = useState(mockVpsSubscriptions);
  const [isRunning, setIsRunning] = useState(true);
  const [checkInterval] = useState(60);

  const getStatusColor = (status: string) => {
    if (status === "available") return "text-primary bg-primary/10";
    if (status === "unavailable") return "text-destructive bg-destructive/10";
    return "text-muted-foreground bg-muted";
  };

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
                监控 {subscriptions.length} 款 VPS，检查间隔 {checkInterval} 秒
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
                    <DialogTitle className="text-primary">添加 VPS 监控</DialogTitle>
                    <DialogDescription>
                      配置需要监控的 VPS 型号
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <div className="space-y-2">
                      <Label>VPS 型号</Label>
                      <Select>
                        <SelectTrigger>
                          <SelectValue placeholder="选择型号" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="vps-starter">VPS Starter</SelectItem>
                          <SelectItem value="vps-value">VPS Value</SelectItem>
                          <SelectItem value="vps-essential">VPS Essential</SelectItem>
                          <SelectItem value="vps-comfort">VPS Comfort</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>区域</Label>
                      <Select>
                        <SelectTrigger>
                          <SelectValue placeholder="选择区域" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="IE">爱尔兰 (IE)</SelectItem>
                          <SelectItem value="DE">德国 (DE)</SelectItem>
                          <SelectItem value="FR">法国 (FR)</SelectItem>
                          <SelectItem value="US">美国 (US)</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-2">
                      <Label>监控机房</Label>
                      <div className="grid grid-cols-3 gap-2">
                        {["gra", "rbx", "sbg", "bhs", "waw", "lon"].map(dc => (
                          <div key={dc} className="flex items-center space-x-2">
                            <Checkbox id={`vps-${dc}`} />
                            <label htmlFor={`vps-${dc}`} className="text-sm uppercase">{dc}</label>
                          </div>
                        ))}
                      </div>
                    </div>
                    <div className="space-y-3 pt-2">
                      <div className="flex items-center justify-between">
                        <Label>监控 Linux</Label>
                        <Switch defaultChecked />
                      </div>
                      <div className="flex items-center justify-between">
                        <Label>监控 Windows</Label>
                        <Switch defaultChecked />
                      </div>
                      <div className="flex items-center justify-between">
                        <Label>有货通知</Label>
                        <Switch defaultChecked />
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
              <p className="text-lg font-bold">{subscriptions.length}</p>
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
                {subscriptions.filter(s => 
                  Object.values(s.lastStatus).some(v => v.linux === "available" || v.windows === "available")
                ).length}
              </p>
            </div>
          </div>

          {/* Subscriptions List */}
          <TerminalCard
            title="VPS 订阅列表"
            icon={<MonitorDot className="h-4 w-4" />}
          >
            <div className="space-y-4">
              {subscriptions.map((sub, index) => (
                <div 
                  key={sub.id}
                  className="p-4 rounded-sm border border-border hover:border-primary/30 transition-all"
                  style={{ animationDelay: `${index * 50}ms` }}
                >
                  <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4">
                    {/* Subscription Info */}
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-3">
                        <span className="font-bold text-lg text-foreground">{sub.displayName}</span>
                        <span className="text-xs text-muted-foreground font-mono">({sub.planCode})</span>
                        <span className="text-xs px-2 py-0.5 bg-muted rounded-sm">{sub.ovhSubsidiary}</span>
                      </div>
                      
                      {/* OS Monitoring */}
                      <div className="flex gap-3 mb-3 text-xs">
                        <span className={cn(
                          "px-2 py-1 rounded-sm",
                          sub.monitorLinux ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"
                        )}>
                          Linux: {sub.monitorLinux ? "监控中" : "未监控"}
                        </span>
                        <span className={cn(
                          "px-2 py-1 rounded-sm",
                          sub.monitorWindows ? "bg-accent/10 text-accent" : "bg-muted text-muted-foreground"
                        )}>
                          Windows: {sub.monitorWindows ? "监控中" : "未监控"}
                        </span>
                      </div>

                      {/* Datacenter Status Grid */}
                      <div className="overflow-x-auto">
                        <table className="w-full text-xs">
                          <thead>
                            <tr className="text-muted-foreground uppercase">
                              <th className="text-left py-1 pr-4">机房</th>
                              {sub.monitorLinux && <th className="text-center py-1 px-2">Linux</th>}
                              {sub.monitorWindows && <th className="text-center py-1 px-2">Windows</th>}
                            </tr>
                          </thead>
                          <tbody>
                            {sub.datacenters.map(dc => {
                              const status = sub.lastStatus[dc];
                              return (
                                <tr key={dc} className="border-t border-border/50">
                                  <td className="py-2 pr-4 uppercase font-mono">{dc}</td>
                                  {sub.monitorLinux && (
                                    <td className="py-2 px-2 text-center">
                                      <span className={cn(
                                        "px-2 py-0.5 rounded-sm",
                                        getStatusColor(status?.linux || "unknown")
                                      )}>
                                        {status?.linux === "available" ? "有货" : "无货"}
                                      </span>
                                    </td>
                                  )}
                                  {sub.monitorWindows && (
                                    <td className="py-2 px-2 text-center">
                                      <span className={cn(
                                        "px-2 py-0.5 rounded-sm",
                                        getStatusColor(status?.windows || "unknown")
                                      )}>
                                        {status?.windows === "available" ? "有货" : status?.windows === "unavailable" ? "无货" : "-"}
                                      </span>
                                    </td>
                                  )}
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex gap-2">
                      <Button variant="ghost" size="sm">
                        <Settings2 className="h-4 w-4 mr-1" />
                        设置
                      </Button>
                      <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive">
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  </div>
                </div>
              ))}
            </div>

            {subscriptions.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <MonitorDot className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无 VPS 监控订阅</p>
              </div>
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default VpsMonitorPage;
