import { useQuery } from "@tanstack/react-query";
import axios from "axios";
import { api } from "@/lib/http";
import { qk } from "@/lib/query";

export interface DatacenterInfo {
  datacenter: string;
  availability: string;
}

export interface AvailabilityItem {
  fqn: string;
  memory: string;
  planCode: string;
  server: string;
  storage: string;
  systemStorage?: string;
  datacenters: DatacenterInfo[];
}

/** 从默认 OVH 账户 endpoint（优先）或全局 settings 决定公开 API 区域 */
async function resolveAvailabilityEndpoint(): Promise<string> {
  try {
    const accRes = await api.get<{ accounts?: { isDefault?: boolean; endpoint?: string }[] }>("/accounts");
    const accounts = accRes.data?.accounts || [];
    const def = accounts.find((a) => a.isDefault) || accounts[0];
    if (def?.endpoint) return def.endpoint;
  } catch {
    /* fall through */
  }
  try {
    const res = await api.get("/settings");
    return res.data?.endpoint || "ovh-eu";
  } catch {
    return "ovh-eu";
  }
}

function endpointToPublicApiBase(endpoint: string): string {
  switch (endpoint) {
    case "ovh-us":
      return "https://api.us.ovhcloud.com";
    case "ovh-ca":
      return "https://ca.api.ovh.com";
    default:
      return "https://eu.api.ovh.com";
  }
}

/** 查询 OVH 公开 API 的实时可用性。
 *  - 1 分钟新鲜期：访问触发；过期才会再请求
 *  - 不做后台轮询：服务器列表页右上角"刷新"按钮会一并 refetch 这个 query
 *  - 切 tab / 切窗口都不会自动重发
 */
