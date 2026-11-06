import { useAccounts } from "@/hooks/use-accounts";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";

export function AccountMultiSelect({ value, onChange }: {
  value: string[];
  onChange: (ids: string[]) => void;
}) {
  const { data: accounts = [], isPending, isError } = useAccounts();
  if (isPending) return <p className="text-xs text-muted-foreground">正在加载账户…</p>;
  if (isError) return <p className="text-xs text-destructive">账户加载失败，请刷新后重试</p>;
  if (!accounts.length) return <p className="text-xs text-muted-foreground">没有可用账户，请先添加 OVH 账户</p>;

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">已选 {value.length} / {accounts.length} 个账户</span>
        <Button type="button" variant="ghost" size="sm" onClick={() => onChange(accounts.map((a) => a.id))}>
          全选账户
        </Button>
      </div>
      <div className="max-h-44 overflow-y-auto space-y-1">
        {accounts.map((account) => (
          <label key={account.id} className="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border px-3 py-2 text-sm hover:bg-muted/40">
            <Checkbox
              checked={value.includes(account.id)}
              onCheckedChange={(checked) => onChange(checked
                ? [...new Set([...value, account.id])]
                : value.filter((id) => id !== account.id))}
            />
            <span className="min-w-0 break-words">{account.name} · {account.zone}{account.isDefault ? "（默认）" : ""}</span>
          </label>
        ))}
      </div>
    </div>
  );
}
