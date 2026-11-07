/**
 * 购买时长(ISO 8601 整月/整年,如 P1M / P12M / P1Y)。与后端 orderterm 同一口径:
 * 空值 = 1 个月(兼容旧任务),P1Y 与 P12M 视为同一周期。
 */
export const DEFAULT_ORDER_DURATION = "P1M";

/** 还查不到 OVH 实际报价时给出的候选周期;真正能不能买以下单前的后端核验为准 */
export const FALLBACK_ORDER_DURATIONS = ["P1M", "P3M", "P6M", "P12M", "P24M", "P36M"];

const DURATION_RE = /^P([1-9]\d*)(M|Y)$/i;

export function isOrderDuration(value: unknown): value is string {
  return typeof value === "string" && DURATION_RE.test(value.trim());
}

/** 周期折算成月数;非法或空值按 1 个月 */
export function durationMonths(value?: string | null): number {
  const match = DURATION_RE.exec((value || DEFAULT_ORDER_DURATION).trim());
  if (!match) return 1;
  return Number(match[1]) * (match[2].toUpperCase() === "Y" ? 12 : 1);
}

/** 规整成后端存储的 PnM 形式(P1Y → P12M) */
export function normalizeDuration(value?: string | null): string {
  return `P${durationMonths(value)}M`;
}

export function isDefaultDuration(value?: string | null): boolean {
  return durationMonths(value) === 1;
}

/** OVH 列表为空(查不到)时不做限制,交给后端核验 */
export function supportsDuration(offered: string[] | undefined, value: string): boolean {
  return !offered?.length || offered.some((option) => durationMonths(option) === durationMonths(value));
}

type Translate = (key: string, options?: Record<string, unknown>) => string;

/** "1 个月" / "1 年" / "18 months" */
export function formatDuration(t: Translate, value?: string | null): string {
  const months = durationMonths(value);
  if (months % 12 === 0) {
    const years = months / 12;
    return t(years === 1 ? "orderDuration.year" : "orderDuration.years", { count: years });
  }
  return t(months === 1 ? "orderDuration.month" : "orderDuration.months", { count: months });
}
