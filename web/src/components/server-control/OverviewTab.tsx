import { useState, useMemo } from "react";
import { Cpu, HardDrive, MemoryStick, MapPin, Globe, Wifi, AlertTriangle, Sparkles, ArrowRight, PartyPopper, Edit2, Wallet } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ReverseDnsDialog } from "./ReverseDnsDialog";
import type { OwnedServer, HardwareLotteryItem, ServerPricing } from "@/hooks/use-server-control";
import { useServerHardware, useServerIps, useServerNetworkInterfaces, useServerReverseDns, useServerPricing } from "@/hooks/use-server-control";
import { fmtMoney } from "@/i18n/format";
import { errorMessage } from "@/lib/api-error";
import { useHideIp, maskSensitive } from "@/hooks/use-hide-ip";
import { Skeleton } from "@/components/common/Skeleton";
import { PartialNotice, DetailErrorTag } from "@/components/common/PartialNotice";
import { MrtgTrafficChart } from "./MrtgTrafficChart";
import { LotteryConfetti } from "./LotteryConfetti";
import { useTranslation } from "react-i18next";
import { useScopedServerAccountId } from "./server-scope";

/** 概览 Tab：硬件 + 网络（IP / 接口 / MRTG 流量）。服务信息胶囊条已上提到 ServerTabs 同行 */
export function OverviewTab({ server, accountId: propAccountId }: { server: OwnedServer; accountId?: string }) {
  const { t } = useTranslation();
  const accountId = useScopedServerAccountId(propAccountId || server.accountId);
  const hw = useServerHardware(server.serviceName, accountId);
  const pricing = useServerPricing(server.serviceName, accountId);
  const ips = useServerIps(server.serviceName, accountId);
  const interfaces = useServerNetworkInterfaces(server.serviceName, accountId);
  const reverseQ = useServerReverseDns(server.serviceName, accountId);
  const { hidden } = useHideIp();
  const [reverseOpen, setReverseOpen] = useState(false);
  const [selectedIp, setSelectedIp] = useState<string>("");

  const revMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const r of reverseQ.data || []) {
      if (r.ipReverse && r.reverse) map.set(r.ipReverse, r.reverse);
    }
    return map;
  }, [reverseQ.data]);

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
          .map((g) => {
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
        <div className="relative overflow-hidden flex items-start gap-3 rounded-xl border border-amber-400/40 bg-gradient-to-r from-amber-400/15 via-orange-400/10 to-transparent p-3.5">
          <LotteryConfetti sessionKey={server.serviceName} />
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-amber-300 to-orange-500 flex items-center justify-center flex-shrink-0 shadow-[0_0_14px_rgba(251,191,36,0.45)] mt-0.5">
            <PartyPopper className={`text-white ${lottery.tier === 3 ? "w-5 h-5" : "w-4 h-4"}`} />
          </div>
          <div className="min-w-0 text-[12px] leading-snug flex-1">
            <div className="font-semibold text-amber-600 dark:text-amber-300">
              {t(`maint.overview.lottery.title${Math.min(lottery.tier || 1, 3)}`)}
            </div>
            <div className="text-muted-foreground mt-0.5">
              {t("maint.overview.lottery.bannerDesc", {
                items: lottery.items
                  .map((i) => {
                    const label = t(`maint.overview.lottery.kind.${i.kind}`);
                    if (i.gainPct && i.gainPct >= 1) {
                      return `${label} ${t("maint.overview.lottery.gainPct", { pct: Math.round(i.gainPct) })}`;
                    }
                    if (i.mediaUp) {
                      return `${label} ${t("maint.overview.lottery.gainMedia")}`;
                    }
                    return label;
                  })
                  .join(" · "),
              })}
            </div>
            <div className="mt-2 space-y-1">
              {lottery.items.map((i) => (
                <p key={i.kind} className="text-[11px] text-muted-foreground">
                  <span className="font-medium text-foreground/80">{t(`maint.overview.lottery.kind.${i.kind}`)}</span>
                  ：{i.ordered} → <span className="font-semibold text-amber-600 dark:text-amber-300">{i.actual}</span>
                  {i.gainPct && i.gainPct >= 1 && (
                    <span className="ml-1.5 inline-flex items-center px-1.5 py-0.5 leading-none rounded text-[10px] bg-amber-500/15 text-amber-600 dark:text-amber-400 font-mono">
                      +{Math.round(i.gainPct)}%
                    </span>
                  )}
                  {i.mediaUp && (
                    <span className="ml-1.5 inline-flex items-center px-1.5 py-0.5 leading-none rounded text-[10px] bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 font-mono">
                      {t("maint.overview.lottery.gainMedia")}
                    </span>
                  )}
                </p>
              ))}
            </div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-6 xl:grid-cols-5 gap-2.5 sm:gap-3">
        <InfoCard className="md:col-span-2 xl:col-span-1" icon={<Cpu className="w-4 h-4" />} label={t("maint.overview.info.cpu")} value={cpuText} loading={hw.isPending} won={wonOf("cpu")} />
        <InfoCard className="md:col-span-2 xl:col-span-1" icon={<MemoryStick className="w-4 h-4" />} label={t("maint.overview.info.mem")} value={memText} loading={hw.isPending} won={wonOf("memory")} />
        <InfoCard className="md:col-span-2 xl:col-span-1" icon={<HardDrive className="w-4 h-4" />} label={t("maint.overview.info.disk")} value={diskText} loading={hw.isPending} won={wonOf("disk")} />
        <InfoCard className="md:col-span-3 xl:col-span-1" icon={<MapPin className="w-4 h-4" />} label={t("maint.overview.info.dc")} value={(server.datacenter || "—").toUpperCase()} />
        <CostInfoCard className="col-span-2 md:col-span-3 xl:col-span-1" pricing={pricing.data} loading={pricing.isPending} error={pricing.isError ? pricing.error : null} />
      </div>

      {/* 网络：IP 列表 + 接口 + MRTG 流量 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3.5 sm:gap-4">
        {/* IP 列表 */}
        <div className="surface-card rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Globe className="w-4 h-4 text-muted-foreground" />
              <h3 className="text-sm font-semibold">{t("maint.overview.ipTitle")}</h3>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setSelectedIp(server.ip || "");
                setReverseOpen(true);
              }}
              className="h-7 text-xs px-2.5 gap-1"
            >
              <Globe className="w-3 h-3 text-primary" />
              反向 DNS
            </Button>
          </div>
          {ips.isPending ? (
            <div className="p-4">
              <Skeleton className="h-20 rounded-md" />
            </div>
          ) : (
            <div className="divide-y divide-border">
              {(ips.data && ips.data.length > 0 ? ips.data : [{ ip: server.ip, type: "IPv4" }]).map((entry) => (
                <div key={entry.ip} className="px-4 py-3 flex items-center justify-between text-[13px] gap-2">
                  <div className="space-y-0.5 min-w-0">
                    <code className="font-mono">{maskSensitive(entry.ip, hidden)}</code>
                    {revMap.get(entry.ip) && (
                      <div className="text-[11px] text-muted-foreground font-mono truncate max-w-[280px]" title={revMap.get(entry.ip)}>
                        ↩ {revMap.get(entry.ip)}
                      </div>
                    )}
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className="text-[11px] text-muted-foreground">{entry.type}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 text-muted-foreground hover:text-foreground"
                      title="配置反向 DNS"
                      onClick={() => {
                        setSelectedIp(entry.ip);
                        setReverseOpen(true);
                      }}
                    >
                      <Edit2 className="w-3 h-3" />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 网卡接口 */}
        <div className="surface-card rounded-xl overflow-hidden">
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
      <ReverseDnsDialog open={reverseOpen}
        onOpenChange={setReverseOpen}
        serviceName={server.serviceName}
        defaultIp={selectedIp}
        accountId={accountId}
      />
    </div>
  );
}

function InfoCard({
  icon,
  label,
  value,
  loading,
  won,
  className,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  loading?: boolean;
  /** 该项中奖明细（实配优于订购），有值时卡片高亮 + 中奖徽标 */
  won?: HardwareLotteryItem;
  className?: string;
}) {
  const { t } = useTranslation();
  const tip = won ? t("maint.overview.lottery.tip", { ordered: won.ordered, actual: won.actual }) : value;
  return (
    <div
      className={
        (won
          ? "relative overflow-hidden border border-amber-400/60 rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0 bg-gradient-to-br from-amber-400/10 via-transparent to-orange-500/10 shadow-[0_0_0_1px_rgba(251,191,36,0.15),0_4px_18px_-6px_rgba(251,191,36,0.45)] transition-shadow hover:shadow-[0_0_0_1px_rgba(251,191,36,0.35),0_6px_24px_-6px_rgba(251,191,36,0.6)]"
          : "surface-card rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0") +
        (className ? ` ${className}` : "")
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

/** 续费周期文本格式化（ISO8601 duration，如 P1M / P12M / P1Y） */
function formatRenewalPeriod(
  duration: string,
  t: ReturnType<typeof useTranslation>["t"]
): string {
  const m = /^P(\d+)([MY])$/i.exec(duration || "");
  if (!m || !m[1] || !m[2]) return "";
  const n = Number(m[1]);
  const unit = m[2].toUpperCase();
  if (unit === "Y") return n === 1 ? t("maint.overview.info.perYear") : t("maint.overview.info.perYears", { n });
  if (n === 12) return t("maint.overview.info.perYear");
  return n === 1 ? t("maint.overview.info.perMonth") : t("maint.overview.info.perMonths", { n });
}

/** 服务器费用信息格：续费价格（主机 + 附加选项，不含税）+ 公开目录安装费 */
function CostInfoCard({
  pricing,
  loading,
  error,
  className,
}: {
  pricing?: ServerPricing;
  loading: boolean;
  error: unknown;
  className?: string;
}) {
  const { t } = useTranslation();
  const r = pricing?.renewal;
  const inst = pricing?.installation;

  const mainText = r
    ? `${fmtMoney(r.amount, r.currency)} ${formatRenewalPeriod(r.duration, t)}${r.partial ? "+" : ""}`.trim()
    : "—";
  const setupText = inst?.ok && inst.amount != null && inst.currency
    ? t("maint.overview.info.setup", { amount: `${fmtMoney(inst.amount, inst.currency)}${inst.partial ? "+" : ""}` })
    : inst && !inst.ok
      ? t("maint.overview.info.setupUnknown")
      : "";

  let tooltip = "";
  if (error) {
    tooltip = t("maint.overview.info.priceFailed", { detail: errorMessage(error) });
  } else if (r) {
    const withTax =
      inst?.ok && inst.amount != null && inst.currency
        ? fmtMoney(inst.amount + (inst.tax || 0), inst.currency)
        : "—";
    tooltip = t("maint.overview.info.priceTitle", {
      options: r.options > 0 ? t("maint.overview.info.priceOptions", { n: r.options }) : "",
      withTax,
    });
    if (r.partial) tooltip += " " + t("maint.overview.info.pricePartial");
  }

  return (
    <div
      className={`surface-card rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0 ${className || "col-span-2 lg:col-span-1"}`}
      title={tooltip || undefined}
    >
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center flex-shrink-0">
        <Wallet className="w-4 h-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-[11px] text-muted-foreground truncate">{t("maint.overview.info.cost")}</div>
        {loading ? (
          <Skeleton className="h-4 w-24 mt-1" />
        ) : (
          <>
            <div className="text-[13px] font-semibold tabular-nums truncate">{mainText}</div>
            {setupText && <div className="text-[10.5px] text-muted-foreground tabular-nums truncate">{setupText}</div>}
          </>
        )}
      </div>
    </div>
  );
}
