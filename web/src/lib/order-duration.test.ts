import { describe, expect, it } from "vitest";
import {
  durationMonths,
  formatDuration,
  isDefaultDuration,
  isOrderDuration,
  normalizeDuration,
  supportsDuration,
} from "./order-duration";

const t = (key: string, opts?: Record<string, unknown>) => `${key}:${opts?.count}`;

describe("order-duration", () => {
  it("empty and legacy values mean one month, same as the backend", () => {
    expect(durationMonths(undefined)).toBe(1);
    expect(durationMonths("")).toBe(1);
    expect(normalizeDuration("")).toBe("P1M");
    expect(isDefaultDuration(null)).toBe(true);
  });

  it("years and months normalize to the stored PnM form", () => {
    expect(normalizeDuration("P1Y")).toBe("P12M");
    expect(normalizeDuration("p24m")).toBe("P24M");
    expect(durationMonths("P2Y")).toBe(24);
  });

  it("only whole months or years are valid periods", () => {
    for (const ok of ["P1M", "P12M", "P1Y", "p3m"]) {
      expect(isOrderDuration(ok), ok).toBe(true);
    }
    for (const bad of ["P0M", "1M", "P1D", "P1.5Y", "", null, 12]) {
      expect(isOrderDuration(bad), String(bad)).toBe(false);
    }
  });

  it("a yearly choice matches an OVH offer spelled either way", () => {
    expect(supportsDuration(["P1M", "P1Y"], "P12M")).toBe(true);
    expect(supportsDuration(["P1M"], "P12M")).toBe(false);
    // unknown offers never block saving; the backend re-checks before ordering
    expect(supportsDuration(undefined, "P36M")).toBe(true);
    expect(supportsDuration([], "P36M")).toBe(true);
  });

  it("labels pick singular/plural month and year keys", () => {
    expect(formatDuration(t, "P1M")).toBe("orderDuration.month:1");
    expect(formatDuration(t, "P6M")).toBe("orderDuration.months:6");
    expect(formatDuration(t, "P12M")).toBe("orderDuration.year:1");
    expect(formatDuration(t, "P36M")).toBe("orderDuration.years:3");
  });
});
