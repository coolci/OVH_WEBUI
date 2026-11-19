// 服务器列表排序(抢购优先):
//   1. 预订型号(只在库存出现、尚未进入销售目录)永远置顶 —— 这是最需要抢的一批;
//   2. 其余正式销售的型号默认"新的在上":先看本程序记录的首次上架时间(后端 firstSeen),
//      再看型号代码开头的年代(24ska01 = 2024 年那一代,26sk50b-v1 = 2026),
//      都一样时保持目录原顺序;
//   3. 用户点了按价格排序时,非预订部分改按价格排,预订仍然置顶。

export interface SortableServer {
  planCode: string;
  prerelease?: boolean;
  firstSeen?: number;
}

/** 型号代码开头两位是 OVH 的产品年代:24ska01 → 2024。认不出返回 0 */
export function planYear(planCode: string): number {
  const m = /^(\d{2})/.exec(planCode);
  return m ? 2000 + Number(m[1]) : 0;
}

/** 首次上架多久内算"新上架"(天) */
export const NEW_PLAN_DAYS = 14;

/** firstSeen 是本程序首次在目录里看到它的 unix 秒;0 / 缺省 = 开始记录前就已在售 */
export function isNewPlan(firstSeen: number | undefined, nowMs: number = Date.now()): boolean {
  if (!firstSeen) return false;
  return nowMs / 1000 - firstSeen < NEW_PLAN_DAYS * 86400;
}

export function sortServersForSniping<T extends SortableServer>(list: T[], priceCompare?: (a: T, b: T) => number): T[] {
  const order = new Map<T, number>();
  list.forEach((s, i) => order.set(s, i));
  const newestFirst = (a: T, b: T): number => {
    const fa = a.firstSeen || 0;
    const fb = b.firstSeen || 0;
    if (fa !== fb) return fb - fa;
    const ya = planYear(a.planCode);
    const yb = planYear(b.planCode);
    if (ya !== yb) return yb - ya;
    return (order.get(a) ?? 0) - (order.get(b) ?? 0);
  };
  return [...list].sort((a, b) => {
    const pa = a.prerelease ? 1 : 0;
    const pb = b.prerelease ? 1 : 0;
    if (pa !== pb) return pb - pa;
    if (!a.prerelease && priceCompare) {
      const c = priceCompare(a, b);
      if (c !== 0) return c;
    }
    return newestFirst(a, b);
  });
}
