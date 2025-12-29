import { useState, useEffect } from "react";
import { Bell, Search, Wifi, WifiOff } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useLocation } from "react-router-dom";

const pathNames: Record<string, string> = {
  "/": "仪表盘",
  "/servers": "服务器列表",
  "/queue": "抢购队列",
  "/history": "购买历史",
  "/monitor": "独服监控",
  "/vps-monitor": "VPS 监控",
  "/server-control": "服务器控制",
  "/account": "账户管理",
  "/settings": "系统设置",
  "/logs": "系统日志",
};

export function TopBar() {
  const [time, setTime] = useState(new Date());
  const [isOnline, setIsOnline] = useState(true);
  const location = useLocation();

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

  const currentPath = pathNames[location.pathname] || "未知页面";

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
        <Button variant="ghost" size="icon" className="relative">
          <Bell className="h-4 w-4" />
          <span className="absolute -top-0.5 -right-0.5 h-2 w-2 bg-accent rounded-full animate-pulse" />
        </Button>

        {/* Time display */}
        <div className="hidden sm:flex items-center gap-2 text-xs text-muted-foreground font-mono">
          <span>{time.toLocaleDateString("zh-CN")}</span>
          <span className="text-primary">{time.toLocaleTimeString("zh-CN", { hour12: false })}</span>
        </div>
      </div>
    </div>
  );
}
