import { AccountMultiSelect } from "@/components/common/AccountMultiSelect";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { MAX_ORDER_QUANTITY } from "@/lib/order-limits";
import { AutoPayCheck } from "@/components/orders/AutoPayCheck";

import type { MonitorNotifyOptions } from "@/lib/monitor-options";

export type { MonitorNotifyOptions };

function OptionCheck({
  checked,
  onChange,
  label,
  hint,
  className,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  hint?: string;
  className?: string;
}) {
  return (
    <label
      className={cn(
        "flex cursor-pointer items-start gap-2.5 rounded-xl border border-border px-3.5 py-2.5 transition-colors hover:bg-muted/40",
        className
      )}
    >
      <Checkbox
        className="mt-0.5"
        checked={checked}
        onCheckedChange={(v) => onChange(!!v)}
      />
      <span>
        <span className="block text-sm">{label}</span>
        {hint ? <span className="mt-0.5 block text-[11px] text-muted-foreground">{hint}</span> : null}
      </span>
    </label>
  );
}

/** 有货/无货提醒、有货自动下单、抢到后自动付款 */
export function MonitorOptionsFields({
  value,
  onChange,
}: {
  value: MonitorNotifyOptions;
  onChange: (next: MonitorNotifyOptions) => void;
}) {
  const set = (patch: Partial<MonitorNotifyOptions>) => onChange({ ...value, ...patch });

  return (
    <div className="space-y-3">
      <p className="text-xs font-medium text-muted-foreground">通知与下单</p>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <OptionCheck
          checked={value.notifyAvailable}
          onChange={(v) => set({ notifyAvailable: v })}
          label="有货时提醒"
          hint="库存从无到有时推送"
        />
        <OptionCheck
          checked={value.notifyUnavailable}
          onChange={(v) => set({ notifyUnavailable: v })}
          label="无货时提醒"
          hint="库存从有到无时推送"
        />
        <OptionCheck
          className="sm:col-span-2"
          checked={value.autoOrder}
          onChange={(v) =>
            set({
              autoOrder: v,
              autoPay: v ? value.autoPay : false,
            })
          }
          label="有货时自动下单"
          hint="补货时所有选中账户分别加入抢购队列"
        />
      </div>

      {value.autoOrder ? (
        <div className="space-y-3 rounded-xl border border-border p-3.5">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-muted-foreground">
              下单账户 <span className="text-destructive">*</span>
            </label>
            <AccountMultiSelect
              value={value.autoOrderAccountIds}
              onChange={(ids) => set({ autoOrderAccountIds: ids })}
            />
            <p className="mt-1 text-[11px] text-muted-foreground">新订阅默认选中全部现有账户，可取消不需要的账户。每个账户独立下单。</p>
          </div>
          <OptionCheck
            checked={value.autoPay}
            onChange={(v) => set({ autoPay: v })}
            label="抢到后自动付款"
            hint="默认关闭。开启后分别使用各下单账户的默认支付方式扣款"
          />
          <AutoPayCheck accountIds={value.autoOrderAccountIds} enabled={value.autoOrder && value.autoPay} />
          <div>
            <label className="mb-1.5 block text-xs font-medium text-muted-foreground">每账户、每配置、每机房下单数量</label>
            <Input
              type="number"
              min={1}
              max={MAX_ORDER_QUANTITY}
              value={value.quantity}
              onChange={(e) => {
                const n = Number(e.target.value);
                if (Number.isFinite(n)) {
                  set({ quantity: Math.max(1, Math.min(MAX_ORDER_QUANTITY, Math.floor(n))) });
                }
              }}
              placeholder="默认 1"
            />
            <p className="mt-1.5 text-[11px] text-muted-foreground">
              总下单量 = {value.autoOrderAccountIds.length} 个账户 × 检测出的配置数 × 可用数据中心数 × {value.quantity}
            </p>
          </div>
        </div>
      ) : null}
    </div>
  );
}
