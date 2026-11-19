import { describe, expect, it } from "vitest";
import { isNewPlan, planYear, sortServersForSniping } from "./server-sort";

const codes = (list: { planCode: string }[]) => list.map((s) => s.planCode);

describe("server-sort", () => {
  it("预订置顶,正式销售按首次上架时间与年代新的在上", () => {
    const list = [
      { planCode: "24ska01", firstSeen: 0 },
      { planCode: "25skle01", firstSeen: 0 },
      { planCode: "26sk50b-v1", firstSeen: 0 },
      { planCode: "24sk202", firstSeen: 1_800_000_000 }, // 记录开始后才上架,即便代码年代老也排最前
      { planCode: "27skle03", prerelease: true },
      { planCode: "26sk10b-v1", firstSeen: 0 },
    ];
    expect(codes(sortServersForSniping(list))).toEqual([
      "27skle03",
      "24sk202",
      "26sk50b-v1",
      "26sk10b-v1",
      "25skle01",
      "24ska01",
    ]);
  });

  it("按价格排序时预订仍置顶,其余按价格", () => {
    const price: Record<string, number> = { a: 30, b: 10, c: 20 };
    const list = [
      { planCode: "24a" },
      { planCode: "24b" },
      { planCode: "26pre", prerelease: true },
      { planCode: "24c" },
    ];
    const cmp = (x: { planCode: string }, y: { planCode: string }) =>
      (price[x.planCode.slice(2)] ?? 0) - (price[y.planCode.slice(2)] ?? 0);
    expect(codes(sortServersForSniping(list, cmp))).toEqual(["26pre", "24b", "24c", "24a"]);
  });

  it("不改动原数组;新上架只认 14 天内且有记录时间的", () => {
    const list = [{ planCode: "24a" }, { planCode: "26b" }];
    sortServersForSniping(list);
    expect(codes(list)).toEqual(["24a", "26b"]);
    const now = 1_800_000_000_000;
    expect(isNewPlan(0, now)).toBe(false);
    expect(isNewPlan(undefined, now)).toBe(false);
    expect(isNewPlan(now / 1000 - 3 * 86400, now)).toBe(true);
    expect(isNewPlan(now / 1000 - 20 * 86400, now)).toBe(false);
    expect(planYear("26sk50b-v1")).toBe(2026);
    expect(planYear("kimsufi")).toBe(0);
  });
});
