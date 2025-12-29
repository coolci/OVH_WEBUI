import { Activity, Database, Cpu, HardDrive } from "lucide-react";

export function StatusBar() {
  return (
    <div className="h-full flex items-center justify-between px-4 text-xs text-muted-foreground font-mono">
      {/* Left side - System info */}
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-1.5">
          <span className="text-primary">●</span>
          <span>系统正常</span>
        </div>
        
        <div className="hidden sm:flex items-center gap-1.5">
          <Activity className="h-3 w-3 text-primary" />
          <span>监控: 活跃</span>
        </div>
        
        <div className="hidden md:flex items-center gap-1.5">
          <Database className="h-3 w-3 text-accent" />
          <span>队列: 0</span>
        </div>
      </div>

      {/* Right side - Resource info */}
      <div className="flex items-center gap-4">
        <div className="hidden sm:flex items-center gap-1.5">
          <Cpu className="h-3 w-3" />
          <span>API: EU</span>
        </div>
        
        <div className="hidden md:flex items-center gap-1.5">
          <HardDrive className="h-3 w-3" />
          <span>缓存: 有效</span>
        </div>

        <div className="flex items-center gap-1">
          <span className="text-primary">OVH_SNIPER</span>
          <span className="text-muted-foreground">© 2024</span>
        </div>
      </div>
    </div>
  );
}
