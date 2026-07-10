import { AppLayout } from "@/components/layout/AppLayout";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import {
  Activity,
  RefreshCw,
  Server,
  Network,
  Gauge,
  Radio,
} from "lucide-react";
import { useState, useEffect } from "react";
import { useMyServers } from "@/hooks/useApi";
import { api, ApiError } from "@/lib/api";
import { toast } from "sonner";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { PageHeader } from "@/components/common/PageHeader";
import { EmptyState } from "@/components/common/EmptyState";
import { CapabilityNotice } from "@/components/common/CapabilityNotice";
import { MrtgTrafficChart } from "@/components/server-control/MrtgTrafficChart";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { cn } from "@/lib/utils";

/**
 * 性能监控
 * - 主数据源：MRTG 网络流量（几乎所有独服可用）
 * - 次要：OVH /statistics 主机指标（多数机型 notAvailable，软降级不报错）
 */
const PerformancePage = () => {
  const { data: serversData, isLoading: isLoadingServers, refetch } = useMyServers();
  const [selectedServer, setSelectedServer] = useState<string>("");
  const [hostStatsState, setHostStatsState] = useState<
    "idle" | "loading" | "available" | "unavailable" | "error"
  >("idle");
  const [hostHint, setHostHint] = useState<string>("");
  const [refreshKey, setRefreshKey] = useState(0);

  const servers = (serversData?.servers || []).filter((s) => {
    const st = (s.state || "").toLowerCase();
    return st === "ok" || st === "active";
  });

  const selectedMeta = servers.find((s) => s.serviceName === selectedServer);

  useEffect(() => {
    if (!selectedServer && servers.length > 0) {
      setSelectedServer(servers[0].serviceName);
    }
  }, [servers, selectedServer]);

  // 探测主机 /statistics 是否开通（不阻塞主 UI；失败 → soft unavailable）
  useEffect(() => {
    if (!selectedServer) return;
    let cancelled = false;
    setHostStatsState("loading");
    setHostHint("");
    (async () => {
      try {
        const result = await api.getServerStatistics(selectedServer, "daily");
        if (cancelled) return;
        if (result?.notAvailable || result?.success === false) {
          setHostStatsState("unavailable");
          setHostHint(
            result?.hint ||
              result?.error ||
              "此机型未开通主机级指标接口"
          );
          return;
        }
        if (result?.success && result?.statistics) {
          setHostStatsState("available");
          return;
        }
        setHostStatsState("unavailable");
        setHostHint("未返回可用时序数据");
      } catch (e: unknown) {
        if (cancelled) return;
        if (e instanceof ApiError && (e.status === 404 || e.status === 0)) {
          setHostStatsState("unavailable");
          setHostHint(e.message);
          return;
        }
        // 其它错误也当 soft（避免红屏）
        setHostStatsState("unavailable");
        setHostHint(e instanceof Error ? e.message : "探测失败");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [selectedServer, refreshKey]);

  const onRefresh = () => {
    void refetch();
    setRefreshKey((k) => k + 1);
    toast.success("已刷新");
  };

  return (
    <>
      <Helmet>
        <title>性能监控 | OVH WebUI</title>
      </Helmet>
      <AppLayout>
        <div className="space-y-6 sm:space-y-7">
          <PageHeader
            icon={Activity}
            title="性能监控"
            description="网络流量观测 · OVH MRTG 官方带宽时序"
            action={
              <Button
                variant="outline"
                size="sm"
                className="min-h-9 rounded-full px-4"
                onClick={onRefresh}
              >
                <RefreshCw className="h-3.5 w-3.5" />
                刷新
              </Button>
            }
          />

          {/* 控制条 */}
          <div className="glass-panel rounded-2xl p-4 sm:p-5">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div className="grid flex-1 grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4 max-w-3xl">
                <div className="space-y-1.5">
                  <label className="section-label">服务器</label>
                  <Select
                    value={selectedServer}
                    onValueChange={setSelectedServer}
                    disabled={isLoadingServers || servers.length === 0}
                  >
                    <SelectTrigger className="min-h-11 rounded-xl border-border/80 bg-background/50">
                      <SelectValue
                        placeholder={isLoadingServers ? "加载中…" : "选择服务器"}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {servers.map((s) => (
                        <SelectItem key={s.serviceName} value={s.serviceName}>
                          <span className="font-mono text-xs sm:text-sm">
                            {s.name || s.serviceName}
                          </span>
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <label className="section-label">实例信息</label>
                  <div className="flex min-h-11 items-center gap-2 rounded-xl border border-border/80 bg-background/40 px-3">
                    {selectedMeta ? (
                      <>
                        <Chip tone="success">{selectedMeta.state || "ok"}</Chip>
                        <span className="truncate font-mono text-[11px] text-muted-foreground">
                          {selectedMeta.datacenter?.toUpperCase()}
                          {selectedMeta.ip ? ` · ${selectedMeta.ip}` : ""}
                        </span>
                      </>
                    ) : (
                      <span className="text-xs text-muted-foreground">未选择</span>
                    )}
                  </div>
                </div>
              </div>

              <div className="flex flex-wrap gap-2">
                <MetaPill
                  icon={Network}
                  label="流量"
                  value="MRTG"
                  active
                />
                <MetaPill
                  icon={Gauge}
                  label="主机指标"
                  value={
                    hostStatsState === "loading"
                      ? "检测中"
                      : hostStatsState === "available"
                        ? "可用"
                        : "未开通"
                  }
                  active={hostStatsState === "available"}
                  muted={hostStatsState !== "available"}
                />
              </div>
            </div>
          </div>

          {isLoadingServers ? (
            <div className="space-y-4">
              <Skeleton className="h-28 rounded-2xl" />
              <Skeleton className="h-[420px] rounded-2xl" />
            </div>
          ) : !selectedServer ? (
            <div className="glass-panel rounded-2xl">
              <EmptyState
                icon={Server}
                title="暂无可用服务器"
                description="请先在「服务器控制」确认账户下有运行中的独服"
              />
            </div>
          ) : (
            <>
              {/* 主：MRTG */}
              <section className="space-y-3">
                <div className="flex items-center justify-between gap-2 px-0.5">
                  <div className="flex items-center gap-2">
                    <Radio className="h-4 w-4 text-primary" />
                    <h2 className="text-sm font-semibold tracking-tight">网络流量</h2>
                    <span className="text-[11px] text-muted-foreground">
                      OVH MRTG · 公网带宽
                    </span>
                  </div>
                </div>
                <MrtgTrafficChart key={`${selectedServer}-${refreshKey}`} serviceName={selectedServer} />
              </section>

              {/* 次：主机 statistics 能力说明 */}
              <section className="space-y-3">
                <div className="flex items-center gap-2 px-0.5">
                  <Gauge className="h-4 w-4 text-muted-foreground" />
                  <h2 className="text-sm font-semibold tracking-tight text-muted-foreground">
                    主机级指标
                  </h2>
                </div>
                {hostStatsState === "loading" ? (
                  <Skeleton className="h-24 rounded-2xl" />
                ) : hostStatsState === "available" ? (
                  <CapabilityNotice
                    icon={Gauge}
                    tone="info"
                    title="主机级 /statistics 已开通"
                    description="此机型返回了 OVH statistics 数据。详细时序请结合 MRTG 查看网络侧。"
                  />
                ) : (
                  <CapabilityNotice
                    icon={Gauge}
                    tone="muted"
                    title="主机 CPU / 内存接口未开通"
                    description={
                      hostHint ||
                      "OVH 独服默认不提供带内主机 agent 指标；/statistics 在多数机型上不存在。"
                    }
                    alternative="网络带宽请使用上方 MRTG 流量图（官方监控）"
                  />
                )}
              </section>
            </>
          )}
        </div>
      </AppLayout>
    </>
  );
};

function MetaPill({
  icon: Icon,
  label,
  value,
  active,
  muted,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  active?: boolean;
  muted?: boolean;
}) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-[11px]",
        active && !muted
          ? "border-primary/30 bg-primary/10 text-primary"
          : "border-border/80 bg-muted/30 text-muted-foreground"
      )}
    >
      <Icon className="h-3.5 w-3.5" />
      <span className="font-medium">{label}</span>
      <span className="font-mono opacity-90">{value}</span>
    </div>
  );
}

export default PerformancePage;
