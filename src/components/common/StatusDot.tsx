import { cn } from "@/lib/utils";

type Tone = "success" | "warning" | "danger" | "info" | "muted";

const toneClasses: Record<Tone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-destructive",
  info: "bg-info",
  muted: "bg-muted-foreground/50",
};

/** 小状态点：tone 决定颜色，pulse 决定是否脉冲（运行中/已连接通常 pulse） */
export function StatusDot({
  tone,
  pulse = false,
  size = "sm",
  className,
}: {
  tone: Tone;
  pulse?: boolean;
  size?: "xs" | "sm" | "md";
  className?: string;
}) {
  const dim = size === "xs" ? "w-1.5 h-1.5" : size === "sm" ? "w-2 h-2" : "w-2.5 h-2.5";
  return (
    <span
      className={cn("inline-block rounded-full flex-shrink-0", dim, toneClasses[tone], pulse && "animate-pulse", className)}
    />
  );
}
