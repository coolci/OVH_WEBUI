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
  default: "border-border",
  accent: "border-accent/30",
  warning: "border-warning/30",
  danger: "border-destructive/30",
};

const headerVariantStyles = {
  default: "border-border",
  accent: "border-accent/30",
  warning: "border-warning/30",
  danger: "border-destructive/30",
};

export const TerminalCard = forwardRef<HTMLDivElement, TerminalCardProps>(({ 
  title, 
  icon, 
  children, 
  className,
  headerAction,
  variant = "default"
}, ref) => {
  return (
    <div 
      ref={ref}
      className={cn(
        "terminal-card overflow-hidden",
        variantStyles[variant],
        className
      )}
    >
      {title && (
        <div className={cn(
          "flex items-center justify-between px-3 sm:px-4 py-2 border-b bg-muted/30",
          headerVariantStyles[variant]
        )}>
          <div className="flex items-center gap-2 min-w-0">
            {/* Terminal window dots */}
            <div className="flex items-center gap-1 mr-1 sm:mr-2 flex-shrink-0">
              <span className="h-1.5 w-1.5 sm:h-2 sm:w-2 rounded-full bg-destructive/60" />
              <span className="h-1.5 w-1.5 sm:h-2 sm:w-2 rounded-full bg-warning/60" />
              <span className="h-1.5 w-1.5 sm:h-2 sm:w-2 rounded-full bg-primary/60" />
            </div>
            
            {icon && <span className="text-primary flex-shrink-0">{icon}</span>}
            <span className="text-xs sm:text-sm font-medium text-foreground truncate">{title}</span>
          </div>
          {headerAction}
        </div>
      )}
      <div className="p-3 sm:p-4">
        {children}
      </div>
    </div>
  );
});

TerminalCard.displayName = "TerminalCard";
