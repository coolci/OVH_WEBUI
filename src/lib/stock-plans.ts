import type { AvailabilityItem } from "@/hooks/use-availability";
import type { ServerPlan } from "@/hooks/use-servers";
import { describeOptionCodes } from "@/lib/option-groups";

// 与后端 catalog.ecoStockPlanCode 同口径;库存记录只是候选,下单仍须销售目录确认。
const ecoStockPlanCode = /^[0-9]{2}(sk|sys|rise)[a-z0-9]*(-v[0-9]+)?(-[a-z]+)?$/i;

// 仅用于显示/搜索的别名(移植上游 v0.4.17);下单始终使用原始 planCode 与版本号。
export function stockPlanName(planCode: string): string {
  const match = /^\d{2}(sk|sysstor|sys|rise)le(\d+)(?:-v\d+)?(?:-[a-z]+)?$/i.exec(planCode);
  if (!match) return "";
  const family: Record<string, string> = { sk: "KS", sys: "SYS", sysstor: "SYS-STOR", rise: "RISE" };
  return `${family[match[1].toLowerCase()]}-LE-${Number(match[2])}`;
}

export function matchesServerSearch(server: Pick<ServerPlan, "planCode" | "name"> & Partial<Pick<ServerPlan, "cpu" | "memory" | "storage">>, query: string): boolean {
  const text = `${server.planCode} ${server.name} ${stockPlanName(server.planCode)} ${server.cpu ?? ""} ${server.memory ?? ""} ${server.storage ?? ""}`.toLowerCase();
  const wanted = query.trim().toLowerCase();
  return text.includes(wanted) || text.replace(/[\s-]/g, "").includes(wanted.replace(/[\s-]/g, ""));
}

export function mergeStockPlans(plans: ServerPlan[], stock: AvailabilityItem[] = []): ServerPlan[] {
  // 预售候选自动发现已停用，保留原实现便于回溯。
  return plans;
  //   const known = new Set(plans.map((p) => p.planCode));
  //   const groups = new Map<string, AvailabilityItem[]>();
  //   for (const item of stock) {
  //     if (known.has(item.planCode) || !ecoStockPlanCode.test(item.planCode)) continue;
  //     const group = groups.get(item.planCode) || [];
  //     group.push(item);
  //     groups.set(item.planCode, group);
  //   }
  //   return [...plans, ...Array.from(groups, ([planCode, variants]): ServerPlan => ({
  //     planCode,
  //     name: variants[0].server || planCode,
  //     prerelease: true,
  //     cpu: "—",
  //     memory: describeOptionCodes([...new Set(variants.map((v) => v.memory).filter(Boolean))]),
  //     storage: describeOptionCodes([...new Set(variants.map((v) => v.storage).filter(Boolean))]),
  //     bandwidth: "—",
  //     vrackBandwidth: "—",
  //     defaultOptions: [],
  //     availableOptions: ([
  //       ["memory", "memory"], ["systemStorage", "system-storage"], ["storage", "storage"],
  //     ] as const).flatMap(([field, family]) =>
  //       [...new Set(variants.map((v) => v[field]).filter((v): v is string => !!v))]
  //         .map((value) => ({ value, family, label: describeOptionCodes([value]) }))
  //     ),
  //     datacenters: [],
  //   }))];
}
