import React from "react";
import { Skeleton } from "@/components/common/Skeleton";
import { cn } from "@/lib/utils";

export interface InfoCardProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  loading?: boolean;
  className?: string;
}

/**
 * 硬件/网络规格指标信息卡片 (4 列卡片)
 */
export function InfoCard({
  icon,
  label,
  value,
  loading = false,
  className,
}: InfoCardProps) {
  return (
    <div
      className={cn(
        "surface-card border border-border/80 rounded-xl px-3.5 py-3 flex items-center gap-3 min-w-0 transition-colors hover:border-border",
        className
      )}
    >
      <div className="w-9 h-9 rounded-lg bg-secondary/80 border border-border/60 flex items-center justify-center text-primary flex-shrink-0">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-[11px] text-muted-foreground">{label}</div>
        {loading ? (
          <Skeleton className="h-4 w-24 mt-1" />
        ) : (
          <div
            className="text-[13px] font-semibold truncate text-foreground font-mono"
            title={value}
          >
            {value}
          </div>
        )}
      </div>
    </div>
  );
}
