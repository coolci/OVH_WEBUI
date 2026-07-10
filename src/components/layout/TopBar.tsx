import { useState, useEffect } from "react";
import {
  Bell,
  Wifi,
  WifiOff,
  Loader2,
  CheckCircle2,
  AlertTriangle,
  Info,
  XCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { useLocation } from "react-router-dom";
import { useBackendConnection } from "@/hooks/useApi";
import { useRecentLogs } from "@/hooks/use-logs";
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
  "/vps-control": "VPS 控制",
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
    const diff = Date.now() - date.getTime();
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
  const [isNotificationOpen, setIsNotificationOpen] = useState(false);
  const [readNotifications, setReadNotifications] = useState<Set<string>>(new Set());
  const location = useLocation();
  const { isConnected, isChecking } = useBackendConnection();
  // 通知铃铛：只拉最近 20 条，15s 轮询，后台标签页自动停
  const { data: logsData, isLoading, refetch } = useRecentLogs(20, true);

  useEffect(() => {
    const timer = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  const currentPath = pathNames[location.pathname] || "OVH WebUI";
  const notifications = logsData?.logs || [];
  const unreadCount = notifications.filter((n) => n?.id && !readNotifications.has(n.id)).length;

  const handleMarkAllRead = () => {
    setReadNotifications(new Set(notifications.map((n) => n.id).filter(Boolean)));
  };

  return (
    <div className="flex-1 flex items-center justify-between gap-2 min-w-0">
      <div className="flex items-center gap-2 text-sm min-w-0">
        <span className="hidden sm:inline text-[11px] font-mono text-muted-foreground/80">~/</span>
        <span className="text-foreground font-semibold tracking-tight truncate max-w-[46vw] sm:max-w-none">
          {currentPath}
        </span>
      </div>

      <div className="flex items-center gap-1.5 sm:gap-2 flex-shrink-0">
        <div
          className={cn(
            "flex items-center gap-1.5 text-[10px] sm:text-[11px] px-2 sm:px-2.5 py-1 rounded-full border font-medium",
            isChecking
              ? "border-border text-muted-foreground bg-muted/40"
              : isConnected
                ? "border-primary/25 text-primary bg-primary/10"
                : "border-destructive/35 text-destructive bg-destructive/10"
          )}
          title={isConnected ? "后端已连接" : "后端离线"}
        >
          {isChecking ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : isConnected ? (
            <Wifi className="h-3 w-3" />
          ) : (
            <WifiOff className="h-3 w-3" />
          )}
          <span className="hidden sm:inline font-mono">
            {isChecking ? "检测" : isConnected ? "在线" : "离线"}
          </span>
        </div>

        {/* 时间：仅 sm+ */}
        <span className="hidden md:inline text-xs font-mono text-muted-foreground tabular-nums">
          {time.toLocaleTimeString("zh-CN", { hour12: false })}
        </span>

        {/* 通知 */}
        <Popover open={isNotificationOpen} onOpenChange={setIsNotificationOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="relative h-9 w-9 min-h-9 min-w-9 touch-manipulation"
              onClick={() => void refetch()}
            >
              <Bell className="h-4 w-4" />
              {unreadCount > 0 && (
                <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-destructive text-[10px] text-destructive-foreground flex items-center justify-center">
                  {unreadCount > 9 ? "9+" : unreadCount}
                </span>
              )}
            </Button>
          </PopoverTrigger>
          <PopoverContent
            align="end"
            className="w-[min(92vw,360px)] p-0 border-border"
            sideOffset={8}
          >
            <div className="flex items-center justify-between px-3 py-2 border-b border-border">
              <span className="text-sm font-medium">最近日志</span>
              <Button
                variant="ghost"
                size="sm"
                className="h-8 text-xs"
                onClick={handleMarkAllRead}
              >
                全部已读
              </Button>
            </div>
            <ScrollArea className="h-[min(50dvh,320px)]">
              {isLoading ? (
                <div className="flex justify-center py-8">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                </div>
              ) : notifications.length === 0 ? (
                <div className="py-8 text-center text-sm text-muted-foreground">暂无日志</div>
              ) : (
                <div className="divide-y divide-border/50">
                  {notifications.slice(0, 30).map((log) => (
                    <div
                      key={log.id || `${log.timestamp}-${log.message}`}
                      className={cn(
                        "flex gap-2 px-3 py-2.5 text-xs",
                        log.id && !readNotifications.has(log.id) && "bg-primary/5"
                      )}
                    >
                      {getLogIcon(log.level)}
                      <div className="min-w-0 flex-1">
                        <p className="text-foreground break-words leading-snug">{log.message}</p>
                        <p className="text-muted-foreground mt-0.5">
                          {formatTimeAgo(log.timestamp)} · {log.source || "system"}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </ScrollArea>
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}
