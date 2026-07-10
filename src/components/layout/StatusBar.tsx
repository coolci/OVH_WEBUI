import { Activity, Database, Cpu, HardDrive } from "lucide-react";
import { useStats, useBackendConnection } from "@/hooks/useApi";

export function StatusBar() {
  const { data: stats } = useStats();
  const { isConnected } = useBackendConnection();

  return (
    <div className="h-full flex items-center justify-between px-5 text-[11px] text-muted-foreground font-mono tracking-tight">
      <div className="flex items-center gap-4 min-w-0">
        <div className="flex items-center gap-1.5">
          <span className={isConnected ? "text-primary" : "text-destructive"}>●</span>
          <span>{isConnected ? "后端在线" : "后端离线"}</span>
        </div>

        <div className="hidden sm:flex items-center gap-1.5">
          <Activity className="h-3 w-3 text-primary" />
          <span>监控: {stats?.monitorRunning ? "运行中" : "停止"}</span>
        </div>

        <div className="hidden md:flex items-center gap-1.5">
          <Database className="h-3 w-3 text-accent" />
          <span>队列: {stats?.activeQueues ?? 0}</span>
        </div>
      </div>

      <div className="flex items-center gap-4 flex-shrink-0">
        <div className="hidden sm:flex items-center gap-1.5">
          <Cpu className="h-3 w-3" />
          <span>目录: {stats?.totalServers ?? 0}</span>
        </div>

        <div className="hidden md:flex items-center gap-1.5">
          <HardDrive className="h-3 w-3" />
          <span>可用: {stats?.availableServers ?? 0}</span>
        </div>

        <div className="flex items-center gap-1">
          <span className="text-primary">OVH_WEBUI</span>
          <span className="text-muted-foreground">v3</span>
        </div>
      </div>
    </div>
  );
}
