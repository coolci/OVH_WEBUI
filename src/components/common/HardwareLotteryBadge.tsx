import { PartyPopper } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { HardwareLottery } from "@/hooks/use-server-control";
import { cn } from "@/lib/utils";

/** 只展示后端已确认的中奖结果，缺少配置或读取失败时不作推断。 */
export function HardwareLotteryBadge({
  lottery,
  className,
}: {
  lottery?: HardwareLottery | null;
  className?: string;
}) {
  const { t } = useTranslation();
  if (!lottery?.checked || !lottery.won) return null;

  const details = lottery.items.map((item) => {
    let diff = `${t(`maint.overview.lottery.kind.${item.kind}`)}: ${item.ordered} → ${item.actual}`;
    if (item.gainPct && item.gainPct >= 1) {
      diff += ` (+${Math.round(item.gainPct)}%)`;
    } else if (item.mediaUp) {
      diff += ` (${t("maint.overview.lottery.gainMedia")})`;
    }
    return diff;
  }).join("\n");

  const badgeLabel =
    lottery.tier === 3
      ? t("maint.overview.lottery.title3")
      : lottery.tier === 2
      ? t("maint.overview.lottery.title2")
      : t("maint.overview.lottery.deviceBadge");

  return (
    <span
      title={details}
      className={cn(
        "inline-flex shrink-0 items-center gap-1 rounded-md border border-amber-400/35 bg-amber-400/10 px-1.5 py-0.5 text-[10px] font-semibold leading-tight text-amber-700 dark:text-amber-300",
        lottery.tier === 3 && "border-amber-400/60 bg-amber-400/20 font-bold",
        className,
      )}
    >
      <PartyPopper className="h-3 w-3 shrink-0" aria-hidden="true" />
      {badgeLabel}
    </span>
  );
}
