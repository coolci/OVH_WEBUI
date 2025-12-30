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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
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

const getUniqueDatacenters = (datacenters: Array<{ datacenter: string; availability: string }>) => {
  const map = new Map<string, { datacenter: string; availability: string }>();
  datacenters.forEach((dc) => {
    const existing = map.get(dc.datacenter);
    if (!existing) {
      map.set(dc.datacenter, dc);
      return;
    }
    const currentInfo = getAvailabilityInfo(dc.availability);
    const existingInfo = getAvailabilityInfo(existing.availability);
    if (currentInfo.priority < existingInfo.priority) {
      map.set(dc.datacenter, dc);
    }
  });
  return Array.from(map.values());
};

const getDefaultDatacenter = (datacenters: Array<{ datacenter: string; availability: string }>) => {
  const unique = getUniqueDatacenters(datacenters);
  return unique[0]?.datacenter || "";
};

const formatPrice = (price?: number, currencyCode?: string) => {
  if (typeof price === "number" && Number.isFinite(price)) {
    if (currencyCode && currencyCode !== "EUR") {
      return `${price.toFixed(2)} ${currencyCode}`;
    }
    return `€${price.toFixed(2)}`;
  }
  return "--";
};

const ServersPage = () => {
  const getOptionCategory = (option: string) => {
    const lower = option.toLowerCase();
    if (lower.includes("ram-") || lower.includes("memory")) {
      return "memory";
    }
    if (
      lower.includes("softraid-") ||
      lower.includes("hybrid") ||
      lower.includes("disk") ||
      lower.includes("nvme") ||
      lower.includes("raid")
    ) {
      return "storage";
    }
    return "other";
  };

  const getPlanOptions = (server: any) => {
    const baseOptions = Array.isArray(server?.availableOptions) ? server.availableOptions : [];
    const extraOptions = configOptionsByPlan[server?.planCode] || [];
    const merged = new Map<string, { label: string; value: string }>();
    baseOptions.forEach((opt: any) => {
      if (opt?.value) merged.set(opt.value, opt);
    });
    extraOptions.forEach((opt) => {
      if (opt?.value && !merged.has(opt.value)) merged.set(opt.value, opt);
    });
    return Array.from(merged.values());
  };

  const getDefaultOptionsFromServer = (server: any) => {
    const defaults = Array.isArray(server?.defaultOptions) ? server.defaultOptions : [];
    return defaults
      .map((opt: any) => (typeof opt === "string" ? opt : opt?.value))
      .filter(Boolean);
  };

  const getEffectiveOptionsForServer = (server: any, selected: string[]) => {
    const selectedOptions = selected.filter(Boolean);
    const defaults = getDefaultOptionsFromServer(server);
    const hasMemory = selectedOptions.some((opt) => getOptionCategory(opt) === "memory");
    const hasStorage = selectedOptions.some((opt) => getOptionCategory(opt) === "storage");
    let memoryDefault = "";
    let storageDefault = "";
    defaults.forEach((opt) => {
      const category = getOptionCategory(opt);
      if (category === "memory" && !memoryDefault) memoryDefault = opt;
      if (category === "storage" && !storageDefault) storageDefault = opt;
    });
    const effective = [...selectedOptions];
    if (!hasMemory && memoryDefault) effective.push(memoryDefault);
    if (!hasStorage && storageDefault) effective.push(storageDefault);
    const seen = new Set<string>();
    return effective.filter((opt) => {
      if (seen.has(opt)) return false;
      seen.add(opt);
      return true;
    });
  };

  const updateAvailabilityForPlan = async (planCode: string, options: string[], server?: any) => {
    setLoadingAvailabilityByPlan((prev) => ({ ...prev, [planCode]: true }));
    try {
      const effectiveOptions = getEffectiveOptionsForServer(server, options);
      const availability = await api.getAvailability(planCode, effectiveOptions);
      setAvailabilityByPlan((prev) => ({ ...prev, [planCode]: availability || {} }));
    } catch {
      setAvailabilityByPlan((prev) => ({ ...prev, [planCode]: {} }));
    } finally {
      setLoadingAvailabilityByPlan((prev) => ({ ...prev, [planCode]: false }));
    }
  };

  const loadConfigOptionsForPlan = async (planCode: string) => {
    if (configOptionsByPlan[planCode]) {
      return;
    }
    setIsLoadingOptions(true);
    try {
      const availability = await api.manualCheckDedicated(planCode);
      const optionSet = new Set<string>();
      const configs = availability && typeof availability === "object"
        ? Object.values(availability)
        : [];
      configs.forEach((cfg: any) => {
        if (Array.isArray(cfg?.options)) {
          cfg.options.forEach((opt: string) => optionSet.add(opt));
        }
      });
      const optionsList = Array.from(optionSet)
        .sort()
        .map((opt) => ({ label: opt, value: opt }));
      setConfigOptionsByPlan((prev) => ({ ...prev, [planCode]: optionsList }));
    } finally {
      setIsLoadingOptions(false);
    }
  };
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
  const [isLoadingOptions, setIsLoadingOptions] = useState(false);
  const [optionsByPlan, setOptionsByPlan] = useState<Record<string, string[]>>({});
  const [configOptionsByPlan, setConfigOptionsByPlan] = useState<Record<string, Array<{ label: string; value: string }>>>({});
  const [availabilityByPlan, setAvailabilityByPlan] = useState<Record<string, Record<string, string>>>({});
  const [loadingAvailabilityByPlan, setLoadingAvailabilityByPlan] = useState<Record<string, boolean>>({});
  const [configSheetOpen, setConfigSheetOpen] = useState(false);
  const [configSheetServer, setConfigSheetServer] = useState<any>(null);
  const [configSheetFilter, setConfigSheetFilter] = useState("");

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
  const loadPrice = async (server: any, datacenter: string, options: string[]) => {
    setIsLoadingPrice(true);
    setPriceInfo(null);
    try {
      const effectiveOptions = getEffectiveOptionsForServer(server, options);
      const result = await api.getServerPrice(server.planCode, datacenter, effectiveOptions);
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
  // Load price when selection changes
  useEffect(() => {
    if (selectedServer && selectedDc) {
      loadPrice(selectedServer, selectedDc, selectedOptions);
    }
  }, [selectedServer, selectedDc, selectedOptions]);

  // Load config options when plan changes
  useEffect(() => {
    if (!selectedServer?.planCode) {
      return;
    }
    loadConfigOptionsForPlan(selectedServer.planCode);
  }, [selectedServer?.planCode]);

  // Load config options when plan changes?
  useEffect(() => {
    if (!selectedServer?.planCode) {
      return;
    }
    updateAvailabilityForPlan(selectedServer.planCode, selectedOptions, selectedServer);
  }, [selectedServer?.planCode, selectedOptions]);

  const handleAddToQueue = async () => {
    if (!selectedServer) return;
    const datacenter = selectedDc || getDefaultDatacenter(selectedServer.datacenters || []);
    if (!datacenter) {
      toast.error("请选择机房");
      return;
    }
    const effectiveOptions = getEffectiveOptionsForServer(selectedServer, selectedOptions);
    
    setIsAddingToQueue(true);
    try {
      await api.addQueueItem({
        planCode: selectedServer.planCode,
        datacenter,
        options: effectiveOptions,
        retryInterval: 30,
      });
      toast.success(`已添加 ${selectedServer.planCode} @ ${datacenter.toUpperCase()} 到队列`);
      closeDialog();
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsAddingToQueue(false);
    }
  };

  const handleQuickOrder = async () => {
    if (!selectedServer) {
      return;
    }
    const datacenter = selectedDc || getDefaultDatacenter(selectedServer.datacenters || []);
    if (!datacenter) {
      toast.error("请选择机房");
      return;
    }

    let quickOrderPrice = priceInfo?.prices?.withTax;
    if (!quickOrderPrice) {
      try {
        const effectiveOptions = getEffectiveOptionsForServer(selectedServer, selectedOptions);
        const priceResult = await api.getServerPrice(
          selectedServer.planCode,
          datacenter,
          effectiveOptions
        );
        if (priceResult?.success && priceResult.price) {
          setPriceInfo(priceResult.price);
          quickOrderPrice = priceResult.price.prices?.withTax;
        }
      } catch {
        // ignore price errors, fallback to queue
      }
    }

    if (!quickOrderPrice) {
      setIsAddingToQueue(true);
      try {
        const effectiveOptions = getEffectiveOptionsForServer(selectedServer, selectedOptions);
        await api.addQueueItem({
          planCode: selectedServer.planCode,
          datacenter,
          options: effectiveOptions,
          retryInterval: 30,
        });
        toast.success(`该机房无可定价配置，已加入队列：${selectedServer.planCode} @ ${datacenter.toUpperCase()}`);
        closeDialog();
      } catch (err: any) {
        toast.error(err.message);
      } finally {
        setIsAddingToQueue(false);
      }
      return;
    }
    
    setIsQuickOrdering(true);
    try {
      const result = await api.quickOrder({ 
        planCode: selectedServer.planCode, 
        datacenter,
        options: getEffectiveOptionsForServer(selectedServer, selectedOptions),
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
    const planOptions = optionsByPlan[server.planCode] || [];
    setSelectedOptions(planOptions);
    loadConfigOptionsForPlan(server.planCode);
    updateAvailabilityForPlan(server.planCode, planOptions, server);
    
    if (datacenter) {
      setSelectedDc(datacenter);
    } else {
      setSelectedDc(getDefaultDatacenter(server.datacenters || []));
    }
  };

  const closeDialog = () => {
    setSelectedServer(null);
    setSelectedDc("");
    setSelectedOptions([]);
    setPriceInfo(null);
  };

  const openConfigSheet = (server: any) => {
    setConfigSheetServer(server);
    setConfigSheetFilter("");
    setConfigSheetOpen(true);
    loadConfigOptionsForPlan(server.planCode);
  };

  const closeConfigSheet = () => {
    setConfigSheetOpen(false);
    setConfigSheetServer(null);
    setConfigSheetFilter("");
  };

  const updatePlanOptions = (planCode: string, nextOptions: string[], server?: any) => {
    setOptionsByPlan((byPlan) => ({ ...byPlan, [planCode]: nextOptions }));
    updateAvailabilityForPlan(planCode, nextOptions, server);
  };

  const addOptionForPlan = (server: any, option: string) => {
    const planCode = server.planCode;
    const current = optionsByPlan[planCode] || [];
    if (current.includes(option)) {
      return;
    }
    const category = getOptionCategory(option);
    const next = category === "other"
      ? [...current, option]
      : [...current.filter((opt) => getOptionCategory(opt) !== category), option];
    updatePlanOptions(planCode, next, server);
  };

  const removeOptionForPlan = (server: any, option: string) => {
    const planCode = server.planCode;
    const current = optionsByPlan[planCode] || [];
    updatePlanOptions(planCode, current.filter((opt) => opt !== option), server);
  };

  const toggleOptionForPlan = (server: any, option: string) => {
    const planCode = server.planCode;
    const current = optionsByPlan[planCode] || [];
    if (current.includes(option)) {
      removeOptionForPlan(server, option);
      return;
    }
    addOptionForPlan(server, option);
  };

  const serverList = servers || [];

  const hasAvailableForPlan = (server: any) => {
    const planAvailability = availabilityByPlan[server.planCode];
    if (planAvailability && Object.keys(planAvailability).length > 0) {
      return Object.values(planAvailability).some(
        (status) => status !== "unavailable" && status !== "unknown"
      );
    }
    return server.datacenters?.some((dc: any) =>
      dc.availability !== "unavailable" && dc.availability !== "unknown"
    );
  };
  
  const filteredServers = serverList.filter(server => {
    const matchesSearch = 
      (server.name || "").toLowerCase().includes(searchTerm.toLowerCase()) ||
      server.planCode.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (server.cpu || "").toLowerCase().includes(searchTerm.toLowerCase());
    
    if (filterAvailability === "all") return matchesSearch;
    if (filterAvailability === "available") {
      return matchesSearch && hasAvailableForPlan(server);
    }
    return matchesSearch;
  });

  const availableCount = serverList.filter(hasAvailableForPlan).length;
  const sheetOptions = configSheetServer ? getPlanOptions(configSheetServer) : [];
  const sheetSelectedOptions = configSheetServer
    ? optionsByPlan[configSheetServer.planCode] || []
    : [];
  const sheetDefaultOptions = configSheetServer ? getDefaultOptionsFromServer(configSheetServer) : [];
  const sheetFilterKeyword = configSheetFilter.trim().toLowerCase();
  const sheetFilteredOptions = sheetOptions.filter((opt: any) => {
    if (!sheetFilterKeyword) return true;
    const label = (opt?.label || "").toLowerCase();
    const value = (opt?.value || "").toLowerCase();
    return label.includes(sheetFilterKeyword) || value.includes(sheetFilterKeyword);
  });
  const groupedSheetOptions = sheetFilteredOptions.reduce(
    (acc: Record<string, Array<{ label: string; value: string }>>, opt: any) => {
      const value = opt?.value || "";
      const label = opt?.label || value;
      const category = getOptionCategory(value);
      if (!acc[category]) acc[category] = [];
      acc[category].push({ label, value });
      return acc;
    },
    { memory: [], storage: [], other: [] } as Record<string, Array<{ label: string; value: string }>>
  );

  return (
    <>
      <Helmet>
        <title>服务器列表 | OVH Sniper</title>
        <meta name="description" content="浏览所有OVH服务器型号和实时可用性" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-4 sm:space-y-6">
          {/* Header */}
          <div className="flex flex-col gap-3 sm:gap-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div>
                <h1 className="text-xl sm:text-2xl font-bold text-primary flex items-center gap-2">
                  <span className="text-muted-foreground">&gt;</span>
                  服务器列表
                  <span className="cursor-blink">_</span>
                </h1>
                <p className="text-xs sm:text-sm text-muted-foreground mt-1">
                  共 {serverList.length} 款，{availableCount} 款有货
                </p>
              </div>
              
              <Button 
                variant="terminal" 
                onClick={handleRefresh}
                disabled={isRefreshing}
                size="sm"
                className="w-fit text-xs sm:text-sm"
              >
                <RefreshCw className={cn("h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2", isRefreshing && "animate-spin")} />
                {isRefreshing ? "刷新中" : "刷新"}
              </Button>
            </div>

            {/* Filters */}
            <div className="flex flex-col sm:flex-row gap-2 sm:gap-4">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input 
                  placeholder="搜索型号、配置..." 
                  className="pl-9 h-9 text-sm"
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                />
              </div>
              
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" className="w-full sm:w-auto h-9 text-xs sm:text-sm">
                    <Filter className="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
                    {filterAvailability === "all" ? "全部" : "有货"}
                    <ChevronDown className="h-3 w-3 sm:h-4 sm:w-4 ml-1 sm:ml-2" />
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
          </div>

          {/* Server List */}
          {isLoading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          ) : (
            <div className="space-y-4">
              {filteredServers.map((server) => {
                const hasAvailable = hasAvailableForPlan(server);
                const selectedPlanOptions = optionsByPlan[server.planCode] || [];
                
                return (
                  <TerminalCard 
                    key={server.planCode}
                    className={cn(
                      "transition-all",
                      hasAvailable && "border-primary/20"
                    )}
                  >
                    <div className="flex flex-col gap-3 sm:gap-4">
                      {/* Server Info Header */}
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1.5 sm:gap-2 flex-wrap">
                            <Server className="h-4 w-4 sm:h-5 sm:w-5 text-primary flex-shrink-0" />
                            <h3 className="text-sm sm:text-lg font-bold text-foreground truncate">{server.name || server.planCode}</h3>
                            <span className="text-[10px] sm:text-xs text-muted-foreground font-mono">({server.planCode})</span>
                          </div>
                          <p className="text-xs sm:text-sm text-muted-foreground mt-0.5 sm:mt-1" />
                        </div>
                        
                        {hasAvailable && (
                          <StatusBadge status="available" label="有货" className="flex-shrink-0 text-[10px] sm:text-xs" />
                        )}
                      </div>

                      {/* Specs Grid */}
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs sm:text-sm">
                        <div className="flex items-center gap-1.5 sm:gap-2 p-1.5 sm:p-2 bg-muted/30 rounded-sm">
                          <Cpu className="h-3 w-3 sm:h-4 sm:w-4 text-accent flex-shrink-0" />
                          <div className="min-w-0 flex-1">
                            <p className="text-[10px] sm:text-xs text-muted-foreground">CPU</p>
                            <p className="text-foreground truncate text-[11px] sm:text-sm">{server.cpu || 'N/A'}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-1.5 sm:gap-2 p-1.5 sm:p-2 bg-muted/30 rounded-sm">
                          <MemoryStick className="h-3 w-3 sm:h-4 sm:w-4 text-accent flex-shrink-0" />
                          <div className="min-w-0">
                            <p className="text-[10px] sm:text-xs text-muted-foreground">内存</p>
                            <p className="text-foreground text-[11px] sm:text-sm">{server.ram || 'N/A'}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-1.5 sm:gap-2 p-1.5 sm:p-2 bg-muted/30 rounded-sm">
                          <HardDrive className="h-3 w-3 sm:h-4 sm:w-4 text-accent flex-shrink-0" />
                          <div className="min-w-0">
                            <p className="text-[10px] sm:text-xs text-muted-foreground">存储</p>
                            <p className="text-foreground truncate text-[11px] sm:text-sm">{server.storage || 'N/A'}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-1.5 sm:gap-2 p-1.5 sm:p-2 bg-muted/30 rounded-sm">
                          <Activity className="h-3 w-3 sm:h-4 sm:w-4 text-accent flex-shrink-0" />
                          <div className="min-w-0">
                            <p className="text-[10px] sm:text-xs text-muted-foreground">带宽</p>
                            <p className="text-foreground text-[11px] sm:text-sm">{server.bandwidth || 'N/A'}</p>
                          </div>
                        </div>
                      </div>

                                                {/* Config Options */}
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <p className="text-[10px] sm:text-xs text-muted-foreground uppercase tracking-wider">配置选项</p>
                            <Button
                              variant="outline"
                              size="sm"
                              className="h-6 px-2 text-[10px] sm:text-xs"
                              onClick={() => openConfigSheet(server)}
                            >
                              <Settings2 className="h-3 w-3 mr-1" />
                              配置
                            </Button>
                          </div>
                          {selectedPlanOptions.length > 0 && (
                            <div className="flex flex-wrap items-center gap-1.5">
                              {selectedPlanOptions.slice(0, 4).map((opt) => (
                                <span
                                  key={opt}
                                  className="px-2 py-1 text-[10px] sm:text-xs rounded border border-border/70 bg-muted/50 text-foreground truncate max-w-[160px]"
                                >
                                  {opt}
                                </span>
                              ))}
                              {selectedPlanOptions.length > 4 && (
                                <span className="px-2 py-1 text-[10px] sm:text-xs rounded border border-border/70 bg-muted/50">
                                  +{selectedPlanOptions.length - 4}
                                </span>
                              )}
                            </div>
                          )}
                        </div>

{/* Datacenter Availability */}
                        <div className="space-y-2">
                          <div className="flex items-center justify-between">
                            <p className="text-[10px] sm:text-xs text-muted-foreground uppercase tracking-wider">机房可用性</p>
                            {loadingAvailabilityByPlan[server.planCode] && (
                              <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
                            )}
                          </div>
                          <div className="grid grid-cols-3 sm:grid-cols-4 lg:grid-cols-6 gap-1.5 sm:gap-2">
                            {getUniqueDatacenters(server.datacenters || []).slice(0, 6).map(dc => {
                              const planAvailability = availabilityByPlan[server.planCode] || {};
                              const availability = planAvailability[dc.datacenter] ?? dc.availability;
                              const info = getAvailabilityInfo(availability);
                              return (
                                <div 
                                  key={dc.datacenter}
                                  className={cn(
                                    "flex items-center justify-between p-1.5 sm:p-2 rounded-sm border border-border/50 cursor-pointer hover:border-primary/50 transition-colors",
                                    info.color
                                  )}
                                  onClick={() => openDialog(server, 'quick', dc.datacenter)}
                                >
                                  <p className="font-mono text-[10px] sm:text-xs uppercase">{dc.datacenter}</p>
                                  <span className="font-mono text-[10px] sm:text-xs font-bold">{info.label}</span>
                                </div>
                              );
                            })}
                          </div>
                        
                        {/* Actions */}
                        <div className="flex gap-2 mt-2 sm:mt-3">
                          <Button 
                            size="sm" 
                            variant="outline"
                            className="flex-1 h-8 text-xs"
                            onClick={() => openDialog(server, 'queue')}
                          >
                            <ShoppingCart className="h-3 w-3 sm:h-4 sm:w-4 mr-1" />
                            <span className="hidden xs:inline">加入</span>队列
                          </Button>
                          <Button 
                            size="sm" 
                            className="flex-1 h-8 text-xs"
                            onClick={() => openDialog(server, 'quick')}
                          >
                            <Zap className="h-3 w-3 sm:h-4 sm:w-4 mr-1" />
                            <span className="hidden xs:inline">快速</span>下单
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

      <Sheet open={configSheetOpen} onOpenChange={(open) => (open ? setConfigSheetOpen(true) : closeConfigSheet())}>
        <SheetContent side="right" className="w-full sm:max-w-xl bg-card border-l border-border/60 p-0">
          <div className="flex h-full flex-col">
            <div className="border-b border-border/60 p-4 sm:p-6">
              <SheetHeader className="space-y-2">
                <SheetTitle className="text-primary">配置选项</SheetTitle>
                <SheetDescription>
                  {configSheetServer ? (
                    <span>
                      {configSheetServer.name || configSheetServer.planCode}
                      <span className="ml-2 text-xs font-mono text-muted-foreground">
                        {configSheetServer.planCode}
                      </span>
                    </span>
                  ) : (
                    "选择服务器以配置选项"
                  )}
                </SheetDescription>
              </SheetHeader>
              {configSheetServer && (
                <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
                  <span>已选 {sheetSelectedOptions.length}</span>
                  {sheetSelectedOptions.length > 0 && (
                    <button
                      type="button"
                      className="text-xs text-muted-foreground hover:text-foreground"
                      onClick={() => updatePlanOptions(configSheetServer.planCode, [], configSheetServer)}
                    >
                      清空
                    </button>
                  )}
                </div>
              )}
            </div>
            <div className="flex-1 overflow-y-auto p-4 sm:p-6 space-y-4">
              {configSheetServer ? (
                <>
                  <div className="flex items-center gap-2">
                    <Input
                      placeholder="搜索配置项"
                      className="h-9 text-xs sm:text-sm"
                      value={configSheetFilter}
                      onChange={(e) => setConfigSheetFilter(e.target.value)}
                    />
                    <span className="text-xs text-muted-foreground">
                      可选 {sheetFilteredOptions.length}
                    </span>
                  </div>

                  {isLoadingOptions && sheetOptions.length === 0 ? (
                    <div className="flex items-center justify-center py-6">
                      <Loader2 className="h-5 w-5 animate-spin text-primary" />
                    </div>
                  ) : sheetFilteredOptions.length === 0 ? (
                    <div className="text-center py-6 text-muted-foreground">
                      <Settings2 className="h-8 w-8 mx-auto mb-2 opacity-30" />
                      <p className="text-sm">没有匹配的配置项</p>
                    </div>
                  ) : (
                    <div className="space-y-4">
                      {[
                        { key: "memory", label: "内存", icon: MemoryStick },
                        { key: "storage", label: "存储", icon: HardDrive },
                        { key: "other", label: "其他", icon: Settings2 },
                      ].map((group) => {
                        const options = groupedSheetOptions[group.key] || [];
                        if (options.length === 0) return null;
                        const Icon = group.icon;
                        return (
                          <div key={group.key} className="space-y-2">
                            <div className="flex items-center justify-between">
                              <div className="flex items-center gap-2 text-xs sm:text-sm">
                                <Icon className="h-3.5 w-3.5 text-accent" />
                                <span className="font-medium">{group.label}</span>
                              </div>
                              <span className="text-[10px] sm:text-xs text-muted-foreground">{options.length} 项</span>
                            </div>
                            <div className="grid gap-2 sm:grid-cols-2">
                              {options.map((opt) => {
                                const isSelected = sheetSelectedOptions.includes(opt.value);
                                const isDefault = sheetDefaultOptions.includes(opt.value);
                                return (
                                  <button
                                    key={opt.value}
                                    type="button"
                                    className={cn(
                                      "rounded border px-3 py-2 text-left transition-colors",
                                      isSelected
                                        ? "border-primary/60 bg-primary/10 text-primary"
                                        : "border-border bg-muted/20 hover:bg-muted/40"
                                    )}
                                    onClick={() => toggleOptionForPlan(configSheetServer, opt.value)}
                                  >
                                    <div className="flex items-start justify-between gap-2">
                                      <div>
                                        <p className="text-xs sm:text-sm font-medium">{opt.label || opt.value}</p>
                                        <p className="text-[10px] sm:text-xs text-muted-foreground font-mono">
                                          {opt.value}
                                        </p>
                                      </div>
                                      {isDefault && (
                                        <span className="text-[9px] px-1 rounded bg-muted/70 text-muted-foreground">
                                          默认
                                        </span>
                                      )}
                                    </div>
                                  </button>
                                );
                              })}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </>
              ) : (
                <div className="text-center py-10 text-muted-foreground">
                  <Settings2 className="h-8 w-8 mx-auto mb-2 opacity-30" />
                  <p className="text-sm">请选择服务器以配置选项</p>
                </div>
              )}
            </div>
          </div>
        </SheetContent>
      </Sheet>


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
          
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>服务器型号</Label>
              <div className="p-3 bg-muted/50 rounded border border-border">
                <div className="flex items-center justify-between">
                  <div>
                    <p className="font-medium">{selectedServer?.name || selectedServer?.planCode}</p>
                    <p className="text-xs text-muted-foreground font-mono">{selectedServer?.planCode}</p>
                  </div>
                  {priceInfo?.prices?.withTax && (
                    <p className="text-sm text-accent">
                      {formatPrice(priceInfo.prices?.withTax, priceInfo.prices?.currencyCode)}/月
                    </p>
                  )}
                </div>
              </div>
            </div>

            <div className="space-y-2">
              <Label>附加选项</Label>
              <Select value={selectedDc} onValueChange={setSelectedDc}>
                <SelectTrigger>
                  <SelectValue placeholder="选择机房" />
                </SelectTrigger>
                <SelectContent>
                  {getUniqueDatacenters(selectedServer?.datacenters || []).map((dc: any) => {
                    const planAvailability = availabilityByPlan[selectedServer?.planCode || ""] || {};
                    const availability = planAvailability[dc.datacenter] ?? dc.availability;
                    const info = getAvailabilityInfo(availability);
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

            <div className="space-y-2">
              <Label>附加选项</Label>
              {selectedOptions.length === 0 ? (
                <div className="text-xs text-muted-foreground">
                  当前未选择附加选项，请在服务器卡片的“配置”中设置。
                </div>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {selectedOptions.map((opt) => (
                    <span
                      key={opt}
                      className="px-2 py-1 text-xs rounded border border-border bg-muted/40 truncate max-w-[220px]"
                    >
                      {opt}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>

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
