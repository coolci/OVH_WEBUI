// 按"具体配置组合"判断库存(纯函数,不依赖 React / 网络,便于单测)。
//
// OVH 库存接口的粒度是 FQN = 一整套硬件组合(型号.内存.硬盘…),不是型号。
// "这个型号在 BHS 有货"不等于"你选的 32G + 2×1.2T NVMe 在 BHS 有货"。

import { isOrderable } from "@/lib/availability";

export interface VariantLike {
  fqn: string;
  planCode: string;
  datacenters?: { datacenter?: string; availability?: string }[];
}

/** 以完整型号切除前缀，允许自定义型号包含点号。 */
export function fqnAddonParts(fqn: string, planCode: string): string[] {
  if (!fqn) return [];
  const prefix = planCode + ".";
  return fqn.startsWith(prefix) ? fqn.slice(prefix.length).split(".").filter(Boolean) : [];
}

/**
 * FQN 段 vs catalog option value 的匹配。
 * OVH availability 接口的 FQN 段是"短前缀",比如 `ram-128g-noecc-2933` / `softraid-4x3840nvme-pcie-gen4`,
 * 但 catalog 返回的 option.value 是带 plan 后缀的完整 planCode,比如 `ram-128g-noecc-2933-rise` /
 * `softraid-4x3840nvme-pcie-gen4-24adv01-v2`。
 * 直接相等匹配永远不命中 → 全红 bug。
 * 这里用双向前缀:相等 / 任一为对方 "x-" 前缀 都算 match,加 "-" 防 `ram-1` 错配 `ram-128g`。
 */
export function partMatchesValue(part: string, value: string): boolean {
  if (!part || !value) return false;
  if (part === value) return true;
  if (value.startsWith(part + "-")) return true;
  if (part.startsWith(value + "-")) return true;
  return false;
}

/**
 * 所选配置在各机房的状态。dcMap 是"型号级"(任一配置)的机房状态,用来列出所有机房;
 * 所选配置在某机房没有任何覆盖它的库存条目时,记为 unavailable。
 * picks 只放会出现在 FQN 里的配件(内存 / 硬盘等),带宽、CPU 不在 FQN 里,不能参与匹配。
 */
export function selectionDcStatus(
  dcMap: Record<string, string>,
  variants: VariantLike[],
  picks: string[],
): Record<string, string> {
  const req = picks.filter(Boolean);
  const best: Record<string, string> = {};
  for (const v of variants) {
    const parts = fqnAddonParts(v.fqn, v.planCode);
    if (!req.every((val) => parts.some((p) => partMatchesValue(p, val)))) continue;
    for (const dc of v.datacenters || []) {
      const code = dc.datacenter?.toLowerCase();
      if (!code || isOrderable(best[code])) continue;
      best[code] = dc.availability || "unknown";
    }
  }
  const out: Record<string, string> = {};
  for (const k of Object.keys(dcMap)) out[k] = best[k.toLowerCase()] ?? "unavailable";
  for (const [k, v] of Object.entries(best)) if (!(k in out)) out[k] = v;
  return out;
}

/**
 * 有货、但不在当前销售目录选项里的配件段(比如目录只卖 3×4T,库存里却是 2×4T 有货)。
 * 这类配置在选项里选不到,用户只看到"机房有货、选项全红",需要单独说明。
 */
export function stockOnlyParts(variants: VariantLike[], catalogValues: string[]): string[] {
  const out = new Set<string>();
  for (const v of variants) {
    if (!(v.datacenters || []).some((d) => isOrderable(d.availability))) continue;
    for (const p of fqnAddonParts(v.fqn, v.planCode)) {
      if (!catalogValues.some((val) => partMatchesValue(p, val))) out.add(p);
    }
  }
  return Array.from(out);
}
