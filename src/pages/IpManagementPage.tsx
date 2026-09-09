import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import {
  Network, RefreshCw, ArrowRightLeft, Globe, Shield, Server,
  Cloud, Building2, User
} from "lucide-react";
import { useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { useIps } from "@/hooks/use-ip";
import { useAccounts } from "@/hooks/use-accounts";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import { ReverseDnsDialog } from "@/components/ip/ReverseDnsDialog";
import { FirewallDialog } from "@/components/ip/FirewallDialog";
import { MoveIpDialog } from "@/components/ip/MoveIpDialog";
import type { IpAsset } from "@/lib/types";

function formatIpType(item: IpAsset): { label: string; tone: "info" | "success" | "warning" | "default" } {
  const t = (item.type || "").toLowerCase();
  const sName = (item.routedTo?.serviceName || "").toLowerCase();
  if (t === "failover") return { label: "Failover 附加", tone: "warning" };
  if (t === "dedicated") return { label: "独服主 IP", tone: "info" };
  if (t === "vps") return { label: "VPS 实例 IP", tone: "info" };
  if (t === "cloud") return { label: "Cloud IP", tone: "success" };
  if (sName.startsWith("vps-") || sName.includes(".vps.")) return { label: "VPS 实例 IP", tone: "info" };
  if (sName.startsWith("ns") || sName.startsWith("server") || sName.includes(".eu") || sName.includes(".net")) {
    return { label: "独服主 IP", tone: "info" };
  }
  if (!t || t === "unknown") return { label: "独立 IP", tone: "default" };
  return { label: item.type || "IP", tone: "default" };
}

function formatServerBadge(item: IpAsset) {
  const serviceName = item.routedTo?.serviceName;
  if (!serviceName) {
    return <span className="text-muted-foreground text-xs">未挂载</span>;
  }

  const sLower = serviceName.toLowerCase();
  const isVps = item.type === "vps" || sLower.startsWith("vps-") || sLower.includes(".vps.");
  const isDedicated = item.type === "dedicated" || sLower.startsWith("ns") || sLower.startsWith("server") || sLower.includes(".eu") || sLower.includes(".net");

  if (isVps) {
    return (
      <div className="flex items-center gap-1.5 min-w-0" title={serviceName}>
        <Cloud className="w-3.5 h-3.5 text-blue-500 flex-shrink-0" />
        <span className="text-[10px] px-1 py-0.5 rounded bg-blue-500/10 text-blue-600 font-semibold flex-shrink-0">
          VPS
        </span>
        <span className="font-mono text-xs text-foreground font-medium truncate">
          {serviceName}
        </span>
      </div>
    );
  }

  if (isDedicated) {
    return (
      <div className="flex items-center gap-1.5 min-w-0" title={serviceName}>
        <Server className="w-3.5 h-3.5 text-indigo-500 flex-shrink-0" />
        <span className="text-[10px] px-1 py-0.5 rounded bg-indigo-500/10 text-indigo-600 font-semibold flex-shrink-0">
          独服
        </span>
        <span className="font-mono text-xs text-foreground font-medium truncate">
          {serviceName}
        </span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1.5 min-w-0" title={serviceName}>
      <Building2 className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
      <span className="font-mono text-xs text-foreground font-medium truncate">
        {serviceName}
      </span>
    </div>
  );
}

function IpManagementPage() {
  const { data: accountsData } = useAccounts();
  const [activeAccount] = useActiveServerControlAccount();
  const accounts = accountsData?.accounts || [];

  const [selectedAccountId, setSelectedAccountId] = useState<string>("all");
  const { data: ips, isLoading, isFetching, refetch } = useIps(selectedAccountId);

  const [movingIp, setMovingIp] = useState<IpAsset | null>(null);
  const [reverseIp, setReverseIp] = useState<IpAsset | null>(null);
  const [firewallIp, setFirewallIp] = useState<IpAsset | null>(null);
  const [search, setSearch] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");

  const ipList = ips || [];
  const filtered = ipList.filter((item) => {
    if (typeFilter !== "all" && item.type !== typeFilter) return false;
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      const ip = item.ip.toLowerCase();
      const sName = (item.routedTo?.serviceName || "").toLowerCase();
      const accName = (item.accountName || "").toLowerCase();
      if (!ip.includes(q) && !sName.includes(q) && !accName.includes(q)) return false;
    }
    return true;
  });

  const totalFailover = ipList.filter((i) => i.type === "failover").length;
  const totalDedicated = ipList.filter((i) => i.type === "dedicated").length;
  const totalVps = ipList.filter((i) => i.type === "vps").length;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={Network}
        title="IP 资产中心"
        description="全局统揽各账户名下的独服主 IP、VPS 实例 IP 与 Failover 附加 IP 资产，并支持反向解析与硬件防火墙管理"
        action={
          <div className="flex items-center gap-2 flex-wrap">
            <Select value={selectedAccountId} onValueChange={setSelectedAccountId}>
              <SelectTrigger className="h-9 w-[220px] text-xs font-medium">
                <User className="w-3.5 h-3.5 mr-1.5 text-muted-foreground" />
                <SelectValue placeholder="选择所属账户..." />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">🌐 全部账户 (聚合视图)</SelectItem>
                {accounts.map((acc) => (
                  <SelectItem key={acc.id} value={acc.id}>
                    {acc.name} ({acc.zone || acc.endpoint})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching} className="h-9">
              <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isFetching ? "animate-spin" : ""}`} />
              刷新资产
            </Button>
          </div>
        }
      />

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <Card>
          <CardContent className="p-4 flex items-center justify-between">
            <div>
              <div className="text-xs text-muted-foreground">总 IP 块数</div>
              <div className="text-2xl font-bold font-mono">{ipList.length}</div>
            </div>
            <div className="w-10 h-10 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
              <Network className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4 flex items-center justify-between">
            <div>
              <div className="text-xs text-muted-foreground">独服主 IP</div>
              <div className="text-2xl font-bold font-mono text-indigo-500">{totalDedicated}</div>
            </div>
            <div className="w-10 h-10 rounded-xl bg-indigo-500/10 flex items-center justify-center text-indigo-500">
              <Server className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4 flex items-center justify-between">
            <div>
              <div className="text-xs text-muted-foreground">VPS 实例 IP</div>
              <div className="text-2xl font-bold font-mono text-blue-500">{totalVps}</div>
            </div>
            <div className="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center text-blue-500">
              <Cloud className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4 flex items-center justify-between">
            <div>
              <div className="text-xs text-muted-foreground">Failover (可漂移)</div>
              <div className="text-2xl font-bold font-mono text-amber-500">{totalFailover}</div>
            </div>
            <div className="w-10 h-10 rounded-xl bg-amber-500/10 flex items-center justify-center text-amber-500">
              <ArrowRightLeft className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardContent className="p-4">
          <div className="flex flex-col sm:flex-row gap-3 items-center justify-between">
            <div className="flex flex-col sm:flex-row gap-3 w-full sm:w-auto">
              <Input
                placeholder="搜索 IP、服务器名或所属账户..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="w-full sm:w-[260px] text-xs h-9"
              />
              <Select value={typeFilter} onValueChange={setTypeFilter}>
                <SelectTrigger className="w-full sm:w-[180px] text-xs h-9">
                  <SelectValue placeholder="所有类型" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">所有类型</SelectItem>
                  <SelectItem value="dedicated">独服主 IP</SelectItem>
                  <SelectItem value="vps">VPS 实例 IP</SelectItem>
                  <SelectItem value="failover">Failover (附加)</SelectItem>
                  <SelectItem value="cloud">Cloud IP</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <span className="text-xs text-muted-foreground self-start sm:self-center">
              共 <strong>{filtered.length}</strong> 条记录
            </span>
          </div>
        </CardContent>
      </Card>

      {isLoading ? (
        <Card>
          <CardContent className="p-6 space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-14 rounded-xl" />
            ))}
          </CardContent>
        </Card>
      ) : filtered.length === 0 ? (
        <Card>
          <EmptyState icon={Network} title="未找到匹配的 IP 资产" />
        </Card>
      ) : (
        <Card>
          <div className="table-scroll">
            <table className="w-full min-w-[720px]">
              <thead>
                <tr className="text-left text-[11px] font-medium text-muted-foreground border-b border-border">
                  <th className="px-4 py-3">IP / CIDR</th>
                  <th className="px-4 py-3">类型</th>
                  <th className="px-4 py-3">当前挂载目标</th>
                  {selectedAccountId === "all" && <th className="px-4 py-3">所属账户</th>}
                  <th className="px-4 py-3">区域/机房</th>
                  <th className="px-4 py-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border text-[13px]">
                {filtered.map((item) => {
                  const typeInfo = formatIpType(item);
                  const isFailover = item.type === "failover";
                  const itemAccountId = item.accountId || selectedAccountId;

                  return (
                    <tr key={item.ip} className="hover:bg-muted/40 transition-colors">
                      <td className="px-4 py-3 font-mono font-semibold">
                        {item.ip}
                      </td>
                      <td className="px-4 py-3">
                        <Chip tone={typeInfo.tone}>{typeInfo.label}</Chip>
                      </td>
                      <td className="px-4 py-3 max-w-[240px]">
                        {formatServerBadge(item)}
                      </td>
                      {selectedAccountId === "all" && (
                        <td className="px-4 py-3 text-xs">
                          <span className="font-medium text-foreground">
                            {item.accountName || "默认账户"}
                          </span>
                          {item.accountZone && (
                            <span className="text-muted-foreground ml-1 font-mono text-[10px]">
                              ({item.accountZone})
                            </span>
                          )}
                        </td>
                      )}
                      <td className="px-4 py-3 uppercase text-xs text-muted-foreground">
                        {item.country || item.region || (item as any).campus || "—"}
                      </td>
                      <td className="px-4 py-3 text-right space-x-1.5">
                        {isFailover && (
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 text-xs px-2 text-amber-600 hover:text-amber-700 hover:bg-amber-50 dark:text-amber-400 dark:hover:bg-amber-950/30"
                            onClick={() => setMovingIp(item)}
                          >
                            <ArrowRightLeft className="w-3 h-3 mr-1" />
                            漂移
                          </Button>
                        )}
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs px-2"
                          onClick={() => setReverseIp(item)}
                        >
                          <Globe className="w-3 h-3 mr-1 text-primary" />
                          PTR 解析
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs px-2 text-blue-600 hover:text-blue-700 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-950/30"
                          onClick={() => setFirewallIp(item)}
                        >
                          <Shield className="w-3 h-3 mr-1" />
                          防火墙
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* 跨机漂移弹窗 */}
      {movingIp && (
        <MoveIpDialog
          ip={movingIp.ip}
          currentService={movingIp.routedTo?.serviceName}
          accountId={movingIp.accountId || selectedAccountId}
          onClose={() => setMovingIp(null)}
        />
      )}

      {/* 反向解析 PTR 弹窗 */}
      {reverseIp && (
        <ReverseDnsDialog
          ip={reverseIp.ip}
          accountId={reverseIp.accountId || selectedAccountId}
          onClose={() => setReverseIp(null)}
        />
      )}

      {/* 硬件防火墙弹窗 */}
      {firewallIp && (
        <FirewallDialog
          ip={firewallIp.ip}
          accountId={firewallIp.accountId || selectedAccountId}
          onClose={() => setFirewallIp(null)}
        />
      )}
    </div>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>IP 资产中心 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <IpManagementPage />
    </AppLayout>
  </>
);

export default Page;