export function useAvailability() {
  return useQuery({
    queryKey: qk.availability.all("account-endpoint"),
    queryFn: async () => {
      const endpoint = await resolveAvailabilityEndpoint();
      const baseUrl = endpointToPublicApiBase(endpoint);
      const res = await axios.get<AvailabilityItem[]>(
        `${baseUrl}/v1/dedicated/server/datacenter/availabilities`,
        { timeout: 30000 }
      );
      return res.data;
    },
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

/**
 * 按 planCode 索引,保留所有 FQN 变体(不合并)。
 * 抢购对话框用它按用户当前选配实时算 DC 可用 + 给每个 option chip 上绿/红点。
 */
export function buildVariantIndex(
  items: AvailabilityItem[] | undefined
): Record<string, AvailabilityItem[]> {
  const out: Record<string, AvailabilityItem[]> = {};
  if (!items) return out;
  for (const item of items) {
    const pc = item.planCode;
    if (!pc) continue;
    (out[pc] ||= []).push(item);
  }
  return out;
}

/** FQN 第一段是 planCode,后面才是 addon 段 */
function fqnAddonParts(fqn: string): string[] {
  if (!fqn) return [];
  return fqn.split(".").slice(1);
}

/**
 * FQN 段 vs catalog option value 的匹配。
 * OVH availability 接口的 FQN 段是"短前缀",比如 `ram-128g-noecc-2933` / `softraid-4x3840nvme-pcie-gen4`,
 * 但 catalog 返回的 option.value 是带 plan 后缀的完整 planCode,比如 `ram-128g-noecc-2933-rise` /
 * `softraid-4x3840nvme-pcie-gen4-24adv01-v2`。
 * 直接相等匹配永远不命中 → 全红 bug。
 * 这里用双向前缀:相等 / 任一为对方 "x-" 前缀 都算 match,加 "-" 防 `ram-1` 错配 `ram-128g`。
 */
function partMatchesValue(part: string, value: string): boolean {
  if (!part || !value) return false;
  if (part === value) return true;
  if (value.startsWith(part + "-")) return true;
  if (part.startsWith(value + "-")) return true;
  return false;
}

/** variant 的 FQN 是否覆盖所有 required option value */
export function variantCoversAll(v: AvailabilityItem, required: string[]): boolean {
  if (required.length === 0) return true;
  const parts = fqnAddonParts(v.fqn);
  return required.every((val) => parts.some((p) => partMatchesValue(p, val)));
}

/** 找出 grouped options 里哪个 value 跟这个 FQN 段对得上(返回第一个,通常该组只有一个匹配) */
export function fqnMatchesOption(fqn: string, optionValue: string): boolean {
  const parts = fqnAddonParts(fqn);
  return parts.some((p) => partMatchesValue(p, optionValue));
}

/**
 * 这个 addon 是否出现在任何"有 DC 有货"的 FQN 里。
 * 语义:不强求其它组的当前选配匹配,只判断"换成这个 addon 后,理论上能找到某种组合在某 DC 有货"。
 * 用在 option chip 的绿/红点上,给用户"这个 addon 至少存在于某个有货组合中"的提示。
 * (DC 红绿用的 variantDcStatus 仍按当前完整选配判定,免得用户点完进队列又被 OVH 拒。)
 *
 * 兼容 hasStockWithOption 的旧签名以减少调用方改动;currentPicks / swapGroup 现在不参与判定。
 */
export function hasStockWithOption(
  variants: AvailabilityItem[] | undefined,
  _currentPicks: Record<string, string>,
  _swapGroup: string,
  candidate: string,
  dcCodes?: string[]
): boolean {
  if (!variants || variants.length === 0) return true;
  const filter = dcCodes && dcCodes.length > 0 ? new Set(dcCodes.map((c) => c.toLowerCase())) : null;
  for (const v of variants) {
    if (!fqnMatchesOption(v.fqn, candidate)) continue;
    for (const dc of v.datacenters || []) {
      const code = dc.datacenter?.toLowerCase();
      if (!code) continue;
      if (filter && !filter.has(code)) continue;
      const s = dc.availability;
      if (s && s !== "unavailable" && s !== "unknown") return true;
    }
  }
  return false;
}

/** 用户当前选配组合下,每个 DC 的真实状态 */
export function variantDcStatus(
  variants: AvailabilityItem[] | undefined,
  pickedAddons: string[]
): Record<string, string> {
  const out: Record<string, string> = {};
  if (!variants) return out;
  const req = pickedAddons.filter(Boolean);
  for (const v of variants) {
    if (!variantCoversAll(v, req)) continue;
    for (const dc of v.datacenters || []) {
      const code = dc.datacenter?.toLowerCase();
      if (!code) continue;
      const incoming = dc.availability;
      const existing = out[code];
      const isAvail = (s: string | undefined) => !!s && s !== "unavailable" && s !== "unknown";
      if (isAvail(existing)) continue;
      out[code] = incoming;
    }
  }
  return out;
}

/**
 * 把 OVH availabilities 数组聚合成 `{ [planCode]: { [dcCode]: status } }` 的查表。
 * 同一 planCode 下可能有多个 FQN 变体（不同 memory / storage），同 DC 取"最好的"那个：
 * 任一变体可用 → 标可用；都不可用 → unavailable；都缺 → unknown。
 */
export function buildAvailabilityMap(
  items: AvailabilityItem[] | undefined
): Record<string, Record<string, string>> {
  const out: Record<string, Record<string, string>> = {};
  if (!items) return out;
  for (const item of items) {
    const pc = item.planCode;
    if (!pc) continue;
    if (!out[pc]) out[pc] = {};
    for (const dc of item.datacenters || []) {
      const code = dc.datacenter?.toLowerCase();
      if (!code) continue;
      const existing = out[pc][code];
      const incoming = dc.availability;
      // 已经标记为可用就不被后续覆盖；否则用最新值
      const isAvail = (v: string | undefined) => !!v && v !== "unavailable" && v !== "unknown";
      if (isAvail(existing)) continue;
      out[pc][code] = incoming;
    }
  }
  return out;
}

// ─────────────────────────────── 价格计算（对齐 ovhjk/parser/price.go） ───────────────────────────────

export interface CatalogPricing {
  phase: number;
  description: string;
  interval: number;
  intervalUnit: string;
  price: number; // 微欧元（÷ 1e8 得欧元）
  tax: number; // 微欧元
  mode: string;
  capacities?: string[];
}

export interface CatalogAddonFamily {
  name: string;
  addons: string[];
  default?: string;
}

export interface CatalogPlan {
  planCode: string;
  invoiceName: string;
  product: string;
  pricings: CatalogPricing[];
  addonFamilies?: CatalogAddonFamily[];
}

export interface CatalogData {
  catalogId: number;
  locale: { currencyCode: string; subsidiary: string; taxRate: number };
  plans: CatalogPlan[];
  addons: CatalogPlan[];
}

export interface PriceInfo {
  /** 月费不含税（欧元 / 当地货币） */
  price: number;
  /** 月费税费 */
  tax: number;
  /** 月费含税 */
  total: number;
  /** 一次性安装费不含税 */
  installPrice: number;
  /** 安装费税费 */
  installTax: number;
  /** 货币代码 EUR / USD / CAD 等 */
  currency: string;
}

/**
 * 拉取 OVH 公共目录（每个 subsidiary 各自一份：不同币、不同税、不同促销价）。
 * - 走我们的后端 /api/catalog?subsidiary=XX：后端 SQLite 缓存 2 小时，
 *   首次拉完落库，之后 F5 / 新 tab / 换浏览器都能秒回（~10ms）。
 * - subsidiary 不传时后端按 config.Zone 兜底，前端不需要先调 settings。
 * - 缓存策略与 useServers 对齐：2 小时新鲜、24 小时 gc、不自动 refetch
 */
export function useOvhCatalog(subsidiary?: string) {
  return useQuery({
    queryKey: ["ovh-catalog", "eco", subsidiary || "auto"] as const,
    queryFn: async () => {
      const params: Record<string, string> = {};
      if (subsidiary) params.subsidiary = subsidiary;
      const res = await api.get<CatalogData>("/catalog", { params });
      return res.data;
    },
    staleTime: 2 * 60 * 60_000,
    gcTime: 24 * 60 * 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

/** 给定 catalog 构建按 planCode 索引的查表 + addon 查表 */
export interface CatalogIndex {
  planByCode: Record<string, CatalogPlan>;
  addonByCode: Record<string, CatalogPlan>;
  currency: string;
}
export function buildCatalogIndex(catalog: CatalogData | undefined): CatalogIndex {
  if (!catalog) return { planByCode: {}, addonByCode: {}, currency: "EUR" };
  const planByCode: Record<string, CatalogPlan> = {};
  for (const p of catalog.plans || []) planByCode[p.planCode] = p;
  const addonByCode: Record<string, CatalogPlan> = {};
  for (const a of catalog.addons || []) addonByCode[a.planCode] = a;
  return { planByCode, addonByCode, currency: catalog.locale?.currencyCode || "EUR" };
}

/** 月费:取 intervalUnit=month, interval=1, mode=default 且不是安装费的那条。
 *  注意:OVH 同一个 plan 的 pricings 里,安装费跟月费两条都满足 interval=1+month+default,
 *  只是 capacities 不一样(installation vs renew/[])。漏掉这个过滤会把安装费当月费,
 *  服务器列表卡片显示的价格被错误叠加(=月费+安装费)。 */
function monthlyPrice(pricings: CatalogPricing[] | undefined): { price: number; tax: number; ok: boolean } {
  if (!pricings) return { price: 0, tax: 0, ok: false };
  for (const pr of pricings) {
    if ((pr.capacities || []).includes("installation")) continue;
    if (pr.intervalUnit === "month" && pr.interval === 1 && pr.mode === "default") {
      return { price: pr.price / 1e8, tax: pr.tax / 1e8, ok: true };
    }
  }
  return { price: 0, tax: 0, ok: false };
}

/** 安装费：mode=default 且 capacities 含 'installation' */
function installationPrice(pricings: CatalogPricing[] | undefined): { price: number; tax: number } {
  if (!pricings) return { price: 0, tax: 0 };
  for (const pr of pricings) {
    if (pr.mode !== "default") continue;
    if ((pr.capacities || []).includes("installation")) {
      return { price: pr.price / 1e8, tax: pr.tax / 1e8 };
    }
  }
  return { price: 0, tax: 0 };
}

/** 在 family.addons 里按前缀匹配 fqn 维度，返回 addon planCode（旧 ovhjk 同款逻辑） */
function matchAddonCode(addons: string[], fqnDim: string): string {
  if (!fqnDim) return "";
  return addons.find((c) => c.startsWith(fqnDim)) || "";
}

/** 对应 family 在 FQN 里的维度值 */
function fqnDimensionForFamily(item: AvailabilityItem, familyName: string): string {
  switch (familyName) {
    case "memory":
      return item.memory || "";
    case "storage":
      return item.storage || "";
    case "system-storage":
      return item.systemStorage || "";
    default:
      return "";
  }
}

/** 计算单个 AvailabilityItem 的总价（base + 各 family 匹配的 addon） */
export function computePrice(item: AvailabilityItem | undefined, idx: CatalogIndex): PriceInfo | null {
  if (!item) return null;
  const plan = idx.planByCode[item.planCode];
  if (!plan) return null;

  const base = monthlyPrice(plan.pricings);
  if (!base.ok) return null;

  let totalPrice = base.price;
  let totalTax = base.tax;
  const baseInstall = installationPrice(plan.pricings);
  let installPrice = baseInstall.price;
  let installTax = baseInstall.tax;

  for (const fam of plan.addonFamilies || []) {
    const dim = fqnDimensionForFamily(item, fam.name);
    if (!dim) continue;
    const addonCode = matchAddonCode(fam.addons || [], dim);
    if (!addonCode) continue;
    const addon = idx.addonByCode[addonCode];
    if (!addon) continue;
    const ap = monthlyPrice(addon.pricings);
    if (ap.ok) {
      totalPrice += ap.price;
      totalTax += ap.tax;
    }
    const ai = installationPrice(addon.pricings);
    installPrice += ai.price;
    installTax += ai.tax;
  }

  return {
    price: totalPrice,
    tax: totalTax,
    total: totalPrice + totalTax,
    installPrice,
    installTax,
    currency: idx.currency,
  };
}

/**
 * 给一组 availability items + catalog，按 planCode 算出代表价（同 planCode 多变体时
 * 取第一个能算出的 item 的价格）。返回 `{ [planCode]: PriceInfo }`。
 */
export function buildPriceMap(
  items: AvailabilityItem[] | undefined,
  idx: CatalogIndex
): Record<string, PriceInfo> {
  const out: Record<string, PriceInfo> = {};
  if (!items) return out;
  for (const item of items) {
    if (out[item.planCode]) continue;
    const p = computePrice(item, idx);
    if (p) out[item.planCode] = p;
  }
  return out;
}

/**
 * 用用户选中的 addon planCode 列表直接算价：
 * 总价 = 基础 plan 月费 + 各 addon 月费（每个 addon 自带定价，按 planCode 查 catalog）
 *
 * 与 `computePrice` 的区别：后者用 FQN 维度前缀匹配 addon；这里调用方已经知道每个组挑了哪个 addonCode，
 * 直接累加更准确（用户切换内存 / 存储等组时实时反映）。
 */
export function computePriceFromOptions(
  planCode: string,
  selectedAddonCodes: string[],
  idx: CatalogIndex
): PriceInfo | null {
  const plan = idx.planByCode[planCode];
  if (!plan) return null;
  const base = monthlyPrice(plan.pricings);
  if (!base.ok) return null;
  let totalPrice = base.price;
  let totalTax = base.tax;
  const baseInstall = installationPrice(plan.pricings);
  let installPrice = baseInstall.price;
  let installTax = baseInstall.tax;
  for (const code of selectedAddonCodes) {
    if (!code) continue;
    const addon = idx.addonByCode[code];
    if (!addon) continue;
    const ap = monthlyPrice(addon.pricings);
    if (ap.ok) {
      totalPrice += ap.price;
      totalTax += ap.tax;
    }
    const ai = installationPrice(addon.pricings);
    installPrice += ai.price;
    installTax += ai.tax;
  }
  return {
    price: totalPrice,
    tax: totalTax,
    total: totalPrice + totalTax,
    installPrice,
    installTax,
    currency: idx.currency,
  };
}

/** 友好显示：€42.99/月 含税 €51.59/月 */
export function formatPrice(p: PriceInfo | undefined | null): string {
  if (!p) return "—";
  const sym = p.currency === "EUR" ? "€" : p.currency === "USD" ? "$" : p.currency === "CAD" ? "CA$" : p.currency + " ";
  return `${sym}${p.price.toFixed(2)} / 月`;
}
