import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import { Activity, Loader2, Server } from "lucide-react";
import { useMyServers } from "@/hooks/useApi";

const PerformancePage = () => {
  const { data: serversData, isLoading: isLoadingServers } = useMyServers();
  const servers = serversData?.servers || [];

  return (
    <>
      <Helmet>
        <title>性能监控 | OVH Sniper</title>
        <meta name="description" content="性能监控功能已取消" />
      </Helmet>

      <AppLayout>
        <div className="space-y-6">
          <div>
            <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
              <span className="text-muted-foreground">&gt;</span>
              性能监控
              <span className="cursor-blink">_</span>
            </h1>
            <p className="text-sm text-muted-foreground mt-1">
              性能监控功能已取消
            </p>
          </div>

          {isLoadingServers ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
            </div>
          ) : servers.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Server className="h-12 w-12 mx-auto mb-4 opacity-30" />
              <p>暂无可监控的服务器</p>
            </div>
          ) : (
            <div className="text-center py-12 text-muted-foreground">
              <Activity className="h-8 w-8 mx-auto mb-2 opacity-30" />
              <p>性能监控卡片已移除</p>
            </div>
          )}
        </div>
      </AppLayout>
    </>
  );
};

export default PerformancePage;
