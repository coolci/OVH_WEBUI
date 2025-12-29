import { TerminalCard } from "@/components/ui/terminal-card";
import { Server, ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";

interface ServerInfo {
  planCode: string;
  name: string;
  cpu: string;
  memory: string;
  datacenters: { datacenter: string; availability: string }[];
}

export function ServerAvailability() {
  // Mock data - available servers
  const servers: ServerInfo[] = [
    {
      planCode: "24ska01",
      name: "KS-A",
      cpu: "Intel Xeon E3-1245v5",
      memory: "32GB DDR4",
      datacenters: [
        { datacenter: "gra", availability: "1H" },
        { datacenter: "sbg", availability: "24H" },
      ]
    },
    {
      planCode: "24sk30",
      name: "KS-30",
      cpu: "AMD Ryzen 5 3600",
      memory: "64GB DDR4",
      datacenters: [
        { datacenter: "rbx", availability: "72H" },
      ]
    },
    {
      planCode: "24rise01",
      name: "RISE-1",
      cpu: "AMD Ryzen 5 PRO 4650G",
      memory: "64GB DDR4",
      datacenters: [
        { datacenter: "gra", availability: "1H" },
        { datacenter: "rbx", availability: "1H" },
        { datacenter: "bhs", availability: "24H" },
      ]
    },
  ];

  const getAvailabilityColor = (availability: string) => {
    if (availability === "1H") return "text-primary bg-primary/20";
    if (availability === "24H") return "text-accent bg-accent/20";
    if (availability === "72H") return "text-warning bg-warning/20";
    return "text-muted-foreground bg-muted";
  };

  return (
    <TerminalCard
      title="可用服务器"
      icon={<Server className="h-4 w-4" />}
      headerAction={
        <Link to="/servers">
          <Button variant="ghost" size="sm" className="text-xs text-accent hover:text-accent">
            查看全部 <ArrowRight className="ml-1 h-3 w-3" />
          </Button>
        </Link>
      }
    >
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
            {servers.map((server, index) => (
              <tr 
                key={server.planCode}
                className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                style={{ animationDelay: `${index * 100}ms` }}
              >
                <td className="py-3 px-2">
                  <div>
                    <p className="font-medium text-foreground">{server.name}</p>
                    <p className="text-xs text-muted-foreground font-mono">{server.planCode}</p>
                  </div>
                </td>
                <td className="py-3 px-2 hidden md:table-cell">
                  <div className="text-xs text-muted-foreground">
                    <p>{server.cpu}</p>
                    <p>{server.memory}</p>
                  </div>
                </td>
                <td className="py-3 px-2">
                  <div className="flex flex-wrap gap-1.5">
                    {server.datacenters.map(dc => (
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
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </TerminalCard>
  );
}
