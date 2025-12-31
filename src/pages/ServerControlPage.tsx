import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { 
  Cpu, 
  Search,
  RefreshCw,
  Power,
  HardDrive,
  Network,
  Terminal,
  Activity,
  Server,
  Monitor,
  Loader2,
  Calendar,
  Clock,
  AlertTriangle,
  Users,
  Wrench,
  CheckCircle2,
  XCircle,
  Copy
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import {
} from "@/components/ui/dropdown-menu";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
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
  const [serverTemplates, setServerTemplates] = useState<any[]>([]);
  const [plannedInterventions, setPlannedInterventions] = useState<any[]>([]);
  const [historicalInterventions, setHistoricalInterventions] = useState<any[]>([]);
  const [installStatus, setInstallStatus] = useState<any>(null);
  const [isLoadingDetails, setIsLoadingDetails] = useState(false);
  const [detailsWarning, setDetailsWarning] = useState<string | null>(null);
  const [isRebooting, setIsRebooting] = useState(false);
  
  const [isReinstallDialogOpen, setIsReinstallDialogOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState("");
  const [isReinstalling, setIsReinstalling] = useState(false);
  const [isLoadingTemplates, setIsLoadingTemplates] = useState(false);
  
  const [isContactDialogOpen, setIsContactDialogOpen] = useState(false);
  const [contactAdmin, setContactAdmin] = useState("");
  const [contactTech, setContactTech] = useState("");
  const [contactBilling, setContactBilling] = useState("");
  const [isChangingContact, setIsChangingContact] = useState(false);
  
  const [isIpmiDialogOpen, setIsIpmiDialogOpen] = useState(false);
  const [ipmiInfo, setIpmiInfo] = useState<any>(null);
  const [isLoadingIpmi, setIsLoadingIpmi] = useState(false);
  
  
  
  
  

  const servers = serversData?.servers || [];

  const formatUnitValue = (value: any) => {
    if (value === null || value === undefined) return "未知";
    if (typeof value === "object") {
      if (Array.isArray(value)) return value.join(", ");
      if ("value" in value || "unit" in value) {
        const unitValue = value as { value?: string | number; unit?: string };
        return `${unitValue.value ?? ""} ${unitValue.unit ?? ""}`.trim() || "未知";
      }
      return JSON.stringify(value);
    }
    return String(value);
  };

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

  useEffect(() => {
    if (installStatus && !installStatus.allDone && !installStatus.hasError) {
      const interval = setInterval(async () => {
        if (selectedServer) {
          const status = await api.getInstallStatus(selectedServer.serviceName).catch(() => null);
          if (status?.status) {
            setInstallStatus(status.status);
          }
        }
      }, 5000);
      return () => clearInterval(interval);
    }
  }, [installStatus, selectedServer]);

  const loadServerDetails = async (serviceName: string) => {
    setIsLoadingDetails(true);
    setDetailsWarning(null);
    try {
      const info = await api.getServiceInfo(serviceName).catch((error: any) => {
        setDetailsWarning(`服务信息获取失败: ${error?.message || error}`);
        return null;
      });

      const statusValue = info?.serviceInfo?.status || "";
      const isExpired =
        typeof statusValue === "string" && statusValue.toLowerCase().includes("expired");

      setServiceInfo(info?.serviceInfo);
      setServerDetails(info?.serviceInfo);

      if (isExpired) {
        setDetailsWarning("该服务已过期，部分接口不可用");
        setServerHardware(null);
        setServerIps([]);
        setServerTasks([]);
        setPlannedInterventions([]);
        setHistoricalInterventions([]);
        setInstallStatus(null);
        return;
      }

      const [hardware, ips, tasks, planned, historical, install] = await Promise.all([
        api.getServerHardware(serviceName).catch(() => null),
        api.getServerIps(serviceName).catch(() => null),
        api.getServerTasks(serviceName).catch(() => null),
        api.getPlannedInterventions(serviceName).catch(() => null),
        api.getInterventions(serviceName).catch(() => null),
        api.getInstallStatus(serviceName).catch(() => null),
      ]);
      
      setServerHardware(hardware?.hardware);
      setServerIps(ips?.ips || []);
      setServerTasks(tasks?.tasks || []);
      setPlannedInterventions(planned?.plannedInterventions || []);
      setHistoricalInterventions(historical?.interventions || []);
      setInstallStatus(install?.status || null);
    } catch (error) {
      console.error('加载服务器详情失败:', error);
    } finally {
      setIsLoadingDetails(false);
    }
  };

  const handleReboot = async () => {
    if (!selectedServer) return;
    setIsRebooting(true);
    try {
      await api.rebootServer(selectedServer.serviceName, 'hardware');
      toast.success(`服务器${type === 'soft' ? '软' : '硬'}重启已发起`);
      const tasks = await api.getServerTasks(selectedServer.serviceName);
      setServerTasks(tasks?.tasks || []);
    } catch (error: any) {
      toast.error(`重启失败: ${error.message}`);
    } finally {
      setIsRebooting(false);
    }
  };

  const handleOpenReinstall = async () => {
    if (!selectedServer) return;
    setIsReinstallDialogOpen(true);
    setIsLoadingTemplates(true);
    try {
      const result = await api.getServerTemplates(selectedServer.serviceName);
      setServerTemplates(result?.templates || []);
    } catch (error) {
      toast.error("加载模板列表失败");
    } finally {
      setIsLoadingTemplates(false);
    }
  };

  const handleOpenIpmi = async () => {
    if (!selectedServer) return;
    setIsIpmiDialogOpen(true);
    setIsLoadingIpmi(true);
    setIpmiInfo(null);
    try {
      const result = await api.getIpmiAccess(selectedServer.serviceName);
      if (result.success) {
        const ipmiData = result.ipmiInfos || result.console || result.ipmi || null;
        setIpmiInfo(ipmiData);
      } else {
        toast.error(result.error || "获取IPMI信息失败");
      }
    } catch (error: any) {
      toast.error(`获取IPMI信息失败: ${error.message}`);
    } finally {
      setIsLoadingIpmi(false);
    }
  };






  const handleReinstall = async () => {
    if (!selectedServer || !selectedTemplate) return;
    setIsReinstalling(true);
    try {
      await api.reinstallServer(selectedServer.serviceName, selectedTemplate);
      toast.success("重装系统任务已发起");
      setIsReinstallDialogOpen(false);
      setSelectedTemplate("");
      const [tasks, install] = await Promise.all([
        api.getServerTasks(selectedServer.serviceName),
        api.getInstallStatus(selectedServer.serviceName),
      ]);
      setServerTasks(tasks?.tasks || []);
      setInstallStatus(install?.status || null);
    } catch (error: any) {
      toast.error(`重装失败: ${error.message}`);
    } finally {
      setIsReinstalling(false);
    }
  };

  const handleChangeContact = async () => {
    if (!selectedServer) return;
    setIsChangingContact(true);
    try {
      await api.changeContact(selectedServer.serviceName, {
        contactAdmin: contactAdmin || undefined,
        contactTech: contactTech || undefined,
        contactBilling: contactBilling || undefined,
      });
      toast.success("联系人变更请求已发送");
      setIsContactDialogOpen(false);
    } catch (error: any) {
      toast.error(`变更失败: ${error.message}`);
    } finally {
      setIsChangingContact(false);
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
                  headerAction={null}
                >
                  {isLoadingDetails ? (
                    <div className="flex items-center justify-center py-12">
                      <Loader2 className="h-8 w-8 animate-spin text-primary" />
                    </div>
                  ) : (
                    <Tabs defaultValue="overview" className="space-y-4">
                      {detailsWarning && (
                        <div className="p-3 rounded-sm border border-warning/30 bg-warning/5 text-warning text-xs">
                          {detailsWarning}
                        </div>
                      )}
                      <TabsList className="bg-muted/50 border border-border">
                        <TabsTrigger value="overview">概览</TabsTrigger>
                        <TabsTrigger value="hardware">硬件</TabsTrigger>
                        <TabsTrigger value="network">网络</TabsTrigger>
                        <TabsTrigger value="tasks">任务</TabsTrigger>
                        <TabsTrigger value="maintenance">维护</TabsTrigger>
                      </TabsList>

                      <TabsContent value="overview" className="space-y-4">
                        {/* Installation Progress */}
                        {installStatus && !installStatus.noInstallation && (
                          <div className={cn(
                            "p-4 border rounded-sm",
                            installStatus.hasError ? "border-destructive/30 bg-destructive/5" :
                            installStatus.allDone ? "border-primary/30 bg-primary/5" :
                            "border-warning/30 bg-warning/5"
                          )}>
                            <div className="flex items-center justify-between mb-2">
                              <h3 className="text-sm font-medium flex items-center gap-2">
                                <HardDrive className="h-4 w-4" />
                                系统安装进度
                              </h3>
                              {installStatus.allDone ? (
                                <CheckCircle2 className="h-4 w-4 text-primary" />
                              ) : installStatus.hasError ? (
                                <XCircle className="h-4 w-4 text-destructive" />
                              ) : (
                                <Loader2 className="h-4 w-4 animate-spin text-warning" />
                              )}
                            </div>
                            <Progress value={installStatus.progressPercentage || 0} className="h-2 mb-2" />
                            <div className="flex justify-between text-xs text-muted-foreground">
                              <span>{installStatus.completedSteps}/{installStatus.totalSteps} 步骤</span>
                              <span>{installStatus.progressPercentage}%</span>
                            </div>
                            {installStatus.steps && (
                              <div className="mt-3 space-y-1 max-h-32 overflow-y-auto">
                                {installStatus.steps.map((step: any, i: number) => (
                                  <div key={i} className="flex items-center gap-2 text-xs">
                                    {step.status === "done" ? (
                                      <CheckCircle2 className="h-3 w-3 text-primary" />
                                    ) : step.status === "error" ? (
                                      <XCircle className="h-3 w-3 text-destructive" />
                                    ) : (
                                      <Clock className="h-3 w-3 text-muted-foreground" />
                                    )}
                                    <span className={step.status === "error" ? "text-destructive" : ""}>
                                      {step.comment}
                                    </span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        )}

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
                            onClick={handleReboot}
                            disabled={isRebooting}
                          >
                            <Power className="h-5 w-5 text-warning" />
                            <span className="text-xs">硬重启</span>
                          </Button>
                          <Button 
                            variant="outline" 
                            className="h-auto py-4 flex-col gap-2"
                            onClick={handleOpenReinstall}
                          >
                            <HardDrive className="h-5 w-5 text-accent" />
                            <span className="text-xs">重装</span>
                          </Button>
                          <Button 
                            variant="outline" 
                            className="h-auto py-4 flex-col gap-2"
                            onClick={handleOpenIpmi}
                          >
                            <Terminal className="h-5 w-5 text-primary" />
                            <span className="text-xs">控制台</span>
                          </Button>
                          <Button 
                            variant="outline" 
                            className="h-auto py-4 flex-col gap-2"
                            onClick={() => setIsContactDialogOpen(true)}
                          >
                            <Users className="h-5 w-5" />
                            <span className="text-xs">联系人</span>
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
                                <p className="text-muted-foreground">
                                  {serverHardware.processorName || serverHardware.processor || '未知'}
                                </p>
                              </div>
                              <div className="p-4 border border-border rounded-sm">
                                <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                                  <Monitor className="h-4 w-4 text-accent" />
                                  内存
                                </h3>
                                <p className="text-muted-foreground">
                                  {formatUnitValue(serverHardware.memorySize)}
                                </p>
                              </div>
                              <div className="p-4 border border-border rounded-sm">
                                <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                                  <HardDrive className="h-4 w-4 text-accent" />
                                  存储
                                </h3>
                                <p className="text-muted-foreground">
                                  {serverHardware.diskGroups?.map((d: any) => d.description || formatUnitValue(d.size)).join(', ') || '未知'}
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

                      <TabsContent value="maintenance">
                        <div className="space-y-6">
                          {/* Planned Interventions */}
                          <div className="space-y-4">
                            <h3 className="text-sm font-medium flex items-center gap-2">
                              <AlertTriangle className="h-4 w-4 text-warning" />
                              计划维护
                            </h3>
                            {plannedInterventions.length > 0 ? (
                              <div className="space-y-2">
                                {plannedInterventions.map((intervention: any, index: number) => (
                                  <div key={index} className="p-4 border border-warning/30 bg-warning/5 rounded-sm">
                                    <div className="flex items-start gap-3">
                                      <AlertTriangle className="h-5 w-5 text-warning mt-0.5" />
                                      <div className="flex-1">
                                        <p className="font-medium">{intervention.type || '计划维护'}</p>
                                        <p className="text-sm text-muted-foreground mt-1">
                                          {intervention.comment || intervention.description}
                                        </p>
                                        {intervention.date && (
                                          <p className="text-xs text-warning mt-2 flex items-center gap-1">
                                            <Calendar className="h-3 w-3" />
                                            {new Date(intervention.date).toLocaleString("zh-CN")}
                                          </p>
                                        )}
                                      </div>
                                    </div>
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <div className="text-center py-4 text-muted-foreground border border-border rounded-sm">
                                <CheckCircle2 className="h-8 w-8 mx-auto mb-2 opacity-30" />
                                <p className="text-sm">暂无计划维护</p>
                              </div>
                            )}
                          </div>

                          {/* Historical Interventions */}
                          <div className="space-y-4">
                            <h3 className="text-sm font-medium flex items-center gap-2">
                              <Wrench className="h-4 w-4" />
                              历史干预记录
                            </h3>
                            {historicalInterventions.length > 0 ? (
                              <div className="space-y-2 max-h-64 overflow-y-auto">
                                {historicalInterventions.map((intervention: any, index: number) => (
                                  <div key={index} className="p-3 border border-border rounded-sm">
                                    <div className="flex items-start justify-between gap-3">
                                      <div className="flex-1">
                                        <p className="font-medium text-sm">{intervention.type || '干预'}</p>
                                        {intervention.description && (
                                          <p className="text-xs text-muted-foreground mt-1">
                                            {intervention.description}
                                          </p>
                                        )}
                                      </div>
                                      <span className={cn(
                                        "px-2 py-0.5 rounded-sm text-xs",
                                        intervention.status === "done" ? "bg-primary/20 text-primary" :
                                        intervention.status === "error" ? "bg-destructive/20 text-destructive" :
                                        "bg-muted text-muted-foreground"
                                      )}>
                                        {intervention.status || 'N/A'}
                                      </span>
                                    </div>
                                    <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                                      {intervention.date && (
                                        <span className="flex items-center gap-1">
                                          <Calendar className="h-3 w-3" />
                                          {new Date(intervention.date).toLocaleString("zh-CN")}
                                        </span>
                                      )}
                                      {intervention.interventionId && (
                                        <span className="font-mono">#{intervention.interventionId}</span>
                                      )}
                                    </div>
                                  </div>
                                ))}
                              </div>
                            ) : (
                              <div className="text-center py-4 text-muted-foreground border border-border rounded-sm">
                                <Wrench className="h-8 w-8 mx-auto mb-2 opacity-30" />
                                <p className="text-sm">暂无历史干预记录</p>
                              </div>
                            )}
                          </div>
                        </div>
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

      {/* Reinstall Dialog */}
      <Dialog open={isReinstallDialogOpen} onOpenChange={setIsReinstallDialogOpen}>
        <DialogContent className="terminal-card border-primary/30">
          <DialogHeader>
            <DialogTitle className="text-primary flex items-center gap-2">
              <HardDrive className="h-5 w-5" />
              重装系统
            </DialogTitle>
            <DialogDescription>
              选择系统模板进行重装，此操作将清除所有数据
            </DialogDescription>
          </DialogHeader>
          
          <div className="space-y-4 py-4">
            <div className="p-3 bg-destructive/10 border border-destructive/30 rounded-sm">
              <p className="text-sm text-destructive flex items-center gap-2">
                <AlertTriangle className="h-4 w-4" />
                警告：重装系统将删除服务器上的所有数据！
              </p>
            </div>
            
            <div className="space-y-2">
              <Label>系统模板</Label>
              {isLoadingTemplates ? (
                <div className="flex items-center justify-center py-4">
                  <Loader2 className="h-6 w-6 animate-spin text-primary" />
                </div>
              ) : (
                <Select value={selectedTemplate} onValueChange={setSelectedTemplate}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择系统模板" />
                  </SelectTrigger>
                  <SelectContent className="max-h-64">
                    {serverTemplates.map((template: any) => (
                      <SelectItem key={template.name} value={template.name}>
                        <div className="flex flex-col">
                          <span>{template.name}</span>
                          {template.description && (
                            <span className="text-xs text-muted-foreground">{template.description}</span>
                          )}
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>
          </div>
          
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">取消</Button>
            </DialogClose>
            <Button 
              variant="destructive" 
              onClick={handleReinstall} 
              disabled={!selectedTemplate || isReinstalling}
            >
              {isReinstalling ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <HardDrive className="h-4 w-4 mr-2" />}
              确认重装
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Contact Change Dialog */}
      <Dialog open={isContactDialogOpen} onOpenChange={setIsContactDialogOpen}>
        <DialogContent className="terminal-card border-primary/30">
          <DialogHeader>
            <DialogTitle className="text-primary flex items-center gap-2">
              <Users className="h-5 w-5" />
              变更联系人
            </DialogTitle>
            <DialogDescription>
              修改服务器的管理员、技术和账单联系人
            </DialogDescription>
          </DialogHeader>
          
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>管理员联系人 (NIC Handle)</Label>
              <Input 
                value={contactAdmin}
                onChange={(e) => setContactAdmin(e.target.value)}
                placeholder="例如: xx12345-ovh"
              />
            </div>
            <div className="space-y-2">
              <Label>技术联系人 (NIC Handle)</Label>
              <Input 
                value={contactTech}
                onChange={(e) => setContactTech(e.target.value)}
                placeholder="例如: xx12345-ovh"
              />
            </div>
            <div className="space-y-2">
              <Label>账单联系人 (NIC Handle)</Label>
              <Input 
                value={contactBilling}
                onChange={(e) => setContactBilling(e.target.value)}
                placeholder="例如: xx12345-ovh"
              />
            </div>
          </div>
          
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">取消</Button>
            </DialogClose>
            <Button onClick={handleChangeContact} disabled={isChangingContact}>
              {isChangingContact ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Users className="h-4 w-4 mr-2" />}
              提交变更
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* IPMI Dialog */}
      <Dialog open={isIpmiDialogOpen} onOpenChange={setIsIpmiDialogOpen}>
        <DialogContent className="terminal-card border-primary/30">
          <DialogHeader>
            <DialogTitle className="text-primary flex items-center gap-2">
              <Terminal className="h-5 w-5" />
              IPMI 控制台
            </DialogTitle>
            <DialogDescription>
              远程管理服务器的KVM和BIOS访问
            </DialogDescription>
          </DialogHeader>
          
          <div className="space-y-4 py-4">
            {isLoadingIpmi ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="h-6 w-6 animate-spin text-primary" />
              </div>
            ) : ipmiInfo ? (
              <>
                <div className="space-y-3">
                  {ipmiInfo.url && (
                    <div className="p-3 bg-muted/30 rounded-sm">
                      <p className="text-xs text-muted-foreground mb-1">控制台 URL</p>
                      <div className="flex items-center justify-between gap-2">
                        <p className="font-mono text-xs break-all">{ipmiInfo.url}</p>
                        <Button variant="ghost" size="sm" onClick={() => {
                          navigator.clipboard.writeText(ipmiInfo.url);
                          toast.success("已复制");
                        }}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </div>
                      <Button
                        variant="outline"
                        size="sm"
                        className="mt-2 w-full"
                        asChild
                      >
                        <a href={ipmiInfo.url} target="_blank" rel="noopener noreferrer">
                          打开控制台
                        </a>
                      </Button>
                    </div>
                  )}
                  {ipmiInfo.ip && (
                    <div className="p-3 bg-muted/30 rounded-sm">
                      <p className="text-xs text-muted-foreground mb-1">IP 地址</p>
                      <p className="font-mono flex items-center justify-between">
                        {ipmiInfo.ip}
                        <Button variant="ghost" size="sm" onClick={() => {
                          navigator.clipboard.writeText(ipmiInfo.ip);
                          toast.success("已复制");
                        }}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </p>
                    </div>
                  )}
                  {ipmiInfo.login && (
                    <div className="p-3 bg-muted/30 rounded-sm">
                      <p className="text-xs text-muted-foreground mb-1">用户名</p>
                      <p className="font-mono flex items-center justify-between">
                        {ipmiInfo.login}
                        <Button variant="ghost" size="sm" onClick={() => {
                          navigator.clipboard.writeText(ipmiInfo.login);
                          toast.success("已复制");
                        }}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </p>
                    </div>
                  )}
                  {ipmiInfo.password && (
                    <div className="p-3 bg-muted/30 rounded-sm">
                      <p className="text-xs text-muted-foreground mb-1">密码</p>
                      <p className="font-mono flex items-center justify-between">
                        {ipmiInfo.password}
                        <Button variant="ghost" size="sm" onClick={() => {
                          navigator.clipboard.writeText(ipmiInfo.password);
                          toast.success("已复制");
                        }}>
                          <Copy className="h-3 w-3" />
                        </Button>
                      </p>
                    </div>
                  )}
                  {(ipmiInfo.expires || ipmiInfo.expiration) && (
                    <p className="text-xs text-muted-foreground text-center">
                      有效期至: {new Date(ipmiInfo.expires || ipmiInfo.expiration).toLocaleString("zh-CN")}
                    </p>
                  )}
                </div>
                <div className="p-3 bg-warning/10 border border-warning/30 rounded-sm">
                  <p className="text-xs text-warning">
                    ⚠️ IPMI凭证每次请求都会重新生成，请及时使用
                  </p>
                </div>
              </>
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                <Terminal className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>IPMI信息不可用</p>
                <p className="text-xs mt-2">此服务器可能不支持IPMI</p>
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





    </>
  );
};

export default ServerControlPage;
