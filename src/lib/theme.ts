/**
 * 三态主题:浅色 / 深色 / 跟随系统。
 *
 * 存储键 "ovh-theme",取值 light | dark | system(默认 system)。
 * 实际生效靠 <html> 上的 dark class —— tailwind.config 里 darkMode: ["class"],
 * globals.css 的 .dark 块定义了整套翻转后的 token。
 *
 * 防闪烁的另一半在 index.html:那里有一段内联脚本在首帧之前就把 class 挂好,
 * 两个文件里的解析规则必须保持一致 —— 改这里时同步改那里。
 */

export type ThemeMode = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

export const STORAGE_KEY = "ovh-theme";
export const THEME_MODES: ThemeMode[] = ["system", "light", "dark"];

export const THEME_LABELS: Record<ThemeMode, string> = {
  system: "跟随系统",
  light: "浅色",
  dark: "深色",
};

export function isThemeMode(v: unknown): v is ThemeMode {
  return v === "light" || v === "dark" || v === "system";
}

/** Migrate the previous top-bar preference only when no settings preference exists. */
export function migrateThemePreference() {
  try {
    const current = localStorage.getItem(STORAGE_KEY);
    const legacy = localStorage.getItem("theme");
    if (!isThemeMode(current) && isThemeMode(legacy)) localStorage.setItem(STORAGE_KEY, legacy);
  } catch { /* Private browsing may disable storage. */ }
}
