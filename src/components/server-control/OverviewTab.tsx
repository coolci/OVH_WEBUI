import { useState } from "react";
import { Cpu, HardDrive, MemoryStick, MapPin, Globe, Wifi, Activity, Shield, ArrowRightLeft } from "lucide-react";
import type { OwnedServer } from "@/hooks/use-server-control";
import {
  useServerHardware,
  useServerIps,
  useServerNetworkInterfaces,
  useServerMonitoring,
  useToggleMonitoring,
} from "@/hooks/use-server-control";
import { useHideIp, maskSensitive } from "@/hooks/use-hide-ip";
import { useActiveAccount } from "@/hooks/use-active-account";
import { Skeleton } from "@/components/common/Skeleton";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { MrtgTrafficChart } from "./MrtgTrafficChart";
import { ReverseDnsDialog } from "@/components/ip/ReverseDnsDialog";
import { FirewallDialog } from "@/components/ip/FirewallDialog";
import { MoveIpDialog } from "@/components/ip/MoveIpDialog";

/** IP type 展示：dedicated/failover + IPv4/IPv6，避免 raw unknown */
function formatIpTypeLabel(entry: {
  type?: string;
  family?: string;
  ip?: string;
  inferred?: boolean;
}): string {
  const rawType = String(entry.type || "").toLowerCase();
  const family =
    entry.family || (String(entry.ip || "").includes(":") ? "ipv6" : "ipv4");

  let typeLabel: string;
  if (rawType === "dedicated") {
    typeLabel = "独享";
  } else if (rawType === "failover") {
    typeLabel = "故障转移";
  } else if (rawType === "unknown" || rawType === "n/a" || !rawType) {
    typeLabel = family === "ipv6" ? "IPv6" : "IPv4";
  } else {
    typeLabel = entry.type || "";
  }

  const famLabel = family === "ipv6" ? "v6" : family === "ipv4" ? "v4" : "";
  if (typeLabel === "独享" || typeLabel === "故障转移") {
    return famLabel ? `${typeLabel} · ${famLabel}` : typeLabel;
  }
  return typeLabel || famLabel || "IP";
}

