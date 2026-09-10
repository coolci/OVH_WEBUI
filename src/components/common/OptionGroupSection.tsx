import { Cpu, MemoryStick, HardDrive, HardDriveDownload, Wifi, Network, Server, Check } from "lucide-react";
import {
  OPTION_GROUP_LABELS,
  formatOptionDisplay,
  type OptionGroupKey,
} from "@/lib/option-groups";
import type { ServerOption } from "@/hooks/use-servers";
import { StatusDot } from "@/components/common/StatusDot";
import { cn } from "@/lib/utils";

/** option 组 → 图标映射 */
const ICON_MAP: Record<OptionGroupKey, React.ComponentType<{ className?: string }>> = {
  cpu: Cpu,
  memory: MemoryStick,
  systemStorage: HardDriveDownload,
  storage: HardDrive,
  bandwidth: Wifi,
  vrack: Network,
  other: Server,
};

/** 单组配置选择器: 结构化等宽网格磁贴, 对齐 DatacenterPicker 风格。
 *  - hasStock 返回 undefined 时不渲染绿/红点(availability 数据还没来)
 *  - defaultValueSet 里的项加"默认"微章
 *  - 右侧配标准单选 Check 点，彻底解决排版松散问题 */
export function OptionGroupSection({
  groupKey,
  options,
  picked,
  defaultValueSet,
  hasStock,
  onPick,
}: {
  groupKey: OptionGroupKey;
  options: ServerOption[];
  picked: string;
  defaultValueSet: Set<string>;
  /** 给定 option value,跟用户其它选配组合后能否凑出至少一个 DC 有货。
   *  undefined 表示 OVH availability 数据没回,不渲染绿/红点。 */
  hasStock?: (value: string) => boolean;
  onPick: (value: string) => void;
}) {
  const Icon = ICON_MAP[groupKey];
  const currentPickedOpt = options.find((o) => o.value === picked);

  return (
    <div className="space-y-2">
      {/* 分组头部：图标 + 名称 + 选项数量 + 已选概括 */}
      <div className="flex items-center justify-between gap-2 text-xs">
        <h3 className="font-semibold text-foreground flex items-center gap-1.5">
          <Icon className="w-3.5 h-3.5 text-primary" />
          <span>{OPTION_GROUP_LABELS[groupKey]}</span>
          <span className="text-[11px] font-normal text-muted-foreground">
            · 可选 {options.length} 种配置
          </span>
        </h3>
        {currentPickedOpt && (
          <span className="text-[11px] text-muted-foreground font-mono truncate max-w-[200px]">
            当前: <span className="text-foreground font-medium">{formatOptionDisplay(currentPickedOpt, groupKey)}</span>
          </span>
        )}
      </div>

      {/* 结构化选项网格：消除松散排列 */}
      <div
        className={cn(
          "grid gap-2",
          options.length > 3 ? "grid-cols-1 sm:grid-cols-2 lg:grid-cols-3" : "grid-cols-1 sm:grid-cols-2"
        )}
      >
        {options.map((opt) => {
          const active = picked === opt.value;
          const isDefault = defaultValueSet.has(opt.value);
          const inStock = hasStock ? hasStock(opt.value) : undefined;
          const displayText = formatOptionDisplay(opt, groupKey);

          return (
            <button
              key={opt.value}
              type="button"
              onClick={() => onPick(opt.value)}
              className={cn(
                "flex items-center justify-between rounded-xl border px-3 py-2.5 text-left transition-all touch-manipulation select-none",
                active
                  ? "border-primary/80 bg-primary/10 text-foreground ring-1 ring-primary/40 shadow-sm"
                  : "border-border/80 bg-secondary/30 hover:bg-secondary/60 hover:border-border text-foreground/90"
              )}
              title={inStock === false ? `${opt.value} (当前组合在所有 DC 缺货)` : opt.value}
            >
              <div className="flex items-center gap-2 min-w-0 pr-2">
                {inStock !== undefined && (
                  <StatusDot
                    tone={inStock ? "success" : "danger"}
                    size="xs"
                    pulse={inStock}
                    className="flex-shrink-0"
                  />
                )}
                <span className="text-xs font-semibold tracking-tight truncate">
                  {displayText}
                </span>
                {isDefault && (
                  <span
                    className={cn(
                      "text-[10px] px-1.5 py-0.5 rounded-md font-normal flex-shrink-0",
                      active ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"
                    )}
                  >
                    默认
                  </span>
                )}
              </div>

              {/* 单选选中指示器 */}
              <div className="flex-shrink-0">
                <div
                  className={cn(
                    "w-4 h-4 rounded-full border flex items-center justify-center transition-all",
                    active
                      ? "border-primary bg-primary text-primary-foreground"
                      : "border-border/80 bg-background/50"
                  )}
                >
                  {active && <Check className="w-2.5 h-2.5 stroke-[3]" />}
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
