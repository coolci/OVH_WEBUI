import { AppLayout } from "@/components/layout/AppLayout";
import { StatsOverview } from "@/components/dashboard/StatsOverview";
import { QueuePreview } from "@/components/dashboard/QueuePreview";
import { MonitorPreview } from "@/components/dashboard/MonitorPreview";
import { RecentLogs } from "@/components/dashboard/RecentLogs";
import { ServerAvailability } from "@/components/dashboard/ServerAvailability";
import { Helmet } from "react-helmet-async";

const Index = () => {
  return (
    <>
      <Helmet>
        <title>仪表盘 | OVH Sniper</title>
        <meta name="description" content="OVH服务器抢购工具 - 实时监控服务器可用性，自动抢购稀缺服务器" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                系统概览
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                实时监控服务器可用性和抢购状态
              </p>
            </div>
            
            <div className="hidden md:flex items-center gap-2 text-xs text-muted-foreground">
              <span className="h-2 w-2 rounded-full bg-primary animate-pulse" />
              系统运行中
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
