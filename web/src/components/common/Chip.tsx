import * as React from "react";
import { cn } from "@/lib/utils";

type Tone = "default" | "success" | "warning" | "danger" | "info" | "solid";

const toneClasses: Record<Tone, string> = {
  default: "bg-secondary text-secondary-foreground border border-border/80",
  success: "bg-primary/10 text-primary border border-primary/20",
  warning: "bg-warning/10 text-warning border border-warning/20",
  danger: "bg-destructive/10 text-destructive border border-destructive/20",
  info: "bg-sky-500/10 text-sky-400 border border-sky-500/25",
  solid: "bg-primary text-primary-foreground shadow-sm",
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
