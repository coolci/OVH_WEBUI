import { useState, useEffect } from "react";
import { Bell, Search, Wifi, WifiOff, X, Loader2, Trash2, CheckCircle2, AlertTriangle, Info, XCircle } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useLocation } from "react-router-dom";
import { useLogs } from "@/hooks/useApi";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";

const pathNames: Record<string, string> = {
  "/": "仪表盘",
  "/servers": "服务器列表",
  "/queue": "抢购队列",
  "/history": "购买历史",
  "/monitor": "独服监控",
  "/vps-monitor": "VPS 监控",
  "/server-control": "服务器控制",
  "/account": "账户管理",
  "/contact-change": "联系人变更",
  "/performance": "性能监控",
  "/telegram-order": "Telegram 下单",
  "/settings": "系统设置",
  "/logs": "系统日志",
};

const getLogIcon = (level: string) => {
  switch (level?.toLowerCase()) {
    case "error":
      return <XCircle className="h-4 w-4 text-destructive flex-shrink-0" />;
    case "warning":
    case "warn":
      return <AlertTriangle className="h-4 w-4 text-warning flex-shrink-0" />;
    case "success":
      return <CheckCircle2 className="h-4 w-4 text-primary flex-shrink-0" />;
    default:
      return <Info className="h-4 w-4 text-accent flex-shrink-0" />;
  }
};

const formatTimeAgo = (timestamp: string) => {
  try {
    const date = new Date(timestamp);
    const now = Date.now();
    const diff = now - date.getTime();
    
    if (diff < 60000) return "刚刚";
    if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
    return `${Math.floor(diff / 86400000)}天前`;
  } catch {
    return "未知";
  }
};

export function TopBar() {
  const [time, setTime] = useState(new Date());
  const [isOnline, setIsOnline] = useState(true);
  const [isNotificationOpen, setIsNotificationOpen] = useState(false);
  const [readNotifications, setReadNotifications] = useState<Set<string>>(new Set());
  const location = useLocation();
  
  const { data: logs, isLoading, refetch } = useLogs(20);

  useEffect(() => {
    const timer = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    const handleOnline = () => setIsOnline(true);
    const handleOffline = () => setIsOnline(false);
    
    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  // 每30秒刷新日志
  useEffect(() => {
    const interval = setInterval(refetch, 30000);
    return () => clearInterval(interval);
  }, [refetch]);

  const currentPath = pathNames[location.pathname] || "未知页面";
  
  const notifications = logs || [];
  const unreadCount = notifications.filter(n => !readNotifications.has(n.id)).length;
  
  const handleMarkAllRead = () => {
    const allIds = new Set(notifications.map(n => n.id));
    setReadNotifications(allIds);
  };

  const handleOpenChange = (open: boolean) => {
    setIsNotificationOpen(open);
    if (open) {
      refetch();
    }
  };

  return (
    <div className="flex-1 flex items-center justify-between gap-4">
      {/* Breadcrumb / Current path */}
      <div className="flex items-center gap-2 text-sm">
        <span className="text-primary">~</span>
        <span className="text-muted-foreground">/</span>
        <span className="text-foreground">{currentPath}</span>
        <span className="text-primary cursor-blink">▌</span>
      </div>

      {/* Search (hidden on mobile) */}
      <div className="hidden md:flex flex-1 max-w-sm">
        <div className="relative w-full">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input 
            placeholder="搜索服务器、型号..." 
            className="pl-9 bg-muted/50 border-border focus:border-primary focus:ring-1 focus:ring-primary/30"
          />
        </div>
      </div>

      {/* Right side indicators */}
      <div className="flex items-center gap-3">
        {/* Connection status */}
        <div className="flex items-center gap-1.5">
          {isOnline ? (
            <>
              <Wifi className="h-4 w-4 status-online" />
              <span className="hidden sm:inline text-xs text-muted-foreground">在线</span>
            </>
          ) : (
            <>
              <WifiOff className="h-4 w-4 status-offline" />
              <span className="hidden sm:inline text-xs text-destructive">离线</span>
            </>
          )}
        </div>

        {/* Notifications */}
        <Popover open={isNotificationOpen} onOpenChange={handleOpenChange}>
          <PopoverTrigger asChild>
            <Button variant="ghost" size="icon" className="relative">
              <Bell className="h-4 w-4" />
              {unreadCount > 0 && (
                <span className="absolute -top-0.5 -right-0.5 h-4 w-4 bg-accent rounded-full text-[10px] font-bold flex items-center justify-center text-background animate-pulse">
                  {unreadCount > 9 ? "9+" : unreadCount}
                </span>
              )}
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-80 p-0 terminal-card border-primary/30" align="end">
            <div className="flex items-center justify-between p-3 border-b border-border">
              <h4 className="font-semibold text-sm text-primary">系统通知</h4>
              <div className="flex items-center gap-1">
                {unreadCount > 0 && (
                  <Button 
                    variant="ghost" 
                    size="sm" 
                    className="h-7 text-xs"
                    onClick={handleMarkAllRead}
                  >
                    全部已读
                  </Button>
                )}
                <Button 
                  variant="ghost" 
                  size="icon" 
                  className="h-7 w-7"
                  onClick={() => refetch()}
                >
                  <Loader2 className={cn("h-3 w-3", isLoading && "animate-spin")} />
                </Button>
              </div>
            </div>
            
            <ScrollArea className="h-80">
              {isLoading && notifications.length === 0 ? (
                <div className="flex items-center justify-center py-8">
                  <Loader2 className="h-6 w-6 animate-spin text-primary" />
                </div>
              ) : notifications.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
                  <Bell className="h-8 w-8 mb-2 opacity-30" />
                  <p className="text-sm">暂无通知</p>
                </div>
              ) : (
                <div className="divide-y divide-border">
                  {notifications.map((notification) => {
                    const isRead = readNotifications.has(notification.id);
                    return (
                      <div 
                        key={notification.id}
                        className={cn(
                          "p-3 hover:bg-muted/50 transition-colors cursor-pointer",
                          !isRead && "bg-primary/5"
                        )}
                        onClick={() => {
                          setReadNotifications(prev => new Set([...prev, notification.id]));
                        }}
                      >
                        <div className="flex gap-3">
                          {getLogIcon(notification.level)}
                          <div className="flex-1 min-w-0">
                            <p className={cn(
                              "text-sm line-clamp-2",
                              !isRead && "font-medium"
                            )}>
                              {notification.message}
                            </p>
                            <div className="flex items-center gap-2 mt-1">
                              <span className="text-xs text-muted-foreground">
                                {notification.source}
                              </span>
                              <span className="text-xs text-muted-foreground">•</span>
                              <span className="text-xs text-muted-foreground">
                                {formatTimeAgo(notification.timestamp)}
                              </span>
                            </div>
                          </div>
                          {!isRead && (
                            <span className="h-2 w-2 bg-accent rounded-full flex-shrink-0 mt-1" />
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </ScrollArea>
            
            <div className="p-2 border-t border-border">
              <Button 
                variant="ghost" 
                className="w-full text-xs text-muted-foreground hover:text-foreground"
                onClick={() => {
                  setIsNotificationOpen(false);
                  window.location.href = "/logs";
                }}
              >
                查看全部日志
              </Button>
            </div>
          </PopoverContent>
        </Popover>

        {/* Time display */}
        <div className="hidden sm:flex items-center gap-2 text-xs text-muted-foreground font-mono">
          <span>{time.toLocaleDateString("zh-CN")}</span>
          <span className="text-primary">{time.toLocaleTimeString("zh-CN", { hour12: false })}</span>
        </div>
      </div>
    </div>
  );
}
