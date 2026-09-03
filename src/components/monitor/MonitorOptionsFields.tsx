import { AccountSelect } from "@/components/common/AccountSelect";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export type MonitorNotifyOptions = {
  notifyAvailable: boolean;
  notifyUnavailable: boolean;
  autoOrder: boolean;
  autoPay: boolean;
  quantity: number;
  autoOrderAccountId: string;
};

export const DEFAULT_MONITOR_OPTIONS: MonitorNotifyOptions = {
  notifyAvailable: true,
  notifyUnavailable: false,
  autoOrder: false,
  autoPay: false,
  quantity: 1,
  autoOrderAccountId: "",
};

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
          hint="补货瞬间按所选账户加入抢购队列"
        />
      </div>

      {value.autoOrder ? (
        <div className="space-y-3 rounded-xl border border-border p-3.5">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-muted-foreground">
              下单账户 <span className="text-destructive">*</span>
            </label>
            <AccountSelect
              value={value.autoOrderAccountId}
              onChange={(id) => set({ autoOrderAccountId: id })}
              allowEmpty
              placeholder="选择自动下单账户"
            />
            <p className="mt-1 text-[11px] text-muted-foreground">必须选账户才会真正下单，不选则只通知</p>
          </div>
          <OptionCheck
            checked={value.autoPay}
            onChange={(v) => set({ autoPay: v })}
            label="抢到后自动付款"
            hint="默认关闭。开启后用该 OVH 账户的默认支付方式扣款"
          />
          <div>
            <label className="mb-1.5 block text-xs font-medium text-muted-foreground">下单数量</label>
            <Input
              type="number"
              min={1}
              max={100}
              value={value.quantity}
              onChange={(e) => {
                const n = Number(e.target.value);
                if (Number.isFinite(n)) {
                  set({ quantity: Math.max(1, Math.min(100, Math.floor(n))) });
                }
              }}
              placeholder="默认 1"
            />
            <p className="mt-1.5 text-[11px] text-muted-foreground">
              总下单量 = 检测出的配置数 × 可用数据中心数 × 数量
            </p>
          </div>
        </div>
      ) : null}
    </div>
  );
}
