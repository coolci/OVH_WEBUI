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
  Settings2,
  Activity,
  Shield,
  Server,
  MoreVertical,
  Monitor
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
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

interface ManagedServer {
  serviceName: string;
  name: string;
  datacenter: string;
  state: "ok" | "error" | "hacked" | "hackedBlocked";
  monitoring: boolean;
  ip: string;
  os: string;
  status: string;
  renewalType: boolean;
}

const mockServers: ManagedServer[] = [
  {
    serviceName: "ns123456.ip-192-168-1.eu",
    name: "web-server-01",
    datacenter: "gra",
    state: "ok",
    monitoring: true,
    ip: "192.168.1.100",
    os: "Ubuntu 22.04 LTS",
    status: "active",
    renewalType: true,
  },
  {
    serviceName: "ns123457.ip-192-168-1.eu",
    name: "db-server-01",
    datacenter: "rbx",
    state: "ok",
    monitoring: true,
    ip: "192.168.1.101",
    os: "Debian 12",
    status: "active",
    renewalType: true,
  },
  {
    serviceName: "ns123458.ip-192-168-1.eu",
    name: "app-server-01",
    datacenter: "sbg",
    state: "error",
    monitoring: false,
    ip: "192.168.1.102",
    os: "CentOS 8",
    status: "suspended",
    renewalType: false,
  },
];

const ServerControlPage = () => {
  const [servers] = useState(mockServers);
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedServer, setSelectedServer] = useState<ManagedServer | null>(mockServers[0]);
  const [isLoading, setIsLoading] = useState(false);

  const handleRefresh = () => {
    setIsLoading(true);
    setTimeout(() => setIsLoading(false), 1500);
  };

  const filteredServers = servers.filter(server => 
    server.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    server.serviceName.toLowerCase().includes(searchTerm.toLowerCase()) ||
    server.ip.includes(searchTerm)
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
            
            <Button variant="terminal" onClick={handleRefresh} disabled={isLoading}>
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
                <div className="space-y-2">
                  {filteredServers.map((server) => (
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
                        <span className="font-medium">{server.name}</span>
                        <span className={cn(
                          "px-2 py-0.5 rounded-sm text-xs uppercase",
                          getStateColor(server.state)
                        )}>
                          {server.state}
                        </span>
                      </div>
                      <div className="text-xs text-muted-foreground space-y-1">
                        <p className="font-mono">{server.ip}</p>
                        <p>{server.datacenter.toUpperCase()} | {server.os}</p>
                      </div>
                    </div>
                  ))}

                  {filteredServers.length === 0 && (
                    <p className="text-center py-8 text-muted-foreground">
                      未找到服务器
                    </p>
                  )}
                </div>
              </TerminalCard>
            </div>

            {/* Server Details */}
            <div className="lg:col-span-2">
              {selectedServer ? (
                <TerminalCard
                  title={selectedServer.name}
                  icon={<Cpu className="h-4 w-4" />}
                  headerAction={
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem>
                          <Power className="h-4 w-4 mr-2" />
                          重启服务器
                        </DropdownMenuItem>
                        <DropdownMenuItem>
                          <HardDrive className="h-4 w-4 mr-2" />
                          重装系统
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
                            <p className={selectedServer.renewalType ? "text-primary" : "text-destructive"}>
                              {selectedServer.renewalType ? "已开启" : "已关闭"}
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
                            <p>{selectedServer.status}</p>
                          </div>
                        </div>
                      </div>

                      {/* Quick Actions */}
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                        <Button variant="outline" className="h-auto py-4 flex-col gap-2">
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
                        <div className="p-4 border border-border rounded-sm">
                          <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                            <Cpu className="h-4 w-4 text-accent" />
                            处理器
                          </h3>
                          <p className="text-muted-foreground">Intel Xeon E3-1245v5 @ 3.5GHz</p>
                        </div>
                        <div className="p-4 border border-border rounded-sm">
                          <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                            <Monitor className="h-4 w-4 text-accent" />
                            内存
                          </h3>
                          <p className="text-muted-foreground">32GB DDR4 ECC</p>
                        </div>
                        <div className="p-4 border border-border rounded-sm">
                          <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                            <HardDrive className="h-4 w-4 text-accent" />
                            存储
                          </h3>
                          <p className="text-muted-foreground">2x 480GB SSD (RAID 1)</p>
                        </div>
                      </div>
                    </TabsContent>

                    <TabsContent value="network">
                      <div className="space-y-4">
                        <div className="p-4 border border-border rounded-sm">
                          <h3 className="text-sm font-medium mb-3 flex items-center gap-2">
                            <Network className="h-4 w-4 text-accent" />
                            网络配置
                          </h3>
                          <div className="space-y-2 text-sm">
                            <div className="flex justify-between">
                              <span className="text-muted-foreground">主 IP</span>
                              <span className="font-mono">{selectedServer.ip}</span>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-muted-foreground">网关</span>
                              <span className="font-mono">192.168.1.1</span>
                            </div>
                            <div className="flex justify-between">
                              <span className="text-muted-foreground">带宽</span>
                              <span>500 Mbps</span>
                            </div>
                          </div>
                        </div>
                        
                        <Button variant="terminal" size="sm">
                          <Settings2 className="h-4 w-4 mr-2" />
                          管理 IP 地址
                        </Button>
                      </div>
                    </TabsContent>

                    <TabsContent value="tasks">
                      <div className="text-center py-8 text-muted-foreground">
                        <Activity className="h-12 w-12 mx-auto mb-4 opacity-30" />
                        <p>暂无进行中的任务</p>
                      </div>
                    </TabsContent>
                  </Tabs>
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
