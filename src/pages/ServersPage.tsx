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
  Cpu,
  HardDrive,
  MemoryStick,
  Activity,
  Loader2,
  Zap,
  DollarSign,
  Settings2
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import api from "@/lib/api";
import { useServers } from "@/hooks/useApi";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
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
import { Checkbox } from "@/components/ui/checkbox";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

const getAvailabilityInfo = (availability: string) => {
  if (availability === "unavailable" || availability === "unknown") {
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
  return { color: "text-muted-foreground bg-muted", label: availability || "未知", priority: 5 };
};

const ServersPage = () => {
  const { data: servers, isLoading, refetch } = useServers();
  const [searchTerm, setSearchTerm] = useState("");
  const [filterAvailability, setFilterAvailability] = useState<string>("all");
  const [isRefreshing, setIsRefreshing] = useState(false);
  
  // 弹窗状态
  const [selectedServer, setSelectedServer] = useState<any>(null);
  const [selectedDc, setSelectedDc] = useState<string>("");
  const [selectedOptions, setSelectedOptions] = useState<string[]>([]);
  const [isAddingToQueue, setIsAddingToQueue] = useState(false);
  const [isQuickOrdering, setIsQuickOrdering] = useState(false);
  const [priceInfo, setPriceInfo] = useState<any>(null);
  const [isLoadingPrice, setIsLoadingPrice] = useState(false);
  const [dialogMode, setDialogMode] = useState<'queue' | 'quick'>('queue');

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await api.refreshServers(true);
      await refetch();
      toast.success("服务器列表已刷新");
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsRefreshing(false);
    }
  };

  // 加载价格
  const loadPrice = async (planCode: string, datacenter: string, options: string[]) => {
    setIsLoadingPrice(true);
    setPriceInfo(null);
    try {
      const result = await api.getServerPrice(planCode, datacenter, options);
      if (result.success && result.price) {
        setPriceInfo(result.price);
      }
    } catch (err) {
      console.error('Failed to load price:', err);
    } finally {
      setIsLoadingPrice(false);
    }
  };

  // 当选择变化时加载价格
  useEffect(() => {
    if (selectedServer && selectedDc) {
      loadPrice(selectedServer.planCode, selectedDc, selectedOptions);
    }
  }, [selectedServer, selectedDc, selectedOptions]);

  const handleAddToQueue = async () => {
    if (!selectedServer || !selectedDc) return;
    
    setIsAddingToQueue(true);
    try {
      await api.addQueueItem({
        planCode: selectedServer.planCode,
        datacenter: selectedDc,
        options: selectedOptions,
        retryInterval: 30,
      });
      toast.success(`已添加 ${selectedServer.planCode} @ ${selectedDc.toUpperCase()} 到队列`);
      closeDialog();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsAddingToQueue(false);
    }
  };

  const handleQuickOrder = async () => {
    if (!selectedServer || !selectedDc) {
      toast.error("请选择机房");
      return;
    }
    
    setIsQuickOrdering(true);
    try {
      const result = await api.quickOrder({ 
        planCode: selectedServer.planCode, 
        datacenter: selectedDc,
        options: selectedOptions,
      });
      if (result.success) {
        toast.success(result.message || "下单成功！请前往OVH支付订单");
        closeDialog();
      } else {
        toast.error(result.message || "下单失败");
      }
    } catch (err: any) {
      if (err.message?.includes("Failed to fetch") || err.message?.includes("NetworkError")) {
        toast.error("无法连接到后端服务，请检查后端是否运行");
      } else {
        toast.error(err.message || "下单请求失败");
      }
    } finally {
      setIsQuickOrdering(false);
    }
  };

  const openDialog = (server: any, mode: 'queue' | 'quick', datacenter?: string) => {
    setSelectedServer(server);
    setDialogMode(mode);
    setPriceInfo(null);
    setSelectedOptions([]);
    
    if (datacenter) {
      setSelectedDc(datacenter);
    } else {
      const availableDc = server.datacenters?.find((dc: any) => 
        dc.availability !== "unavailable" && dc.availability !== "unknown"
      );
      setSelectedDc(availableDc?.datacenter || "");
    }
  };

  const closeDialog = () => {
    setSelectedServer(null);
    setSelectedDc("");
    setSelectedOptions([]);
    setPriceInfo(null);
  };

  const toggleOption = (option: string) => {
    setSelectedOptions(prev => 
      prev.includes(option) 
        ? prev.filter(o => o !== option)
        : [...prev, option]
    );
  };

  const serverList = servers || [];
  
  const filteredServers = serverList.filter(server => {
    const matchesSearch = 
      (server.name || "").toLowerCase().includes(searchTerm.toLowerCase()) ||
      server.planCode.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (server.cpu || "").toLowerCase().includes(searchTerm.toLowerCase());
    
    if (filterAvailability === "all") return matchesSearch;
    if (filterAvailability === "available") {
      return matchesSearch && server.datacenters?.some(dc => 
        dc.availability !== "unavailable" && dc.availability !== "unknown"
      );
    }
    return matchesSearch;
  });

  const availableCount = serverList.filter(s => 
    s.datacenters?.some(d => d.availability !== "unavailable" && d.availability !== "unknown")
  ).length;

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
                共 {serverList.length} 款服务器，{availableCount} 款有库存
              </p>
            </div>
            
            <Button 
              variant="terminal" 
              onClick={handleRefresh}
              disabled={isRefreshing}
            >
              <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
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
          {isLoading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          ) : (
            <div className="space-y-4">
              {filteredServers.map((server) => {
                const hasAvailable = server.datacenters?.some(dc => 
                  dc.availability !== "unavailable" && dc.availability !== "unknown"
                );
                
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
                              <h3 className="text-lg font-bold text-foreground">{server.name || server.planCode}</h3>
                              <span className="text-xs text-muted-foreground font-mono">({server.planCode})</span>
                            </div>
                            <p className="text-sm text-muted-foreground mt-1">
                              €{server.price?.toFixed(2) || 'N/A'} / 月
                            </p>
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
                              <p className="text-foreground truncate">{server.cpu || 'N/A'}</p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                            <MemoryStick className="h-4 w-4 text-accent" />
                            <div>
                              <p className="text-xs text-muted-foreground">内存</p>
                              <p className="text-foreground">{server.ram || 'N/A'}</p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                            <HardDrive className="h-4 w-4 text-accent" />
                            <div>
                              <p className="text-xs text-muted-foreground">存储</p>
                              <p className="text-foreground">{server.storage || 'N/A'}</p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 p-2 bg-muted/30 rounded-sm">
                            <Activity className="h-4 w-4 text-accent" />
                            <div>
                              <p className="text-xs text-muted-foreground">带宽</p>
                              <p className="text-foreground">{server.bandwidth || 'N/A'}</p>
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* Datacenter Availability */}
                      <div className="lg:w-80 space-y-2">
                        <p className="text-xs text-muted-foreground uppercase tracking-wider mb-2">机房可用性</p>
                        <div className="grid grid-cols-2 gap-2">
                          {server.datacenters?.slice(0, 6).map(dc => {
                            const info = getAvailabilityInfo(dc.availability);
                            const isAvailable = dc.availability !== "unavailable" && dc.availability !== "unknown";
                            return (
                              <div 
                                key={dc.datacenter}
                                className={cn(
                                  "flex items-center justify-between p-2 rounded-sm border border-border/50 cursor-pointer hover:border-primary/50 transition-colors",
                                  info.color
                                )}
                                onClick={() => isAvailable && openDialog(server, 'quick', dc.datacenter)}
                              >
                                <div>
                                  <p className="font-mono text-xs uppercase">{dc.datacenter}</p>
                                </div>
                                <span className="font-mono text-xs font-bold">{info.label}</span>
                              </div>
                            );
                          })}
                        </div>
                        
                        {/* Actions */}
                        <div className="flex gap-2 mt-3">
                          <Button 
                            size="sm" 
                            variant="outline"
                            className="flex-1"
                            disabled={!hasAvailable}
                            onClick={() => openDialog(server, 'queue')}
                          >
                            <ShoppingCart className="h-4 w-4 mr-1" />
                            加入队列
                          </Button>
                          <Button 
                            size="sm" 
                            className="flex-1"
                            disabled={!hasAvailable}
                            onClick={() => openDialog(server, 'quick')}
                          >
                            <Zap className="h-4 w-4 mr-1" />
                            快速下单
                          </Button>
                        </div>
                      </div>
                    </div>
                  </TerminalCard>
                );
              })}
            </div>
          )}

          {!isLoading && filteredServers.length === 0 && (
            <div className="text-center py-12 text-muted-foreground">
              <Server className="h-12 w-12 mx-auto mb-4 opacity-30" />
              <p>未找到匹配的服务器</p>
              <p className="text-sm mt-2">请尝试刷新服务器列表</p>
            </div>
          )}
        </div>
      </AppLayout>

      {/* Order Dialog */}
      <Dialog open={!!selectedServer} onOpenChange={() => closeDialog()}>
        <DialogContent className="terminal-card border-primary/30 max-w-lg">
          <DialogHeader>
            <DialogTitle className="text-primary flex items-center gap-2">
              {dialogMode === 'quick' ? <Zap className="h-5 w-5" /> : <ShoppingCart className="h-5 w-5" />}
              {dialogMode === 'quick' ? '快速下单' : '加入抢购队列'}
            </DialogTitle>
            <DialogDescription>
              {dialogMode === 'quick' ? '立即购买服务器' : '配置抢购任务并加入队列'}
            </DialogDescription>
          </DialogHeader>
          
          <Tabs defaultValue="config" className="space-y-4">
            <TabsList className="w-full">
              <TabsTrigger value="config" className="flex-1">配置选择</TabsTrigger>
              <TabsTrigger value="options" className="flex-1">附加选项</TabsTrigger>
            </TabsList>

            <TabsContent value="config" className="space-y-4">
              <div className="space-y-2">
                <Label>服务器型号</Label>
                <div className="p-3 bg-muted/50 rounded border border-border">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="font-medium">{selectedServer?.name || selectedServer?.planCode}</p>
                      <p className="text-xs text-muted-foreground font-mono">{selectedServer?.planCode}</p>
                    </div>
                    <p className="text-sm text-accent">€{selectedServer?.price?.toFixed(2) || 'N/A'}/月</p>
                  </div>
                </div>
              </div>
              
              <div className="space-y-2">
                <Label>目标机房</Label>
                <Select value={selectedDc} onValueChange={setSelectedDc}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择机房" />
                  </SelectTrigger>
                  <SelectContent>
                    {selectedServer?.datacenters?.filter((dc: any) => 
                      dc.availability !== "unavailable" && dc.availability !== "unknown"
                    ).map((dc: any) => {
                      const info = getAvailabilityInfo(dc.availability);
                      return (
                        <SelectItem key={dc.datacenter} value={dc.datacenter}>
                          <span className="flex items-center gap-2">
                            <span className="uppercase font-mono">{dc.datacenter}</span>
                            <span className={cn("text-xs px-1.5 py-0.5 rounded", info.color)}>
                              {info.label}
                            </span>
                          </span>
                        </SelectItem>
                      );
                    })}
                  </SelectContent>
                </Select>
              </div>

              {/* Price Preview */}
              <div className="p-4 bg-primary/5 border border-primary/20 rounded-sm">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <DollarSign className="h-4 w-4 text-primary" />
                    <span className="text-sm font-medium">预估价格</span>
                  </div>
                  {isLoadingPrice ? (
                    <Loader2 className="h-4 w-4 animate-spin text-primary" />
                  ) : priceInfo ? (
                    <div className="text-right">
                      <p className="text-lg font-bold text-primary">
                        €{priceInfo.prices?.withTax?.toFixed(2) || 'N/A'}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        不含税: €{priceInfo.prices?.withoutTax?.toFixed(2) || 'N/A'}
                      </p>
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">选择机房后显示</p>
                  )}
                </div>
              </div>
            </TabsContent>

            <TabsContent value="options" className="space-y-4">
              <Label>附加选项</Label>
              {selectedServer?.availableOptions && selectedServer.availableOptions.length > 0 ? (
                <div className="space-y-2 max-h-48 overflow-y-auto">
                  {selectedServer.availableOptions.map((option: any) => (
                    <div 
                      key={option.value}
                      className="flex items-center space-x-2 p-2 border border-border rounded hover:bg-muted/30 cursor-pointer"
                      onClick={() => toggleOption(option.value)}
                    >
                      <Checkbox 
                        checked={selectedOptions.includes(option.value)}
                        onCheckedChange={() => toggleOption(option.value)}
                      />
                      <div className="flex-1">
                        <p className="text-sm">{option.label}</p>
                        <p className="text-xs text-muted-foreground font-mono">{option.value}</p>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-center py-6 text-muted-foreground">
                  <Settings2 className="h-8 w-8 mx-auto mb-2 opacity-30" />
                  <p className="text-sm">此服务器暂无可选附加选项</p>
                </div>
              )}
              
              {selectedOptions.length > 0 && (
                <div className="p-2 bg-muted/30 rounded text-xs">
                  已选: {selectedOptions.length} 个选项
                </div>
              )}
            </TabsContent>
          </Tabs>

          <DialogFooter className="gap-2">
            <DialogClose asChild>
              <Button variant="outline">取消</Button>
            </DialogClose>
            {dialogMode === 'queue' ? (
              <Button onClick={handleAddToQueue} disabled={!selectedDc || isAddingToQueue}>
                {isAddingToQueue ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <ShoppingCart className="h-4 w-4 mr-2" />}
                添加到队列
              </Button>
            ) : (
              <Button onClick={handleQuickOrder} disabled={!selectedDc || isQuickOrdering}>
                {isQuickOrdering ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <Zap className="h-4 w-4 mr-2" />}
                立即下单
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
};

export default ServersPage;