import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { 
  Activity, 
  Cpu, 
  Network, 
  RefreshCw,
  Loader2,
  Server
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { useMyServers } from "@/hooks/useApi";
import { api } from "@/lib/api";
import { toast } from "sonner";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend
} from "recharts";

interface StatPoint {
  timestamp: number;
  value: number;
}

interface Statistics {
  download?: StatPoint[];
  upload?: StatPoint[];
}

const PerformancePage = () => {
  const { data: serversData, isLoading: isLoadingServers } = useMyServers();
  const [selectedServer, setSelectedServer] = useState<string>("");
  const [period, setPeriod] = useState<'lastday' | 'lastweek' | 'lastmonth' | 'lastyear'>('lastday');
  const [statistics, setStatistics] = useState<Statistics | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const servers = serversData?.servers || [];

  const loadStatistics = async () => {
    if (!selectedServer) return;
    
    setIsLoading(true);
    try {
      const [downloadRes, uploadRes] = await Promise.all([
        api.getServerStatistics(selectedServer, period, "traffic:download").catch((error: any) => ({
          success: false,
          error: error?.message || String(error),
        })),
        api.getServerStatistics(selectedServer, period, "traffic:upload").catch((error: any) => ({
          success: false,
          error: error?.message || String(error),
        })),
      ]);

      if (!downloadRes.success || !uploadRes.success) {
        toast.error(downloadRes.error || uploadRes.error || "加载统计数据失败");
      }

      setStatistics({
        download: downloadRes.statistics || [],
        upload: uploadRes.statistics || [],
      });
    } catch (error: any) {
      toast.error(`加载失败: ${error.message}`);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (selectedServer) {
      loadStatistics();
    }
  }, [selectedServer, period]);

  useEffect(() => {
    if (!autoRefresh || !selectedServer) return;
    
    const interval = setInterval(loadStatistics, 30000);
    return () => clearInterval(interval);
  }, [autoRefresh, selectedServer, period]);

  // Auto select first server
  useEffect(() => {
    if (servers.length > 0 && !selectedServer) {
      setSelectedServer(servers[0].serviceName);
    }
  }, [servers, selectedServer]);

  const formatNetworkData = () => {
    if (!statistics?.download && !statistics?.upload) return [];
    
    const txData = statistics.upload || [];
    const rxData = statistics.download || [];
    
    const combined: { time: string; tx: number; rx: number }[] = [];
    const maxLen = Math.max(txData.length, rxData.length);
    
    for (let i = 0; i < maxLen; i++) {
      const tx = txData[i];
      const rx = rxData[i];
      combined.push({
        time: new Date((tx?.timestamp || rx?.timestamp) * 1000).toLocaleTimeString("zh-CN", {
          hour: '2-digit',
          minute: '2-digit'
        }),
        tx: tx?.value || 0,
        rx: rx?.value || 0
      });
    }
    
    return combined;
  };

  const networkData = formatNetworkData();

  return (
    <>
      <Helmet>
        <title>性能监控 | OVH Sniper</title>
        <meta name="description" content="实时监控服务器性能" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                性能监控
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                实时监控服务器资源使用情况
              </p>
            </div>
            
            <div className="flex gap-2 items-center">
              <Select value={selectedServer} onValueChange={setSelectedServer}>
                <SelectTrigger className="w-48">
                  <SelectValue placeholder="选择服务器" />
                </SelectTrigger>
                <SelectContent>
                  {servers.map(server => (
                    <SelectItem key={server.serviceName} value={server.serviceName}>
                      {server.name || server.serviceName}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              
              <Select value={period} onValueChange={(v) => setPeriod(v as any)}>
                <SelectTrigger className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="lastday">最近1天</SelectItem>
                  <SelectItem value="lastweek">最近7天</SelectItem>
                  <SelectItem value="lastmonth">最近30天</SelectItem>
                  <SelectItem value="lastyear">最近1年</SelectItem>
                </SelectContent>
              </Select>

              <Button 
                variant="terminal" 
                onClick={loadStatistics} 
                disabled={isLoading || !selectedServer}
              >
                <RefreshCw className={cn("h-4 w-4 mr-2", isLoading && "animate-spin")} />
                刷新
              </Button>
            </div>
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
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <TerminalCard
                title="CPU / 内存"
                icon={<Cpu className="h-4 w-4" />}
              >
                <div className="text-center py-12 text-muted-foreground">
                  <Activity className="h-8 w-8 mx-auto mb-2 opacity-30" />
                  <p className="text-sm">后端当前仅支持流量统计</p>
                </div>
              </TerminalCard>

              {/* Network Chart */}
              <TerminalCard
                title="网络流量"
                icon={<Network className="h-4 w-4" />}
                className="lg:col-span-1"
              >
                {isLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                  </div>
                ) : networkData.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Activity className="h-8 w-8 mx-auto mb-2 opacity-30" />
                    <p className="text-sm">暂无数据</p>
                  </div>
                ) : (
                  <div className="h-64">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={networkData}>
                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                        <XAxis 
                          dataKey="time" 
                          stroke="hsl(var(--muted-foreground))" 
                          fontSize={12}
                        />
                        <YAxis 
                          stroke="hsl(var(--muted-foreground))" 
                          fontSize={12}
                          tickFormatter={(v) => `${(v / 1024 / 1024).toFixed(1)}MB`}
                        />
                        <Tooltip 
                          contentStyle={{
                            backgroundColor: 'hsl(var(--card))',
                            border: '1px solid hsl(var(--border))',
                            borderRadius: '4px'
                          }}
                          labelStyle={{ color: 'hsl(var(--foreground))' }}
                          formatter={(value: number, name: string) => [
                            `${(value / 1024 / 1024).toFixed(2)} MB/s`,
                            name === 'tx' ? '上传' : '下载'
                          ]}
                        />
                        <Legend 
                          formatter={(value) => value === 'tx' ? '上传' : '下载'}
                        />
                        <Line 
                          type="monotone" 
                          dataKey="tx" 
                          stroke="hsl(var(--primary))" 
                          strokeWidth={2}
                          dot={false}
                          name="tx"
                        />
                        <Line 
                          type="monotone" 
                          dataKey="rx" 
                          stroke="hsl(var(--accent))" 
                          strokeWidth={2}
                          dot={false}
                          name="rx"
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </TerminalCard>
            </div>
          )}
        </div>
      </AppLayout>
    </>
  );
};

export default PerformancePage;
