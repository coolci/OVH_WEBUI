import { Moon, Sun, Laptop } from "lucide-react";
import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

interface ThemeToggleProps {
  className?: string;
  variant?: "ghost" | "outline";
}

export function ThemeToggle({ className, variant = "ghost" }: ThemeToggleProps) {
  const { theme, setTheme } = useTheme();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant={variant}
          size="icon"
          className={cn(
            "relative h-8 w-8 sm:h-9 sm:w-9 rounded-lg hover:bg-muted/70 transition-colors",
            className
          )}
          aria-label="切换主题"
          title="切换深浅主题"
        >
          <Sun className="h-4 w-4 rotate-0 scale-100 transition-all dark:-rotate-90 dark:scale-0 text-amber-500" />
          <Moon className="absolute h-4 w-4 rotate-90 scale-0 transition-all dark:rotate-0 dark:scale-100 text-sky-400" />
          <span className="sr-only">切换外观主题</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-32 bg-popover border border-border shadow-lg rounded-xl p-1">
        <DropdownMenuItem
          onClick={() => setTheme("light")}
          className={cn(
            "flex items-center gap-2 cursor-pointer text-xs py-2 rounded-lg font-medium",
            theme === "light" && "bg-primary/10 text-primary font-semibold"
          )}
        >
          <Sun className="h-3.5 w-3.5 text-amber-500" />
          <span>浅色纸白</span>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setTheme("dark")}
          className={cn(
            "flex items-center gap-2 cursor-pointer text-xs py-2 rounded-lg font-medium",
            theme === "dark" && "bg-primary/10 text-primary font-semibold"
          )}
        >
          <Moon className="h-3.5 w-3.5 text-sky-400" />
          <span>黑曜夜间</span>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => setTheme("system")}
          className={cn(
            "flex items-center gap-2 cursor-pointer text-xs py-2 rounded-lg font-medium",
            theme === "system" && "bg-primary/10 text-primary font-semibold"
          )}
        >
          <Laptop className="h-3.5 w-3.5 text-muted-foreground" />
          <span>跟随系统</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
