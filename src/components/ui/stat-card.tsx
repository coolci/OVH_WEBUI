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
    icon: "text-muted-foreground",
    value: "text-foreground",
    border: "border-border",
  },
  primary: {
    icon: "text-primary",
    value: "text-primary",
    border: "border-primary/30",
  },
  accent: {
    icon: "text-accent",
    value: "text-accent",
    border: "border-accent/30",
  },
  warning: {
    icon: "text-warning",
    value: "text-warning",
    border: "border-warning/30",
  },
  danger: {
    icon: "text-destructive",
    value: "text-destructive",
    border: "border-destructive/30",
  },
};

export const StatCard = forwardRef<HTMLDivElement, StatCardProps>(({ 
  label, 
  value, 
  icon, 
  trend, 
  trendValue,
  variant = "default",
  className 
}, ref) => {
  const styles = variantStyles[variant];

  return (
    <div 
      ref={ref}
      className={cn(
        "terminal-card p-4 hover-glow transition-all",
        styles.border,
        className
      )}
    >
      <div className="flex items-start justify-between mb-2">
        <span className="text-xs text-muted-foreground uppercase tracking-wider">{label}</span>
        {icon && <span className={cn("h-5 w-5", styles.icon)}>{icon}</span>}
      </div>
      
      <div className="flex items-end justify-between">
        <span className={cn("text-2xl font-bold font-mono", styles.value)}>
          {value}
        </span>
        
        {trend && (
          <div className={cn(
            "flex items-center gap-1 text-xs",
            trend === "up" && "text-primary",
            trend === "down" && "text-destructive",
            trend === "neutral" && "text-muted-foreground"
          )}>
            {trend === "up" && <TrendingUp className="h-3 w-3" />}
            {trend === "down" && <TrendingDown className="h-3 w-3" />}
            {trend === "neutral" && <Minus className="h-3 w-3" />}
            {trendValue && <span>{trendValue}</span>}
          </div>
        )}
      </div>
    </div>
  );
});

StatCard.displayName = "StatCard";
