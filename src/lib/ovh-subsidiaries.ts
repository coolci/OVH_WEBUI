import { endpointRegion } from "@/lib/ovh-regions";
import i18n from "@/i18n";

/**
 * OVH 子公司列表（每个 subsidiary 对应独立目录 + 独立币种 + 独立税率）。
 * - endpoint 决定 API host（eu / us / ca）
 * - subsidiary 决定使用哪份目录、用什么币、加多少税
 */
export interface OvhSubsidiary {
  code: string;
  endpoint: "ovh-eu" | "ovh-us" | "ovh-ca";
  /** 中文标签（地区名 + 币种 + VAT/GST 提示） */
  label: string;
  currency: string;
}

/**
 * 常见的 OVH subsidiary。
 * label 只写国家 + 币种，**税率不在这里写死**，因为 catalog 接口本身返回 locale.taxRate，
 * 用 API 实际值更准（IE 实测是 23% 不是 0%，CA 实测是 0% 不是 GST/PST 另计等）。
 */
export const OVH_SUBSIDIARIES: OvhSubsidiary[] = [
  // 欧洲（eu.api.ovh.com）
  { code: "IE", endpoint: "ovh-eu", label: "爱尔兰 · EUR", currency: "EUR" },
  { code: "FR", endpoint: "ovh-eu", label: "法国 · EUR", currency: "EUR" },
  { code: "DE", endpoint: "ovh-eu", label: "德国 · EUR", currency: "EUR" },
  { code: "GB", endpoint: "ovh-eu", label: "英国 · GBP", currency: "GBP" },
  { code: "IT", endpoint: "ovh-eu", label: "意大利 · EUR", currency: "EUR" },
  { code: "ES", endpoint: "ovh-eu", label: "西班牙 · EUR", currency: "EUR" },
  { code: "PL", endpoint: "ovh-eu", label: "波兰 · PLN", currency: "PLN" },
  { code: "NL", endpoint: "ovh-eu", label: "荷兰 · EUR", currency: "EUR" },
  { code: "PT", endpoint: "ovh-eu", label: "葡萄牙 · EUR", currency: "EUR" },
  { code: "FI", endpoint: "ovh-eu", label: "芬兰 · EUR", currency: "EUR" },
  { code: "CZ", endpoint: "ovh-eu", label: "捷克 · EUR", currency: "EUR" },
  { code: "LT", endpoint: "ovh-eu", label: "立陶宛 · EUR", currency: "EUR" },
  // MA / TN / SN 属于**欧区站点**(eu.api.ovh.com),不是加区 —— 实测拿它们请求
  // ca.api.ovh.com 会 400。这三个的计价币种也各不相同,别想当然填 EUR。
  { code: "MA", endpoint: "ovh-eu", label: "摩洛哥 · MAD", currency: "MAD" },
  { code: "TN", endpoint: "ovh-eu", label: "突尼斯 · TND", currency: "TND" },
  { code: "SN", endpoint: "ovh-eu", label: "塞内加尔 · XOF", currency: "XOF" },

  // 北美（api.us.ovhcloud.com）。US 独立产品线（plans 137 / addons 768，比 EU 多 40%）
  { code: "US", endpoint: "ovh-us", label: "美国 · USD（独立产品线）", currency: "USD" },

  // 加拿大 / 亚太（ca.api.ovh.com）
  { code: "CA", endpoint: "ovh-ca", label: "加拿大 · CAD", currency: "CAD" },
  { code: "QC", endpoint: "ovh-ca", label: "魁北克 · CAD", currency: "CAD" },
  { code: "ASIA", endpoint: "ovh-ca", label: "亚太 · USD", currency: "USD" },
  { code: "SG", endpoint: "ovh-ca", label: "新加坡 · SGD", currency: "SGD" },
  { code: "AU", endpoint: "ovh-ca", label: "澳大利亚 · AUD", currency: "AUD" },
  { code: "IN", endpoint: "ovh-ca", label: "印度 · INR", currency: "INR" },
  // WE / WS 是两个 USD 计价的子公司,属于**加区站点** —— 实测在 eu 站点请求会 400
  { code: "WE", endpoint: "ovh-ca", label: "国际站·英语(WE) · USD", currency: "USD" },
  { code: "WS", endpoint: "ovh-ca", label: "国际站·西语(WS) · USD", currency: "USD" },
];

