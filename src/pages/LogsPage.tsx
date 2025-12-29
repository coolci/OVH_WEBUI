import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { 
  ScrollText, 
  Search,
  Trash2,
  RefreshCw,
  Download,
  Filter,
  ChevronDown,
  AlertCircle,
  AlertTriangle,
  Info,
  Bug,
  Loader2
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import api from "@/lib/api";
import { useLogs } from "@/hooks/useApi";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";

const levelConfig = {
  INFO: {
    icon: Info,
    color: "text-primary",
    bg: "bg-primary/10",
    borderColor: "border-primary/30",
  },
  WARNING: {
    icon: AlertTriangle,
    color: "text-warning",
    bg: "bg-warning/10",
    borderColor: "border-warning/30",
  },
  ERROR: {
    icon: AlertCircle,
    color: "text-destructive",
    bg: "bg-destructive/10",
    borderColor: "border-destructive/30",
  },
  DEBUG: {
    icon: Bug,
    color: "text-muted-foreground",
    bg: "bg-muted",
    borderColor: "border-border",
  },
};

const LogsPage = () => {
  const { data: logs, isLoading, refetch } = useLogs();
  const [searchTerm, setSearchTerm] = useState("");
  const [levelFilter, setLevelFilter] = useState<string>("all");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(refetch, 5000);
    return () => clearInterval(interval);
  }, [autoRefresh, refetch]);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await refetch();
    setIsRefreshing(false);
  };

  const handleFlush = async () => {
    try {
      await api.flushLogs();
      toast.success("日志已刷新到磁盘");
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const handleClear = async () => {
    try {
      await api.clearLogs();
      toast.success("日志已清空");
      refetch();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  const logList = logs || [];

  const filteredLogs = logList.filter(log => {
    const matchesSearch = 
      log.message.toLowerCase().includes(searchTerm.toLowerCase()) ||
      log.source.toLowerCase().includes(searchTerm.toLowerCase());
    
    if (levelFilter === "all") return matchesSearch;
    return matchesSearch && log.level.toUpperCase() === levelFilter;
  }).reverse();

  const formatTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleTimeString("zh-CN", { hour12: false });
    } catch {
      return "--:--:--";
    }
  };

  const logCounts = {
    INFO: logList.filter(l => l.level.toUpperCase() === "INFO").length,
    WARNING: logList.filter(l => l.level.toUpperCase() === "WARNING").length,
    ERROR: logList.filter(l => l.level.toUpperCase() === "ERROR").length,
    DEBUG: logList.filter(l => l.level.toUpperCase() === "DEBUG").length,
  };

  return (
    <>
      <Helmet>
        <title>系统日志 | OVH Sniper</title>
        <meta name="description" content="查看系统运行日志" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                系统日志
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                共 {logList.length} 条日志
              </p>
            </div>
            
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                <Switch 
                  id="auto-refresh" 
                  checked={autoRefresh}
                  onCheckedChange={setAutoRefresh}
                />
                <Label htmlFor="auto-refresh" className="text-sm">自动刷新</Label>
              </div>
              
              <div className="flex gap-2">
                <Button variant="outline" size="sm" onClick={handleRefresh} disabled={isRefreshing}>
                  <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
                  刷新
                </Button>
                <Button variant="outline" size="sm" onClick={handleFlush}>
                  <Download className="h-4 w-4 mr-2" />
                  写入磁盘
                </Button>
                <Button variant="destructive" size="sm" onClick={handleClear}>
                  <Trash2 className="h-4 w-4 mr-2" />
                  清空
                </Button>
              </div>
            </div>
          </div>

          {/* Stats */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {(Object.keys(levelConfig) as Array<keyof typeof levelConfig>).map(level => {
              const config = levelConfig[level];
              const Icon = config.icon;
              return (
                <div 
                  key={level}
                  className={cn(
                    "terminal-card p-4 cursor-pointer transition-all",
                    levelFilter === level && config.borderColor
                  )}
                  onClick={() => setLevelFilter(levelFilter === level ? "all" : level)}
                >
                  <div className={cn("flex items-center gap-2 mb-1", config.color)}>
                    <Icon className="h-4 w-4" />
                    <span className="text-xs uppercase">{level}</span>
                  </div>
                  <p className={cn("text-2xl font-bold", config.color)}>
                    {logCounts[level]}
                  </p>
                </div>
              );
            })}
          </div>

          {/* Filters */}
          <div className="flex flex-col sm:flex-row gap-4">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input 
                placeholder="搜索日志内容..." 
                className="pl-9"
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
            </div>
            
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" className="w-full sm:w-auto">
                  <Filter className="h-4 w-4 mr-2" />
                  {levelFilter === "all" ? "全部级别" : levelFilter}
                  <ChevronDown className="h-4 w-4 ml-2" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem onClick={() => setLevelFilter("all")}>
                  全部级别
                </DropdownMenuItem>
                {Object.keys(levelConfig).map(level => (
                  <DropdownMenuItem key={level} onClick={() => setLevelFilter(level)}>
                    {level}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          {/* Logs */}
          <TerminalCard
            title="日志输出"
            icon={<ScrollText className="h-4 w-4" />}
          >
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : filteredLogs.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <ScrollText className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无日志记录</p>
              </div>
            ) : (
              <div className="space-y-1 font-mono text-xs max-h-[600px] overflow-y-auto">
                {filteredLogs.map((log, index) => {
                  const config = levelConfig[log.level.toUpperCase() as keyof typeof levelConfig] || levelConfig.DEBUG;
                  const Icon = config.icon;
                  
                  return (
                    <div 
                      key={log.id}
                      className={cn(
                        "flex items-start gap-2 p-2 rounded-sm border-l-2 transition-colors hover:bg-muted/30",
                        config.borderColor,
                        config.bg
                      )}
                    >
                      <Icon className={cn("h-3.5 w-3.5 mt-0.5 flex-shrink-0", config.color)} />
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-muted-foreground">{formatTime(log.timestamp)}</span>
                          <span className={cn("uppercase font-bold", config.color)}>[{log.level}]</span>
                          <span className="text-accent">[{log.source}]</span>
                        </div>
                        <p className="text-foreground/90 break-words mt-0.5">{log.message}</p>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default LogsPage;
