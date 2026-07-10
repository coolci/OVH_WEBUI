import { Cpu, HardDrive, MemoryStick, MapPin, Globe, Wifi } from "lucide-react";
import type { OwnedServer } from "@/hooks/use-server-control";
import {
  useServerHardware,
  useServerIps,
  useServerNetworkInterfaces,
} from "@/hooks/use-server-control";
import { useHideIp, maskSensitive } from "@/hooks/use-hide-ip";
import { Skeleton } from "@/components/common/Skeleton";
import { MrtgTrafficChart } from "./MrtgTrafficChart";

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
  const { hidden } = useHideIp();

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
              {ipEntries.map((entry) => (
                <div
                  key={entry.ip}
                  className="flex items-center justify-between gap-3 px-4 py-3 text-[13px]"
                >
                  <code className="min-w-0 truncate font-mono text-[12px] sm:text-[13px]">
                    {maskSensitive(entry.ip, hidden)}
                  </code>
                  <span className="flex-shrink-0 text-[11px] text-muted-foreground">
                    {formatIpTypeLabel(entry)}
                  </span>
                </div>
              ))}
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
