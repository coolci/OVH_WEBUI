import { describe, expect, it } from "vitest";
import { selectionDcStatus, stockOnlyParts } from "./variant-stock";

const plan = "24sk50-v1";
const makeVariant = (mem: string, disk: string, ok: string[]) => ({
  planCode: plan,
  fqn: `${plan}.${mem}.${disk}`,
  datacenters: ["gra", "bhs", "waw", "rbx"].map((dc) => ({
    datacenter: dc,
    availability: ok.includes(dc) ? "1H-low" : "unavailable",
  })),
});

const variants = [
  makeVariant("ram-32g-ecc-2400", "softraid-2x1200nvme", []),
  makeVariant("ram-32g-ecc-2400", "softraid-2x4000sa", ["bhs"]),
  makeVariant("ram-64g-ecc-2400", "softraid-2x1200nvme", []),
  makeVariant("ram-64g-ecc-2400", "softraid-2x4000sa", ["gra", "waw"]),
];
const planLevel = { gra: "1H-low", bhs: "1H-low", waw: "1H-low", rbx: "unavailable" };

describe("variant-stock", () => {
  it("机房按所选配置显示:选 32G + 2×1.2T NVMe 时全线缺货", () => {
    const sel = selectionDcStatus(planLevel, variants, [
      "ram-32g-ecc-2400-24sk50-v1",
      "softraid-2x1200nvme-24sk50-v1",
    ]);
    expect(sel).toEqual({
      gra: "unavailable",
      bhs: "unavailable",
      waw: "unavailable",
      rbx: "unavailable",
    });
  });

  it("所选配置有货的机房照常显示有货", () => {
    const sel = selectionDcStatus(planLevel, variants, [
      "ram-64g-ecc-2400-24sk50-v1",
      "softraid-2x4000sa-24sk50-v1",
    ]);
    expect(sel.gra).toBe("1H-low");
    expect(sel.waw).toBe("1H-low");
    expect(sel.bhs).toBe("unavailable");
  });

  it("只选了内存时,任一含该内存的配置有货即可", () => {
    const sel = selectionDcStatus(planLevel, variants, ["ram-32g-ecc-2400-24sk50-v1"]);
    expect(sel.bhs).toBe("1H-low");
    expect(sel.gra).toBe("unavailable");
  });

  it("找出有货但目录里选不到的配置段", () => {
    const catalog = [
      "ram-32g-ecc-2400-24sk50-v1",
      "ram-64g-ecc-2400-24sk50-v1",
      "softraid-3x4000sa-24sk50-v1",
      "softraid-2x2000sa-24sk50-v1",
      "hardraid-3x960ssd-24sk50-v1",
      "softraid-2x450nvme-24sk50-v1",
      "softraid-2x1200nvme-24sk50-v1",
    ];
    expect(stockOnlyParts(variants, catalog)).toEqual(["softraid-2x4000sa"]);
    expect(stockOnlyParts(variants, [...catalog, "softraid-2x4000sa-24sk50-v1"])).toEqual([]);
  });
});
