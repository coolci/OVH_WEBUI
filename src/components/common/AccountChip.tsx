import { useAccounts, accountChipColor, findAccountByID } from "@/hooks/use-accounts";
import { cn } from "@/lib/utils";

/** 显示某个账户的小 chip:`主号 · IE`。多账户场景所有任务行尾都用它。
 *  accountId 在列表里找不到 → 显示"未知账户 · ?"(被删账户的旧数据)
 */
export function AccountChip({ accountId, className }: { accountId: string; className?: string }) {
  const { data: accounts } = useAccounts();
  const acc = findAccountByID(accounts, accountId);

  if (!acc) {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10.5px] font-medium",
          "bg-muted text-muted-foreground",
          className
        )}
        title={accountId ? `账户 ${accountId} 不存在` : "未指定账户"}
      >
        未知账户
      </span>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10.5px] font-medium whitespace-nowrap",
        accountChipColor(acc.zone),
        className
      )}
      title={`OVH 账户:${acc.name}(${acc.zone},${acc.endpoint})`}
    >
      {acc.name} · {acc.zone}
    </span>
  );
}
