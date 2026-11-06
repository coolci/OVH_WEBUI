import * as React from "react";
import { ThemeProvider as NextThemesProvider } from "next-themes";

import { migrateThemePreference } from "@/lib/theme";

migrateThemePreference();

export function ThemeProvider({
  children,
  ...props
}: React.ComponentProps<typeof NextThemesProvider>) {
  return <NextThemesProvider {...props}>{children}</NextThemesProvider>;
}
