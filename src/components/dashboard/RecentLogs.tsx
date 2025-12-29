import { TerminalCard } from "@/components/ui/terminal-card";
import { ScrollText, ArrowRight, AlertCircle, Info, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";

interface LogEntry {
  id: string;
  timestamp: string;
  level: "INFO" | "WARNING" | "ERROR" | "DEBUG";
  message: string;
  source: string;
}

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
  // Mock data
  const logs: LogEntry[] = [
    { 
      id: "1", 
      timestamp: "2024-12-29T10:30:45", 
      level: "INFO", 
      message: "服务器可用性检查完成，发现 3 台可用",
      source: "monitor"
    },
    { 
      id: "2", 
      timestamp: "2024-12-29T10:30:30", 
      level: "WARNING", 
      message: "队列任务 #12 重试次数达到 50 次",
      source: "queue"
    },
    { 
      id: "3", 
      timestamp: "2024-12-29T10:30:15", 
      level: "INFO", 
      message: "24ska01 库存变化: gra 从 unavailable 变为 available",
      source: "monitor"
    },
    { 
      id: "4", 
      timestamp: "2024-12-29T10:30:00", 
      level: "ERROR", 
      message: "OVH API 请求失败: 429 Too Many Requests",
      source: "api"
    },
  ];

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString("zh-CN", { hour12: false });
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
      <div className="space-y-2 font-mono text-xs">
        {logs.map((log, index) => {
          const config = levelConfig[log.level];
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
    </TerminalCard>
  );
}
