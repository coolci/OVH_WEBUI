import { describe, expect, it, vi } from "vitest";
import type { AvailabilityItem } from "@/hooks/use-availability";
import type { ServerPlan } from "@/hooks/use-servers";

vi.mock("@/lib/option-groups", () => ({
  describeOptionCodes: (v: string[]) => v.join(" / "),
}));

import { preorderOptions, preorderVariantCoversAll } from "./preorder-options";
import { mergeStockPlans, stockPlanName } from "./stock-plans";

describe("stock-plans and preorder-options", () => {
  it("stock-only Eco models become preorder candidates; catalog models stay untouched", () => {
    const plans = [{ planCode: "24sk202" }] as ServerPlan[];
    const stock = [
      {
        planCode: "27skle03-v1",
        server: "",
        memory: "ram-64g",
        storage: "softraid-2x480ssd",
        fqn: "27skle03-v1.ram-64g.softraid-2x480ssd",
      },
      {
        planCode: "27skle03-v1",
        server: "",
        memory: "ram-32g",
        storage: "softraid-2x480ssd",
        fqn: "27skle03-v1.ram-32g.softraid-2x480ssd",
      },
      {
        planCode: "24sk202",
        server: "",
        memory: "ram-16g",
        storage: "",
        fqn: "24sk202.ram-16g",
      },
      {
        planCode: "25scale01-v1",
        server: "",
        memory: "ram-1t",
        storage: "",
        fqn: "25scale01-v1.ram-1t",
      },
    ] as AvailabilityItem[];

    const merged = mergeStockPlans(plans, stock);
    expect(merged).toHaveLength(2);
    expect(merged[0]).toEqual({ planCode: "24sk202" });
    const pre = merged[1];
    expect(pre).toBeDefined();
    expect(pre?.planCode).toBe("27skle03-v1");
    expect(pre?.prerelease).toBe(true);
    expect(pre?.name).toBe(stockPlanName("27skle03-v1"));
    expect(
      pre?.availableOptions
        ?.filter((o) => o.family === "memory")
        .map((o) => o.value)
        .sort()
    ).toEqual(["ram-32g", "ram-64g"]);
    // nothing to add: the original array is returned unchanged
    const existingItem = stock[2];
    expect(existingItem).toBeDefined();
    if (existingItem) {
      expect(mergeStockPlans(plans, [existingItem])).toBe(plans);
    }
  });

  it("preorders never submit CPU, which is not a matching condition", () => {
    expect(preorderOptions(["cpu-intel-x", "ram-64g", "processor-a", "softraid-2x480ssd"])).toEqual([
      "ram-64g",
      "softraid-2x480ssd",
    ]);
    expect(preorderOptions(["xeon-e5", "ram-64g"], ["xeon-e5"])).toEqual(["ram-64g"]);
  });

  it("form feedback flags a configuration absent from the stock feed", () => {
    const fqn = "27skle03-v1.ram-64g-ddr4-ecc-2933.softraid-2x480ssd";
    expect(
      preorderVariantCoversAll(fqn, ["ram-64g-ddr4-ecc-2933-27skle03-v1", "softraid-2x480ssd"])
    ).toBe(true);
    expect(preorderVariantCoversAll(fqn, ["ram-64g-ddr4-ecc-2666"])).toBe(false);
    expect(preorderVariantCoversAll(fqn, ["softraid-2x480ssd-2x8000sa"])).toBe(false);
  });

  it("LE aliases treat a 3-digit number as model + configuration variant", () => {
    expect(stockPlanName("27skle03-v1")).toBe("KS-LE-3");
    expect(stockPlanName("25skle042-us")).toBe("KS-LE-4");
    expect(stockPlanName("25sysle013")).toBe("SYS-LE-1");
    expect(stockPlanName("25sysle023")).toBe("SYS-LE-2");
    expect(stockPlanName("25sysle012-v1-us")).toBe("SYS-LE-1");
    expect(stockPlanName("27sysstorle01-v1")).toBe("SYS-STOR-LE-1");
    expect(stockPlanName("24sk202")).toBe("");
  });
});
