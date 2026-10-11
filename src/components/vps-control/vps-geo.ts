// OpenStack 区域代号(2025+ cloud VPS / OVH Public Cloud 同款命名)
export const OS_ZONE_MAP: Record<string, string> = {
  "os-us-west-or-1": "美国西部·俄勒冈",
  "os-us-west-or-2": "美国西部·俄勒冈",
  "os-us-east-va-1": "美国东部·弗吉尼亚",
  "os-eu-west-fr-1": "法国·格拉夫林",
  "os-eu-west-de-1": "德国·法兰克福",
  "os-eu-west-pl-1": "波兰·华沙",
  "os-eu-west-uk-1": "英国·伦敦",
  "os-eu-south-it-1": "意大利·米兰",
  "os-eu-north-fi-1": "芬兰·赫尔辛基",
  "os-ca-east-bhs-1": "加拿大·博阿尔诺",
  "os-asia-southeast-sg-1": "新加坡",
  "os-asia-south-in-1": "印度·孟买",
  "os-au-southeast-syd-1": "澳大利亚·悉尼",
};

// 老款 VPS / Dedicated 机房三字母代号
export const LEGACY_DC_MAP: Record<string, string> = {
  bhs: "加拿大·博阿尔诺 (BHS)",
  gra: "法国·格拉夫林 (GRA)",
  rbx: "法国·鲁贝 (RBX)",
  sbg: "法国·斯特拉斯堡 (SBG)",
  waw: "波兰·华沙 (WAW)",
  fra: "德国·法兰克福 (FRA)",
  lon: "英国·伦敦 (LON)",
  lim: "德国·林堡 (LIM)",
  eri: "英国·埃里斯 (ERI)",
  vin: "美国·弗吉尼亚 (VIN)",
  hil: "美国·俄勒冈·希尔斯伯勒 (HIL)",
  sgp: "新加坡 (SGP)",
  syd: "澳大利亚·悉尼 (SYD)",
};

export function zoneFriendly(zone?: string, cluster?: string): { label: string; code: string } | null {
  const combined = `${zone || ""} ${cluster || ""}`.toLowerCase();
  if (!combined.trim()) return null;

  // 1. OpenStack zone 标准代号匹配 (os-us-west-or-1 等)
  const m = combined.match(/os-[a-z0-9-]+/i);
  if (m) {
    const code = m[0].toLowerCase();
    if (OS_ZONE_MAP[code]) return { label: OS_ZONE_MAP[code], code };
  }

  // 2. 常见机房与区域关键字匹配
  if (/or-|oregon|hil|pdx/.test(combined)) return { label: "美西·俄勒冈", code: "or" };
  if (/va-|virginia|vin|iad/.test(combined)) return { label: "美东·弗吉尼亚", code: "va" };
  if (/gra/.test(combined)) return { label: "法国·格拉夫林", code: "gra" };
  if (/rbx/.test(combined)) return { label: "法国·鲁贝", code: "rbx" };
  if (/sbg/.test(combined)) return { label: "法国·斯特拉斯堡", code: "sbg" };
  if (/bhs|beauhar/.test(combined)) return { label: "加拿大·博阿尔诺", code: "bhs" };
  if (/fra|frankfurt/.test(combined)) return { label: "德国·法兰克福", code: "fra" };
  if (/waw|warsaw/.test(combined)) return { label: "波兰·华沙", code: "waw" };
  if (/lon|london|eri/.test(combined)) return { label: "英国·伦敦", code: "lon" };
  if (/lim/.test(combined)) return { label: "德国·林堡", code: "lim" };
  if (/sgp|singapore/.test(combined)) return { label: "新加坡", code: "sgp" };
  if (/syd|sydney/.test(combined)) return { label: "澳大利亚·悉尼", code: "syd" };
  if (/in-|mumbai/.test(combined)) return { label: "印度·孟买", code: "in" };
  if (/it-|milan/.test(combined)) return { label: "意大利·米兰", code: "it" };
  if (/fi-|helsinki/.test(combined)) return { label: "芬兰·赫尔辛基", code: "fi" };

  // 3. 经典三字母机房代号
  for (const [key, label] of Object.entries(LEGACY_DC_MAP)) {
    if (combined.includes(key)) {
      return { label, code: key };
    }
  }

  // 4. 大区兜底
  if (combined.includes("us-west")) return { label: "美国西部", code: "us-west" };
  if (combined.includes("us-east")) return { label: "美国东部", code: "us-east" };
  if (combined.includes("eu-west")) return { label: "西欧机房", code: "eu-west" };
  if (combined.includes("ca-east")) return { label: "加拿大东部", code: "ca-east" };
  if (combined.includes("openstack")) return { label: "OpenStack 云机房", code: "openstack" };

  return null;
}

/** humanZoneShort OVH zone 字段转可读名 */
export function humanZoneShort(zone?: string, cluster?: string): string {
  const friendly = zoneFriendly(zone, cluster);
  if (!friendly) {
    if (!zone && !cluster) return "—";
    const raw = (zone || cluster || "").trim();
    const clean = raw.replace(/^Region\s+OpenStack:?\s*/i, "").trim();
    return clean || "OpenStack 云机房";
  }
  return friendly.label;
}
