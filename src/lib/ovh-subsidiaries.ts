/**
 * OVH 子公司列表（每个 subsidiary 对应独立目录 + 独立币种 + 独立税率）。
 * - endpoint 决定 API host（eu / us / ca）
 * - subsidiary 决定使用哪份目录、用什么币、加多少税
 */
export interface OvhSubsidiary {
  code: string;
  endpoint: "ovh-eu" | "ovh-us" | "ovh-ca";
  /** 官方 IAM 命名基准（PRD §2.2.2：EU 为 go-ovh-fr，US 为 go-ovh-us，CA 为 go-ovh-ca） */
  iam: "go-ovh-fr" | "go-ovh-us" | "go-ovh-ca";
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
  // 欧洲（eu.api.ovh.com，IAM 统一下发为 go-ovh-fr）
  { code: "IE", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "爱尔兰 · EUR", currency: "EUR" },
  { code: "FR", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "法国 · EUR", currency: "EUR" },
  { code: "DE", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "德国 · EUR", currency: "EUR" },
  { code: "GB", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "英国 · GBP", currency: "GBP" },
  { code: "IT", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "意大利 · EUR", currency: "EUR" },
  { code: "ES", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "西班牙 · EUR", currency: "EUR" },
  { code: "PL", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "波兰 · PLN", currency: "PLN" },
  { code: "NL", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "荷兰 · EUR", currency: "EUR" },
  { code: "PT", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "葡萄牙 · EUR", currency: "EUR" },
  { code: "FI", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "芬兰 · EUR", currency: "EUR" },
  { code: "CZ", endpoint: "ovh-eu", iam: "go-ovh-fr", label: "捷克 · EUR", currency: "EUR" },

  // 北美（api.us.ovhcloud.com）。US 独立产品线（plans 137 / addons 768，比 EU 多 40%）
  { code: "US", endpoint: "ovh-us", iam: "go-ovh-us", label: "美国 · USD（独立产品线）", currency: "USD" },

  // 加拿大 / 亚太（ca.api.ovh.com）
  { code: "CA", endpoint: "ovh-ca", iam: "go-ovh-ca", label: "加拿大 · CAD", currency: "CAD" },
  { code: "QC", endpoint: "ovh-ca", iam: "go-ovh-ca", label: "魁北克 · CAD", currency: "CAD" },
  { code: "ASIA", endpoint: "ovh-ca", iam: "go-ovh-ca", label: "亚太 · USD", currency: "USD" },
  { code: "SG", endpoint: "ovh-ca", iam: "go-ovh-ca", label: "新加坡 · SGD", currency: "SGD" },
  { code: "AU", endpoint: "ovh-ca", iam: "go-ovh-ca", label: "澳大利亚 · AUD", currency: "AUD" },
  { code: "IN", endpoint: "ovh-ca", iam: "go-ovh-ca", label: "印度 · INR", currency: "INR" },
];

/** 根据子公司代码自动派生 Endpoint */
export function deriveEndpoint(subsidiary: string): "ovh-eu" | "ovh-us" | "ovh-ca" {
  const match = OVH_SUBSIDIARIES.find((s) => s.code.toUpperCase() === subsidiary.toUpperCase());
  return match?.endpoint || "ovh-eu";
}

/** 根据子公司代码自动派生 IAM */
export function deriveIam(subsidiary: string): "go-ovh-fr" | "go-ovh-us" | "go-ovh-ca" {
  const ep = deriveEndpoint(subsidiary);
  if (ep === "ovh-us") return "go-ovh-us";
  if (ep === "ovh-ca") return "go-ovh-ca";
  return "go-ovh-fr";
}

/** 根据当前 endpoint 推断默认 subsidiary */
export function defaultSubsidiaryForEndpoint(endpoint: string | undefined): string {
  switch (endpoint) {
    case "ovh-us":
      return "US";
    case "ovh-ca":
      return "CA";
    default:
      return "IE";
  }
}
