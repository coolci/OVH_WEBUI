import { TerminalCard } from "@/components/ui/terminal-card";
import { ScrollText, ArrowRight, AlertCircle, Info, AlertTriangle, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";
import { useLogs } from "@/hooks/useApi";
import { useEffect } from "react";

const levelConfig = {
  INFO: {
    icon: Info,
    color: "text-primary",
    bg: "bg-primary/10",
  },
  WARNING: {
    icon: AlertTriangle,
    color: "text-warning",
    bg: "bg-warning/10",
  },
  ERROR: {
    icon: AlertCircle,
    color: "text-destructive",
    bg: "bg-destructive/10",
  },
  DEBUG: {
    icon: Info,
    color: "text-muted-foreground",
    bg: "bg-muted",
  },
};

export function RecentLogs() {
  const { data: logs, isLoading, refetch } = useLogs(10);

  useEffect(() => {
    const interval = setInterval(refetch, 5000);
    return () => clearInterval(interval);
  }, [refetch]);

  const displayLogs = logs?.slice(-10).reverse() || [];

  const formatTime = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleTimeString("zh-CN", { hour12: false });
    } catch {
      return "--:--:--";
    }
  };

  return (
    <TerminalCard
      title="系统日志"
      icon={<ScrollText className="h-4 w-4" />}
      headerAction={
        <Link to="/logs">
          <Button variant="ghost" size="sm" className="text-xs text-accent hover:text-accent">
            查看全部 <ArrowRight className="ml-1 h-3 w-3" />
          </Button>
        </Link>
      }
    >
      {isLoading && !logs ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : displayLogs.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          <ScrollText className="h-8 w-8 mx-auto mb-2 opacity-50" />
          <p className="text-sm">暂无日志</p>
        </div>
      ) : (
        <div className="space-y-2 font-mono text-xs">
          {displayLogs.map((log, index) => {
            const config = levelConfig[log.level as keyof typeof levelConfig] || levelConfig.DEBUG;
            const Icon = config.icon;
            
            return (
              <div 
                key={log.id}
                className={cn(
                  "flex items-start gap-2 p-2 rounded-sm",
                  config.bg
                )}
                style={{ animationDelay: `${index * 50}ms` }}
              >
                <Icon className={cn("h-3.5 w-3.5 mt-0.5 flex-shrink-0", config.color)} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="text-muted-foreground">{formatTime(log.timestamp)}</span>
                    <span className={cn("uppercase font-medium", config.color)}>[{log.level}]</span>
                    <span className="text-accent">[{log.source}]</span>
                  </div>
                  <p className="text-foreground/90 break-words">{log.message}</p>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </TerminalCard>
  );
}
