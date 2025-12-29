import { TerminalCard } from "@/components/ui/terminal-card";
import { Server, ArrowRight, Loader2, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";
import { useServers } from "@/hooks/useApi";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import { toast } from "sonner";

export function ServerAvailability() {
  const { data: servers, isLoading, refetch } = useServers();
  const [isRefreshing, setIsRefreshing] = useState(false);

  useEffect(() => {
    const interval = setInterval(refetch, 60000);
    return () => clearInterval(interval);
  }, [refetch]);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await api.refreshServers();
      await refetch();
      toast.success("服务器列表已刷新");
    } catch (err: any) {
      toast.error(err.message);
    } finally {
      setIsRefreshing(false);
    }
  };

  // 统计可用服务器
  const availableServers = servers?.filter((server) =>
    server.datacenters?.some((dc) => 
      dc.availability !== "unavailable" && dc.availability !== "unknown"
    )
  ) || [];

  const displayServers = availableServers.slice(0, 5);

  const getAvailabilityColor = (availability: string) => {
    if (availability === "1H" || availability === "available") return "text-primary bg-primary/20";
    if (availability === "24H") return "text-accent bg-accent/20";
    if (availability === "72H") return "text-warning bg-warning/20";
    return "text-muted-foreground bg-muted";
  };

  return (
    <TerminalCard
      title="可用服务器"
      icon={<Server className="h-4 w-4" />}
      headerAction={
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            className="text-xs"
            onClick={handleRefresh}
            disabled={isRefreshing}
          >
            <RefreshCw className={cn("h-3 w-3 mr-1", isRefreshing && "animate-spin")} />
            刷新
          </Button>
          <Link to="/servers">
            <Button variant="ghost" size="sm" className="text-xs text-accent hover:text-accent">
              查看全部 <ArrowRight className="ml-1 h-3 w-3" />
            </Button>
          </Link>
        </div>
      }
    >
      {isLoading && !servers ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-6 w-6 animate-spin text-primary" />
        </div>
      ) : displayServers.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          <Server className="h-8 w-8 mx-auto mb-2 opacity-50" />
          <p className="text-sm">暂无可用服务器</p>
          <p className="text-xs mt-1">请先刷新服务器列表</p>
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                <th className="text-left py-2 px-2">型号</th>
                <th className="text-left py-2 px-2 hidden md:table-cell">配置</th>
                <th className="text-left py-2 px-2">可用机房</th>
              </tr>
            </thead>
            <tbody>
              {displayServers.map((server, index) => {
                const availableDcs = server.datacenters?.filter(
                  (dc) => dc.availability !== "unavailable" && dc.availability !== "unknown"
                ) || [];
                
                return (
                  <tr 
                    key={server.planCode}
                    className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                    style={{ animationDelay: `${index * 100}ms` }}
                  >
                    <td className="py-3 px-2">
                      <div>
                        <p className="font-medium text-foreground">{server.name || server.planCode}</p>
                        <p className="text-xs text-muted-foreground font-mono">{server.planCode}</p>
                      </div>
                    </td>
                    <td className="py-3 px-2 hidden md:table-cell">
                      <div className="text-xs text-muted-foreground">
                        <p>{server.cpu || 'N/A'}</p>
                        <p>{server.ram || 'N/A'}</p>
                      </div>
                    </td>
                    <td className="py-3 px-2">
                      <div className="flex flex-wrap gap-1.5">
                        {availableDcs.slice(0, 4).map(dc => (
                          <span
                            key={dc.datacenter}
                            className={cn(
                              "px-2 py-0.5 rounded-sm text-xs font-mono uppercase",
                              getAvailabilityColor(dc.availability)
                            )}
                          >
                            {dc.datacenter} <span className="opacity-70">{dc.availability}</span>
                          </span>
                        ))}
                        {availableDcs.length > 4 && (
                          <span className="text-xs text-muted-foreground">+{availableDcs.length - 4}</span>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {availableServers.length > 5 && (
            <p className="text-xs text-muted-foreground text-center py-2">
              还有 {availableServers.length - 5} 个可用服务器...
            </p>
          )}
        </div>
      )}
    </TerminalCard>
  );
}
