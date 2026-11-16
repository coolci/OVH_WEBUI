/**
 * 库存可用性判据 —— 必须和后端 ovh.IsAvailableForOrder 用同一套规则。
 *
 * OVH 的 dedicated.AvailabilityEnum（三区一致）:
 *   ['120H','1440H','1H-high','1H-low','2160H','240H','24H','480H','720H','72H',
 *    'comingSoon','unavailable','unknown']
 *
 * 只有 \d+H(-high|-low)? 这一族是「多久能交付」的承诺，也就是真能下单。
 * comingSoon 是「即将上线但还没开卖」，unknown 是「OVH 没给状态」，两个都下不了单。
 *
 * 以前前端写的是黑名单 `s !== "unavailable" && s !== "unknown"` ——
 * 那会把 comingSoon 判成有货：列表上亮绿点、抢购对话框允许下单，
 * 而后端按白名单判定不可订，任务进队列后空转。前端说有、后端说没有，
 * 用户完全无从判断。后端 helpers.go 的注释里就明确禁止这种写法。
 */
const ORDERABLE = /^(\d+)H(-high|-low)?$/;

const DC_DISPLAY_TO_API: Record<string, string> = {
  "par-a": "eu-west-par-a",
  "par-b": "eu-west-par-b",
  "par-c": "eu-west-par-c",
  tor: "ca-east-tor-a",
  mum: "ynm",
};

const DC_API_TO_DISPLAY: Record<string, string> = {
  "eu-west-par-a": "par-a",
  "eu-west-par-b": "par-b",
  "eu-west-par-c": "par-c",
  "ca-east-tor-a": "tor",
  "ca-east-tor": "tor",
  ynm: "mum",
};

/** 这个可用性取值是否真的可以下单 */
export function isOrderable(availability?: string | null): boolean {
  return !!availability && ORDERABLE.test(availability);
}

/** 一组机房状态里有没有任意一个可下单 */
export function anyOrderable(values: Array<string | undefined | null>): boolean {
  return values.some(isOrderable);
}

/**
 * 可用性状态优先级权重：
 * 有货状态（1H-high > 1H-low > 24H > 72H > 240H > ...）严格高于 comingSoon / unavailable / unknown，
 * 确保同一机房存在多硬件变体时，后续缺货变体绝不会覆盖有货变体。
 */
export function availabilityRank(availability?: string | null): number {
  if (!availability) return 0;
  const s = availability.trim();
  if (!s) return 0;
  if (s === "available") return 900;
  const m = ORDERABLE.exec(s);
  if (m) {
    const hours = Number(m[1]) || 1;
    const suffix = m[2] || "";
    if (hours === 1) {
      if (suffix === "-high") return 1000;
      if (suffix === "-low") return 990;
      return 995;
    }
    const bonus = suffix === "-high" ? 2 : suffix === "-low" ? 0 : 1;
    return Math.max(100, 850 - Math.min(740, Math.floor(hours / 3))) + bonus;
  }
  if (s === "comingSoon") return 30;
  if (s === "price_check_failed") return 25;
  if (s === "unavailable") return 20;
  if (s === "unknown") return 10;
  return 5;
}

/** 合并同机房多变体状态时选取优先级更高的可用性值 */
export function pickBetterAvailability(
  existing: string | undefined,
  incoming: string | undefined
): string | undefined {
  if (!incoming) return existing;
  if (!existing) return incoming;
  return availabilityRank(incoming) >= availabilityRank(existing) ? incoming : existing;
}

/** 展开机房代码的显示短码与 OVH API 码别名集合，用于双向无损匹配 */
export function expandDcAliases(code: string): string[] {
  const normalized = String(code || "").trim().toLowerCase();
  if (!normalized) return [];
  const out = new Set<string>([normalized]);
  if (DC_DISPLAY_TO_API[normalized]) out.add(DC_DISPLAY_TO_API[normalized]);
  if (DC_API_TO_DISPLAY[normalized]) {
    const disp = DC_API_TO_DISPLAY[normalized];
    out.add(disp);
    if (DC_DISPLAY_TO_API[disp]) out.add(DC_DISPLAY_TO_API[disp]);
  }
  return Array.from(out);
}
