import { cn } from "@/lib/utils";

type StatusType = "online" | "offline" | "warning" | "pending" | "running" | "paused" | "completed" | "failed" | "available" | "unavailable";

interface StatusBadgeProps {
  status: StatusType;
  label?: string;
  size?: "sm" | "md";
  showDot?: boolean;
  className?: string;
}

const statusConfig: Record<StatusType, { color: string; bg: string; glow: string; label: string }> = {
  online: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "shadow-[0_0_10px_hsl(var(--primary)/0.5)]",
    label: "在线",
  },
  offline: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "shadow-[0_0_10px_hsl(var(--destructive)/0.5)]",
    label: "离线",
  },
  warning: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "shadow-[0_0_10px_hsl(var(--warning)/0.5)]",
    label: "警告",
  },
  pending: {
    color: "text-muted-foreground",
    bg: "bg-muted",
    glow: "",
    label: "等待中",
  },
  running: {
    color: "text-accent",
    bg: "bg-accent/10",
    glow: "shadow-[0_0_10px_hsl(var(--accent)/0.5)]",
    label: "运行中",
  },
  paused: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "已暂停",
  },
  completed: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已完成",
  },
  failed: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "失败",
  },
  available: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "shadow-[0_0_8px_hsl(var(--primary)/0.4)]",
    label: "有货",
  },
  unavailable: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "无货",
  },
};

export function StatusBadge({ 
  status, 
  label, 
  size = "md",
  showDot = true,
  className 
}: StatusBadgeProps) {
  const config = statusConfig[status];
  const displayLabel = label || config.label;

  return (
    <span className={cn(
      "inline-flex items-center gap-1.5 rounded-sm border border-current/20",
      config.color,
      config.bg,
      size === "sm" ? "px-1.5 py-0.5 text-xs" : "px-2 py-1 text-xs",
      className
    )}>
      {showDot && (
        <span className={cn(
          "rounded-full",
          size === "sm" ? "h-1.5 w-1.5" : "h-2 w-2",
          status === "running" && "animate-pulse",
          config.glow,
          "bg-current"
        )} />
      )}
      <span className="font-medium">{displayLabel}</span>
    </span>
  );
}
