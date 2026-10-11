import { useTranslation } from "react-i18next";
import { Monitor, Sun, Moon } from "lucide-react";
import { useTheme } from "@/hooks/use-theme";
import type { ThemeMode } from "@/lib/theme";
import { Section } from "./Section";

/**
 * 外观: 浅色 / 深色 / 跟随系统。
 * 纯前端偏好，存浏览器 localStorage。
 */
export function AppearanceSection() {
  const { t } = useTranslation();
  const { mode, resolved, setMode } = useTheme();

  const options: { value: ThemeMode; label: string; desc: string; icon: React.ReactNode }[] = [
    { value: "system", label: t("settings.appearance.system"), desc: t("settings.appearance.systemDesc"), icon: <Monitor className="w-4 h-4" /> },
    { value: "light", label: t("settings.appearance.light"), desc: t("settings.appearance.lightDesc"), icon: <Sun className="w-4 h-4" /> },
    { value: "dark", label: t("settings.appearance.dark"), desc: t("settings.appearance.darkDesc"), icon: <Moon className="w-4 h-4" /> },
  ];

  return (
    <Section title={t("settings.sections.appearance")}>
      <div className="space-y-2">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            onClick={() => setMode(o.value)}
            className={`w-full flex items-center gap-3 rounded-xl border px-4 py-3 text-left transition-colors ${
              mode === o.value
                ? "border-foreground/60 bg-accent text-accent-foreground"
                : "border-border hover:bg-accent/50"
            }`}
          >
            <span className={mode === o.value ? "text-accent-foreground" : "text-muted-foreground"}>{o.icon}</span>
            <span className="flex-1 min-w-0">
              <span className="block text-[13px] font-medium">{o.label}</span>
              <span className={`block text-[11px] ${mode === o.value ? "text-accent-foreground/70" : "text-muted-foreground"}`}>{o.desc}</span>
            </span>
            {mode === o.value && (
              <span className="text-[11px] text-accent-foreground/70 flex-shrink-0">
                {resolved === "dark" ? t("settings.appearance.currentDark") : t("settings.appearance.currentLight")}
              </span>
            )}
          </button>
        ))}
      </div>
      <p className="text-[11px] text-muted-foreground">
        {t("settings.appearance.footnote")}
      </p>
    </Section>
  );
}
