import { TerminalCard } from "@/components/ui/terminal-card";
import { StatusBadge } from "@/components/ui/status-badge";
import { ListOrdered, ArrowRight, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { useQueue } from "@/hooks/useApi";
import { useEffect } from "react";

export function QueuePreview() {
  const { data: queue, isLoading, refetch } = useQueue();

  useEffect(() => {
    const interval = setInterval(refetch, 10000);
    return () => clearInterval(interval);
  }, [refetch]);

  const displayQueue = queue?.slice(0, 5) || [];

  return (
    <TerminalCard
      title="抢购队列"
      icon={<ListOrdered className="h-4 w-4" />}
      headerAction={
        <Link to="/queue">
          <Button variant="ghost" size="sm" className="text-xs text-accent hover:text-accent">
            查看全部 <ArrowRight className="ml-1 h-3 w-3" />
          </Button>
        </Link>
      }
    >
      <div className="space-y-3">
        {isLoading && !queue ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="h-6 w-6 animate-spin text-primary" />
          </div>
        ) : displayQueue.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <ListOrdered className="h-8 w-8 mx-auto mb-2 opacity-50" />
            <p className="text-sm">暂无队列任务</p>
          </div>
        ) : (
          displayQueue.map((item, index) => (
            <div 
              key={item.id}
              className="flex items-center justify-between p-3 bg-muted/30 rounded-sm border border-border/50 hover:border-primary/30 transition-colors"
              style={{ animationDelay: `${index * 100}ms` }}
            >
              <div className="flex items-center gap-3">
                <span className="text-primary font-mono text-sm">#{item.id.slice(0, 6)}</span>
                <div>
                  <p className="font-medium text-sm">{item.planCode}</p>
                  <p className="text-xs text-muted-foreground uppercase">{item.datacenter}</p>
                </div>
              </div>
              
              <div className="flex items-center gap-3">
                <span className="text-xs text-muted-foreground">
                  重试: <span className="text-accent">{item.retryCount}</span>
                </span>
                <StatusBadge status={item.status as any} size="sm" />
              </div>
            </div>
          ))
        )}
      </div>
    </TerminalCard>
  );
}
