import { describe, expect, it } from "vitest";
import type { PurchaseHistory } from "@/hooks/use-history";
import { normalizeHistorySort, sortHistory } from "./history-sort";

type OrderItem = Pick<PurchaseHistory, "status" | "orderStatus" | "retractionState" | "purchaseTime"> & {
  id: string;
};

const ids = (items: OrderItem[]) => items.map((item) => item.id);
const order = (
  id: string,
  orderStatus: string,
  purchaseTime: string,
  patch: Partial<OrderItem> = {}
): OrderItem => ({
  id,
  orderStatus,
  purchaseTime,
  status: "success",
  ...patch,
});

describe("history-sort", () => {
  it("only paid delivering/delivered orders are pinned, with newest first within each group", () => {
    const items = [
      order("unpaid-new", "notPaid", "2026-11-18T12:00:00Z"),
      order("delivering-old", "delivering", "2026-11-16T12:00:00Z"),
      order("delivered-new", "delivered", "2026-11-17T12:00:00Z"),
      order("checking", "checking", "2026-11-18T13:00:00Z"),
      order("retracting", "delivered", "2026-11-18T14:00:00Z", { retractionState: "submitted" }),
      order("failed", "delivered", "2026-11-18T15:00:00Z", { status: "failed" }),
    ];
    const before = ids(items);
    expect(ids(sortHistory(items, "paid-first"))).toEqual([
      "delivered-new",
      "delivering-old",
      "failed",
      "retracting",
      "checking",
      "unpaid-new",
    ]);
    expect(ids(items)).toEqual(before);
  });

  it("chronological choices remove status priority, compare timezones correctly and keep unknown times last", () => {
    const items = [
      order("paid-old", "delivered", "2026-11-16T12:00:00Z"),
      order("new", "notPaid", "2026-11-18T12:00:00Z"),
      order("middle", "notPaid", "2026-11-18T19:00:00+08:00"),
      order("unknown", "notPaid", "invalid"),
    ];
    expect(ids(sortHistory(items, "newest"))).toEqual(["new", "middle", "paid-old", "unknown"]);
    expect(ids(sortHistory(items, "oldest"))).toEqual(["paid-old", "middle", "new", "unknown"]);
    for (const value of [null, undefined, "", "stale-setting", "paid-first"]) {
      expect(normalizeHistorySort(value)).toBe("paid-first");
    }
    expect(normalizeHistorySort("newest")).toBe("newest");
    expect(normalizeHistorySort("oldest")).toBe("oldest");
  });
});
