import { Activity, Database, Cpu, HardDrive, Github, Flame } from "lucide-react";
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

      <div className="flex items-center gap-3.5 flex-shrink-0">
        <div className="hidden sm:flex items-center gap-1.5">
          <Cpu className="h-3 w-3 text-muted-foreground/70" />
          <span>目录: {stats?.totalServers ?? 0}</span>
        </div>

        <div className="hidden md:flex items-center gap-1.5">
          <HardDrive className="h-3 w-3 text-muted-foreground/70" />
          <span>可用: {stats?.availableServers ?? 0}</span>
        </div>

        {/* 鸣谢开源作者 cola */}
        <div className="hidden xl:flex items-center gap-2 pl-2.5 border-l border-border/60">
          <span className="text-muted-foreground/60 text-[10px]">致敬开源:</span>
          <a
            href="https://t.me/gocola"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-foreground/85 hover:text-primary transition-colors group"
            title="Telegram 联系 cola (@gocola)"
          >
            <img
              src="/author-avatar.png"
              alt="cola"
              className="w-3.5 h-3.5 rounded-full object-cover ring-1 ring-amber-500/50"
            />
            <span className="font-semibold">cola</span>
            <span className="text-sky-400 font-mono text-[10px]">@gocola</span>
          </a>
          <span className="text-border/60">·</span>
          <a
            href="https://github.com/gokele/ovh"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-muted-foreground hover:text-foreground transition-colors group"
            title="GitHub 项目: gokele/ovh"
          >
            <Github className="w-3 h-3 text-muted-foreground group-hover:text-foreground transition-colors" />
            <span className="underline decoration-border group-hover:decoration-foreground underline-offset-2">
              gokele/ovh
            </span>
          </a>
        </div>

        <div className="flex items-center gap-1.5 pl-2 border-l border-border/60">
          <span className="font-semibold text-foreground/80">OVH</span>
          <span className="font-semibold text-primary">统御</span>
        </div>
      </div>
    </div>
  );
}
