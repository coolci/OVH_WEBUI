/**
 * OVH 标准数据中心列表（前端固定的 12 个 + 别名映射）
 * - code：前端显示用的小写代码（如 mum）
 * - apiCode：OVH 后端 API 返回的代码（如 ynm，仅当与 code 不同时设置）
 * - name：中文城市
 * - region：所在国家 / 区域
 */
export interface DataCenter {
  code: string;
  apiCode?: string;
  name: string;
  region: string;
}

export const OVH_DATACENTERS: DataCenter[] = [
  { code: "gra", name: "格拉夫尼茨", region: "法国" },
  { code: "sbg", name: "斯特拉斯堡", region: "法国" },
  { code: "rbx", name: "鲁贝", region: "法国" },
  { code: "bhs", name: "博阿尔诺", region: "加拿大" },
  { code: "mum", apiCode: "ynm", name: "孟买", region: "印度" },
  { code: "waw", name: "华沙", region: "波兰" },
  { code: "fra", name: "法兰克福", region: "德国" },
  { code: "lon", name: "伦敦", region: "英国" },
  { code: "hil", name: "俄勒冈", region: "美国西部" },
  { code: "vin", name: "弗吉尼亚", region: "美国东部" },
  { code: "sgp", name: "新加坡", region: "新加坡" },
  { code: "syd", name: "悉尼", region: "澳大利亚" },
];

/** 从可用性 map（res.data.availability）里查某个 DC 的状态，自动处理 mum/ynm 别名 */
export function lookupDcStatus(
  availMap: Record<string, string> | undefined,
  dc: DataCenter
): string | undefined {
  if (!availMap) return undefined;
  return availMap[dc.code] || (dc.apiCode ? availMap[dc.apiCode] : undefined);
}
