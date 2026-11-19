import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  icon?: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

/** Standardized enterprise page header with icon badge, title hierarchy, and responsive actions. */
export function PageHeader({ icon: Icon, title, description, action, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4 pb-3.5 border-b border-border/60",
        className
      )}
    >
      <div className="flex items-center gap-3 min-w-0">
        {Icon && (
          <div className="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-primary/10 border border-primary/20 text-primary flex items-center justify-center shrink-0">
            <Icon className="w-4 h-4 sm:w-[18px] sm:h-[18px]" strokeWidth={2} />
          </div>
        )}
        <div className="min-w-0 flex-1">
          <h1 className="text-[17px] sm:text-xl font-bold tracking-tight text-foreground leading-snug truncate">
            {title}
          </h1>
          {description && (
            <p className="mt-0.5 text-xs sm:text-[13px] text-muted-foreground leading-normal line-clamp-2 sm:truncate">
              {description}
            </p>
          )}
        </div>
      </div>
      {action && (
        <div className="flex items-center justify-end gap-2 flex-wrap w-full sm:w-auto flex-shrink-0">
          {action}
        </div>
      )}
    </div>
  );
}
