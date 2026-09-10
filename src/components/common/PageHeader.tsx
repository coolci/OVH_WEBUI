import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  /** Kept optional so existing call sites compile; not rendered. */
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

/** Page title: responsive, clean accent bar, never crushed on mobile, spacious on desktop. */
export function PageHeader({ title, description, action, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-2.5 sm:gap-4 pb-1 sm:pb-2",
        className
      )}
    >
      <div className="relative min-w-0 pl-3 sm:pl-3.5 flex-1 pr-1">
        <span
          aria-hidden
          className="absolute left-0 top-1 bottom-1 w-0.5 rounded-full bg-primary"
        />
        <h1 className="text-base sm:text-lg font-bold leading-tight tracking-tight text-foreground whitespace-nowrap flex-shrink-0">
          {title}
        </h1>
        {description && (
          <p className="mt-0.5 text-xs text-muted-foreground truncate">
            {description}
          </p>
        )}
      </div>
      {action && (
        <div className="flex items-center gap-2 flex-shrink-0">
          {action}
        </div>
      )}
    </div>
  );
}
