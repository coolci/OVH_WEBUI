import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

interface EmptyStateProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  action?: React.ReactNode;
  className?: string;
}

export function EmptyState({ icon: Icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-14 sm:py-16 px-6 text-center",
        className
      )}
    >
      <div className="relative mb-5">
        <div
          className="absolute inset-0 -m-3 rounded-full opacity-50 blur-xl"
          style={{ background: "hsl(var(--primary) / 0.12)" }}
          aria-hidden
        />
        <div className="icon-well relative h-14 w-14">
          <Icon className="h-6 w-6 text-muted-foreground" strokeWidth={1.5} />
        </div>
      </div>
      <p className="text-[15px] font-semibold tracking-tight text-foreground">{title}</p>
      {description && (
        <p className="mt-2 max-w-sm text-[12px] leading-relaxed text-muted-foreground">
          {description}
        </p>
      )}
      {action && <div className="mt-6">{action}</div>}
    </div>
  );
}
