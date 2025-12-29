import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { 
  Server, 
  Search, 
  RefreshCw, 
  Filter,
  ChevronDown,
  ShoppingCart,
  Eye,
  Cpu,
  HardDrive,
  MemoryStick,
  Activity
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

interface ServerPlan {
  planCode: string;
  name: string;
  description: string;
  cpu: string;
  memory: string;
  storage: string;
  bandwidth: string;
  datacenters: {
    datacenter: string;
    dcName: string;
    availability: string;
  }[];
}

const mockServers: ServerPlan[] = [
  {
    planCode: "24ska01",
    name: "KS-A",
    description: "入门级独立服务器",
    cpu: "Intel Xeon E3-1245v5 @ 3.5GHz",
    memory: "32GB DDR4 ECC",
    storage: "2x 480GB SSD",
    bandwidth: "500Mbps 无限流量",
    datacenters: [
      { datacenter: "gra", dcName: "Gravelines", availability: "1H" },
      { datacenter: "sbg", dcName: "Strasbourg", availability: "24H" },
      { datacenter: "rbx", dcName: "Roubaix", availability: "unavailable" },
    ]
  },
  {
    planCode: "24sk30",
    name: "KS-30",
    description: "高性能入门服务器",
    cpu: "AMD Ryzen 5 3600 @ 3.6GHz",
    memory: "64GB DDR4 ECC",
    storage: "2x 500GB NVMe",
    bandwidth: "1Gbps 无限流量",
    datacenters: [
      { datacenter: "gra", dcName: "Gravelines", availability: "available" },
      { datacenter: "rbx", dcName: "Roubaix", availability: "72H" },
      { datacenter: "bhs", dcName: "Beauharnois", availability: "unavailable" },
    ]
  },
  {
    planCode: "24rise01",
    name: "RISE-1",
    description: "专业级服务器",
    cpu: "AMD Ryzen 5 PRO 4650G @ 3.7GHz",
    memory: "64GB DDR4 ECC",
    storage: "2x 500GB NVMe",
    bandwidth: "1Gbps 无限流量",
    datacenters: [
      { datacenter: "gra", dcName: "Gravelines", availability: "1H" },
      { datacenter: "rbx", dcName: "Roubaix", availability: "1H" },
      { datacenter: "bhs", dcName: "Beauharnois", availability: "24H" },
      { datacenter: "sbg", dcName: "Strasbourg", availability: "unavailable" },
    ]
  },
  {
    planCode: "24rise02",
    name: "RISE-2",
    description: "高性能专业服务器",
    cpu: "AMD Ryzen 7 PRO 4750G @ 3.6GHz",
    memory: "128GB DDR4 ECC",
    storage: "2x 1TB NVMe",
    bandwidth: "1Gbps 无限流量",
    datacenters: [
      { datacenter: "gra", dcName: "Gravelines", availability: "unavailable" },
      { datacenter: "rbx", dcName: "Roubaix", availability: "unavailable" },
    ]
  },
  {
    planCode: "24adv01",
    name: "ADV-1",
    description: "企业级服务器",
    cpu: "AMD EPYC 7313P @ 3.0GHz",
    memory: "128GB DDR4 ECC",
    storage: "2x 960GB NVMe",
    bandwidth: "1Gbps 无限流量",
    datacenters: [
      { datacenter: "gra", dcName: "Gravelines", availability: "72H" },
      { datacenter: "rbx", dcName: "Roubaix", availability: "unavailable" },
      { datacenter: "sbg", dcName: "Strasbourg", availability: "unavailable" },
    ]
  },
];

const getAvailabilityInfo = (availability: string) => {
  if (availability === "unavailable") {
    return { color: "text-destructive bg-destructive/10", label: "无货", priority: 4 };
  }
  if (availability === "1H" || availability === "available") {
    return { color: "text-primary bg-primary/20", label: availability === "available" ? "有货" : "1H", priority: 1 };
  }
  if (availability === "24H") {
    return { color: "text-accent bg-accent/20", label: "24H", priority: 2 };
  }
  if (availability === "72H") {
    return { color: "text-warning bg-warning/20", label: "72H", priority: 3 };
  }
  return { color: "text-muted-foreground bg-muted", label: "未知", priority: 5 };
};

const ServersPage = () => {
  const [searchTerm, setSearchTerm] = useState("");
  const [filterAvailability, setFilterAvailability] = useState<string>("all");
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefresh = () => {
    setIsRefreshing(true);
    setTimeout(() => setIsRefreshing(false), 1500);
  };

  const filteredServers = mockServers.filter(server => {
    const matchesSearch = 
      server.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      server.planCode.toLowerCase().includes(searchTerm.toLowerCase()) ||
      server.cpu.toLowerCase().includes(searchTerm.toLowerCase());
    
    if (filterAvailability === "all") return matchesSearch;
    if (filterAvailability === "available") {
      return matchesSearch && server.datacenters.some(dc => dc.availability !== "unavailable");
    }
    return matchesSearch;
  });

  return (
    <>
      <Helmet>
        <title>服务器列表 | OVH Sniper</title>
        <meta name="description" content="浏览所有OVH服务器型号和实时可用性" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                服务器列表
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                共 {mockServers.length} 款服务器，{mockServers.filter(s => s.datacenters.some(d => d.availability !== "unavailable")).length} 款有库存
              </p>
            </div>
            
            <Button 
              variant="terminal" 
              onClick={handleRefresh}
              disabled={isRefreshing}
            >
              <RefreshCw className={cn("h-4 w-4", isRefreshing && "animate-spin")} />
              {isRefreshing ? "刷新中..." : "刷新列表"}
            </Button>
          </div>

          {/* Filters */}
          <div className="flex flex-col sm:flex-row gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input 
                placeholder="搜索型号、配置..." 
                className="pl-9"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
            </div>
            
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" className="w-full sm:w-auto">
                  <Filter className="h-4 w-4 mr-2" />
                  {filterAvailability === "all" ? "全部" : "仅有货"}
                  <ChevronDown className="h-4 w-4 ml-2" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem onClick={() => setFilterAvailability("all")}>
                  全部服务器
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setFilterAvailability("available")}>
                  仅显示有货
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          {/* Server List */}
          <div className="space-y-4">
            {filteredServers.map((server, index) => {
              const hasAvailable = server.datacenters.some(dc => dc.availability !== "unavailable");
              
              return (
                <TerminalCard 
                  key={server.planCode}
                  className={cn(
                    "transition-all",
                    hasAvailable && "border-primary/20"
                  )}
                >
                  <div className="flex flex-col lg:flex-row lg:items-start gap-4">
                    {/* Server Info */}
                    <div className="flex-1 space-y-3">
                      <div className="flex items-start justify-between">
                        <div>
                          <div className="flex items-center gap-2">
                            <Server className="h-5 w-5 text-primary" />
                            <h3 className="text-lg font-bold text-foreground">{server.name}</h3>
                            <span className="text-xs text-muted-foreground font-mono">({server.planCode})</span>
                          </div>
                          <p className="text-sm text-muted-foreground mt-1">{server.description}</p>
                        </div>
                        
                        {hasAvailable && (
                          <StatusBadge status="available" label="有库存" />
                        )}
                      </div>

                      {/* Specs Grid */}
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                        <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                          <Cpu className="h-4 w-4 text-accent" />
                          <div className="min-w-0">
                            <p className="text-xs text-muted-foreground">CPU</p>
                            <p className="text-foreground truncate">{server.cpu.split('@')[0]}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                          <MemoryStick className="h-4 w-4 text-accent" />
                          <div>
                            <p className="text-xs text-muted-foreground">内存</p>
                            <p className="text-foreground">{server.memory}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                          <HardDrive className="h-4 w-4 text-accent" />
                          <div>
                            <p className="text-xs text-muted-foreground">存储</p>
                            <p className="text-foreground">{server.storage}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                          <Activity className="h-4 w-4 text-accent" />
                          <div>
                            <p className="text-xs text-muted-foreground">带宽</p>
                            <p className="text-foreground">{server.bandwidth}</p>
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Datacenter Availability */}
                    <div className="lg:w-80 space-y-2">
                      <p className="text-xs text-muted-foreground uppercase tracking-wider mb-2">机房可用性</p>
                      <div className="grid grid-cols-2 gap-2">
                        {server.datacenters.map(dc => {
                          const info = getAvailabilityInfo(dc.availability);
                          return (
                            <div 
                              key={dc.datacenter}
                              className={cn(
                                "flex items-center justify-between p-2 rounded-sm border border-border/50",
                                info.color
                              )}
                            >
                              <div>
                                <p className="font-mono text-xs uppercase">{dc.datacenter}</p>
                                <p className="text-xs opacity-70">{dc.dcName}</p>
                              </div>
                              <span className="font-mono text-xs font-bold">{info.label}</span>
                            </div>
                          );
                        })}
                      </div>
                      
                      {/* Actions */}
                      <div className="flex gap-2 mt-3">
                        <Button variant="ghost" size="sm" className="flex-1">
                          <Eye className="h-4 w-4 mr-1" />
                          详情
                        </Button>
                        <Button 
                          size="sm" 
                          className="flex-1"
                          disabled={!hasAvailable}
                        >
                          <ShoppingCart className="h-4 w-4 mr-1" />
                          加入队列
                        </Button>
                      </div>
                    </div>
                  </div>
                </TerminalCard>
              );
            })}
          </div>

          {filteredServers.length === 0 && (
            <div className="text-center py-12 text-muted-foreground">
              <Server className="h-12 w-12 mx-auto mb-4 opacity-30" />
              <p>未找到匹配的服务器</p>
            </div>
          )}
        </div>
      </AppLayout>
    </>
  );
};

export default ServersPage;
