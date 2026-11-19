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
    <Card className="surface-card group hover:border-primary/40 transition-all duration-200 rounded-xl overflow-hidden border-border/80 hover:-translate-y-0.5 flex flex-col">
      <CardContent className="p-3.5 sm:p-4 lg:p-5 flex flex-col flex-1 justify-between">
        <div>
          <div className="flex items-center justify-between gap-2 mb-2.5">
            <span className="text-[11px] sm:text-xs font-semibold tracking-wide text-muted-foreground truncate">{label}</span>
            <div className="w-8 h-8 rounded-lg bg-secondary/70 border border-border/60 flex items-center justify-center transition-transform duration-200 group-hover:scale-105 flex-shrink-0">
              <Icon className="w-4 h-4 text-foreground/85" strokeWidth={1.8} />
            </div>
          </div>
          <div className="flex items-end justify-between gap-2">
            <div className="flex items-baseline flex-wrap gap-y-0.5 min-w-0">
              {loading ? (
                <Skeleton className="w-16 h-7 sm:h-8 rounded-lg" />
              ) : (
                <span className="text-2xl sm:text-[26px] lg:text-3xl font-bold font-mono tabular-nums leading-none tracking-tight text-foreground">
                  {value ?? 0}
                </span>
              )}
              {extra}
            </div>
            {sparkPoints && (
              <div className="hidden sm:block flex-shrink-0">
                <Sparkline points={sparkPoints} color={sparkColor} />
              </div>
            )}
          </div>
          {hint && (
            <p className="text-[11px] text-muted-foreground/80 mt-2 truncate">
              {hint}
            </p>
          )}
        </div>
        <Link
          to={linkTo}
          className="mt-3 pt-2.5 border-t border-border/40 inline-flex items-center justify-between text-xs font-medium text-muted-foreground hover:text-primary transition-colors"
        >
          <span>{linkText}</span>
          <ChevronRight className="w-3.5 h-3.5 group-hover:translate-x-0.5 transition-transform" />
        </Link>
      </CardContent>
    </Card>
  );
}
