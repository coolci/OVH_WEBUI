import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface PageHeaderProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

/**
 * 统一页面顶栏：渐变图标井 + 标题层次 + 操作区
 */
export function PageHeader({ icon: Icon, title, description, action, className }: PageHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between sm:gap-5",
        className
      )}
    >
      <div className="flex min-w-0 items-center gap-3.5 sm:gap-4">
        <div className="icon-well h-12 w-12 sm:h-[3.25rem] sm:w-[3.25rem]">
          <Icon className="h-5 w-5 text-primary sm:h-[1.4rem] sm:w-[1.4rem]" strokeWidth={1.7} />
        </div>
        <div className="min-w-0">
          <h1 className="text-[1.35rem] font-semibold leading-tight tracking-tight text-foreground sm:text-2xl">
            {title}
          </h1>
          {description && (
            <p className="mt-1 line-clamp-2 text-[12px] leading-relaxed text-muted-foreground sm:line-clamp-1 sm:text-[13px]">
              {description}
            </p>
          )}
        </div>
      </div>
      {action && (
        <div className="flex flex-wrap items-center gap-2 sm:flex-shrink-0 sm:justify-end">
          {action}
        </div>
      )}
    </div>
  );
}
