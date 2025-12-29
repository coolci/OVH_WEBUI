import { StatCard } from "@/components/ui/stat-card";
import { 
  ListOrdered, 
  CheckCircle2, 
  XCircle, 
  Server, 
  Activity,
  Zap
} from "lucide-react";

interface Stats {
  activeQueues: number;
  totalServers: number;
  availableServers: number;
  purchaseSuccess: number;
  purchaseFailed: number;
  queueProcessorRunning: boolean;
  monitorRunning: boolean;
}

interface StatsOverviewProps {
  stats?: Stats;
}

export function StatsOverview({ stats }: StatsOverviewProps) {
  // Mock data for demo
  const mockStats: Stats = stats || {
    activeQueues: 3,
    totalServers: 156,
    availableServers: 24,
    purchaseSuccess: 47,
    purchaseFailed: 5,
    queueProcessorRunning: true,
    monitorRunning: true,
  };

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4">
      <StatCard
        label="活跃队列"
        value={mockStats.activeQueues}
        icon={<ListOrdered className="h-5 w-5" />}
        variant="accent"
        trend="up"
        trendValue="+2"
      />
      
      <StatCard
        label="总服务器"
        value={mockStats.totalServers}
        icon={<Server className="h-5 w-5" />}
        variant="default"
      />
      
      <StatCard
        label="可用服务器"
        value={mockStats.availableServers}
        icon={<Zap className="h-5 w-5" />}
        variant="primary"
        trend="up"
        trendValue="+8"
      />
      
      <StatCard
        label="购买成功"
        value={mockStats.purchaseSuccess}
        icon={<CheckCircle2 className="h-5 w-5" />}
        variant="primary"
      />
      
      <StatCard
        label="购买失败"
        value={mockStats.purchaseFailed}
        icon={<XCircle className="h-5 w-5" />}
        variant="danger"
      />
      
      <StatCard
        label="监控状态"
        value={mockStats.monitorRunning ? "运行中" : "已停止"}
        icon={<Activity className="h-5 w-5" />}
        variant={mockStats.monitorRunning ? "primary" : "warning"}
      />
    </div>
  );
}
