import { useEffect, useMemo, useRef, useState } from "react";
import { Check, ChevronsUpDown, X } from "lucide-react";
import { cn } from "@/lib/utils";

type PlanOption = {
  planCode: string;
  name?: string;
  cpu?: string;
  memory?: string;
  storage?: string;
};

const MAX_VISIBLE = 80;

/** 服务器型号输入与下拉搜索选择器：直接内嵌在当前容器内，绝对不会被 Radix Dialog 模态层拦截或阻断。 */
export function PlanCodeCombobox({
  value,
  onChange,
  servers,
  placeholder,
  className,
  disabled,
}: {
  value: string;
  onChange: (planCode: string) => void;
  servers: PlanOption[];
  placeholder?: string;
  className?: string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const wrapRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // 点击外部自动关闭
  useEffect(() => {
    if (!open) return;
    const handlePointerDown = (e: MouseEvent | TouchEvent) => {
      if (wrapRef.current && !wrapRef.current.contains(e.target as Node)) {
        setOpen(false);
        setSearchQuery("");
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("touchstart", handlePointerDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("touchstart", handlePointerDown);
    };
  }, [open]);

  // 当用户在下拉列表中搜索时按搜索词过滤；若未输入新词，展示所有服务器
  const filtered = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter((s) => {
      const hay = `${s.planCode} ${s.name || ""} ${s.cpu || ""} ${s.memory || ""} ${s.storage || ""}`.toLowerCase();
      return hay.includes(q);
    });
  }, [servers, searchQuery]);

  const matched = useMemo(
    () => servers.find((s) => s.planCode.toLowerCase() === (searchQuery || value).trim().toLowerCase()),
    [servers, searchQuery, value]
  );

  const trimmed = searchQuery.trim();
  const showCustom = trimmed.length > 0 && !matched;
  const visible = filtered.slice(0, MAX_VISIBLE);
  const hiddenCount = filtered.length - visible.length;

  const pick = (code: string) => {
    onChange(code);
    setSearchQuery("");
    setOpen(false);
  };

  const commitCustom = () => {
    if (!trimmed) return;
    onChange(trimmed);
    setSearchQuery("");
    setOpen(false);
  };

  const handleClear = (e: React.MouseEvent) => {
    e.stopPropagation();
    onChange("");
    setSearchQuery("");
    setOpen(true);
    inputRef.current?.focus();
  };

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (disabled) return;
    if (open) {
      setOpen(false);
      setSearchQuery("");
    } else {
      setOpen(true);
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  };

  return (
    <div ref={wrapRef} className="relative w-full">
      <div className="relative flex items-center">
        <input
          ref={inputRef}
          type="text"
          value={open ? (searchQuery !== "" ? searchQuery : value) : value}
          disabled={disabled}
          autoComplete="off"
          spellCheck={false}
          placeholder={placeholder || "输入或搜索型号，例如 24ska01"}
          className={cn(
            "flex h-10 w-full rounded-xl border border-border/80 bg-background px-3 py-2 text-sm font-mono tracking-tight",
            "pr-16 text-foreground placeholder:text-muted-foreground transition-colors",
            "focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary/40 focus-visible:border-primary/60",
            "disabled:cursor-not-allowed disabled:opacity-50",
            className
          )}
          onClick={() => {
            if (!disabled && !open) {
              setOpen(true);
              inputRef.current?.select();
            }
          }}
          onChange={(e) => {
            setSearchQuery(e.target.value);
            if (!open) setOpen(true);
          }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" && !open) {
              e.preventDefault();
              setOpen(true);
            } else if (e.key === "Escape") {
              setOpen(false);
              setSearchQuery("");
            } else if (e.key === "Enter") {
              e.preventDefault();
              if (visible.length === 1 && !showCustom) {
                pick(visible[0].planCode);
              } else if (showCustom) {
                commitCustom();
              }
            }
          }}
        />
        <div className="absolute right-2 flex items-center gap-0.5 text-muted-foreground">
          {(value || searchQuery) && !disabled && (
            <button
              type="button"
              className="rounded p-1 hover:bg-secondary hover:text-foreground transition-colors"
              aria-label="清空型号"
              onClick={handleClear}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
          <button
            type="button"
            className="rounded p-1 hover:bg-secondary hover:text-foreground transition-colors"
            aria-label={open ? "收起列表" : "展开列表"}
            onClick={handleToggle}
            tabIndex={-1}
          >
            <ChevronsUpDown className="h-4 w-4 opacity-60 hover:opacity-100" />
          </button>
        </div>
      </div>

      {/* 展开的型号建议下拉框 */}
      {open && !disabled && (
        <div
          className="absolute left-0 top-full mt-1.5 w-full z-[300] max-h-72 overflow-y-auto overscroll-contain rounded-xl border border-border/80 bg-popover text-popover-foreground shadow-2xl p-1 animate-in fade-in-0 zoom-in-95 duration-100"
          onMouseDown={(e) => e.stopPropagation()}
        >
          {showCustom && (
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-lg px-2.5 py-2 text-left text-sm hover:bg-secondary/70 transition-colors border-b border-border/40 mb-1"
              onClick={commitCustom}
            >
              <span className="text-[12px] text-muted-foreground">使用自定义型号:</span>
              <code className="truncate font-mono text-[13px] font-semibold text-primary">{trimmed}</code>
            </button>
          )}

          {visible.length > 0 ? (
            visible.map((s) => {
              const selected = s.planCode.toLowerCase() === value.trim().toLowerCase();
              return (
                <button
                  key={s.planCode}
                  type="button"
                  onClick={() => pick(s.planCode)}
                  className={cn(
                    "flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition-colors",
                    selected
                      ? "bg-secondary text-foreground font-semibold"
                      : "hover:bg-secondary/60 text-foreground/90"
                  )}
                >
                  <Check className={cn("h-4 w-4 shrink-0", selected ? "opacity-100 text-primary" : "opacity-0")} />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-baseline gap-2">
                      <code className="truncate font-mono text-[13px] font-bold">{s.planCode}</code>
                      {s.name && (
                        <span className="truncate text-[12px] text-muted-foreground">{s.name}</span>
                      )}
                    </div>
                    {(s.cpu || s.memory || s.storage) && (
                      <div className="mt-0.5 truncate text-[11px] text-muted-foreground">
                        {[s.cpu, s.memory, s.storage].filter(Boolean).join(" · ")}
                      </div>
                    )}
                  </div>
                </button>
              );
            })
          ) : !showCustom ? (
            <div className="px-3 py-6 text-center text-[12px] text-muted-foreground">
              {servers.length === 0 ? "目录加载中，也可直接输入型号" : "没有匹配的目录型号"}
            </div>
          ) : null}

          {hiddenCount > 0 && (
            <div className="px-2 py-1.5 text-center text-[11px] text-muted-foreground border-t border-border/30 mt-1">
              还有 {hiddenCount} 款型号，输入关键词以缩小筛选范围
            </div>
          )}
        </div>
      )}
    </div>
  );
}
