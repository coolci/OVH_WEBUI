import { useTheme as useNextTheme } from "next-themes";
import { isThemeMode, type ThemeMode, type ResolvedTheme } from "@/lib/theme";

export function useTheme(): { mode: ThemeMode; resolved: ResolvedTheme; setMode: (mode: ThemeMode) => void } {
  const { theme, resolvedTheme, setTheme } = useNextTheme();
  return { mode: isThemeMode(theme) ? theme : "system", resolved: resolvedTheme === "dark" ? "dark" : "light", setMode: setTheme };
}
