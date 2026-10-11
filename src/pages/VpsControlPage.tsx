import { useEffect, useState } from "react";
import { Cloud, Eye, EyeOff, RefreshCw } from "lucide-react";
import { Helmet } from "react-helmet-async";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { LoadFailed, LoadFailedBanner } from "@/components/common/LoadFailed";
import { DeviceSwitcherCard } from "@/components/common/DeviceSwitcherCard";
import { VpsDetail } from "@/components/vps-control/VpsDetail";
import { useOwnedVps, type OwnedVps } from "@/hooks/use-vps-control";
import { useHideIp } from "@/hooks/use-hide-ip";
import { useActiveVpsControlAccount } from "@/hooks/use-active-account";
import { useAccounts } from "@/hooks/use-accounts";
import { useServerAliases, useSetServerAlias, aliasOf } from "@/hooks/use-server-aliases";
import { isUsEndpoint, regionLabel, endpointRegion } from "@/lib/ovh-regions";
import { cn } from "@/lib/utils";

const EMPTY_VPS_LIST: OwnedVps[] = [];

function VpsControlPage() {
  const q = useOwnedVps();
  const { hidden, toggle } = useHideIp();
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [activeAccount, setActiveAccount] = useActiveVpsControlAccount();
  const { data: accounts } = useAccounts();
  const vpsList = q.data || EMPTY_VPS_LIST;

  useEffect(() => {
    if (!accounts || accounts.length === 0) return;
    const exists = accounts.some((a) => a.id === activeAccount);
    if (!exists) {
      const def = accounts.find((a) => a.isDefault) || accounts[0];
      if (def?.id) {
        setActiveAccount(def.id);
      }
    }
  }, [accounts, activeAccount, setActiveAccount]);

  useEffect(() => {
    setSelectedName(null);
  }, [activeAccount]);

  // 自动选第一台（首次加载或列表变更/当前选中失效后）
  useEffect(() => {
    if (vpsList.length === 0) {
      setSelectedName(null);
      return;
    }
    if (!selectedName || !vpsList.some((v) => v.serviceName === selectedName)) {
      setSelectedName(vpsList[0].serviceName);
    }
  }, [vpsList, selectedName]);

  const selected = vpsList.find((v) => v.serviceName === selectedName) || null;
  const aliases = useServerAliases();
  const setAlias = useSetServerAlias();
  const activeEndpoint = (accounts || []).find((a) => a.id === activeAccount)?.endpoint;

  return (
    <div className="space-y-4">
      <PageHeader
        title="VPS 控制"
        description="已购 VPS 的电源 / 快照 / 重装 / 控制台管理"
        icon={Cloud}
        action={
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" className="h-8 text-xs gap-1.5 px-3 rounded-lg" onClick={toggle}>
              {hidden ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
              <span>{hidden ? "显示 IP" : "隐藏 IP"}</span>
            </Button>
            <Button
              size="sm"
              className="h-8 gap-1.5 text-xs font-medium rounded-lg shadow-sm px-3"
              onClick={() => q.refetch()}
              disabled={q.isFetching}
              title="刷新 VPS 状态"
            >
              <RefreshCw className={cn("w-3.5 h-3.5", q.isFetching && "animate-spin")} />
              <span>刷新</span>
            </Button>
          </div>
        }
      />

      {/* 一体化 VPS 设备导航控制台 */}
      <DeviceSwitcherCard
        deviceType="vps"
        accounts={(accounts || []).map((a) => ({
          id: a.id,
          name: a.name,
        }))}
        activeAccount={activeAccount || ""}
        onAccountChange={(v) => setActiveAccount(v || "")}
        items={vpsList.map((v) => ({
          serviceName: v.serviceName,
          name: aliasOf(aliases.data, v.serviceName, v.displayName || v.serviceName),
          rawName: v.serviceName,
          datacenter: (v.zone || v.cluster || "VPS").toUpperCase(),
          subtext: v.model
            ? `${v.model}${v.vcore ? ` · ${v.vcore}C` : ""}${v.memoryMB ? `/${(v.memoryMB / 1024).toFixed(0)}G` : ""}`
            : "OVH VPS",
          state: v.state,
        }))}
        selectedName={selectedName || undefined}
        onSelectName={(name) => setSelectedName(name)}
        isFetching={q.isFetching}
        hidden={hidden}
      />

      {/* 内容区 */}
      {q.isError && vpsList.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <CardContent className="p-4">
            <LoadFailed icon={Cloud} title="VPS 列表读取失败" error={q.error} onRetry={() => q.refetch()} />
          </CardContent>
        </Card>
      ) : !q.isPending && vpsList.length === 0 ? (
        <Card className="surface-card rounded-xl border-border">
          <CardContent className="py-12">
            <EmptyState icon={Cloud} title="该账户下暂无 VPS" description="可以去 OVH 官网下单,或换个有 VPS 的账户" />
          </CardContent>
        </Card>
      ) : selected ? (
        <>
          {q.isError && (
            <LoadFailedBanner
              title="VPS 列表刷新失败,下面是上次拿到的数据"
              error={q.error}
              onRetry={() => q.refetch()}
            />
          )}
          <VpsDetail
            server={selected}
            aliases={aliases}
            onSetAlias={setAlias}
            isUS={isUsEndpoint(activeEndpoint)}
            region={regionLabel(endpointRegion(activeEndpoint))}
          />
        </>
      ) : q.isPending ? (
        <Skeleton className="h-96 rounded-2xl" />
      ) : null}
    </div>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>VPS 控制 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <VpsControlPage />
    </AppLayout>
  </>
);

export default Page;
