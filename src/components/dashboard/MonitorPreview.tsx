import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Activity, ArrowRight, Bell, BellOff, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { useSubscriptions, useMonitorStatus } from "@/hooks/useApi";
import { useEffect } from "react";

const statusPriority: Record<string, number> = {
  available: 3,
  price_check_failed: 2,
  unavailable: 1,
  unknown: 0,
};

const normalizeStatus = (status: string) => {
  if (status === "price_check_failed") return "warning";
  if (status === "available") return "available";
  if (status === "unavailable" || status === "unknown") return "unavailable";
  return "unknown";
};

const collectDatacenterStatuses = (lastStatus?: Record<string, string>) => {
  const dcStatus: Record<string, string> = {};
  if (!lastStatus) return dcStatus;

  Object.entries(lastStatus).forEach(([key, status]) => {
    const dc = key.split("|")[0];
    const current = dcStatus[dc];
    if (!current || (statusPriority[status] ?? 0) > (statusPriority[current] ?? 0)) {
      dcStatus[dc] = status;
    }
  });

  return dcStatus;
};

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

  return (
    <TerminalCard
      title="独服监控"
      icon={<Activity className="h-4 w-4" />}
      headerAction={
        <div className="flex items-center gap-2">
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
                  {(() => {
                    const dcStatus = collectDatacenterStatuses(sub.lastStatus);
                    const dcs = sub.datacenters?.length
                      ? sub.datacenters
                      : Object.keys(dcStatus);
                    return Array.from(new Set(dcs)).slice(0, 5).map((dcName) => {
                      const status = dcStatus[dcName] || "unknown";
                      return (
                        <div 
                          key={dcName}
                          className="flex items-center gap-1.5 text-xs"
                        >
                          <span className="text-muted-foreground uppercase">{dcName}:</span>
                          <StatusBadge 
                            status={normalizeStatus(status)} 
                            size="sm" 
                            showDot={true}
                          />
                        </div>
                      );
                    });
                  })()}
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