/** 子公司标签（国家 + 币种，跟随语言），用于下拉框等展示位。
 *  静态的 label 字段仍是中文默认值,给没接 i18n 的旧调用方兜底。 */
/** 子公司代码 → 它所在的 API endpoint;不认识的按欧区 */
export function endpointForZone(zone: string): string {
  return OVH_SUBSIDIARIES.find((s) => s.code === zone)?.endpoint || "ovh-eu";
}

export function subsidiaryLabel(code: string): string {
  const hit = OVH_SUBSIDIARIES.find((s) => s.code === code);
  if (!hit) return code;
  return i18n.t(`commons.subsidiary.${code.toLowerCase()}`, { defaultValue: hit.label }) as string;
}

/** 根据当前 endpoint 推断默认 subsidiary。
 *  对齐后端 ovh.DefaultSubsidiaryForEndpoint;大区判定走 lib/ovh-regions,
 *  免得这里又漏掉 kimsufi-ca / soyoustart-ca 这类同区别名(它们也属于 CA 站点)。 */
export function defaultSubsidiaryForEndpoint(endpoint: string | undefined): string {
  switch (endpointRegion(endpoint)) {
    case "US":
      return "US";
    case "CA":
      return "CA";
    default:
      return "IE";
  }
}

/**
 * 账户实际下单用的子公司:账户里配置的 zone,没配置就按 endpoint 推。
 * 与后端 catalog.SubsidiaryOfAccount 同一口径 —— 服务器列表、购物车、下单都按它,
 * 价格也必须按它取,否则列表和价格来自两份不同的目录(加区账户会全部显示 "—")。
 */
export function accountSubsidiary(zone: string | undefined, endpoint: string | undefined): string {
  const z = (zone || "").trim().toUpperCase();
  return z || defaultSubsidiaryForEndpoint(endpoint);
}

/** 服务器列表页手动选的价格地区,按账户分别记忆 */
export interface PriceSubsidiaryPick {
  account: string;
  code: string;
}

/**
 * 价格地区:手动选过且仍属于该账户所在大区时用手动值,否则跟账户实际下单的子公司走。
 *
 * 以前手动选择是全局记忆的、下拉框也列出三个大区的全部子公司:在加区账户上选过 WE,
 * 切回欧区账户后价格仍按 WE(加区站点)的目录取,而列表是欧区目录 —— 价格全是 "—"。
 */
export function resolvePriceSubsidiary(
  pick: PriceSubsidiaryPick | null | undefined,
  accountId: string,
  zone: string | undefined,
  endpoint: string | undefined,
): string {
  const fallback = accountSubsidiary(zone, endpoint);
  if (!pick || pick.account !== accountId) return fallback;
  const code = (pick.code || "").trim().toUpperCase();
  return subsidiariesForEndpoint(endpoint).some((s) => s.code === code) ? code : fallback;
}

/**
 * 某个账户能用的子公司。
 *
 * OVH 的 EU / US / CA 是三套互不相通的系统：账户在哪个站点，就只能用那个站点的
 * 子公司下单、只能看到那个站点的库存。以前 VPS 页面把三个区的子公司混在一个
 * 下拉框里，美区账户能选"爱尔兰"，订阅建出来后端才拒（400），
 * 而用户只看到一个红色报错，不知道错在哪。不该给的选项就别给。
 */
export function subsidiariesForEndpoint(endpoint: string | undefined): OvhSubsidiary[] {
  const region = endpointRegion(endpoint);
  return OVH_SUBSIDIARIES.filter((s) => endpointRegion(s.endpoint) === region);
}
