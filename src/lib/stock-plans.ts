import type { AvailabilityItem } from "@/hooks/use-availability";
import type { ServerPlan } from "@/hooks/use-servers";
import { describeOptionCodes } from "@/lib/option-groups";

// 与后端 catalog.ecoStockPlanCode 同口径;库存记录只是候选,下单仍须销售目录确认。
const ecoStockPlanCode = /^[0-9]{2}(sk|sys|rise)[a-z0-9]*(-v[0-9]+)?(-[a-z]+)?$/i;

export function mergeStockPlans(plans: ServerPlan[], stock: AvailabilityItem[] = []): ServerPlan[] {
  const known = new Set(plans.map((p) => p.planCode));
  const groups = new Map<string, AvailabilityItem[]>();
  for (const item of stock) {
    if (known.has(item.planCode) || !ecoStockPlanCode.test(item.planCode)) continue;
    const group = groups.get(item.planCode) || [];
    group.push(item);
    groups.set(item.planCode, group);
  }
  return [...plans, ...Array.from(groups, ([planCode, variants]): ServerPlan => ({
    planCode,
    name: variants[0].server || planCode,
    prerelease: true,
    cpu: "—",
    memory: describeOptionCodes([...new Set(variants.map((v) => v.memory).filter(Boolean))]),
    storage: describeOptionCodes([...new Set(variants.map((v) => v.storage).filter(Boolean))]),
    bandwidth: "—",
    vrackBandwidth: "—",
    defaultOptions: [],
    availableOptions: ([
      ["memory", "memory"], ["systemStorage", "system-storage"], ["storage", "storage"],
    ] as const).flatMap(([field, family]) =>
      [...new Set(variants.map((v) => v[field]).filter((v): v is string => !!v))]
        .map((value) => ({ value, family, label: describeOptionCodes([value]) }))
    ),
    datacenters: [],
  }))];
}
