import { Link } from "react-router-dom";
import { ChevronRight, type LucideIcon } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/common/Skeleton";

export function Sparkline({ points, color = "currentColor" }: { points: number[]; color?: string }) {
  const min = Math.min(...points);
  const max = Math.max(...points);
  const range = max - min || 1;
  const width = 52;
  const height = 20;
  const path = points
    .map((p, i) => {
      const x = (i / (points.length - 1)) * width;
      const y = height - ((p - min) / range) * (height - 4) - 2;
      return `${i === 0 ? "M" : "L"} ${x.toFixed(1)} ${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg width={width} height={height} className="overflow-visible opacity-60">
      <path d={path} fill="none" stroke={color} strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

interface KpiCardProps {
  label: string;
  value: number | undefined;
  extra?: React.ReactNode;
  hint?: string;
  icon: LucideIcon;
  linkTo: string;
  linkText: string;
  loading?: boolean;
  sparkColor?: string;
  sparkPoints?: number[];
}

export function KpiCard({
  label,
  value,
  extra,
  hint,
  icon: Icon,
  linkTo,
  linkText,
  loading,
  sparkColor,
  sparkPoints,
}: KpiCardProps) {
  return (
    <Card className="surface-card group hover:border-primary/40 transition-all duration-200 rounded-2xl overflow-hidden border-border/80 shadow-sm hover:-translate-y-0.5">
      <CardContent className="p-3 sm:p-4.5">
        <div className="flex items-center justify-between mb-2">
          <span className="text-[11px] sm:text-xs font-semibold uppercase tracking-wider text-muted-foreground/80">{label}</span>
          <div className="w-8 h-8 sm:w-9 sm:h-9 rounded-xl bg-secondary/60 border border-border/60 flex items-center justify-center transition-transform duration-200 group-hover:scale-105 flex-shrink-0">
            <Icon className="w-4 h-4 text-foreground/90" strokeWidth={1.8} />
          </div>
        </div>
        <div className="flex items-baseline justify-between">
          <div className="flex items-baseline min-w-0">
            {loading ? (
              <Skeleton className="w-16 h-7 sm:h-8 rounded-lg" />
            ) : (
              <span className="text-xl sm:text-2xl lg:text-3xl font-bold font-mono leading-none tracking-tight text-foreground truncate">
                {value ?? 0}
              </span>
            )}
            {extra}
          </div>
          {sparkPoints && (
            <div className="hidden xs:block flex-shrink-0 ml-1">
              <Sparkline points={sparkPoints} color={sparkColor} />
            </div>
          )}
        </div>
        {hint && (
          <p className="text-[11px] text-muted-foreground/75 mt-1.5 truncate">
            {hint}
          </p>
        )}
        <Link
          to={linkTo}
          className="mt-2.5 inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-primary transition-colors"
        >
          {linkText}
          <ChevronRight className="w-3.5 h-3.5 group-hover:translate-x-0.5 transition-transform" />
        </Link>
      </CardContent>
    </Card>
  );
}
