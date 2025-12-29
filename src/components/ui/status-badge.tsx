import { cn } from "@/lib/utils";

// 后端支持的所有状态类型
type StatusType = 
  // 基础状态
  | "online" | "offline" | "warning" | "pending" 
  // 系统状态
  | "normal" | "healthy" | "unhealthy" | "connected" | "disconnected"
  // 运行状态
  | "running" | "paused" | "stopped" | "starting" | "stopping"
  // 完成状态
  | "completed" | "done" | "success" | "failed" | "error"
  // 库存状态
  | "available" | "unavailable" | "unknown" | "limited"
  // 队列状态
  | "queued" | "processing" | "retry" | "active" | "inactive"
  // 服务器状态
  | "ok" | "hacked" | "hackedBlocked" | "maintenance" | "rescue"
  // 联系人变更状态
  | "todo" | "doing" | "refused"
  // 订单状态
  | "delivered" | "delivering" | "cancelled" | "notPaid" | "expired" | "paid"
  // 账户状态
  | "complete" | "incomplete" | "verified" | "unverified"
  // 监控状态
  | "enabled" | "disabled";

interface StatusBadgeProps {
  status: StatusType | string;
  label?: string;
  size?: "sm" | "md";
  showDot?: boolean;
  className?: string;
}

const statusConfig: Record<string, { color: string; bg: string; glow: string; label: string }> = {
  // 基础状态
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
  
  // 系统状态
  normal: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "正常",
  },
  healthy: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "shadow-[0_0_8px_hsl(var(--primary)/0.4)]",
    label: "健康",
  },
  unhealthy: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "异常",
  },
  connected: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "shadow-[0_0_8px_hsl(var(--primary)/0.4)]",
    label: "已连接",
  },
  disconnected: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "未连接",
  },
  
  // 运行状态
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
  stopped: {
    color: "text-muted-foreground",
    bg: "bg-muted",
    glow: "",
    label: "已停止",
  },
  starting: {
    color: "text-accent",
    bg: "bg-accent/10",
    glow: "",
    label: "启动中",
  },
  stopping: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "停止中",
  },
  
  // 完成状态
  completed: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已完成",
  },
  done: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已完成",
  },
  success: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "成功",
  },
  failed: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "失败",
  },
  error: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "错误",
  },
  
  // 库存状态
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
  unknown: {
    color: "text-muted-foreground",
    bg: "bg-muted",
    glow: "",
    label: "未知",
  },
  limited: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "库存紧张",
  },
  
  // 队列状态
  queued: {
    color: "text-accent",
    bg: "bg-accent/10",
    glow: "",
    label: "排队中",
  },
  processing: {
    color: "text-accent",
    bg: "bg-accent/10",
    glow: "shadow-[0_0_8px_hsl(var(--accent)/0.4)]",
    label: "处理中",
  },
  retry: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "重试中",
  },
  active: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "shadow-[0_0_8px_hsl(var(--primary)/0.4)]",
    label: "活跃",
  },
  inactive: {
    color: "text-muted-foreground",
    bg: "bg-muted",
    glow: "",
    label: "非活跃",
  },
  
  // 服务器状态
  ok: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "正常",
  },
  hacked: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "shadow-[0_0_10px_hsl(var(--destructive)/0.5)]",
    label: "被入侵",
  },
  hackedBlocked: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "已封锁",
  },
  maintenance: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "维护中",
  },
  rescue: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "救援模式",
  },
  
  // 联系人变更状态
  todo: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "待处理",
  },
  doing: {
    color: "text-accent",
    bg: "bg-accent/10",
    glow: "",
    label: "处理中",
  },
  refused: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "已拒绝",
  },
  
  // 订单状态
  delivered: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已交付",
  },
  delivering: {
    color: "text-accent",
    bg: "bg-accent/10",
    glow: "",
    label: "交付中",
  },
  cancelled: {
    color: "text-destructive",
    bg: "bg-destructive/10",
    glow: "",
    label: "已取消",
  },
  notPaid: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "未支付",
  },
  expired: {
    color: "text-muted-foreground",
    bg: "bg-muted",
    glow: "",
    label: "已过期",
  },
  paid: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已支付",
  },
  
  // 账户状态
  complete: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "活跃",
  },
  incomplete: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "未完善",
  },
  verified: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已验证",
  },
  unverified: {
    color: "text-warning",
    bg: "bg-warning/10",
    glow: "",
    label: "未验证",
  },
  
  // 监控状态
  enabled: {
    color: "text-primary",
    bg: "bg-primary/10",
    glow: "",
    label: "已启用",
  },
  disabled: {
    color: "text-muted-foreground",
    bg: "bg-muted",
    glow: "",
    label: "已禁用",
  },
};

// 默认配置（用于未知状态）
const defaultConfig = {
  color: "text-muted-foreground",
  bg: "bg-muted",
  glow: "",
  label: "未知",
};

export function StatusBadge({ 
  status, 
  label, 
  size = "md",
  showDot = true,
  className 
}: StatusBadgeProps) {
  const config = statusConfig[status] || defaultConfig;
  const displayLabel = label || config.label;
  const isAnimated = status === "running" || status === "processing" || status === "doing" || status === "starting" || status === "delivering";

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
          isAnimated && "animate-pulse",
          config.glow,
          "bg-current"
        )} />
      )}
      <span className="font-medium">{displayLabel}</span>
    </span>
  );
}

// 辅助函数：根据后端状态获取对应的StatusBadge状态
export function getStatusFromBackend(backendStatus: string): StatusType | string {
  // 统一转换为小写进行匹配
  const normalizedStatus = backendStatus?.toLowerCase() || "unknown";
  
  // 直接返回如果已经是支持的状态
  if (statusConfig[normalizedStatus]) {
    return normalizedStatus;
  }
  
  // 状态映射（后端可能返回的变体）
  const statusMap: Record<string, string> = {
    "active": "online",
    "inactive": "offline",
    "in_progress": "processing",
    "waiting": "pending",
    "success": "success",
    "succeed": "success",
    "fail": "failed",
    "err": "error",
    "complete": "complete",
    "not_paid": "notPaid",
    "not-paid": "notPaid",
    "hacked_blocked": "hackedBlocked",
    "hacked-blocked": "hackedBlocked",
  };
  
  return statusMap[normalizedStatus] || normalizedStatus;
}
