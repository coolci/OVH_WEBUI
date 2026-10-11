import { useMemo, useState, useEffect } from "react";
import { Server, RefreshCw } from "lucide-react";
import { Helmet } from "react-helmet-async";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { MonitorSubscribeDialog } from "@/components/monitor/MonitorSubscribeDialog";
import { CacheBadge } from "@/components/servers/CacheBadge";
import { ServerFilterBar } from "@/components/servers/ServerFilterBar";
import { ServerCard } from "@/components/servers/ServerCard";
import { ServerDetailDialog } from "@/components/servers/ServerDetailDialog";
import { useServers } from "@/hooks/use-servers";
import { useMonitorList } from "@/hooks/use-monitor";
import { useAccountInfo } from "@/hooks/use-account";
import {
  useAvailability,
  buildAvailabilityMap,
  buildVariantIndex,
  useOvhCatalog,
  buildCatalogIndex,
  computePriceFromOptions,
  type PriceInfo,
} from "@/hooks/use-availability";
import { OVH_DATACENTERS } from "@/lib/datacenters";
import { cn } from "@/lib/utils";

const SUB_LS_KEY = "ovh_sniper_price_subsidiary";
const SUB_MANUAL_LS_KEY = "ovh_sniper_price_subsidiary_manual";

function ServersPage() {
  const q = useServers();
  const availQ = useAvailability();
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);
  const variantIndex = useMemo(() => buildVariantIndex(availQ.data), [availQ.data]);

  const account = useAccountInfo();
  const accountSub = account.data?.ovhSubsidiary;

  const [subsidiary, setSubsidiary] = useState<string>(() => {
    try {
      const manualPicked = localStorage.getItem(SUB_MANUAL_LS_KEY) === "1";
      if (manualPicked) return localStorage.getItem(SUB_LS_KEY) || "IE";
    } catch { /* ignore */ }
    return "IE";
  });

  useEffect(() => {
    if (!accountSub) return;
    let manualPicked = false;
    try {
      manualPicked = localStorage.getItem(SUB_MANUAL_LS_KEY) === "1";
    } catch { /* ignore */ }
    if (!manualPicked) setSubsidiary(accountSub);
  }, [accountSub]);

  const changeSubsidiary = (v: string) => {
    setSubsidiary(v);
    try {
      localStorage.setItem(SUB_LS_KEY, v);
      localStorage.setItem(SUB_MANUAL_LS_KEY, "1");
    } catch { /* ignore */ }
  };

  const resetSubsidiaryToAccount = () => {
    try {
      localStorage.removeItem(SUB_MANUAL_LS_KEY);
      localStorage.removeItem(SUB_LS_KEY);
    } catch { /* ignore */ }
    if (accountSub) setSubsidiary(accountSub);
  };

  const catalogQ = useOvhCatalog(subsidiary);
  const catalogIdx = useMemo(() => buildCatalogIndex(catalogQ.data), [catalogQ.data]);

  const priceMap = useMemo(() => {
    const out: Record<string, PriceInfo> = {};
    for (const srv of q.data || []) {
      const defaults = (srv.defaultOptions || []).map((o) => o.value).filter(Boolean);
      const p = computePriceFromOptions(srv.planCode, defaults, catalogIdx);
      if (p) out[srv.planCode] = p;
    }
    return out;
  }, [q.data, catalogIdx]);

  const [search, setSearch] = useState("");
  const [onlyAvailable, setOnlyAvailable] = useState(false);
  const [detailPlanCode, setDetailPlanCode] = useState<string | null>(null);

  const monitorList = useMonitorList();
  const monitoredCodes = useMemo(
    () => new Set((monitorList.data || []).map((s) => s.planCode)),
    [monitorList.data]
  );
  const [monitorTarget, setMonitorTarget] = useState<{
    planCode: string;
    serverName?: string;
    datacenters: string[];
  } | null>(null);

  const existingMonitor = monitorTarget
    ? monitorList.data?.find((s) => s.planCode === monitorTarget.planCode)
    : undefined;

  const list = useMemo(() => q.data || [], [q.data]);
  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    let out = list;
    if (s) {
      out = out.filter((srv) =>
        `${srv.planCode} ${srv.name} ${srv.cpu} ${srv.memory} ${srv.storage}`.toLowerCase().includes(s)
      );
    }
    if (onlyAvailable) {
      out = out.filter((srv) => {
        const map = availMap[srv.planCode];
        if (map) {
          return Object.values(map).some((v) => v && v !== "unavailable" && v !== "unknown");
        }
        return srv.datacenters?.some((dc) => dc.availability && dc.availability !== "unavailable" && dc.availability !== "unknown") ?? false;
      });
    }
    return out;
  }, [list, search, onlyAvailable, availMap]);

  const detailServer = detailPlanCode ? list.find((s) => s.planCode === detailPlanCode) || null : null;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Server}
        title="服务器列表"
        description="目录、价格、可用性全部走访问触发的缓存，2 小时内复用"
        action={
          <div className="flex items-center gap-2">
            <CacheBadge />
            <Button
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg shadow-sm px-3"
              onClick={() => {
                q.forceRefresh();
                catalogQ.refetch();
                availQ.refetch();
              }}
              disabled={q.isRefreshing || catalogQ.isRefetching || availQ.isRefetching}
              title="一键强刷目录与价格库存"
            >
              <RefreshCw
                className={cn(
                  "w-3.5 h-3.5",
                  (q.isRefreshing || catalogQ.isRefetching || availQ.isRefetching) && "animate-spin"
                )}
              />
              <span>刷新</span>
            </Button>
          </div>
        }
      />

      <ServerFilterBar
        search={search}
        onSearchChange={setSearch}
        onlyAvailable={onlyAvailable}
        onToggleOnlyAvailable={() => setOnlyAvailable((v) => !v)}
        subsidiary={subsidiary}
        onSubsidiaryChange={changeSubsidiary}
        accountSub={accountSub}
        onResetSubsidiary={resetSubsidiaryToAccount}
        totalCount={filtered.length}
        isLoading={q.isPending}
      />

      {q.isPending ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-[260px] rounded-xl" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <EmptyState
            icon={Server}
            title="未找到服务器"
            description={list.length === 0 ? "API 未返回服务器，检查 API 设置" : "没有匹配的搜索结果"}
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
          {filtered.map((srv) => (
            <ServerCard
              key={srv.planCode}
              server={srv}
              realtimeDcMap={availMap[srv.planCode]}
              price={priceMap[srv.planCode]}
              isMonitored={monitoredCodes.has(srv.planCode)}
              onView={() => setDetailPlanCode(srv.planCode)}
              onMonitor={() =>
                setMonitorTarget({
                  planCode: srv.planCode,
                  serverName: srv.name,
                  datacenters: OVH_DATACENTERS.map((dc) => dc.code),
                })
              }
            />
          ))}
        </div>
      )}

      <Dialog open={!!detailServer} onOpenChange={(v) => !v && setDetailPlanCode(null)}>
        <DialogContent className="w-[95vw] sm:w-full sm:max-w-3xl max-h-[90vh] overflow-hidden flex flex-col">
          {detailServer && (
            <ServerDetailDialog
              server={detailServer}
              realtimeDcMap={availMap[detailServer.planCode]}
              variants={variantIndex[detailServer.planCode]}
              defaultPrice={priceMap[detailServer.planCode]}
              catalogIdx={catalogIdx}
              subsidiary={subsidiary}
              isMonitored={monitoredCodes.has(detailServer.planCode)}
              onClose={() => setDetailPlanCode(null)}
              onMonitor={(datacenters) => {
                setDetailPlanCode(null);
                setMonitorTarget({
                  planCode: detailServer.planCode,
                  serverName: detailServer.name,
                  datacenters:
                    datacenters.length > 0 ? datacenters : OVH_DATACENTERS.map((dc) => dc.code),
                });
              }}
            />
          )}
        </DialogContent>
      </Dialog>

      <MonitorSubscribeDialog
        key={`${existingMonitor ? "edit" : "create"}:${monitorTarget?.planCode || ""}`}
        open={!!monitorTarget}
        onOpenChange={(v) => !v && setMonitorTarget(null)}
        mode={existingMonitor ? "edit" : "create"}
        lockPlanCode
        planCode={monitorTarget?.planCode}
        serverName={monitorTarget?.serverName || existingMonitor?.serverName}
        initial={
          existingMonitor ??
          (monitorTarget
            ? { planCode: monitorTarget.planCode, datacenters: monitorTarget.datacenters }
            : null)
        }
      />
    </div>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>服务器列表 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <ServersPage />
    </AppLayout>
  </>
);

export default Page;
