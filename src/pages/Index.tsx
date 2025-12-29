import { AppLayout } from "@/components/layout/AppLayout";
import { StatsOverview } from "@/components/dashboard/StatsOverview";
import { QueuePreview } from "@/components/dashboard/QueuePreview";
import { MonitorPreview } from "@/components/dashboard/MonitorPreview";
import { RecentLogs } from "@/components/dashboard/RecentLogs";
import { ServerAvailability } from "@/components/dashboard/ServerAvailability";
import { StatusBadge } from "@/components/ui/status-badge";
import { Helmet } from "react-helmet-async";
import { useBackendConnection } from "@/hooks/useApi";

const Index = () => {
  const { isConnected, isChecking } = useBackendConnection();

  return (
    <>
      <Helmet>
        <title>仪表盘 | OVH Sniper</title>
        <meta name="description" content="OVH服务器抢购工具 - 实时监控服务器可用性，自动抢购稀缺服务器" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-4 sm:space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 sm:gap-4">
            <div>
              <h1 className="text-xl sm:text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                系统概览
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-xs sm:text-sm text-muted-foreground mt-1">
                实时监控服务器可用性和抢购状态
              </p>
            </div>
            
            <div className="flex items-center gap-2 text-xs">
              <span className="text-muted-foreground">后端:</span>
              {isChecking ? (
                <StatusBadge status="processing" label="检测中" size="sm" showDot />
              ) : (
                <StatusBadge 
                  status={isConnected ? "connected" : "disconnected"} 
                  size="sm" 
                  showDot 
                />
              )}
            </div>
          </div>

          {/* Stats Overview */}
          <StatsOverview />

          {/* Main Grid */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Left Column */}
            <div className="space-y-6">
              <QueuePreview />
              <MonitorPreview />
            </div>

            {/* Right Column */}
            <div className="space-y-6">
              <ServerAvailability />
              <RecentLogs />
            </div>
          </div>
        </div>
      </AppLayout>
    </>
  );
};

export default Index;
