import { MapPin } from "lucide-react";

import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/common/StatusDot";
import { OVH_DATACENTERS, isDcInStock, lookupDcStatus } from "@/lib/datacenters";
import { cn } from "@/lib/utils";

type DatacenterPickerProps = {
  value: string[];
  onChange: (codes: string[]) => void;
  /** planCode × DC 可用性，红绿点和「选可用」都靠它 */
  availability?: Record<string, string>;
  /** 未选型号时只显示占位，不渲染磁贴 */
  disabled?: boolean;
  placeholder?: string;
  /** 默认多选，与服务器列表抢购一致；Telegram 命令等单选场景传 false */
  multiple?: boolean;
};

/**
 * 与服务器列表「抢购」同一套机房选择：12 格磁贴、红绿库存点、缺货也可点、选可用/清空。
 */
export function DatacenterPicker({
  value,
  onChange,
  availability,
  disabled,
  placeholder,
  multiple = true,
}: DatacenterPickerProps) {
  const total = OVH_DATACENTERS.length;
  const okCodes = OVH_DATACENTERS.filter((dc) =>
    isDcInStock(lookupDcStatus(availability, dc))
  ).map((dc) => dc.code);
  const ok = okCodes.length;

  const toggle = (code: string) => {
    if (disabled) return;
    if (!multiple) {
      onChange(value.includes(code) ? [] : [code]);
      return;
    }
    onChange(value.includes(code) ? value.filter((c) => c !== code) : [...value, code]);
  };

  const selectAll = () => {
    if (disabled) return;
    onChange(OVH_DATACENTERS.map((d) => d.code));
  };

  const selectInStock = () => {
    if (disabled || okCodes.length === 0) return;
    onChange(okCodes);
  };

  const clearAll = () => {
    if (disabled) return;
    onChange([]);
  };

  return (
    <div>
      <div className="mb-2.5 flex flex-wrap items-center justify-between gap-2">
        <h3 className="flex items-center gap-1.5 text-[13px] font-semibold text-foreground">
          <MapPin className="h-3.5 w-3.5 text-muted-foreground" />
          <span>数据中心</span>
          <span className="text-xs font-normal text-muted-foreground">
            · 已选 {value.length} / {total}
          </span>
        </h3>
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[11px] text-muted-foreground mr-1">
            {ok > 0 ? `${ok} 个机房有现货` : "全区无现货（可挂机排队抢购）"}
          </span>
          {multiple && (
            <>
              {ok > 0 && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-7 text-[11px] px-2.5 rounded-lg border-border/80 bg-secondary/30 hover:bg-secondary text-foreground"
                  disabled={disabled}
                  onClick={selectInStock}
                  title="一键选中当前有现货的机房"
                >
                  选可用 ({ok})
                </Button>
              )}
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 text-[11px] px-2.5 rounded-lg border-border/80 bg-secondary/30 hover:bg-secondary text-foreground"
                disabled={disabled}
                onClick={selectAll}
                title="全选 12 个机房进行全区监控或排队抢购"
              >
                全选
              </Button>
              {value.length > 0 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 text-[11px] px-2 text-muted-foreground hover:text-foreground"
                  disabled={disabled}
                  onClick={clearAll}
                  title="清空已选机房"
                >
                  清空
                </Button>
              )}
            </>
          )}
        </div>
      </div>

      {disabled ? (
        <div className="rounded-2xl border border-dashed border-border/70 px-3 py-6 text-center text-[13px] text-muted-foreground bg-secondary/10">
          {placeholder || "请先选择型号，再点选机房。缺货机房也可直接选定加入抢购队列。"}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3 sm:gap-2 lg:grid-cols-4">
          {OVH_DATACENTERS.map((dc) => {
            const rawStatus = lookupDcStatus(availability, dc);
            const isOk = isDcInStock(rawStatus);
            const isSelected = value.includes(dc.code);
            return (
              <button
                key={dc.code}
                type="button"
                onClick={() => toggle(dc.code)}
                className={cn(
                  "flex items-center justify-between rounded-xl border px-3 py-2 text-left transition-all",
                  isSelected
                    ? "border-primary/70 bg-primary/10 text-foreground ring-1 ring-primary/40 shadow-sm"
                    : "border-border/60 bg-card/50 hover:bg-secondary/40 hover:border-border text-foreground"
                )}
              >
                <div className="min-w-0 pr-1">
                  <div className="flex items-center gap-1.5">
                    <span className="font-mono text-[12px] font-bold tracking-tight">{dc.code.toUpperCase()}</span>
                    {isOk ? (
                      <span className="text-[10px] text-emerald-500 font-medium">有货</span>
                    ) : (
                      <span className="text-[10px] text-muted-foreground/60 font-normal">缺货</span>
                    )}
                  </div>
                  <div className="truncate text-[10px] text-muted-foreground">
                    {dc.region} · {dc.name}
                  </div>
                </div>
                <div className="flex-shrink-0">
                  <span
                    className={cn(
                      "inline-block w-2 h-2 rounded-full",
                      isOk ? "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.35)]" : "bg-muted-foreground/30"
                    )}
                  />
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
