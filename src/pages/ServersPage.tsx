import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import {
  Server, RefreshCw, Search, Bell, ShoppingCart, Cpu, MemoryStick, HardDrive, Wifi,
  Filter, X, Layers, Zap, TrendingUp, ShieldCheck, Database, Eye,
} from "lucide-react";
import { useMemo, useState } from "react";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Chip } from "@/components/common/Chip";
import { StatusDot } from "@/components/common/StatusDot";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { useServers, type ServerPlan } from "@/hooks/use-servers";
import { useMonitorList } from "@/hooks/use-monitor";
import { MonitorSubscribeDialog } from "@/components/monitor/MonitorSubscribeDialog";
import { useAccountInfo } from "@/hooks/use-account";
import { useCreateQueueItem } from "@/hooks/use-queue";
import { useCacheInfo } from "@/hooks/use-settings";
import { useDefaultAccount } from "@/hooks/use-accounts";
import { AccountSelect } from "@/components/common/AccountSelect";
import { useEffect } from "react";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";
import {
  useAvailability,
  buildAvailabilityMap,
  buildVariantIndex,
  hasStockWithOption,
  useOvhCatalog,
  buildCatalogIndex,
  computePriceFromOptions,
  formatPrice,
  type AvailabilityItem,
  type CatalogIndex,
  type PriceInfo,
} from "@/hooks/use-availability";
import { groupOptions, type OptionGroupKey } from "@/lib/option-groups";
import { OptionGroupSection } from "@/components/common/OptionGroupSection";
import { OVH_DATACENTERS, isDcInStock, lookupDcStatus, mergeDcAvailability } from "@/lib/datacenters";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { OVH_SUBSIDIARIES } from "@/lib/ovh-subsidiaries";

/** 服务器列表：卡片网格 + 详情弹窗 */
/** localStorage key：用户手动选过的 subsidiary（持久化跨刷新） */
const SUB_LS_KEY = "ovh_sniper_price_subsidiary";
const SUB_MANUAL_LS_KEY = "ovh_sniper_price_subsidiary_manual";

