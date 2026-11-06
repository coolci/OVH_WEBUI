import { useState, useMemo } from "react";
import { Command } from "cmdk";
import { useTranslation } from "react-i18next";
import { Check, ChevronsUpDown, Plus, Search, X } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { ServerPlan } from "@/hooks/use-servers";
import { customPlanCode, isSinglePlanCode } from "@/lib/order-inputs";

/** 服务器 planCode 选择器,Combobox(Popover + cmdk Command):
 *  - 触发器看起来像 Input,带 chevron;选了之后显示 planCode + 服务器名
 *  - 弹层里有搜索框 + 过滤列表,每行 planCode + 名称 + CPU 简介
 *  - 键盘导航(↑↓ + Enter),Esc 关
 *  - 搜索结果之外可明确选择使用手动输入的 planCode
 */
export function PlanCodeCombobox({
  value,
  onChange,
  servers,
  placeholder,
  className,
  disabled = false,
}: {
  value: string;
  onChange: (planCode: string) => void;
  servers: (Pick<ServerPlan, "planCode" | "name"> & Partial<Pick<ServerPlan, "cpu" | "memory" | "storage">>)[];
  placeholder?: string;
  className?: string;
  /** 锁定型号(例如编辑已有订阅时 planCode 是主键,不允许改) */
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const customCode = customPlanCode(query, servers);

  const selectCode = (code: string) => {
    onChange(code);
    setOpen(false);
    setQuery("");
  };

  const matched = useMemo(
    () => servers.find((s) => s.planCode === value),
    [servers, value]
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter((s) => {
      const hay = `${s.planCode} ${s.name} ${s.cpu} ${s.memory} ${s.storage}`.toLowerCase();
      return hay.includes(q);
    });
  }, [servers, query]);

  return (
    <Popover open={open && !disabled} onOpenChange={(v) => setOpen(disabled ? false : v)}>
      <div className="relative">
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={disabled}
          className={cn(
            "w-full justify-between font-normal h-10 px-3.5 rounded-xl",
            value && !disabled && "pr-10",
            !value && "text-muted-foreground",
            className
          )}
        >
          {value ? (
            <span className="flex items-baseline gap-2 min-w-0">
              <code className="font-mono font-semibold text-foreground truncate">{value}</code>
              {matched?.name && (
                <span className="text-[12px] text-muted-foreground truncate">{matched.name}</span>
              )}
            </span>
          ) : (
            <span>{placeholder || t("commons.planCode.placeholder")}</span>
          )}
          <ChevronsUpDown className="w-4 h-4 opacity-50 shrink-0" />
        </Button>
      </PopoverTrigger>
      {value && !disabled && (
        <button type="button" onClick={() => onChange("")}
          className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-0.5 hover:bg-muted"
          aria-label={t("commons.planCode.clear")}>
          <X className="w-3.5 h-3.5 text-muted-foreground" />
        </button>
      )}
      </div>
      <PopoverContent
        className="w-[var(--radix-popover-trigger-width)] p-0"
        sideOffset={4}
        // Combobox 嵌在 Radix Dialog 里时,Dialog 会拦 pointerdown / wheel 事件,
        // 导致列表无法用鼠标滚轮 / 触摸板下滑。stopPropagation 让事件留在 Popover 里。
        onWheel={(e) => e.stopPropagation()}
        onTouchMove={(e) => e.stopPropagation()}
      >
        <Command shouldFilter={false}>
          <div className="flex items-center border-b border-border px-3">
            <Search className="w-3.5 h-3.5 text-muted-foreground mr-2 shrink-0" />
            <Command.Input
              value={query}
              onValueChange={setQuery}
              placeholder={t("commons.planCode.searchPlaceholder")}
              className="flex h-10 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              autoFocus
            />
          </div>
          <Command.List
            className="max-h-80 overflow-y-auto overscroll-contain p-1"
            // overscroll-contain 兜底:wheel 到达列表上下边界时不再冒泡到 Dialog
            onWheel={(e) => e.stopPropagation()}
          >
            <Command.Empty className="py-6 text-center text-[12px] text-muted-foreground">
              {query.trim() && !isSinglePlanCode(query) ? t("commons.planCode.singleCode") : t("commons.planCode.noMatch")}
            </Command.Empty>
            {filtered.map((s) => {
              const selected = s.planCode === value;
              return (
                <Command.Item
                  key={s.planCode}
                  value={s.planCode}
                  onSelect={() => selectCode(s.planCode)}
                  className={cn(
                    "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm cursor-pointer",
                    "aria-selected:bg-muted aria-selected:text-foreground",
                    selected && "bg-muted/60"
                  )}
                >
                  <Check
                    className={cn(
                      "w-3.5 h-3.5 shrink-0",
                      selected ? "opacity-100" : "opacity-0"
                    )}
                  />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-baseline gap-2">
                      <code className="font-mono font-semibold text-[13px] truncate">{s.planCode}</code>
                      <span className="text-[12px] text-muted-foreground truncate">{s.name}</span>
                    </div>
                    {(s.cpu || s.memory) && (
                      <div className="text-[11px] text-muted-foreground truncate mt-0.5">
                        {[s.cpu, s.memory, s.storage].filter(Boolean).join(" · ")}
                      </div>
                    )}
                  </div>
                </Command.Item>
              );
            })}
            {customCode && (
              <Command.Item
                value={`custom:${customCode}`}
                onSelect={() => selectCode(customCode)}
                className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-2 text-sm aria-selected:bg-muted aria-selected:text-foreground"
              >
                <Plus className="h-3.5 w-3.5 shrink-0" />
                <span className="min-w-0 break-all">{t("commons.planCode.useCustom", { code: customCode })}</span>
              </Command.Item>
            )}
          </Command.List>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