/** 概览 Tab：硬件 + 网络（IP / 接口 / MRTG 流量） */
export function OverviewTab({ server }: { server: OwnedServer }) {
  const hw = useServerHardware(server.serviceName);
  const ips = useServerIps(server.serviceName);
  const interfaces = useServerNetworkInterfaces(server.serviceName);
  const monitoringQuery = useServerMonitoring(server.serviceName);
  const toggleMonitoringMut = useToggleMonitoring();
  const { hidden } = useHideIp();
  const { activeAccountId } = useActiveAccount();

  const [selectedPtrIp, setSelectedPtrIp] = useState<string | null>(null);
  const [selectedFwIp, setSelectedFwIp] = useState<string | null>(null);
  const [selectedMoveIp, setSelectedMoveIp] = useState<string | null>(null);

  const memText = hw.data?.memorySize
    ? `${hw.data.memorySize.value} ${hw.data.memorySize.unit}`
    : "—";

  const cpuText = hw.data?.processorName
    ? hw.data.coresPerProcessor && hw.data.threadsPerProcessor
      ? `${hw.data.processorName} (${hw.data.coresPerProcessor}核/${hw.data.threadsPerProcessor}线程)`
      : hw.data.processorName
    : "—";

  const diskText =
    hw.data?.diskGroups && hw.data.diskGroups.length > 0
      ? hw.data.diskGroups
          .map((g: {
            numberOfDisks?: number;
            diskType?: string;
            diskSize?: { value: number; unit: string };
          }) => {
            const count = g.numberOfDisks ?? 1;
            const type = g.diskType ?? "";
            const size = g.diskSize ? `${g.diskSize.value} ${g.diskSize.unit}` : "";
            return [`${count} × ${type}`, size].filter(Boolean).join(" ");
          })
          .join(" / ")
      : "—";

  const ipEntries =
    ips.data && ips.data.length > 0
      ? ips.data
      : [{ ip: server.ip, type: "dedicated", family: "ipv4" as const }];

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 sm:gap-3">
        <InfoCard icon={<Cpu className="w-4 h-4" />} label="处理器" value={cpuText} loading={hw.isPending} />
        <InfoCard icon={<MemoryStick className="w-4 h-4" />} label="内存" value={memText} loading={hw.isPending} />
        <InfoCard icon={<HardDrive className="w-4 h-4" />} label="磁盘" value={diskText} loading={hw.isPending} />
        <InfoCard icon={<MapPin className="w-4 h-4" />} label="数据中心" value={server.datacenter.toUpperCase()} />
      </div>

      {/* 硬件自动看门狗监控 */}
      <div className="border border-border rounded-2xl p-4 bg-muted/20 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center flex-shrink-0 mt-0.5">
            <Activity className="w-4 h-4 text-primary" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-semibold">OVH 硬件自动看门狗 (Service Monitoring)</h3>
              <span className={`text-[10px] font-semibold px-2 py-0.5 rounded-full ${
                monitoringQuery.data ? "bg-emerald-500/10 text-emerald-600 border border-emerald-500/20" : "bg-muted text-muted-foreground border border-border"
              }`}>
                {monitoringQuery.isLoading ? "加载中..." : monitoringQuery.data ? "实时监控保护中" : "已关闭监控"}
              </span>
            </div>
            <p className="text-[12px] text-muted-foreground mt-0.5">
              启用后 OVH 机房监控系统将在独服 Ping 连续丢包超时后自动介入，向您发送告警并触发机房巡检干预。
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 self-end sm:self-center">
          <Switch
            checked={Boolean(monitoringQuery.data)}
            disabled={monitoringQuery.isLoading || toggleMonitoringMut.isPending}
            onCheckedChange={async (checked) => {
              try {
                await toggleMonitoringMut.mutateAsync({
                  serviceName: server.serviceName,
                  enabled: checked,
                });
                toast.success(checked ? "硬件自动看门狗已启用" : "硬件自动看门狗已关闭");
              } catch (e: any) {
                toast.error(e?.response?.data?.error || "切换监控状态失败");
              }
            }}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 sm:gap-4">
        <div className="border border-border rounded-2xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-2">
            <Globe className="w-4 h-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">IP 地址</h3>
          </div>
          {ips.isPending ? (
            <div className="p-4">
              <Skeleton className="h-20 rounded-md" />
            </div>
          ) : (
            <div className="divide-y divide-border">
              {ipEntries.map((entry) => {
                const isFailover = String(entry.type || "").toLowerCase() === "failover";
                return (
                  <div
                    key={entry.ip}
                    className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 px-4 py-2.5 text-[13px] hover:bg-muted/20 transition-colors"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <code className="min-w-0 truncate font-mono text-[12px] sm:text-[13px] font-semibold">
                        {maskSensitive(entry.ip, hidden)}
                      </code>
                      <span className="flex-shrink-0 text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border">
                        {formatIpTypeLabel(entry)}
                      </span>
                    </div>
                    <div className="flex items-center gap-1.5 self-end sm:self-center flex-shrink-0">
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-xs px-2 text-muted-foreground hover:text-foreground"
                        onClick={() => setSelectedPtrIp(entry.ip)}
                        title="查看/设置反向 DNS (PTR)"
                      >
                        <Globe className="w-3 h-3 mr-1 text-primary" />
                        PTR
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-xs px-2 text-muted-foreground hover:text-foreground"
                        onClick={() => setSelectedFwIp(entry.ip)}
                        title="配置 Edge 硬件防火墙与过滤规则"
                      >
                        <Shield className="w-3 h-3 mr-1 text-blue-500" />
                        防火墙
                      </Button>
                      {isFailover && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs px-2 text-amber-600 hover:text-amber-700 hover:bg-amber-50 dark:text-amber-400 dark:hover:bg-amber-950/30"
                          onClick={() => setSelectedMoveIp(entry.ip)}
                          title="跨机热迁移/漂移"
                        >
                          <ArrowRightLeft className="w-3 h-3 mr-1" />
                          漂移
                        </Button>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        <div className="border border-border rounded-2xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-2">
            <Wifi className="w-4 h-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">网卡接口</h3>
          </div>
          {interfaces.isPending ? (
            <div className="p-4">
              <Skeleton className="h-20 rounded-md" />
            </div>
          ) : (interfaces.data || []).length === 0 ? (
            <p className="px-4 py-6 text-sm text-muted-foreground text-center">未发现网卡</p>
          ) : (
            <div className="divide-y divide-border">
              {(interfaces.data || []).map((nic) => (
                <div key={nic.mac} className="px-4 py-3 flex items-center justify-between text-[13px]">
                  <code className="font-mono">{nic.mac}</code>
                  <span className="text-[11px] text-muted-foreground">{nic.linkType || "—"}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <MrtgTrafficChart serviceName={server.serviceName} />

      {selectedPtrIp && (
        <ReverseDnsDialog
          ip={selectedPtrIp}
          accountId={activeAccountId}
          onClose={() => setSelectedPtrIp(null)}
        />
      )}

      {selectedFwIp && (
        <FirewallDialog
          ip={selectedFwIp}
          accountId={activeAccountId}
          onClose={() => setSelectedFwIp(null)}
        />
      )}

      {selectedMoveIp && (
        <MoveIpDialog
          ip={selectedMoveIp}
          currentService={server.serviceName}
          accountId={activeAccountId}
          onClose={() => setSelectedMoveIp(null)}
        />
      )}
    </div>
  );
}

function InfoCard({
  icon,
  label,
  value,
  loading,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  loading?: boolean;
}) {
  return (
    <div className="border border-border rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0">
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center flex-shrink-0">
        {icon}
      </div>
      <div className="min-w-0">
        <div className="text-[11px] text-muted-foreground">{label}</div>
        {loading ? (
          <Skeleton className="h-4 w-24 mt-1" />
        ) : (
          <div className="text-[13px] font-semibold truncate" title={value}>
            {value}
          </div>
        )}
      </div>
    </div>
  );
}
