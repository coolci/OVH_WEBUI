import { cn } from "@/lib/utils";
import { ReactNode, forwardRef } from "react";
import { TrendingUp, TrendingDown, Minus } from "lucide-react";

interface StatCardProps {
  label: string;
  value: string | number;
  icon?: ReactNode;
  trend?: "up" | "down" | "neutral";
  trendValue?: string;
  variant?: "default" | "primary" | "accent" | "warning" | "danger";
  className?: string;
}

const variantStyles = {
  default: {
    icon: "text-muted-foreground bg-muted/80",
    value: "text-foreground",
    ring: "hover:border-border",
  },
  primary: {
    icon: "text-primary bg-primary/10",
    value: "text-primary",
    ring: "hover:border-primary/35",
  },
  accent: {
    icon: "text-accent bg-accent/10",
    value: "text-accent",
    ring: "hover:border-accent/35",
  },
  warning: {
    icon: "text-warning bg-warning/10",
    value: "text-warning",
    ring: "hover:border-warning/35",
  },
  danger: {
    icon: "text-destructive bg-destructive/10",
    value: "text-destructive",
    ring: "hover:border-destructive/35",
  },
};

export const StatCard = forwardRef<HTMLDivElement, StatCardProps>(
  ({ label, value, icon, trend, trendValue, variant = "default", className }, ref) => {
    const styles = variantStyles[variant];

    return (
      <div
        ref={ref}
        className={cn(
          "terminal-card p-4 transition-all duration-300 hover-glow",
          styles.ring,
          className
        )}
      >
        <div className="flex items-start justify-between gap-2 mb-3">
          <span className="section-label">{label}</span>
          {icon && (
            <span
              className={cn(
                "h-8 w-8 rounded-lg flex items-center justify-center [&_svg]:h-4 [&_svg]:w-4",
                styles.icon
              )}
            >
              {icon}
            </span>
          )}
        </div>

        <div className="flex items-end justify-between gap-2">
          <span className={cn("text-2xl sm:text-[1.75rem] font-bold font-mono tracking-tight", styles.value)}>
            {value}
          </span>

          {trend && (
            <div
              className={cn(
                "flex items-center gap-0.5 text-[11px] font-medium mb-0.5",
                trend === "up" && "text-primary",
                trend === "down" && "text-destructive",
                trend === "neutral" && "text-muted-foreground"
              )}
            >
              {trend === "up" && <TrendingUp className="h-3 w-3" />}
              {trend === "down" && <TrendingDown className="h-3 w-3" />}
              {trend === "neutral" && <Minus className="h-3 w-3" />}
              {trendValue && <span className="font-mono">{trendValue}</span>}
            </div>
          )}
        </div>
      </div>
    );
  }
);

StatCard.displayName = "StatCard";
