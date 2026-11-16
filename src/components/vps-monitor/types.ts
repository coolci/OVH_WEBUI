import { OVH_SUBSIDIARIES } from "@/lib/ovh-subsidiaries";

export const SUBSIDIARIES = OVH_SUBSIDIARIES.map((s) => ({
  value: s.code,
  label: `${s.code} · ${s.label}`,
}));

/**
 * 在切换 VPS 型号或子公司目录后对齐 `os` 选择值：
 * - 若新目录给出了明确 `osChoices`，旧 `os` 不在其中时立即清空为 `""`（走后端自动选择有效默认镜像，避免漏填必填 `vps_os` 结账报 400）；
 * - 若因切换机型/子公司触发且新目录暂无可选列表（如自定义型号或尚在加载），同样清空跨机型残留的旧 `os`。
 */
export function reconcileVpsOsSelection(
  currentOs: string,
  osChoices: string[] | undefined,
  contextChanged = false,
): string {
  const trimmed = (currentOs || "").trim();
  if (!trimmed) return "";
  if (Array.isArray(osChoices) && osChoices.length > 0) {
    return osChoices.includes(trimmed) ? trimmed : "";
  }
  if (contextChanged) return "";
  return trimmed;
}

export function modelLabel(code: string, models?: { planCode: string; name: string }[]): string {
  return models?.find((m) => m.planCode === code)?.name || code;
}

export function formatTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(
    d.getHours()
  )}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
