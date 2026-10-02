/**
 * OS 名称与日期格式化工具库
 *
 * 将 OVH 原始镜像标识符 (如 ubuntu2404-server_64, debian12_64, windows2022-std_64)
 * 智能解析为友好、专业的人类可读展示文案，并提供统一的 ISO 日期格式化。
 */

/** 格式化日期为 YYYY-MM-DD (如 2026-10-18) */
export function formatDate(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  const d = new Date(input);
  if (Number.isNaN(d.getTime())) return "—";
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** 格式化日期时间为 YYYY-MM-DD HH:mm */
export function formatDateTime(input: string | number | Date | null | undefined): string {
  if (!input) return "—";
  const d = new Date(input);
  if (Number.isNaN(d.getTime())) return "—";
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${y}-${m}-${day} ${hh}:${mm}`;
}

/**
 * 将 OVH OS 模板标识符转换为人类可读的标准名称
 *
 * 示例：
 *   "ubuntu2404-server_64" -> "Ubuntu 24.04 Server"
 *   "ubuntu2204-server_64" -> "Ubuntu 22.04 Server"
 *   "debian12_64"          -> "Debian 12"
 *   "almalinux9_64"        -> "AlmaLinux 9"
 *   "rocky9_64"            -> "Rocky Linux 9"
 *   "centos7_64"           -> "CentOS 7"
 *   "centos8-stream_64"    -> "CentOS 8 Stream"
 *   "fedora40_64"          -> "Fedora 40"
 *   "windows2022-std_64"   -> "Windows Server 2022 Std"
 *   "windows2019-std_64"   -> "Windows Server 2019 Std"
 *   "proxmox-ve-8_64"      -> "Proxmox VE 8"
 *   "freebsd14_64"         -> "FreeBSD 14"
 *   "archlinux_64"         -> "Arch Linux"
 */
export function formatOsDisplayName(raw: string | null | undefined): string {
  if (!raw) return "—";
  const s = raw.trim();
  if (!s) return "—";

  const lower = s.toLowerCase();

  // Ubuntu: ubuntu2404-server_64 / ubuntu2204_64 / ubuntu2410-server
  const ubuntuMatch = lower.match(/^ubuntu(\d{2})(\d{2})(?:-(server|desktop))?(?:_(\d+))?$/);
  if (ubuntuMatch) {
    const major = ubuntuMatch[1];
    const minor = ubuntuMatch[2];
    const flavor = ubuntuMatch[3] ? ` ${ubuntuMatch[3].charAt(0).toUpperCase() + ubuntuMatch[3].slice(1)}` : "";
    return `Ubuntu ${major}.${minor}${flavor}`;
  }

  // Debian: debian12_64 / debian11_64
  const debianMatch = lower.match(/^debian(\d+)(?:_(\d+))?$/);
  if (debianMatch) {
    return `Debian ${debianMatch[1]}`;
  }

  // AlmaLinux: almalinux9_64 / alma9_64
  const almaMatch = lower.match(/^(?:almalinux|alma)(\d+)(?:_(\d+))?$/);
  if (almaMatch) {
    return `AlmaLinux ${almaMatch[1]}`;
  }

  // Rocky Linux: rocky9_64 / rockylinux9_64
  const rockyMatch = lower.match(/^(?:rockylinux|rocky)(\d+)(?:_(\d+))?$/);
  if (rockyMatch) {
    return `Rocky Linux ${rockyMatch[1]}`;
  }

  // CentOS: centos7_64 / centos8-stream_64
  const centosMatch = lower.match(/^centos(\d+)(?:-(stream))?(?:_(\d+))?$/);
  if (centosMatch) {
    const stream = centosMatch[2] ? " Stream" : "";
    return `CentOS ${centosMatch[1]}${stream}`;
  }

  // Fedora: fedora40_64
  const fedoraMatch = lower.match(/^fedora(\d+)(?:_(\d+))?$/);
  if (fedoraMatch) {
    return `Fedora ${fedoraMatch[1]}`;
  }

  // Windows: windows2022-std_64 / win2019-dc_64 / windows2025-std_64
  const winMatch = lower.match(/^(?:windows|win)(\d{4})(?:-(std|dc|datacenter))?(?:_(\d+))?$/);
  if (winMatch) {
    const ed = winMatch[2] ? ` ${winMatch[2].toUpperCase()}` : "";
    return `Windows Server ${winMatch[1]}${ed}`;
  }

  // Proxmox: proxmox-ve-8_64 / proxmox-ve-7_64
  const pveMatch = lower.match(/^proxmox(?:-ve)?-(\d+)(?:_(\d+))?$/);
  if (pveMatch) {
    return `Proxmox VE ${pveMatch[1]}`;
  }

  // VMware ESXi: esxi70_64 / esxi80_64
  const esxiMatch = lower.match(/^esxi(\d)(\d)(?:_(\d+))?$/);
  if (esxiMatch) {
    return `VMware ESXi ${esxiMatch[1]}.${esxiMatch[2]}`;
  }

  // FreeBSD: freebsd14_64 / freebsd13_64
  const bsdMatch = lower.match(/^freebsd(\d+)(?:_(\d+))?$/);
  if (bsdMatch) {
    return `FreeBSD ${bsdMatch[1]}`;
  }

  // Arch Linux
  if (lower.startsWith("archlinux") || lower === "arch") {
    return "Arch Linux";
  }

  // openSUSE
  if (lower.includes("suse")) {
    const vMatch = lower.match(/suse.*?(\d+)/);
    return vMatch ? `openSUSE Leap ${vMatch[1]}` : "openSUSE";
  }

  // If already formatted or other string: strip trailing _64 / _32 / _x86_64
  const cleaned = s.replace(/_(?:64|32|x86_64|arm64)$/i, "");
  // Capitalize first letter if simple lowercase word
  if (cleaned.length > 2 && /^[a-z]+$/i.test(cleaned)) {
    return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
  }
  return cleaned || s;
}
