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

/** Page title: type-only, emerald hairline, matches console chrome. */
export function PageHeader({ title, description, action, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-5",
        className
      )}
    >
      <div className="relative min-w-0 pl-3.5">
        <span
          aria-hidden
          className="absolute left-0 top-[0.15rem] bottom-[0.15rem] w-0.5 rounded-full bg-primary"
        />
        <h1 className="text-[1.22rem] font-semibold leading-tight tracking-tight text-foreground sm:text-[1.38rem]">
          {title}
        </h1>
        {description && (
          <p className="mt-1 max-w-2xl line-clamp-2 text-[12.5px] leading-relaxed text-muted-foreground sm:line-clamp-1">
            {description}
          </p>
        )}
      </div>
      {action && (
        <div className="flex flex-wrap items-center gap-2 sm:flex-shrink-0 sm:justify-end">
          {action}
        </div>
      )}
    </div>
  );
}
