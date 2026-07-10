import { cn } from "@/lib/utils";

/** 环形百分比卡片。左侧标签 + 副信息,右侧环 + 中心 %。
 *  纯 SVG 实现,无 recharts 依赖,极轻量。
 *  颜色按阈值切:< 60% 绿(成功色) / 60-85% 黄(警告) / > 85% 红(危险)
 */
export function MetricRing({
  label,
  subLabel,
  percent,
  size = 96,
}: {
  label: string;
  subLabel: string;
  percent: number;
  size?: number;
}) {
  const safePct = Math.max(0, Math.min(100, percent));
  const tone = safePct >= 85 ? "danger" : safePct >= 60 ? "warning" : "success";

  // 圆环参数:留 8px 内边距,3/4 圆弧从上偏左开始顺时针
  const stroke = 8;
  const r = (size - stroke) / 2;
  const c = 2 * Math.PI * r;
  // 用 270° 弧（3/4 圆），缺口在底部正中
  const visibleArc = 0.75 * c;
  const filled = (safePct / 100) * visibleArc;
  const dashArray = `${filled} ${c}`;

  const toneStroke =
    tone === "danger"
      ? "stroke-destructive"
      : tone === "warning"
        ? "stroke-amber-500"
        : "stroke-emerald-500";
  const toneText =
    tone === "danger"
      ? "text-destructive"
      : tone === "warning"
        ? "text-amber-600 dark:text-amber-400"
        : "text-emerald-600 dark:text-emerald-400";

  return (
    <div className="flex items-center justify-between gap-4 px-5 py-4 h-full">
      <div className="min-w-0">
        <div className="text-[12px] text-muted-foreground">{label}</div>
        <div className={cn("mt-1 text-[18px] font-semibold tabular-nums", toneText)}>
          {subLabel}
        </div>
      </div>
      <div
        className="relative flex-shrink-0"
        style={{ width: size, height: size }}
      >
        <svg
          width={size}
          height={size}
          viewBox={`0 0 ${size} ${size}`}
          // 旋转使缺口落在底部正中:从 12 点顺时针看，270° 弧 → 起点 = -135° (左上)
          style={{ transform: "rotate(135deg)" }}
        >
          {/* 背景轨道 */}
          <circle
            cx={size / 2}
            cy={size / 2}
            r={r}
            fill="none"
            strokeWidth={stroke}
            strokeLinecap="round"
            className="stroke-border"
            strokeDasharray={`${visibleArc} ${c}`}
          />
          {/* 进度 */}
          <circle
            cx={size / 2}
            cy={size / 2}
            r={r}
            fill="none"
            strokeWidth={stroke}
            strokeLinecap="round"
            className={cn(toneStroke, "transition-[stroke-dasharray] duration-700")}
            strokeDasharray={dashArray}
          />
        </svg>
        <div className="absolute inset-0 flex items-center justify-center">
          <span className={cn("text-[20px] font-semibold tabular-nums", toneText)}>
            {Math.round(safePct)}
            <span className="text-[11px] ml-0.5">%</span>
          </span>
        </div>
      </div>
    </div>
  );
}
