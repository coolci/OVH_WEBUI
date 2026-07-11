import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAccounts, useDefaultAccount } from "@/hooks/use-accounts";
import { cn } from "@/lib/utils";
import { useEffect } from "react";

/** 账户下拉选择器。
 *  - 创建队列/狙击/订阅时必填 → required
 *  - 监控订阅的可选 "auto-order 账户" → 留空表示不下单 → optional + allowEmpty
 *  - 默认值:外部未传 value 时,组件 onChange 一次性把默认账户回填
 */
export function AccountSelect({
  value,
  onChange,
  allowEmpty,
  placeholder,
  className,
}: {
  value: string;
  onChange: (id: string) => void;
  /** 允许选"无"(空字符串),用于 auto-order 字段 */
  allowEmpty?: boolean;
  placeholder?: string;
  className?: string;
}) {
  const { data: accounts, isPending } = useAccounts();
  const defaultAcc = useDefaultAccount();

  // 没传 value 时,首次加载完账户就回填默认账户(只对 required 模式有意义)
  useEffect(() => {
    if (allowEmpty) return;
    if (!value && defaultAcc) {
      onChange(defaultAcc.id);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaultAcc?.id]);

  const empty = !accounts || accounts.length === 0;

  return (
    <Select
      value={value || ""}
      onValueChange={(v) => onChange(v === "__none__" ? "" : v)}
      disabled={isPending || empty}
    >
      <SelectTrigger className={cn("w-full min-w-0", className)}>
        <SelectValue placeholder={placeholder || (empty ? "没有可用账户,先去添加" : "选择 OVH 账户")} />
      </SelectTrigger>
      <SelectContent
        position="popper"
        className="w-[var(--radix-select-trigger-width)] max-w-[min(100vw-2rem,32rem)]"
      >
        {allowEmpty && (
          <SelectItem value="__none__">
            <span className="text-muted-foreground">— 不自动下单 —</span>
          </SelectItem>
        )}
        {accounts?.map((a) => {
          const label = `${a.name} · ${a.zone}${a.isDefault ? " (默认)" : ""}`;
          return (
            <SelectItem key={a.id} value={a.id} title={label}>
              <span className="block max-w-full truncate">{label}</span>
            </SelectItem>
          );
        })}
      </SelectContent>
    </Select>
  );
}
