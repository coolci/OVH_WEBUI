import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { 
  Cpu, 
  Search,
  RefreshCw,
  Power,
  HardDrive,
  Network,
  Terminal,
  Activity,
  Shield,
  Server,
  MoreVertical,
  Monitor,
  Loader2,
  Calendar,
  Clock
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useMyServers } from "@/hooks/useApi";
import { api } from "@/lib/api";
import { toast } from "sonner";

interface ManagedServer {
  serviceName: string;
  name: string;
  commercialRange?: string;
  datacenter: string;
  state: string;
  monitoring: boolean;
  reverse?: string;
  ip: string;
  os: string;
  status: string;
}

const ServerControlPage = () => {
  const { data: serversData, isLoading, refetch } = useMyServers();
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedServer, setSelectedServer] = useState<ManagedServer | null>(null);
  const [serverDetails, setServerDetails] = useState<any>(null);
  const [serverHardware, setServerHardware] = useState<any>(null);
  const [serverIps, setServerIps] = useState<any[]>([]);
  const [serverTasks, setServerTasks] = useState<any[]>([]);
  const [serviceInfo, setServiceInfo] = useState<any>(null);
  const [isLoadingDetails, setIsLoadingDetails] = useState(false);
  const [isRebooting, setIsRebooting] = useState(false);

  const servers = serversData?.servers || [];

  useEffect(() => {
    if (servers.length > 0 && !selectedServer) {
      setSelectedServer(servers[0]);
    }
  }, [servers, selectedServer]);

  useEffect(() => {
    if (selectedServer) {
      loadServerDetails(selectedServer.serviceName);
    }
  }, [selectedServer]);

  const loadServerDetails = async (serviceName: string) => {
    setIsLoadingDetails(true);
    try {
      const [details, hardware, ips, tasks, info] = await Promise.all([
        api.getServerDetails(serviceName).catch(() => null),
        api.getServerHardware(serviceName).catch(() => null),
        api.getServerIps(serviceName).catch(() => null),
        api.getServerTasks(serviceName).catch(() => null),
        api.getServiceInfo(serviceName).catch(() => null),
      ]);
      
      setServerDetails(details?.server);
      setServerHardware(hardware?.hardware);
      setServerIps(ips?.ips || []);
      setServerTasks(tasks?.tasks || []);
      setServiceInfo(info?.serviceInfo);
    } catch (error) {
      console.error('加载服务器详情失败:', error);
    } finally {
      setIsLoadingDetails(false);
    }
  };

  const handleReboot = async (type: 'soft' | 'hardware' = 'soft') => {
    if (!selectedServer) return;
    setIsRebooting(true);
    try {
      await api.rebootServer(selectedServer.serviceName, type);
      toast.success(`服务器${type === 'soft' ? '软' : '硬'}重启已发起`);
      // 刷新任务列表
      const tasks = await api.getServerTasks(selectedServer.serviceName);
      setServerTasks(tasks?.tasks || []);
    } catch (error: any) {
      toast.error(`重启失败: ${error.message}`);
    } finally {
      setIsRebooting(false);
    }
  };

  const filteredServers = servers.filter((server: ManagedServer) => 
    server.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    server.serviceName.toLowerCase().includes(searchTerm.toLowerCase()) ||
    server.ip?.includes(searchTerm)
  );

  const getStateColor = (state: string) => {
    switch (state) {
      case "ok": return "text-primary bg-primary/10";
      case "error": return "text-destructive bg-destructive/10";
      case "hacked":
      case "hackedBlocked": return "text-warning bg-warning/10";
      default: return "text-muted-foreground bg-muted";
    }
  };

  return (
    <>
      <Helmet>
        <title>服务器控制 | OVH Sniper</title>
        <meta name="description" content="管理和控制已购服务器" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                服务器控制
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                管理 {servers.length} 台已购服务器
              </p>
            </div>
            
            <Button variant="terminal" onClick={() => refetch()} disabled={isLoading}>
              <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
              刷新列表
            </Button>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Server List */}
            <div className="lg:col-span-1 space-y-4">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索服务器..." 
                  className="pl-9"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                />
              </div>

              <TerminalCard title="服务器列表" icon={<Server className="h-4 w-4" />}>
                {isLoading ? (
                  <div className="flex items-center justify-center py-8">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                  </div>
                ) : (
                  <div className="space-y-2">
                    {filteredServers.map((server: ManagedServer) => (
                      <div
                        key={server.serviceName}
                        className={cn(
                          "p-3 rounded-sm border cursor-pointer transition-all",
                          selectedServer?.serviceName === server.serviceName
                            ? "border-primary bg-primary/5"
                            : "border-border hover:border-primary/30"
                        )}
                        onClick={() => setSelectedServer(server)}
                      >
                        <div className="flex items-center justify-between mb-2">
                          <span className="font-medium">{server.name || server.serviceName.split('.')[0]}</span>
                          <span className={cn(
                            "px-2 py-0.5 rounded-sm text-xs uppercase",
                            getStateColor(server.state)
                          )}>
                            {server.state}
                          </span>
                        </div>
                        <div className="text-xs text-muted-foreground space-y-1">
                          <p className="font-mono">{server.ip}</p>
                          <p>{server.datacenter?.toUpperCase()} | {server.os}</p>
                        </div>
                      </div>
                    ))}

                    {filteredServers.length === 0 && !isLoading && (
                      <p className="text-center py-8 text-muted-foreground">
                        {servers.length === 0 ? "暂无已购服务器" : "未找到服务器"}
                      </p>
                    )}
                  </div>
                )}
              </TerminalCard>
            </div>

            {/* Server Details */}
            <div className="lg:col-span-2">
              {selectedServer ? (
                <TerminalCard
                  title={selectedServer.name || selectedServer.serviceName}
                  icon={<Cpu className="h-4 w-4" />}
                  headerAction={
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" disabled={isRebooting}>
                          {isRebooting ? (
                            <Loader2 className="h-4 w-4 animate-spin" />
                          ) : (
                            <MoreVertical className="h-4 w-4" />
                          )}
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={() => handleReboot('soft')}>
                          <Power className="h-4 w-4 mr-2" />
                          软重启
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => handleReboot('hardware')}>
                          <Power className="h-4 w-4 mr-2 text-warning" />
                          硬重启
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem>
                          <Terminal className="h-4 w-4 mr-2" />
                          IPMI 控制台
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  }
                >
                  {isLoadingDetails ? (
                    <div className="flex items-center justify-center py-12">
                      <Loader2 className="h-8 w-8 animate-spin text-primary" />
                    </div>
                  ) : (
                    <Tabs defaultValue="overview" className="space-y-4">
                      <TabsList className="bg-muted/50 border border-border">
                        <TabsTrigger value="overview">概览</TabsTrigger>
                        <TabsTrigger value="hardware">硬件</TabsTrigger>
                        <TabsTrigger value="network">网络</TabsTrigger>
                        <TabsTrigger value="tasks">任务</TabsTrigger>
                      </TabsList>

                      <TabsContent value="overview" className="space-y-4">
                        {/* Server Info Grid */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                          <div className="p-3 bg-muted/30 rounded-sm">
                            <p className="text-xs text-muted-foreground mb-1">状态</p>
                            <StatusBadge status={selectedServer.state === "ok" ? "online" : "offline"} />
                          </div>
                          <div className="p-3 bg-muted/30 rounded-sm">
                            <p className="text-xs text-muted-foreground mb-1">机房</p>
                            <p className="font-mono uppercase">{selectedServer.datacenter}</p>
                          </div>
                          <div className="p-3 bg-muted/30 rounded-sm">
                            <p className="text-xs text-muted-foreground mb-1">IP 地址</p>
                            <p className="font-mono">{selectedServer.ip}</p>
                          </div>
                          <div className="p-3 bg-muted/30 rounded-sm">
                            <p className="text-xs text-muted-foreground mb-1">操作系统</p>
                            <p className="truncate">{selectedServer.os}</p>
                          </div>
                        </div>

                        {/* Service Info */}
                        <div className="p-4 border border-border rounded-sm">
                          <h3 className="text-sm font-medium mb-3">服务信息</h3>
                          <div className="grid grid-cols-2 gap-4 text-sm">
                            <div>
                              <p className="text-muted-foreground">服务名称</p>
                              <p className="font-mono text-xs break-all">{selectedServer.serviceName}</p>
                            </div>
                            <div>
                              <p className="text-muted-foreground">自动续费</p>
                              <p className={serviceInfo?.renewalType ? "text-primary" : "text-destructive"}>
                                {serviceInfo?.renewalType ? "已开启" : "已关闭"}
                              </p>
                            </div>
                            <div>
                              <p className="text-muted-foreground">监控状态</p>
                              <p className={selectedServer.monitoring ? "text-primary" : "text-muted-foreground"}>
                                {selectedServer.monitoring ? "启用" : "禁用"}
                              </p>
                            </div>
                            <div>
                              <p className="text-muted-foreground">服务状态</p>
                              <p>{serviceInfo?.status || selectedServer.status}</p>
                            </div>
                            {serviceInfo?.expiration && (
                              <div>
                                <p className="text-muted-foreground flex items-center gap-1">
                                  <Calendar className="h-3 w-3" /> 到期时间
                                </p>
                                <p>{new Date(serviceInfo.expiration).toLocaleDateString("zh-CN")}</p>
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Quick Actions */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                          <Button 
                            variant="outline" 
                            className="h-auto py-4 flex-col gap-2"
                            onClick={() => handleReboot('soft')}
                            disabled={isRebooting}
                          >
                            <Power className="h-5 w-5 text-warning" />
                            <span className="text-xs">重启</span>
                          </Button>
                          <Button variant="outline" className="h-auto py-4 flex-col gap-2">
                            <HardDrive className="h-5 w-5 text-accent" />
                            <span className="text-xs">重装</span>
                          </Button>
                          <Button variant="outline" className="h-auto py-4 flex-col gap-2">
                            <Terminal className="h-5 w-5 text-primary" />
                            <span className="text-xs">控制台</span>
                          </Button>
                          <Button variant="outline" className="h-auto py-4 flex-col gap-2">
                            <Shield className="h-5 w-5" />
                            <span className="text-xs">防火墙</span>
                          </Button>
                        </div>
                      </TabsContent>

                      <TabsContent value="hardware">
                        <div className="space-y-4">
                          {serverHardware ? (
                            <>
                              <div className="p-4 border border-border rounded-sm">
                                <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                                  <Cpu className="h-4 w-4 text-accent" />
                                  处理器
                                </h3>
                                <p className="text-muted-foreground">{serverHardware.processorName || serverHardware.processor || '未知'}</p>
                              </div>
                              <div className="p-4 border border-border rounded-sm">
                                <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                                  <Monitor className="h-4 w-4 text-accent" />
                                  内存
                                </h3>
                                <p className="text-muted-foreground">{serverHardware.memorySize || '未知'}</p>
                              </div>
                              <div className="p-4 border border-border rounded-sm">
                                <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                                  <HardDrive className="h-4 w-4 text-accent" />
                                  存储
                                </h3>
                                <p className="text-muted-foreground">
                                  {serverHardware.diskGroups?.map((d: any) => d.description).join(', ') || '未知'}
                                </p>
                              </div>
                            </>
                          ) : (
                            <p className="text-center py-8 text-muted-foreground">暂无硬件信息</p>
                          )}
                        </div>
                      </TabsContent>

                      <TabsContent value="network">
                        <div className="space-y-4">
                          <div className="p-4 border border-border rounded-sm">
                            <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                              <Network className="h-4 w-4 text-accent" />
                              IP 地址
                            </h3>
                            {serverIps.length > 0 ? (
                              <div className="space-y-2 text-sm">
                                {serverIps.map((ip: any, index: number) => (
                                  <div key={index} className="flex justify-between">
                                    <span className="font-mono">{ip.ip || ip}</span>
                                    <span className="text-muted-foreground">{ip.type || 'IPv4'}</span>
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <p className="text-muted-foreground font-mono">{selectedServer.ip}</p>
                            )}
                          </div>
                        </div>
                      </TabsContent>

                      <TabsContent value="tasks">
                        {serverTasks.length > 0 ? (
                          <div className="space-y-2">
                            {serverTasks.map((task: any, index: number) => (
                              <div key={index} className="p-3 border border-border rounded-sm">
                                <div className="flex items-center justify-between mb-2">
                                  <span className="font-medium">{task.function || task.comment}</span>
                                  <span className={cn(
                                    "px-2 py-0.5 rounded-sm text-xs",
                                    task.status === "done" ? "bg-primary/20 text-primary" :
                                    task.status === "error" ? "bg-destructive/20 text-destructive" :
                                    "bg-warning/20 text-warning"
                                  )}>
                                    {task.status}
                                  </span>
                                </div>
                                <div className="text-xs text-muted-foreground flex items-center gap-2">
                                  <Clock className="h-3 w-3" />
                                  {new Date(task.startDate || task.todoDate).toLocaleString("zh-CN")}
                                </div>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="text-center py-8 text-muted-foreground">
                            <Activity className="h-12 w-12 mx-auto mb-4 opacity-30" />
                            <p>暂无进行中的任务</p>
                          </div>
                        )}
                      </TabsContent>
                    </Tabs>
                  )}
                </TerminalCard>
              ) : (
                <div className="flex items-center justify-center h-64 border border-border rounded-sm text-muted-foreground">
                  <div className="text-center">
                    <Server className="h-12 w-12 mx-auto mb-4 opacity-30" />
                    <p>选择一台服务器查看详情</p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </AppLayout>
    </>
  );
};

export default ServerControlPage;