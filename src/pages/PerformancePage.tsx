import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { 
  Activity, 
  Cpu, 
  MemoryStick, 
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
  cpu?: StatPoint[];
  mem?: StatPoint[];
  net_tx?: StatPoint[];
  net_rx?: StatPoint[];
}

const PerformancePage = () => {
  const { data: serversData, isLoading: isLoadingServers } = useMyServers();
  const [selectedServer, setSelectedServer] = useState<string>("");
  const [period, setPeriod] = useState<'hourly' | 'daily' | 'weekly' | 'monthly'>('hourly');
  const [statistics, setStatistics] = useState<Statistics | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const servers = serversData?.servers || [];

  const loadStatistics = async () => {
    if (!selectedServer) return;
    
    setIsLoading(true);
    try {
      const result = await api.getServerStatistics(selectedServer, period);
      if (result.success) {
        setStatistics(result.statistics || null);
      } else {
        toast.error(result.error || "加载统计数据失败");
      }
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

  const formatChartData = (data: StatPoint[] | undefined) => {
    if (!data) return [];
    return data.map(point => ({
      time: new Date(point.timestamp * 1000).toLocaleTimeString("zh-CN", { 
        hour: '2-digit', 
        minute: '2-digit' 
      }),
      value: point.value
    }));
  };

  const formatNetworkData = () => {
    if (!statistics?.net_tx && !statistics?.net_rx) return [];
    
    const txData = statistics.net_tx || [];
    const rxData = statistics.net_rx || [];
    
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

  const cpuData = formatChartData(statistics?.cpu);
  const memData = formatChartData(statistics?.mem);
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
                  <SelectItem value="hourly">最近1小时</SelectItem>
                  <SelectItem value="daily">最近24小时</SelectItem>
                  <SelectItem value="weekly">最近7天</SelectItem>
                  <SelectItem value="monthly">最近30天</SelectItem>
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
              {/* CPU Chart */}
              <TerminalCard
                title="CPU 使用率"
                icon={<Cpu className="h-4 w-4" />}
              >
                {isLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                  </div>
                ) : cpuData.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Activity className="h-8 w-8 mx-auto mb-2 opacity-30" />
                    <p className="text-sm">暂无数据</p>
                  </div>
                ) : (
                  <div className="h-64">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={cpuData}>
                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                        <XAxis 
                          dataKey="time" 
                          stroke="hsl(var(--muted-foreground))" 
                          fontSize={12}
                        />
                        <YAxis 
                          stroke="hsl(var(--muted-foreground))" 
                          fontSize={12}
                          domain={[0, 100]}
                          tickFormatter={(v) => `${v}%`}
                        />
                        <Tooltip 
                          contentStyle={{
                            backgroundColor: 'hsl(var(--card))',
                            border: '1px solid hsl(var(--border))',
                            borderRadius: '4px'
                          }}
                          labelStyle={{ color: 'hsl(var(--foreground))' }}
                          formatter={(value: number) => [`${value.toFixed(1)}%`, 'CPU']}
                        />
                        <Line 
                          type="monotone" 
                          dataKey="value" 
                          stroke="hsl(var(--primary))" 
                          strokeWidth={2}
                          dot={false}
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </TerminalCard>

              {/* Memory Chart */}
              <TerminalCard
                title="内存使用率"
                icon={<MemoryStick className="h-4 w-4" />}
              >
                {isLoading ? (
                  <div className="flex items-center justify-center py-12">
                    <Loader2 className="h-6 w-6 animate-spin text-primary" />
                  </div>
                ) : memData.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Activity className="h-8 w-8 mx-auto mb-2 opacity-30" />
                    <p className="text-sm">暂无数据</p>
                  </div>
                ) : (
                  <div className="h-64">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={memData}>
                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                        <XAxis 
                          dataKey="time" 
                          stroke="hsl(var(--muted-foreground))" 
                          fontSize={12}
                        />
                        <YAxis 
                          stroke="hsl(var(--muted-foreground))" 
                          fontSize={12}
                          domain={[0, 100]}
                          tickFormatter={(v) => `${v}%`}
                        />
                        <Tooltip 
                          contentStyle={{
                            backgroundColor: 'hsl(var(--card))',
                            border: '1px solid hsl(var(--border))',
                            borderRadius: '4px'
                          }}
                          labelStyle={{ color: 'hsl(var(--foreground))' }}
                          formatter={(value: number) => [`${value.toFixed(1)}%`, '内存']}
                        />
                        <Line 
                          type="monotone" 
                          dataKey="value" 
                          stroke="hsl(var(--accent))" 
                          strokeWidth={2}
                          dot={false}
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </TerminalCard>

              {/* Network Chart */}
              <TerminalCard
                title="网络流量"
                icon={<Network className="h-4 w-4" />}
                className="lg:col-span-2"
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
