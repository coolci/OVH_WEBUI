import type { PurchaseHistory } from "@/hooks/use-history";
import type { QueueItem } from "@/hooks/use-queue";
import type { MonitorSubscription } from "@/hooks/use-monitor";

export type CleanupKind = "history" | "queue" | "monitor";
export interface CleanupTarget {
  key: string;
  label: string;
  path: string;
  params?: { optionsKey: string };
}

export const historyCleanupTarget = (item: PurchaseHistory): CleanupTarget => ({
  key: item.id,
  label: `${item.planCode} · ${item.datacenter} · ${item.accountId} · ${item.orderId ? `#${item.orderId}` : item.id}`,
  path: `/purchase-history/${encodeURIComponent(item.id)}`,
});
export const queueCleanupTarget = (item: QueueItem): CleanupTarget => ({
  key: item.id, label: `${item.planCode} · ${item.datacenter} · ${item.accountId} · ${item.id}`,
  path: `/queue/${encodeURIComponent(item.id)}`,
});
export const monitorCleanupTarget = (item: MonitorSubscription): CleanupTarget => {
  // Match the backend's exact OptionsKey, including an explicit empty key.
  const optionsKey = [...(item.options || [])].sort().join(",");
  return {
    key: JSON.stringify([item.planCode, optionsKey]),
    label: `${item.planCode} · ${optionsKey || "全部配置"}`,
    path: `/monitor/subscriptions/${encodeURIComponent(item.planCode)}`,
    params: { optionsKey },
  };
};

export const cleanupDescriptions: Record<CleanupKind, string> = {
  history: "仅删除选中的本地抢购历史记录，不会取消 OVH 订单，也不会退款。",
  queue: "删除选中的抢购任务并停止后续尝试。已发出的建单请求仍可能完成，已创建的 OVH 订单不会因此取消。",
  monitor: "移除选中的监控订阅及其监控变化记录，停止这些订阅后续的通知和自动入队。已存在的抢购任务及 OVH 订单需在对应页面处理。",
};

export function retainVisibleSelection(selected: Set<string>, targets: CleanupTarget[]): Set<string> {
  const visible = new Set(targets.map((target) => target.key));
  const kept = new Set([...selected].filter((key) => visible.has(key)));
  return kept.size === selected.size ? selected : kept;
}

/** Bounded requests, exact identities, and an explicit outcome for every item. */
export async function cleanupTargets(targets: CleanupTarget[], remove: (target: CleanupTarget) => Promise<unknown>) {
  const unique = [...new Map(targets.map((target) => [target.key, target])).values()];
  const succeeded: string[] = [];
  const failed: { target: CleanupTarget; error: unknown }[] = [];
  let next = 0;
  await Promise.all(Array.from({ length: Math.min(4, unique.length) }, async () => {
    while (next < unique.length) {
      const target = unique[next++];
      try {
        await remove(target);
        succeeded.push(target.key);
      } catch (error) {
        // A record removed by another tab is already in the requested state.
        if ((error as { response?: { status?: number } })?.response?.status === 404) succeeded.push(target.key);
        else failed.push({ target, error });
      }
    }
  }));
  return { succeeded, failed };
}
