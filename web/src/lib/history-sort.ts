import type { PurchaseHistory } from "@/hooks/use-history";

export type HistorySort = "paid-first" | "newest" | "oldest";

export function normalizeHistorySort(value: unknown): HistorySort {
  return value === "newest" || value === "oldest" ? value : "paid-first";
}

type SortableHistory = Pick<PurchaseHistory, "status" | "orderStatus" | "retractionState" | "purchaseTime">;

function isPaidDelivery(item: SortableHistory): boolean {
  return item.status === "success"
    && (item.orderStatus === "delivering" || item.orderStatus === "delivered")
    && (!item.retractionState || item.retractionState === "failed");
}

export function sortHistory<T extends SortableHistory>(items: T[], sort: HistorySort): T[] {
  return [...items].sort((a, b) => {
    if (sort === "paid-first") {
      const priority = Number(isPaidDelivery(b)) - Number(isPaidDelivery(a));
      if (priority) return priority;
    }
    const aTime = Date.parse(a.purchaseTime);
    const bTime = Date.parse(b.purchaseTime);
    // Missing/invalid timestamps stay last in either chronological direction.
    if (!Number.isFinite(aTime)) return Number.isFinite(bTime) ? 1 : 0;
    if (!Number.isFinite(bTime)) return -1;
    return sort === "oldest" ? aTime - bTime : bTime - aTime;
  });
}