function ServersPage() {
  const q = useServers();
  // 单次拉取 OVH 公开可用性接口（一条请求拿到所有 planCode × 所有 DC 的状态）
  const availQ = useAvailability();
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);
  // FQN 级索引,抢购对话框按当前选配实时算 DC 可用 + option 绿红点
  const variantIndex = useMemo(() => buildVariantIndex(availQ.data), [availQ.data]);

  // OVH 账户信息：拿 ovhSubsidiary 作为默认价格地区
  const account = useAccountInfo();
  const accountSub = account.data?.ovhSubsidiary;

  // 价格地区（默认跟账户走；用户手动改过后用本地存的）
  const [subsidiary, setSubsidiary] = useState<string>(() => {
    try {
      const manualPicked = localStorage.getItem(SUB_MANUAL_LS_KEY) === "1";
      if (manualPicked) return localStorage.getItem(SUB_LS_KEY) || "IE";
    } catch { /* ignore */ }
    return "IE";
  });

  // 账户子公司返回后，若用户从未手动改过，自动同步成账户的
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
    } catch { /* 隐私模式忽略 */ }
  };
  const resetSubsidiaryToAccount = () => {
    try {
      localStorage.removeItem(SUB_MANUAL_LS_KEY);
      localStorage.removeItem(SUB_LS_KEY);
    } catch { /* ignore */ }
    if (accountSub) setSubsidiary(accountSub);
  };

  // 单次拉取所选 subsidiary 的目录算价格（base plan + addon family 月费累加）
  const catalogQ = useOvhCatalog(subsidiary);
  const catalogIdx = useMemo(() => buildCatalogIndex(catalogQ.data), [catalogQ.data]);
  // 卡片显示价格用每台服务器的 catalog defaultOptions 算,跟详情对话框打开时的初始价格一致。
  // 旧的 buildPriceMap 走 FQN 维度前缀匹配,跟 catalog 默认值可能挑到不同 addon → 卡片价跟弹窗价对不上。
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
  const [selectedCategory, setSelectedCategory] = useState<string>("all");
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

  const list = q.data || [];
  const filtered = useMemo(() => {
    const s = search.trim().toLowerCase();
    let out = list;
    if (s) {
      out = out.filter((srv) =>
        `${srv.planCode} ${srv.name} ${srv.cpu} ${srv.memory} ${srv.storage}`.toLowerCase().includes(s)
      );
    }
    if (selectedCategory !== "all") {
      out = out.filter((srv) => {
        const code = srv.planCode.toLowerCase();
        if (selectedCategory === "ks") {
          return code.startsWith("24sk") || code.startsWith("ks-") || code.includes("kimsufi");
        }
        if (selectedCategory === "rise") {
          return code.startsWith("24rise") || code.startsWith("rise-");
        }
        if (selectedCategory === "adv") {
          return code.startsWith("24adv") || code.startsWith("adv-") || code.startsWith("advance-") || code.startsWith("scale-") || code.startsWith("hgr-");
        }
        if (selectedCategory === "sys") {
          return code.startsWith("24stor") || code.startsWith("stor-") || code.startsWith("sys-") || code.startsWith("game-");
        }
        if (selectedCategory === "mon") {
          return monitoredCodes.has(srv.planCode);
        }
        return true;
      });
    }
    if (onlyAvailable) {
      out = out.filter((srv) => {
        const map = availMap[srv.planCode];
        if (map) {
          // 实时数据：任一 DC 可用即视为可用
          return Object.values(map).some((v) => v && v !== "unavailable" && v !== "unknown");
        }
        // 实时还没到：用目录里的静态字段兜底
        return srv.datacenters.some((dc) => dc.availability && dc.availability !== "unavailable" && dc.availability !== "unknown");
      });
    }
    return out;
  }, [list, search, onlyAvailable, selectedCategory, monitoredCodes, availMap]);

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
              variant="outline"
              onClick={() => {
                // 一键刷三件套：目录强刷（清后端缓存）、catalog（价格）refetch、可用性 refetch
                q.forceRefresh();
                catalogQ.refetch();
                availQ.refetch();
              }}
              // 只看手动刷新状态：q.isRefreshing 是 forceRefresh 期间的 mutation pending；
              // *Q.isRefetching 是 refetch 后的状态。不引入 isFetching/isLoading，
              // 这样首次加载的菊花不会显示在这个按钮上，避免误导。
              disabled={q.isRefreshing || catalogQ.isRefetching || availQ.isRefetching}
            >
              <RefreshCw
                className={`w-4 h-4 ${
                  q.isRefreshing || catalogQ.isRefetching || availQ.isRefetching
                    ? "animate-spin"
                    : ""
                }`}
              />
              刷新
            </Button>
          </div>
        }
      />

      {/* 工具条 */}
      <Card>
        <CardContent className="p-4 flex flex-col sm:flex-row sm:items-center gap-3">
          <div className="relative flex-1 min-w-0">
            <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
            <Input
              placeholder="搜索 planCode / 型号 / CPU / 内存..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9 pr-9 rounded-full"
            />
            {search && (
              <button
                type="button"
                className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full p-0.5 text-muted-foreground hover:text-foreground transition-colors"
                onClick={() => setSearch("")}
                aria-label="清空搜索"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
          <Button
            variant={onlyAvailable ? "default" : "outline"}
            size="sm"
            className="rounded-full"
            onClick={() => setOnlyAvailable((v) => !v)}
          >
            <Filter className="w-3.5 h-3.5" />
            仅显示可用
          </Button>
          {/* 价格地区：每个 subsidiary 独立目录、独立币种、独立税率 */}
          <div className="flex items-center gap-1.5">
            <select
              value={subsidiary}
              onChange={(e) => changeSubsidiary(e.target.value)}
              className="h-9 rounded-full border border-border bg-background px-3 text-[12px] font-medium focus:outline-none focus:ring-1 focus:ring-primary/40 focus:border-primary/60 transition-colors w-full sm:max-w-[260px]"
              title={
                accountSub
                  ? `价格地区。账户当前绑定 ${accountSub}，实际下单按账户结算`
                  : "切换价格地区（subsidiary 决定货币 / 税率 / 实际价格）"
              }
            >
              {OVH_SUBSIDIARIES.map((s) => (
                <option key={s.code} value={s.code}>
                  {s.code} · {s.label}
                  {accountSub === s.code ? " · 我的账户" : ""}
                </option>
              ))}
            </select>
            {accountSub && subsidiary !== accountSub && (
              <Button
                variant="outline"
                size="sm"
                className="h-9 rounded-full text-[11px]"
                onClick={resetSubsidiaryToAccount}
                title={`回到账户绑定的子公司 ${accountSub}`}
              >
                回到 {accountSub}
              </Button>
            )}
          </div>
          <span className="text-[12px] text-muted-foreground whitespace-nowrap">
            {q.isPending ? "加载中..." : `共 ${filtered.length} 款`}
          </span>
        </CardContent>
        {/* 系列分类胶囊选择 */}
        <div className="px-4 py-2.5 bg-muted/20 border-t border-border/50 flex items-center gap-1.5 overflow-x-auto scrollbar-none">
          {[
            {
              id: "all",
              label: "全部型号",
              icon: Layers,
              count: list.length,
            },
            {
              id: "ks",
              label: "Kimsufi / KS",
              icon: Zap,
              count: list.filter(
                (s) =>
                  s.planCode.toLowerCase().startsWith("24sk") ||
                  s.planCode.toLowerCase().startsWith("ks-") ||
                  s.planCode.toLowerCase().includes("kimsufi")
              ).length,
            },
            {
              id: "rise",
              label: "Rise",
              icon: TrendingUp,
              count: list.filter(
                (s) =>
                  s.planCode.toLowerCase().startsWith("24rise") ||
                  s.planCode.toLowerCase().startsWith("rise-")
              ).length,
            },
            {
              id: "adv",
              label: "Advance",
              icon: ShieldCheck,
              count: list.filter(
                (s) =>
                  s.planCode.toLowerCase().startsWith("24adv") ||
                  s.planCode.toLowerCase().startsWith("adv-") ||
                  s.planCode.toLowerCase().startsWith("scale-") ||
                  s.planCode.toLowerCase().startsWith("hgr-")
              ).length,
            },
            {
              id: "sys",
              label: "SYS / 存储",
              icon: Database,
              count: list.filter(
                (s) =>
                  s.planCode.toLowerCase().startsWith("24stor") ||
                  s.planCode.toLowerCase().startsWith("sys-") ||
                  s.planCode.toLowerCase().startsWith("stor-") ||
                  s.planCode.toLowerCase().startsWith("game-")
              ).length,
            },
            {
              id: "mon",
              label: "已监控",
              icon: Eye,
              count: list.filter((s) => monitoredCodes.has(s.planCode)).length,
            },
          ].map((cat) => {
            const Icon = cat.icon;
            const isSelected = selectedCategory === cat.id;
            const isMon = cat.id === "mon";

            return (
              <button
                key={cat.id}
                type="button"
                onClick={() => setSelectedCategory(cat.id)}
                className={cn(
                  "group relative inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-150 select-none whitespace-nowrap border",
                  isSelected
                    ? "bg-background text-foreground shadow-sm border-border/80 font-semibold ring-1 ring-border/50"
                    : "bg-transparent text-muted-foreground/85 border-transparent hover:text-foreground hover:bg-background/60"
                )}
              >
                <Icon
                  className={cn(
                    "w-3.5 h-3.5 transition-colors",
                    isSelected
                      ? isMon
                        ? "text-emerald-500 dark:text-emerald-400"
                        : "text-foreground"
                      : isMon && cat.count > 0
                      ? "text-emerald-500/80 dark:text-emerald-400/80"
                      : "text-muted-foreground/70 group-hover:text-foreground"
                  )}
                />
                <span>{cat.label}</span>
                <span
                  className={cn(
                    "inline-flex items-center justify-center min-w-[20px] h-[18px] px-1.5 rounded-full text-[11px] tabular-nums font-medium border transition-colors",
                    isSelected
                      ? isMon
                        ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 font-bold"
                        : "bg-secondary text-foreground border-border font-bold"
                      : isMon && cat.count > 0
                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 font-semibold"
                      : cat.count === 0
                      ? "bg-transparent text-muted-foreground/40 border-transparent"
                      : "bg-secondary/60 text-muted-foreground border-border/40"
                  )}
                >
                  {cat.count}
                </span>
              </button>
            );
          })}
        </div>
      </Card>

      {/* 网格 */}
      {q.isPending ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-[260px] rounded-2xl" />
          ))}
        </div>
      ) : filtered.length === 0 ? (
        <Card>
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

      {/* 详情弹窗 */}
      <Dialog open={!!detailServer} onOpenChange={(v) => !v && setDetailPlanCode(null)}>
        <DialogContent className="w-[95vw] sm:w-full sm:max-w-3xl max-h-[90vh] overflow-hidden flex flex-col">
          {detailServer ? (
            <DetailContent
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
          ) : null}
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

/** 服务器卡片 */
function ServerCard({
  server,
  realtimeDcMap,
  price,
  isMonitored,
  onView,
  onMonitor,
}: {
  server: ServerPlan;
  realtimeDcMap?: Record<string, string>;
  price?: PriceInfo;
  isMonitored?: boolean;
  onView: () => void;
  onMonitor: () => void;
}) {

  const dcMap = useMemo(
    () => mergeDcAvailability(server.datacenters, realtimeDcMap),
    [server.datacenters, realtimeDcMap]
  );

  const dcStatuses = OVH_DATACENTERS.map((dc) => ({
    dc,
    isOk: isDcInStock(lookupDcStatus(dcMap, dc)),
  }));
  const total = dcStatuses.length;
  const okCount = dcStatuses.filter((s) => s.isOk).length;

  const tone = okCount > 0 ? "success" : "danger";
  const statusText = okCount > 0 ? `${okCount}/${total} 可用` : "暂时缺货";

  return (
    <Card className="overflow-hidden transition-colors hover:bg-secondary/30">
      <CardContent className="p-5 flex flex-col gap-4">
        {/* 头部：planCode + 状态 chip */}
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <h3 className="font-mono text-[15px] font-semibold truncate">{server.planCode}</h3>
            <p className="text-[12px] text-muted-foreground truncate mt-0.5">{server.name}</p>
            <div className="text-[13px] font-semibold mt-1 tabular-nums">
              {price ? (
                formatPrice(price)
              ) : (
                <span className="text-muted-foreground font-normal">— · 价格加载中</span>
              )}
            </div>
          </div>
          <Chip tone={tone as any}>
            {okCount > 0 ? (
              <StatusDot tone="success" pulse size="xs" />
            ) : (
              <StatusDot tone="danger" size="xs" />
            )}
            {statusText}
          </Chip>
        </div>

        {/* 规格 2x2 */}
        <div className="grid grid-cols-2 gap-2 text-[12px]">
          <SpecRow icon={<Cpu className="w-3.5 h-3.5" />} text={server.cpu} />
          <SpecRow icon={<MemoryStick className="w-3.5 h-3.5" />} text={server.memory} />
          <SpecRow icon={<HardDrive className="w-3.5 h-3.5" />} text={server.storage} />
          <SpecRow icon={<Wifi className="w-3.5 h-3.5" />} text={server.bandwidth} />
        </div>

        {/* DC 点阵：12 个标准 OVH DC，只两态 — 绿色有货 / 红色缺货 */}
        <div className="flex flex-wrap items-center gap-1.5 py-1">
          {dcStatuses.map(({ dc, isOk }) => (
            <span
              key={dc.code}
              title={`${dc.name} · ${dc.region}`}
              className="inline-flex items-center gap-1 px-1.5 h-5 rounded-full border border-border text-[10px] font-mono"
            >
              <StatusDot tone={isOk ? "success" : "danger"} size="xs" pulse={isOk} />
              {dc.code.toUpperCase()}
            </span>
          ))}
        </div>

        {/* 操作按钮 */}
        <div className="flex items-center gap-2 pt-1">
          <Button
            variant="outline"
            size="sm"
            className="flex-1"
            onClick={onMonitor}
          >
            <Bell className="w-3.5 h-3.5" />
            {isMonitored ? "监控设置" : "监控"}
          </Button>
          <Button size="sm" className="flex-1" onClick={onView}>
            <ShoppingCart className="w-3.5 h-3.5" />
            抢购
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

/** 单行规格（icon + 文本） */
function SpecRow({ icon, text }: { icon: React.ReactNode; text: string }) {
  return (
    <div className="flex items-center gap-1.5 min-w-0 text-foreground/80">
      <span className="text-muted-foreground flex-shrink-0">{icon}</span>
      <span className="truncate" title={text}>{text}</span>
    </div>
  );
}

/** 详情弹窗内容 */
function DetailContent({
  server,
  realtimeDcMap,
  variants,
  defaultPrice,
  catalogIdx,
  subsidiary,
  isMonitored,
  onClose,
  onMonitor,
}: {
  server: ServerPlan;
  realtimeDcMap?: Record<string, string>;
  /** 此 planCode 在 OVH availability 接口里的所有 FQN 变体 */
  variants?: AvailabilityItem[];
  /** 用默认配置算出的代表价，作为用户尚未变动时的兜底显示 */
  defaultPrice?: PriceInfo;
  /** 目录索引：用户切配置时实时算价用 */
  catalogIdx: CatalogIndex;
  /** 仅用于价格展示的 subsidiary（顶部下拉决定）。实际下单 subsidiary 由后端 cfg.Zone 决定，在设置页改 */
  subsidiary: string;
  isMonitored?: boolean;
  onClose: () => void;
  onMonitor: (datacenters: string[]) => void;
}) {
  const create = useCreateQueueItem();
  const defaultAcc = useDefaultAccount();

  // 抢购表单状态：DC 多选 + 数量 + 重试间隔 + 账户
  const [accountId, setAccountId] = useState("");
  useEffect(() => {
    if (!accountId && defaultAcc) setAccountId(defaultAcc.id);
  }, [defaultAcc?.id, accountId]);
  const [selectedDCs, setSelectedDCs] = useState<string[]>([]);
  const [quantity, setQuantity] = useState("1");
  const [retryInterval, setRetryInterval] = useState("60");
  const qty = Math.max(1, Number(quantity) || 1);
  const totalTasks = selectedDCs.length * qty;
  const dcMap = useMemo(
    () => mergeDcAvailability(server.datacenters, realtimeDcMap),
    [server.datacenters, realtimeDcMap]
  );

  // 按组拆分可选配置 + 默认值集合
  const grouped = useMemo(() => groupOptions(server.availableOptions), [server.availableOptions]);
  const defaultValueSet = useMemo(
    () => new Set((server.defaultOptions || []).map((o) => o.value)),
    [server.defaultOptions]
  );

  // 各组的当前选中值（按 group key 索引）。默认从 catalog 的 defaultOptions 里取该组里命中的那个 value。
  // 用户切配置后,每个 option chip / 每个 DC 的红绿点会实时反映"这套组合是否有 DC 有货",
  // 用户看到红就自己换 —— 不替用户自动改默认值。
  const initialPicked = useMemo(() => {
    const out: Partial<Record<OptionGroupKey, string>> = {};
    (Object.keys(grouped) as OptionGroupKey[]).forEach((g) => {
      const list = grouped[g];
      if (list.length === 0) return;
      const def = list.find((o) => defaultValueSet.has(o.value));
      if (def) out[g] = def.value;
    });
    return out;
  }, [grouped, defaultValueSet]);
  const [picked, setPicked] = useState<Partial<Record<OptionGroupKey, string>>>(initialPicked);

  const ok = OVH_DATACENTERS.filter((dc) => isDcInStock(lookupDcStatus(dcMap, dc))).length;

  // option chip 上的有货预判。
  // OVH availability FQN 只包含 planCode.memory.storage[.systemStorage] 三段,
  // 带宽 / vRack / CPU / other 这些 addon 不在 FQN 里 → 它们的库存跟主机解耦,
  // 主机有货就总能加购,这些组固定绿,不参与 FQN 匹配。
  const optionHasStock = (groupKey: OptionGroupKey, value: string): boolean => {
    if (groupKey === "bandwidth" || groupKey === "vrack" || groupKey === "cpu" || groupKey === "other") {
      return true;
    }
    return hasStockWithOption(variants, picked as Record<string, string>, groupKey, value);
  };

  // 用户选中的所有 option value（非默认值才计入，让 Queue 表单只填差异化部分；
  // 但保险起见全量传过去，让后端忽略相同默认值即可）
  const selectedValues = useMemo(
    () => (Object.values(picked).filter(Boolean) as string[]),
    [picked]
  );

  // 跟随选配实时算价：base plan + 选中的各 addon 月费
  const price = useMemo(() => {
    if (selectedValues.length === 0) return defaultPrice;
    return computePriceFromOptions(server.planCode, selectedValues, catalogIdx) || defaultPrice;
  }, [server.planCode, selectedValues, catalogIdx, defaultPrice]);

  return (
    <>
      <DialogHeader>
        <div className="flex items-start justify-between gap-3 pr-6">
          <div className="min-w-0">
            <DialogTitle className="font-mono text-xl truncate">{server.planCode}</DialogTitle>
            <DialogDescription className="truncate mt-0.5">{server.name}</DialogDescription>
          </div>
          {ok > 0 ? (
            <Chip tone="success"><StatusDot tone="success" pulse size="xs" />当前可用</Chip>
          ) : (
            <Chip tone="danger"><StatusDot tone="danger" size="xs" />暂时缺货</Chip>
          )}
        </div>
      </DialogHeader>

      <div className="overflow-y-auto -mx-6 px-6 space-y-6 flex-1">
        {/* 价格 Hero（随下方配置实时变化） */}
        <div className="border border-border rounded-2xl p-4 bg-secondary/30 flex items-end justify-between gap-3 flex-wrap">
          <div>
            <div className="text-[11px] text-muted-foreground">
              月费 · {subsidiary}
              <span className="ml-2 text-[10px]">
                {selectedValues.length > 0 ? "（随当前选配）" : "（默认配置）"}
              </span>
            </div>
            <div className="text-2xl font-bold tabular-nums mt-0.5">
              {price ? formatPrice(price) : <span className="text-muted-foreground font-normal text-base">— · 价格加载中</span>}
            </div>
          </div>
          {price && (
            <div className="text-right text-[11px] text-muted-foreground space-y-0.5 tabular-nums">
              {price.installPrice > 0 && (
                <div>安装费 {fmtMoney(price.installPrice, price.currency)}（一次性）</div>
              )}
              <div>币种 {price.currency}</div>
            </div>
          )}
        </div>

        {/* 规格 4 卡 */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5 sm:gap-3">
          <SpecCard icon={<Cpu className="w-4 h-4" />} label="CPU" value={server.cpu} />
          <SpecCard icon={<MemoryStick className="w-4 h-4" />} label="内存" value={server.memory} />
          <SpecCard icon={<HardDrive className="w-4 h-4" />} label="硬盘" value={server.storage} />
          <SpecCard icon={<Wifi className="w-4 h-4" />} label="带宽" value={server.bandwidth} />
        </div>

        {/* 硬件配置选择 */}
        {(["cpu", "memory", "systemStorage", "storage", "bandwidth", "vrack", "other"] as OptionGroupKey[])
          .filter((g) => grouped[g].length > 0)
          .map((g) => (
            <OptionGroupSection
              key={g}
              groupKey={g}
              options={grouped[g]}
              picked={picked[g] || ""}
              defaultValueSet={defaultValueSet}
              hasStock={variants && variants.length > 0 ? (value) => optionHasStock(g, value) : undefined}
              onPick={(value) => setPicked((p) => ({ ...p, [g]: value }))}
            />
          ))}

        <DatacenterPicker value={selectedDCs} onChange={setSelectedDCs} availability={dcMap} />

        {/* 抢购参数：账户 / 数量 / 重试间隔 */}
        <div className="border-t border-border pt-4">
          <h3 className="text-[13px] font-semibold mb-2.5 flex items-center gap-1.5">
            <ShoppingCart className="w-3.5 h-3.5 text-muted-foreground" />
            抢购参数
          </h3>
          <div className="space-y-3">
            <div>
              <label className="block text-[11px] text-muted-foreground mb-1">OVH 账户 *</label>
              <AccountSelect value={accountId} onChange={setAccountId} />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-[11px] text-muted-foreground mb-1">每个数据中心数量</label>
                <Input
                  type="number"
                  min={1}
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                />
              </div>
              <div>
                <label className="block text-[11px] text-muted-foreground mb-1">重试间隔（秒）</label>
                <Input
                  type="number"
                  min={10}
                  value={retryInterval}
                  onChange={(e) => setRetryInterval(e.target.value)}
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <DialogFooter className="border-t border-border pt-4 -mx-6 px-6">
        <div className="mr-auto text-[12px] text-muted-foreground">
          {selectedDCs.length > 0
            ? `将创建 ${totalTasks} 个任务（${selectedDCs.length} DC × ${qty}）${selectedValues.length > 0 ? ` · ${selectedValues.length} 项选配` : ""}`
            : "请选数据中心"}
        </div>
        <Button variant="outline" onClick={onClose} disabled={create.isPending}>
          关闭
        </Button>
        <Button
          variant="outline"
          disabled={create.isPending}
          onClick={() => onMonitor(selectedDCs)}
        >
          <Bell className="w-4 h-4" />
          {isMonitored ? "监控设置" : "加入监控"}
        </Button>
        <Button
          disabled={selectedDCs.length === 0 || create.isPending}
          onClick={async () => {
            if (selectedDCs.length === 0) {
              toast.error("请至少选择一个数据中心");
              return;
            }
            if (!accountId) {
              toast.error("请选择 OVH 账户");
              return;
            }
            const result = await create.mutateAsync({
              account_id: accountId,
              planCode: server.planCode,
              datacenters: selectedDCs,
              quantity: qty,
              retryInterval: Number(retryInterval) || 60,
              options: selectedValues,
            });
            if (result.success > 0) {
              toast.success(`已创建 ${result.success}/${result.total} 个抢购任务`);
              onClose();
            }
            if (result.failed > 0) {
              toast.error(`${result.failed} 个任务创建失败`);
            }
          }}
        >
          {create.isPending ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              创建中…
            </>
          ) : (
            <>
              <ShoppingCart className="w-4 h-4" />
              {selectedDCs.length > 0 ? `创建 ${totalTasks} 个任务` : "创建抢购任务"}
            </>
          )}
        </Button>
      </DialogFooter>
    </>
  );
}


/** 简单货币格式化（不需要全名时） */
function fmtMoney(v: number, currency: string): string {
  const sym = currency === "EUR" ? "€" : currency === "USD" ? "$" : currency === "GBP" ? "£" : currency === "CAD" ? "CA$" : `${currency} `;
  return `${sym}${v.toFixed(2)}`;
}

function SpecCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="border border-border rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0">
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center text-foreground flex-shrink-0">
        {icon}
      </div>
      <div className="min-w-0">
        <div className="text-[11px] text-muted-foreground">{label}</div>
        <div className="text-[13px] font-semibold truncate" title={value}>{value}</div>
      </div>
    </div>
  );
}

/** 服务器目录缓存状态徽章：基于 /api/cache/info 显示当前数据是几分钟前的缓存还是已过期 */
function CacheBadge() {
  const info = useCacheInfo();
  const backend = info.data?.backend;
  if (!backend || !backend.hasCachedData) {
    return <span className="text-[11px] text-muted-foreground">尚未加载</span>;
  }
  const ageSec = backend.cacheAge ?? 0;
  const valid = !!backend.cacheValid;

  let text: string;
  if (ageSec < 60) {
    text = `${ageSec} 秒前`;
  } else if (ageSec < 3600) {
    text = `${Math.floor(ageSec / 60)} 分钟前`;
  } else {
    const h = Math.floor(ageSec / 3600);
    const m = Math.floor((ageSec % 3600) / 60);
    text = m > 0 ? `${h} 小时 ${m} 分钟前` : `${h} 小时前`;
  }

  return (
    <span
      className={`inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] border ${
        valid
          ? "border-border text-muted-foreground bg-muted/40"
          : "border-amber-500/30 text-amber-700 dark:text-amber-300 bg-amber-50/60 dark:bg-amber-950/30"
      }`}
      title={
        valid
          ? "数据来自缓存，过期后再次访问才会重新调 OVH"
          : "缓存已过期，下次访问或点刷新会调 OVH 拉新数据"
      }
    >
      {valid ? "缓存" : "缓存已过期"} · {text}
    </span>
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
