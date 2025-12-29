import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { Activity, ArrowRight, Bell, BellOff } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";

interface Subscription {
  planCode: string;
  serverName: string;
  datacenters: string[];
  notifyAvailable: boolean;
  autoOrder: boolean;
  lastStatus: Record<string, string>;
}

export function MonitorPreview() {
  // Mock data
  const subscriptions: Subscription[] = [
    { 
      planCode: "24ska01", 
      serverName: "KS-A", 
      datacenters: ["gra", "rbx", "sbg"],
      notifyAvailable: true,
      autoOrder: true,
      lastStatus: { gra: "unavailable", rbx: "unavailable", sbg: "available" }
    },
    { 
      planCode: "24sk30", 
      serverName: "KS-30", 
      datacenters: ["gra", "rbx"],
      notifyAvailable: true,
      autoOrder: false,
      lastStatus: { gra: "available", rbx: "unavailable" }
    },
  ];

  return (
    <TerminalCard
      title="独服监控"
      icon={<Activity className="h-4 w-4" />}
      headerAction={
        <Link to="/monitor">
          <Button variant="ghost" size="sm" className="text-xs text-accent hover:text-accent">
            管理订阅 <ArrowRight className="ml-1 h-3 w-3" />
          </Button>
        </Link>
      }
    >
      <div className="space-y-3">
        {subscriptions.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <Activity className="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">暂无监控订阅</p>
          </div>
        ) : (
          subscriptions.map((sub, index) => (
            <div 
              key={sub.planCode}
              className="p-3 bg-muted/30 rounded-sm border border-border/50 hover:border-primary/30 transition-colors"
              style={{ animationDelay: `${index * 100}ms` }}
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm">{sub.serverName}</span>
                  <span className="text-xs text-muted-foreground font-mono">({sub.planCode})</span>
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
                {sub.datacenters.map(dc => {
                  const status = sub.lastStatus[dc] || "unknown";
                  return (
                    <div 
                      key={dc}
                      className="flex items-center gap-1.5 text-xs"
                    >
                      <span className="text-muted-foreground uppercase">{dc}:</span>
                      <StatusBadge 
                        status={status === "available" ? "available" : "unavailable"} 
                        size="sm" 
                        showDot={true}
                      />
                    </div>
                  );
                })}
              </div>
            </div>
          ))
        )}
      </div>
    </TerminalCard>
  );
}
