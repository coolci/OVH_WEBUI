import { StatCard } from "@/components/ui/stat-card";
import { 
  ListOrdered, 
  CheckCircle2, 
  XCircle, 
  Server, 
  Activity,
  Zap,
  Loader2
} from "lucide-react";
import { useStats } from "@/hooks/useApi";
import { useEffect } from "react";

export function StatsOverview() {
  const { data: stats, isLoading, error, refetch } = useStats();

  // 自动刷新每30秒
  useEffect(() => {
    const interval = setInterval(refetch, 30000);
    return () => clearInterval(interval);
  }, [refetch]);

  if (isLoading && !stats) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="h-24 bg-muted/30 rounded border border-border animate-pulse" />
        ))}
      </div>
    );
  }

  // 使用默认值如果API未连接
  const displayStats = stats || {
    activeQueues: 0,
    totalServers: 0,
    availableServers: 0,
    purchaseSuccess: 0,
    purchaseFailed: 0,
    queueProcessorRunning: false,
    monitorRunning: false,
  };

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
      <StatCard
        label="活跃队列"
        value={displayStats.activeQueues}
        icon={<ListOrdered className="h-5 w-5" />}
        variant="accent"
      />
      
      <StatCard
        label="总服务器"
        value={displayStats.totalServers}
        icon={<Server className="h-5 w-5" />}
        variant="default"
      />
      
      <StatCard
        label="可用服务器"
        value={displayStats.availableServers}
        icon={<Zap className="h-5 w-5" />}
        variant="primary"
      />
      
      <StatCard
        label="购买成功"
        value={displayStats.purchaseSuccess}
        icon={<CheckCircle2 className="h-5 w-5" />}
        variant="primary"
      />
      
      <StatCard
        label="购买失败"
        value={displayStats.purchaseFailed}
        icon={<XCircle className="h-5 w-5" />}
        variant="danger"
      />
      
      <StatCard
        label="监控状态"
        value={displayStats.monitorRunning ? "运行中" : "已停止"}
        icon={<Activity className="h-5 w-5" />}
        variant={displayStats.monitorRunning ? "primary" : "warning"}
      />
    </div>
  );
}
