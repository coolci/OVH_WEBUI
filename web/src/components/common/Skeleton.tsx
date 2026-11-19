import { cn } from "@/lib/utils";

/** 通用骨架占位：脉冲背景 + 圆角，列表/卡片加载态默认用它 */
export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("animate-pulse rounded-md bg-muted", className)} />;
}
