/**
 * OS 模板归类与品牌色(纯逻辑,不含组件):按 templateName / distribution 推断发行版,
 * 给图标、分组标题配色用。图标组件见 components/server-control/OsIcon。
 */

export type OsKind =
  | "debian"
  | "ubuntu"
  | "windows"
  | "rocky"
  | "alma"
  | "proxmox"
  | "fedora"
  | "esxi"
  | "byoi"
  | "byolinux"
  | "centos"
  | "opensuse"
  | "freebsd"
  | "linux";

/** 按 templateName 推断 OS 类型。优先级靠 if 顺序,把更具体的(byolinux)放前面。 */
export function detectOsKind(templateName: string, distribution?: string, family?: string): OsKind {
  const s = `${templateName} ${distribution || ""} ${family || ""}`.toLowerCase();
  if (s.includes("byolinux")) return "byolinux";
  if (s.includes("byoi")) return "byoi";
  if (s.includes("proxmox")) return "proxmox";
  if (s.includes("esxi") || s.includes("vmware")) return "esxi";
  if (s.includes("windows") || s.includes("win-")) return "windows";
  if (s.includes("debian")) return "debian";
  if (s.includes("ubuntu")) return "ubuntu";
  if (s.includes("rocky")) return "rocky";
  if (s.includes("alma")) return "alma";
  if (s.includes("fedora")) return "fedora";
  if (s.includes("centos")) return "centos";
  if (s.includes("suse")) return "opensuse";
  if (s.includes("freebsd")) return "freebsd";
  return "linux";
}

export interface OsBrand {
  bg: string;
  fallback: string; // 没 SVG 时的字母 badge 内容
}

/** 取某个 OS 类型的品牌色,用来给分组标题做带色背景。 */
export function osBrandColor(kind: OsKind): string {
  return OS_BRANDS[kind]?.bg || "#6B7280";
}

/** 深色底上当**文字色**用的品牌色:向白混合 45%。
 *  品牌色是按白底设计的,Debian #A80030 在近黑底上只有约 2:1,
 *  计数徽章这类"品牌色当字"的场合深色下必须提亮,否则看不清。 */
export function osBrandTextColor(kind: OsKind, dark: boolean): string {
  const hex = OS_BRANDS[kind]?.bg || "#6B7280";
  if (!dark) return hex;
  const mix = (v: number) => Math.round(v + (255 - v) * 0.45);
  const r = mix(parseInt(hex.slice(1, 3), 16));
  const g = mix(parseInt(hex.slice(3, 5), 16));
  const b = mix(parseInt(hex.slice(5, 7), 16));
  return `rgb(${r}, ${g}, ${b})`;
}

export const OS_BRANDS: Record<OsKind, OsBrand> = {
  debian:   { bg: "#A80030", fallback: "D"    },
  ubuntu:   { bg: "#E95420", fallback: "U"    },
  windows:  { bg: "#0078D4", fallback: "W"    },
  rocky:    { bg: "#10B981", fallback: "R"    },
  alma:     { bg: "#0E4F88", fallback: "A"    },
  proxmox:  { bg: "#E57000", fallback: "PVE"  },
  fedora:   { bg: "#294172", fallback: "F"    },
  esxi:     { bg: "#697582", fallback: "ESX"  },
  byoi:     { bg: "#7C3AED", fallback: "BYOI" },
  byolinux: { bg: "#7C3AED", fallback: "BYO"  },
  centos:   { bg: "#932279", fallback: "C"    },
  opensuse: { bg: "#73BA25", fallback: "S"    },
  freebsd:  { bg: "#AB2B28", fallback: "B"    },
  linux:    { bg: "#6B7280", fallback: "L"    },
};
