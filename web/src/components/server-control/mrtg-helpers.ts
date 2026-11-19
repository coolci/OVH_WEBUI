import type { MrtgInterface, MrtgPoint } from "@/hooks/use-mrtg";

/**
 * 按 MAC 并集合并上下行接口：允许单向采样为空而另一方向有数据时照常展示该网卡流量图。
 */
export function mergeMrtgInterfaces(
  downloadInterfaces?: MrtgInterface[] | null,
  uploadInterfaces?: MrtgInterface[] | null,
): Array<{ mac: string; download: MrtgInterface; upload: MrtgInterface }> {
  const dlList = downloadInterfaces || [];
  const ulList = uploadInterfaces || [];
  if (dlList.length === 0 && ulList.length === 0) return [];

  const dlByMac = new Map<string, MrtgInterface>();
  for (const d of dlList) {
    if (d?.mac) dlByMac.set(d.mac, d);
  }
  const ulByMac = new Map<string, MrtgInterface>();
  for (const u of ulList) {
    if (u?.mac) ulByMac.set(u.mac, u);
  }

  const macOrder: string[] = [];
  const seen = new Set<string>();
  for (const d of dlList) {
    if (d?.mac && !seen.has(d.mac)) {
      seen.add(d.mac);
      macOrder.push(d.mac);
    }
  }
  for (const u of ulList) {
    if (u?.mac && !seen.has(u.mac)) {
      seen.add(u.mac);
      macOrder.push(u.mac);
    }
  }

  const out: Array<{ mac: string; download: MrtgInterface; upload: MrtgInterface }> = [];
  for (const mac of macOrder) {
    const d = dlByMac.get(mac) ?? { mac, data: [] };
    const u = ulByMac.get(mac) ?? { mac, data: [] };
    if ((d.data?.length || 0) === 0 && (u.data?.length || 0) === 0) continue;
    out.push({ mac, download: d, upload: u });
  }
  return out;
}

export interface MergedMrtgPoint {
  timestamp: number;
  download: number | null;
  upload: number | null;
}

/**
 * 按 timestamp 并集字典外连接归并排序上下行流量点，严禁用数组下标 upload.data[i] 硬配对。
 */
export function mergeMrtgSeries(
  downloadData?: MrtgPoint[] | null,
  uploadData?: MrtgPoint[] | null,
): MergedMrtgPoint[] {
  const byTs = new Map<number, MergedMrtgPoint>();

  for (const dp of downloadData || []) {
    if (!dp || !Number.isFinite(dp.timestamp)) continue;
    const rawVal = dp.value?.value;
    const val = typeof rawVal === "number" && Number.isFinite(rawVal) ? rawVal : 0;
    const entry = byTs.get(dp.timestamp) ?? { timestamp: dp.timestamp, download: null, upload: null };
    entry.download = val;
    byTs.set(dp.timestamp, entry);
  }

  for (const up of uploadData || []) {
    if (!up || !Number.isFinite(up.timestamp)) continue;
    const rawVal = up.value?.value;
    const val = typeof rawVal === "number" && Number.isFinite(rawVal) ? rawVal : 0;
    const entry = byTs.get(up.timestamp) ?? { timestamp: up.timestamp, download: null, upload: null };
    entry.upload = val;
    byTs.set(up.timestamp, entry);
  }

  return Array.from(byTs.values()).sort((a, b) => a.timestamp - b.timestamp);
}
