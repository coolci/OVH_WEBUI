import * as React from "react";
import { cn } from "@/lib/utils";

type Tone = "default" | "success" | "warning" | "danger" | "info" | "solid";

const toneClasses: Record<Tone, string> = {
  default: "bg-secondary/80 text-foreground/90 border border-border/80",
  success: "bg-primary/10 text-primary border border-primary/25",
  warning: "bg-warning/10 text-warning border border-warning/25",
  danger: "bg-destructive/10 text-destructive border border-destructive/25",
  info: "bg-accent/10 text-accent border border-accent/25",
  solid: "bg-primary text-primary-foreground border border-primary/20 shadow-sm",
};

interface ChipProps extends React.HTMLAttributes<HTMLSpanElement> {
  tone?: Tone;
  size?: "sm" | "md";
}

export function Chip({ tone = "default", size = "sm", className, children, ...rest }: ChipProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full font-medium tracking-tight",
        size === "sm" ? "px-2 py-0.5 text-[11px]" : "px-2.5 py-1 text-xs",
        toneClasses[tone],
        className
      )}
      {...rest}
    >
      {children}
    </span>
  );
}
