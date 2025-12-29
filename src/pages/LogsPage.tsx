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
  Bug
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Switch } from "@/components/ui/switch";
import { Label } from "@/components/ui/label";

interface LogEntry {
  id: string;
  timestamp: string;
  level: "INFO" | "WARNING" | "ERROR" | "DEBUG";
  message: string;
  source: string;
}

const mockLogs: LogEntry[] = [
  { id: "1", timestamp: "2024-12-29T10:30:45", level: "INFO", message: "服务器可用性检查完成，发现 3 台可用", source: "monitor" },
  { id: "2", timestamp: "2024-12-29T10:30:30", level: "WARNING", message: "队列任务 #q-001 重试次数达到 150 次", source: "queue" },
  { id: "3", timestamp: "2024-12-29T10:30:15", level: "INFO", message: "24ska01 库存变化: sbg 从 unavailable 变为 available", source: "monitor" },
  { id: "4", timestamp: "2024-12-29T10:30:00", level: "ERROR", message: "OVH API 请求失败: 429 Too Many Requests", source: "api" },
  { id: "5", timestamp: "2024-12-29T10:29:45", level: "DEBUG", message: "开始检查订阅 24ska01 的可用性", source: "monitor" },
  { id: "6", timestamp: "2024-12-29T10:29:30", level: "INFO", message: "队列处理器已启动", source: "system" },
  { id: "7", timestamp: "2024-12-29T10:29:15", level: "INFO", message: "成功获取服务器列表，共 156 款", source: "api" },
  { id: "8", timestamp: "2024-12-29T10:29:00", level: "WARNING", message: "Telegram 通知发送超时，将重试", source: "telegram" },
  { id: "9", timestamp: "2024-12-29T10:28:45", level: "ERROR", message: "订单创建失败: 库存不足", source: "order" },
  { id: "10", timestamp: "2024-12-29T10:28:30", level: "INFO", message: "用户登录成功", source: "auth" },
  { id: "11", timestamp: "2024-12-29T10:28:15", level: "DEBUG", message: "缓存已刷新，有效期 30 分钟", source: "cache" },
  { id: "12", timestamp: "2024-12-29T10:28:00", level: "INFO", message: "VPS 监控已启动，监控 3 款 VPS", source: "vps-monitor" },
  { id: "13", timestamp: "2024-12-29T10:27:45", level: "WARNING", message: "API 请求频率接近限制 (80/100)", source: "api" },
  { id: "14", timestamp: "2024-12-29T10:27:30", level: "INFO", message: "Telegram webhook 设置成功", source: "telegram" },
  { id: "15", timestamp: "2024-12-29T10:27:15", level: "ERROR", message: "无法连接到 OVH API: 网络超时", source: "api" },
];

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
  const [logs, setLogs] = useState(mockLogs);
  const [searchTerm, setSearchTerm] = useState("");
  const [levelFilter, setLevelFilter] = useState<string>("all");
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    if (!autoRefresh) return;
    
    const interval = setInterval(() => {
      // Simulate new log
      const newLog: LogEntry = {
        id: `new-${Date.now()}`,
        timestamp: new Date().toISOString(),
        level: ["INFO", "WARNING", "ERROR", "DEBUG"][Math.floor(Math.random() * 4)] as LogEntry["level"],
        message: `自动刷新测试日志 ${Date.now()}`,
        source: "system",
      };
      setLogs(prev => [newLog, ...prev].slice(0, 100));
    }, 5000);

    return () => clearInterval(interval);
  }, [autoRefresh]);

  const handleRefresh = () => {
    setIsRefreshing(true);
    setTimeout(() => setIsRefreshing(false), 1000);
  };

  const handleClear = () => {
    setLogs([]);
  };

  const filteredLogs = logs.filter(log => {
    const matchesSearch = 
      log.message.toLowerCase().includes(searchTerm.toLowerCase()) ||
      log.source.toLowerCase().includes(searchTerm.toLowerCase());
    
    if (levelFilter === "all") return matchesSearch;
    return matchesSearch && log.level === levelFilter;
  });

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString("zh-CN", { hour12: false });
  };

  const formatDate = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleDateString("zh-CN");
  };

  const logCounts = {
    INFO: logs.filter(l => l.level === "INFO").length,
    WARNING: logs.filter(l => l.level === "WARNING").length,
    ERROR: logs.filter(l => l.level === "ERROR").length,
    DEBUG: logs.filter(l => l.level === "DEBUG").length,
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
                共 {logs.length} 条日志
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
                <Button variant="outline" size="sm">
                  <Download className="h-4 w-4 mr-2" />
                  导出
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
            <div className="space-y-1 font-mono text-xs max-h-[600px] overflow-y-auto">
              {filteredLogs.length === 0 ? (
                <div className="text-center py-12 text-muted-foreground">
                  <ScrollText className="h-12 w-12 mx-auto mb-4 opacity-30" />
                  <p>暂无日志记录</p>
                </div>
              ) : (
                filteredLogs.map((log, index) => {
                  const config = levelConfig[log.level];
                  const Icon = config.icon;
                  
                  return (
                    <div 
                      key={log.id}
                      className={cn(
                        "flex items-start gap-2 p-2 rounded-sm border-l-2 transition-colors hover:bg-muted/30",
                        config.borderColor,
                        config.bg
                      )}
                      style={{ animationDelay: `${index * 20}ms` }}
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
                })
              )}
            </div>
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default LogsPage;
