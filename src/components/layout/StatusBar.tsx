import { Activity, Database, Cpu, HardDrive } from "lucide-react";
import { useStats, useBackendConnection } from "@/hooks/useApi";
import { cn } from "@/lib/utils";

export function StatusBar() {
  const { data: stats } = useStats();
  const { isConnected } = useBackendConnection();

  return (
    <div className="h-full flex items-center justify-between px-5 text-[11px] text-muted-foreground/80 font-mono tracking-tight">
      <div className="flex items-center gap-4 min-w-0">
        <div className="flex items-center gap-1.5 font-medium">
          <span
            className={cn(
              "h-1.5 w-1.5 rounded-full",
              isConnected ? "bg-primary" : "bg-destructive"
            )}
          />
          <span className={isConnected ? "text-foreground/90" : "text-destructive"}>
            {isConnected ? "后端在线" : "后端离线"}
          </span>
        </div>

        <span className="hidden sm:inline text-border/60">/</span>

        <div className="hidden sm:flex items-center gap-1.5">
          <Activity className="h-3 w-3 text-primary/80" />
          <span>监控: {stats?.monitorRunning ? "运行中" : "停止"}</span>
        </div>

        <span className="hidden md:inline text-border/60">/</span>

        <div className="hidden md:flex items-center gap-1.5">
          <Database className="h-3 w-3 text-accent/80" />
          <span>队列: {stats?.activeQueues ?? 0}</span>
        </div>
      </div>

      <div className="flex items-center gap-4 flex-shrink-0">
        <div className="hidden sm:flex items-center gap-1.5">
          <Cpu className="h-3 w-3 text-muted-foreground/70" />
          <span>目录: {stats?.totalServers ?? 0}</span>
        </div>

        <div className="hidden md:flex items-center gap-1.5">
          <HardDrive className="h-3 w-3 text-muted-foreground/70" />
          <span>可用: {stats?.availableServers ?? 0}</span>
        </div>

        <div className="flex items-center gap-1.5 pl-2 border-l border-white/[0.06]">
          <span className="font-semibold text-foreground/80">OVH</span>
          <span className="font-semibold text-primary">统御</span>
        </div>
      </div>
    </div>
  );
}
