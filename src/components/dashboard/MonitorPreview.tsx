import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Activity, ArrowRight, Bell, BellOff, Loader2, Play, Square } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { useSubscriptions, useMonitorStatus } from "@/hooks/useApi";
import { useEffect } from "react";
import api from "@/lib/api";
import { toast } from "sonner";

export function MonitorPreview() {
  const { data: subscriptions, isLoading, refetch } = useSubscriptions();
  const { data: status, refetch: refetchStatus } = useMonitorStatus();

  useEffect(() => {
    const interval = setInterval(() => {
      refetch();
      refetchStatus();
    }, 10000);
    return () => clearInterval(interval);
  }, [refetch, refetchStatus]);

  const displaySubs = subscriptions?.slice(0, 3) || [];

  const toggleMonitor = async () => {
    try {
      if (status?.running) {
        await api.stopMonitor();
        toast.success("监控已停止");
      } else {
        await api.startMonitor();
        toast.success("监控已启动");
      }
      refetchStatus();
    } catch (err: any) {
      toast.error(err.message);
    }
  };

  return (
    <TerminalCard
      title="独服监控"
      icon={<Activity className="h-4 w-4" />}
      headerAction={
        <div className="flex items-center gap-2">
          <Button
            variant={status?.running ? "destructive" : "terminal"}
            size="sm"
            className="text-xs"
            onClick={toggleMonitor}
          >
            {status?.running ? (
              <><Square className="h-3 w-3 mr-1" /> 停止</>
            ) : (
              <><Play className="h-3 w-3 mr-1" /> 启动</>
            )}
          </Button>
          <Link to="/monitor">
            <Button variant="ghost" size="sm" className="text-xs text-accent hover:text-accent">
              管理订阅 <ArrowRight className="ml-1 h-3 w-3" />
            </Button>
          </Link>
        </div>
      }
    >
      <div className="space-y-3">
        {isLoading && !subscriptions ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-primary" />
          </div>
        ) : displaySubs.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <Activity className="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">暂无监控订阅</p>
          </div>
        ) : (
          <>
            {displaySubs.map((sub, index) => (
              <div 
                key={`${sub.planCode}-${index}`}
                className="p-3 bg-muted/30 rounded-sm border border-border/50 hover:border-primary/30 transition-colors"
                style={{ animationDelay: `${index * 100}ms` }}
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{sub.serverName || sub.planCode}</span>
                    {sub.serverName && (
                      <span className="text-xs text-muted-foreground font-mono">({sub.planCode})</span>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {sub.notifyAvailable ? (
                      <Bell className="h-3 w-3 text-primary" />
                    ) : (
                      <BellOff className="h-3 w-3 text-muted-foreground" />
                    )}
                    {sub.autoOrder && (
                      <span className="text-xs px-1.5 py-0.5 bg-accent/20 text-accent rounded-sm">自动下单</span>
                    )}
                  </div>
                </div>
                
                <div className="flex flex-wrap gap-2">
                  {(sub.datacenters?.length > 0 ? sub.datacenters : Object.keys(sub.lastStatus || {})).slice(0, 5).map(dc => {
                    const dcName = typeof dc === 'string' ? dc : dc;
                    const status = sub.lastStatus?.[dcName] || "unknown";
                    return (
                      <div 
                        key={dcName}
                        className="flex items-center gap-1.5 text-xs"
                      >
                        <span className="text-muted-foreground uppercase">{dcName}:</span>
                        <StatusBadge 
                          status={status === "unavailable" || status === "unknown" ? "unavailable" : "available"} 
                          size="sm" 
                          showDot={true}
                        />
                      </div>
                    );
                  })}
                </div>
              </div>
            ))}
            {(subscriptions?.length || 0) > 3 && (
              <p className="text-xs text-muted-foreground text-center pt-2">
                还有 {(subscriptions?.length || 0) - 3} 个订阅...
              </p>
            )}
          </>
        )}
      </div>
    </TerminalCard>
  );
}
