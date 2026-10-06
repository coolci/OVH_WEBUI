import { Cpu, HardDrive, MemoryStick, MapPin, Globe, Wifi, AlertTriangle, Sparkles, ArrowRight, PartyPopper } from "lucide-react";
import type { OwnedServer, HardwareLotteryItem } from "@/hooks/use-server-control";
import { useServerHardware, useServerIps, useServerNetworkInterfaces } from "@/hooks/use-server-control";
import { useHideIp, maskSensitive } from "@/hooks/use-hide-ip";
import { Skeleton } from "@/components/common/Skeleton";
import { PartialNotice, DetailErrorTag } from "@/components/common/PartialNotice";
import { MrtgTrafficChart } from "./MrtgTrafficChart";
import { useTranslation } from "react-i18next";

/** 概览 Tab：硬件 + 网络（IP / 接口 / MRTG 流量）。服务信息胶囊条已上提到 ServerTabs 同行 */
export function OverviewTab({ server }: { server: OwnedServer }) {
  const { t } = useTranslation();
  const hw = useServerHardware(server.serviceName);
  const ips = useServerIps(server.serviceName);
  const interfaces = useServerNetworkInterfaces(server.serviceName);
  const { hidden } = useHideIp();

  // 内存字段是 { value, unit } 对象
  const memText = hw.data?.memorySize
    ? `${hw.data.memorySize.value} ${hw.data.memorySize.unit}`
    : "—";

  // CPU 字段：processorName + 核线（旧前端写法照搬）
  const cpuText = hw.data?.processorName
    ? hw.data.coresPerProcessor && hw.data.threadsPerProcessor
      ? t("maint.overview.cpuCoresThreads", {
          name: hw.data.processorName,
          cores: hw.data.coresPerProcessor,
          threads: hw.data.threadsPerProcessor,
        })
      : hw.data.processorName
    : "—";

  // 磁盘：把所有 diskGroups 拼成 "N × Type Size" / "N × Type Size" 多组用 / 分隔
  const diskText =
    hw.data?.diskGroups && hw.data.diskGroups.length > 0
      ? hw.data.diskGroups
          .map((g: any) => {
            const count = g.numberOfDisks ?? 1;
            const type = g.diskType ?? "";
            const size = g.diskSize ? `${g.diskSize.value} ${g.diskSize.unit}` : "";
            return [`${count} × ${type}`, size].filter(Boolean).join(" ");
          })
          .join(" / ")
      : "—";

  // 中奖：订购配置（账单 /services + options）与实际交付硬件不同且更好，后端已算好
  const lottery = hw.data?.lottery;
  const wonOf = (kind: HardwareLotteryItem["kind"]) =>
    lottery?.checked ? lottery.items.find((i) => i.kind === kind) : undefined;
  return (
    <div className="space-y-6">
      {/* 列表接口这次没查到这台机器的 serviceInfos：续费状态 / 计费状态都不可信。
          「没查到」不能默默当成「没开自动续费」，否则用户会以为自己已经关过续费了。 */}
      {(server.svcInfoError || server.error) && (
        <div className="flex items-start gap-2 rounded-xl border border-warning/40 bg-warning/5 px-3 py-2 text-[11px] text-foreground/80">
          <AlertTriangle className="w-3.5 h-3.5 text-warning flex-shrink-0 mt-0.5" />
          <span>
            {server.error
              ? t("maint.overview.detailError", { err: server.error })
              : t("maint.overview.svcInfoError", { err: server.svcInfoError })}
          </span>
        </div>
      )}

      {lottery?.checked && lottery.won && (
        <div className="relative overflow-hidden flex items-center gap-2.5 rounded-xl border border-amber-400/40 bg-gradient-to-r from-amber-400/15 via-orange-400/10 to-transparent px-3.5 py-2.5">
          <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-amber-300 to-orange-500 flex items-center justify-center flex-shrink-0 shadow-[0_0_14px_rgba(251,191,36,0.45)]">
            <PartyPopper className="w-4 h-4 text-white" />
          </div>
          <div className="min-w-0 text-[12px] leading-snug">
            <div className="font-semibold text-amber-600 dark:text-amber-300">{t("maint.overview.lottery.bannerTitle")}</div>
            <div className="text-muted-foreground truncate">
              {t("maint.overview.lottery.bannerDesc", {
                plan: lottery.planName || lottery.planCode || "",
                items: lottery.items.map((i) => t(`maint.overview.lottery.kind.${i.kind}`)).join(" · "),
              })}
            </div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 sm:gap-3">
        <InfoCard icon={<Cpu className="w-4 h-4" />} label={t("maint.overview.info.cpu")} value={cpuText} loading={hw.isPending} won={wonOf("cpu")} />
        <InfoCard icon={<MemoryStick className="w-4 h-4" />} label={t("maint.overview.info.mem")} value={memText} loading={hw.isPending} won={wonOf("memory")} />
        <InfoCard icon={<HardDrive className="w-4 h-4" />} label={t("maint.overview.info.disk")} value={diskText} loading={hw.isPending} won={wonOf("disk")} />
        <InfoCard icon={<MapPin className="w-4 h-4" />} label={t("maint.overview.info.dc")} value={(server.datacenter || "—").toUpperCase()} />
      </div>

      {/* 网络：IP 列表 + 接口 + MRTG 流量 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 sm:gap-4">
        {/* IP 列表 */}
        <div className="border border-border rounded-2xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-2">
            <Globe className="w-4 h-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">{t("maint.overview.ipTitle")}</h3>
          </div>
          {ips.isPending ? (
            <div className="p-4">
              <Skeleton className="h-20 rounded-md" />
            </div>
          ) : (
            <div className="divide-y divide-border">
              {(ips.data && ips.data.length > 0 ? ips.data : [{ ip: server.ip, type: "IPv4" }]).map((entry) => (
                <div key={entry.ip} className="px-4 py-3 flex items-center justify-between text-[13px]">
                  <code className="font-mono">{maskSensitive(entry.ip, hidden)}</code>
                  <span className="text-[11px] text-muted-foreground">{entry.type}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 网卡接口 */}
        <div className="border border-border rounded-2xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center gap-2">
            <Wifi className="w-4 h-4 text-muted-foreground" />
            <h3 className="text-sm font-semibold">{t("maint.overview.nicTitle")}</h3>
          </div>
          {interfaces.isPending ? (
            <div className="p-4">
              <Skeleton className="h-20 rounded-md" />
            </div>
          ) : interfaces.isError ? (
            // 「读取失败」和「这台机器没网卡」是两回事，混成同一句会让用户放弃重试
            <p className="px-4 py-6 text-sm text-destructive text-center">{t("maint.overview.nicLoadFailed")}</p>
          ) : (interfaces.data?.items || []).length === 0 ? (
            <p className="px-4 py-6 text-sm text-muted-foreground text-center">{t("maint.overview.nicEmpty")}</p>
          ) : (
            <>
              <PartialNotice
                failedCount={interfaces.data?.failedCount || 0}
                what={t("maint.overview.nicPartialWhat")}
                className="mx-4 mt-3"
              />
              <div className="divide-y divide-border">
                {(interfaces.data?.items || []).map((nic) => (
                  <div key={nic.mac} className="px-4 py-3 flex items-center justify-between text-[13px]">
                    <code className="font-mono">{nic.mac}</code>
                    <span className="text-[11px] text-muted-foreground flex items-center gap-2">
                      <DetailErrorTag message={nic._detailError} />
                      {nic.linkType || "—"}
                    </span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>

      {/* MRTG 流量监控 */}
      <MrtgTrafficChart serviceName={server.serviceName} />
    </div>
  );
}

function InfoCard({
  icon,
  label,
  value,
  loading,
  won,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  loading?: boolean;
  /** 该项中奖明细（实配优于订购），有值时卡片高亮 + 中奖徽标 */
  won?: HardwareLotteryItem;
}) {
  const { t } = useTranslation();
  const tip = won ? t("maint.overview.lottery.tip", { ordered: won.ordered, actual: won.actual }) : value;
  return (
    <div
      className={
        won
          ? "relative overflow-hidden border border-amber-400/60 rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0 bg-gradient-to-br from-amber-400/10 via-transparent to-orange-500/10 shadow-[0_0_0_1px_rgba(251,191,36,0.15),0_4px_18px_-6px_rgba(251,191,36,0.45)] transition-shadow hover:shadow-[0_0_0_1px_rgba(251,191,36,0.35),0_6px_24px_-6px_rgba(251,191,36,0.6)]"
          : "border border-border rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0"
      }
      title={tip}
    >
      <div
        className={
          won
            ? "w-9 h-9 rounded-lg bg-gradient-to-br from-amber-300 to-orange-500 text-white flex items-center justify-center flex-shrink-0 shadow-[0_0_12px_rgba(251,191,36,0.5)]"
            : "w-9 h-9 rounded-lg bg-secondary flex items-center justify-center flex-shrink-0"
        }
      >
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
          <span className="truncate">{label}</span>
          {won && !loading && (
            <span className="inline-flex items-center gap-0.5 rounded-full bg-gradient-to-r from-amber-400 to-orange-500 px-1.5 py-[1px] text-[10px] font-bold text-white shadow-sm flex-shrink-0">
              <Sparkles className="w-2.5 h-2.5 animate-pulse" />
              {t("maint.overview.lottery.badge")}
            </span>
          )}
        </div>
        {loading ? (
          <Skeleton className="h-4 w-24 mt-1" />
        ) : (
          <>
            <div className="text-[13px] font-semibold truncate">{value}</div>
            {won && (
              <div className="mt-0.5 flex items-center gap-1 text-[10.5px] min-w-0">
                <span className="text-muted-foreground flex-shrink-0">{t("maint.overview.lottery.ordered")}</span>
                <span className="text-muted-foreground line-through truncate">{won.ordered}</span>
                <ArrowRight className="w-3 h-3 text-amber-500 flex-shrink-0" />
                <span className="font-semibold text-amber-600 dark:text-amber-300 truncate">{won.actual}</span>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
