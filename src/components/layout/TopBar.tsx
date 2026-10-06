import { useState, useEffect } from "react";
import {
  Bell,
  WifiOff,
  Loader2,
  CheckCircle2,
  AlertTriangle,
  Info,
  XCircle,
  User,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link, useLocation } from "react-router-dom";
import { useBackendConnection } from "@/hooks/useApi";
import { useRecentLogs } from "@/hooks/use-logs";
import { useAccounts } from "@/hooks/use-accounts";
import { ThemeToggle } from "@/components/theme/ThemeToggle";
import { LanguageToggle } from "@/components/layout/LanguageToggle";
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
  "/history": "抢购历史",
  "/monitor": "服务器监控",
  "/vps-monitor": "VPS 补货",
  "/server-control": "服务器控制",
  "/vps-control": "VPS 控制",
  "/account": "账户管理",
  "/contact-change": "联系人变更",
  "/performance": "性能监控",
  "/telegram-order": "云下单",
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
  const accountsQ = useAccounts();
  const defaultAccount = (accountsQ.data || []).find((a) => a.isDefault) || accountsQ.data?.[0];

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
      <div className="flex min-w-0 items-center gap-2">
        <div className="flex items-center gap-1.5 text-xs">
          <span className="text-muted-foreground/70 font-medium">控制台</span>
          <span className="text-border font-mono">/</span>
          <span className="truncate text-xs sm:text-sm font-semibold tracking-tight text-foreground">
            {currentPath}
          </span>
        </div>
      </div>

      <div className="flex items-center gap-1.5 sm:gap-2 flex-shrink-0">
        {/* 默认账户快捷徽标 */}
        {defaultAccount && (
          <Link
            to="/account"
            className="hidden lg:inline-flex items-center gap-1.5 text-[11px] px-2.5 py-1 rounded-lg border border-border/70 hover:border-primary/40 bg-secondary/35 hover:bg-secondary/70 text-muted-foreground hover:text-foreground transition-all duration-150"
            title={`默认下单账户: ${defaultAccount.name} (${defaultAccount.zone || "OVH"})`}
          >
            <User className="h-3 w-3 text-primary" />
            <span className="font-medium truncate max-w-[120px]">{defaultAccount.name}</span>
          </Link>
        )}

        <div
          className={cn(
            "flex items-center gap-1.5 text-[11px] px-2.5 py-1 rounded-lg border transition-colors font-medium",
            isChecking
              ? "border-border/60 text-muted-foreground bg-secondary/30"
              : isConnected
                ? "border-emerald-500/25 text-emerald-600 dark:text-emerald-400 bg-emerald-500/10"
                : "border-destructive/30 text-destructive bg-destructive/10"
          )}
          title={isConnected ? "后端服务在线" : "后端连接已断开"}
        >
          {isChecking ? (
            <Loader2 className="h-2.5 w-2.5 animate-spin" />
          ) : isConnected ? (
            <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400" />
          ) : (
            <WifiOff className="h-2.5 w-2.5" />
          )}
          <span className="tracking-tight">
            {isChecking ? "检测中" : isConnected ? "在线" : "离线"}
          </span>
        </div>

        {/* 实时时钟 */}
        <div className="hidden md:flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border/60 bg-muted/30 text-[11px] font-mono text-muted-foreground tabular-nums">
          <span className="inline-block w-1.5 h-1.5 rounded-full bg-primary/70 animate-pulse" />
          <span>{time.toLocaleTimeString("zh-CN", { hour12: false })}</span>
        </div>

        {/* 多语言切换 */}
        <LanguageToggle />

        {/* 外观主题切换 */}
        <ThemeToggle />

        {/* 通知 */}
        <Popover open={isNotificationOpen} onOpenChange={setIsNotificationOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="relative h-8 w-8 sm:h-9 sm:w-9 touch-manipulation rounded-lg hover:bg-muted/70 transition-colors"
              onClick={() => void refetch()}
              aria-label="查看通知"
            >
              <Bell className="h-4 w-4 text-foreground/80" />
              {unreadCount > 0 && (
                <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-destructive text-[9.5px] font-bold text-destructive-foreground flex items-center justify-center leading-none">
                  {unreadCount > 9 ? "9+" : unreadCount}
                </span>
              )}
            </Button>
          </PopoverTrigger>
          <PopoverContent
            align="end"
            className="w-[min(92vw,360px)] p-0 border border-border bg-popover shadow-lg rounded-xl"
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
