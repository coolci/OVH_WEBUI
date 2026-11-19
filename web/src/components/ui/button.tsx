import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-lg text-sm font-medium tracking-tight ring-offset-background transition-all duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0 active:scale-[0.98]",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground hover:bg-primary/90 shadow-sm hover:shadow active:scale-[0.98] font-medium",
        destructive:
          "bg-destructive text-destructive-foreground hover:bg-destructive/90 shadow-sm hover:shadow active:scale-[0.98] font-medium",
        outline:
          "border border-border/80 bg-card/70 hover:bg-secondary text-foreground shadow-xs hover:border-border active:scale-[0.98] font-medium backdrop-blur-xs",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-secondary/80 border border-border/50 shadow-xs active:scale-[0.98] font-medium",
        soft:
          "border border-primary/25 bg-primary/10 text-primary hover:bg-primary/20 hover:border-primary/45 shadow-xs active:scale-[0.98] font-medium",
        "soft-destructive":
          "border border-destructive/30 bg-destructive/10 text-destructive hover:bg-destructive/20 hover:border-destructive/45 shadow-xs active:scale-[0.98] font-medium",
        ghost: "text-muted-foreground hover:bg-muted hover:text-foreground active:scale-[0.98]",
        link: "text-primary underline-offset-4 hover:underline",
        accent:
          "bg-accent text-accent-foreground hover:bg-accent/90 shadow-sm active:scale-[0.98] font-medium",
        terminal:
          "border border-primary/30 bg-primary/10 text-primary hover:bg-primary/20 font-mono text-[13px] active:scale-[0.98]",
      },
      size: {
        default: "h-10 px-4 py-2",
        sm: "h-8 px-3 text-xs rounded-lg",
        lg: "h-11 px-8 rounded-xl",
        icon: "h-8 w-8 rounded-lg",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
