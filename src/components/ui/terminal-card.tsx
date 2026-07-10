import { cn } from "@/lib/utils";
import { ReactNode, forwardRef } from "react";

interface TerminalCardProps {
  title?: string;
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
  headerAction?: ReactNode;
  variant?: "default" | "accent" | "warning" | "danger";
}

const variantStyles = {
  default: "border-border/90",
  accent: "border-accent/25",
  warning: "border-warning/30",
  danger: "border-destructive/30",
};

const headerAccent = {
  default: "from-primary/10 via-transparent to-transparent",
  accent: "from-accent/15 via-transparent to-transparent",
  warning: "from-warning/15 via-transparent to-transparent",
  danger: "from-destructive/15 via-transparent to-transparent",
};

export const TerminalCard = forwardRef<HTMLDivElement, TerminalCardProps>(
  ({ title, icon, children, className, headerAction, variant = "default" }, ref) => {
    return (
      <div
        ref={ref}
        className={cn("terminal-card", variantStyles[variant], className)}
      >
        {title && (
          <div
            className={cn(
              "relative flex items-center justify-between gap-2 border-b border-border/60 px-3.5 py-3 sm:px-5",
              "bg-gradient-to-r",
              headerAccent[variant]
            )}
          >
            <div className="flex min-w-0 items-center gap-2.5">
              <div className="flex flex-shrink-0 items-center gap-1" aria-hidden>
                <span className="h-1.5 w-1.5 rounded-full bg-destructive/65" />
                <span className="h-1.5 w-1.5 rounded-full bg-warning/65" />
                <span className="h-1.5 w-1.5 rounded-full bg-primary/70" />
              </div>
              {icon && (
                <span className="text-primary flex-shrink-0 [&_svg]:h-3.5 [&_svg]:w-3.5 opacity-90">
                  {icon}
                </span>
              )}
              <span className="text-xs sm:text-sm font-semibold text-foreground/95 truncate tracking-tight">
                {title}
              </span>
            </div>
            {headerAction && (
              <div className="flex items-center gap-1.5 flex-shrink-0">{headerAction}</div>
            )}
          </div>
        )}
        <div className="p-3.5 sm:p-4 md:p-5">{children}</div>
      </div>
    );
  }
);

TerminalCard.displayName = "TerminalCard";
