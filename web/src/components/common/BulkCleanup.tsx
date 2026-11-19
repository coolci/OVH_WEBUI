import { useRef, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { CheckSquare, Shuffle, Trash2, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { CleanupTarget } from "@/lib/bulk-cleanup";
import type { CleanupProgress } from "@/hooks/use-bulk-cleanup";
import { cn } from "@/lib/utils";

/** 支持 Shift+点击连选区间的行级复选框（同时保留键盘 Space 无障碍支持）。 */
export function SelectionCheckbox({
  checked,
  disabled,
  onToggle,
  ariaLabel,
  className,
}: {
  checked: boolean;
  disabled?: boolean;
  onToggle: (shiftKey: boolean) => void;
  ariaLabel: string;
  className?: string;
}) {
  const shiftRef = useRef(false);
  return (
    <Checkbox
      checked={checked}
      disabled={disabled}
      aria-label={ariaLabel}
      className={className}
      onPointerDown={(e) => {
        shiftRef.current = e.shiftKey;
        if (e.shiftKey) e.preventDefault();
      }}
      onClick={(e) => {
        shiftRef.current = e.shiftKey;
      }}
      onCheckedChange={() => {
        const shift = shiftRef.current;
        shiftRef.current = false;
        onToggle(shift);
      }}
    />
  );
}

/** 轻量内联全选控件，直接融入页面原有列表头部栏，不额外堆叠工具栏卡片。 */
export function BulkSelectAllControl({
  totalCount,
  selectedCount,
  allSelected,
  someSelected,
  disabled,
  onToggleAll,
  className,
  showHint = true,
}: {
  totalCount: number;
  selectedCount: number;
  allSelected: boolean;
  someSelected: boolean;
  disabled: boolean;
  onToggleAll: () => void;
  className?: string;
  showHint?: boolean;
}) {
  return (
    <div className={cn("inline-flex items-center gap-2 text-xs", className)}>
      <label className="inline-flex items-center gap-2 cursor-pointer select-none text-foreground/90 hover:text-foreground">
        <Checkbox
          checked={allSelected ? true : someSelected ? "indeterminate" : false}
          disabled={disabled || totalCount === 0}
          onCheckedChange={onToggleAll}
        />
        <span className="font-medium">全选</span>
      </label>
      {selectedCount > 0 ? (
        <span
          role="status"
          className="text-[11px] px-2 py-0.5 leading-none rounded-full bg-primary/15 text-primary font-mono tabular-nums font-semibold"
        >
          已选 {selectedCount}/{totalCount}
        </span>
      ) : showHint && totalCount > 0 ? (
        <span className="hidden md:inline text-[11px] text-muted-foreground/80">
          左键拖动框选 · Shift 连选 · Ctrl 反选
        </span>
      ) : null}
    </div>
  );
}

/**
 * 底部悬浮批量操作坞：仅在已选 ≥ 1 项时浮现，滚动长列表后也始终随手可达。
 */
export function BulkSelectionDock({
  totalCount,
  selectedCount,
  allSelected,
  disabled,
  onToggleAll,
  onInvertSelection,
  onClearSelection,
  onCleanup,
  extraActions,
}: {
  totalCount: number;
  selectedCount: number;
  allSelected: boolean;
  disabled: boolean;
  onToggleAll: () => void;
  onInvertSelection: () => void;
  onClearSelection: () => void;
  onCleanup: () => void;
  extraActions?: ReactNode;
}) {
  if (selectedCount <= 0 || typeof document === "undefined") return null;

  return createPortal(
    <div
      role="region"
      aria-label="批量清理悬浮操作栏"
      data-no-drag-select
      className="fixed bottom-[calc(4.5rem+env(safe-area-inset-bottom,0px))] lg:bottom-11 left-1/2 -translate-x-1/2 z-40 w-[calc(100vw-1.5rem)] max-w-2xl animate-in fade-in slide-in-from-bottom-3 duration-150"
    >
      <div className="flex flex-wrap items-center gap-1.5 sm:gap-2 rounded-2xl border border-primary/30 bg-card/95 backdrop-blur-xl px-3 py-2 shadow-xl shadow-black/20 ring-1 ring-border/60 text-xs">
        <div className="flex items-center gap-2 pr-1">
          <span
            role="status"
            className="inline-flex items-center gap-1 rounded-full bg-primary/15 text-primary px-2.5 py-1 text-[11.5px] font-semibold font-mono tabular-nums"
          >
            已选 {selectedCount} / {totalCount}
          </span>
        </div>

        <Button
          size="sm"
          variant="ghost"
          className="h-7 px-2 text-xs gap-1 rounded-lg text-muted-foreground hover:text-foreground"
          disabled={disabled || totalCount === 0}
          onClick={onToggleAll}
          title={allSelected ? "取消全选" : "全选当前可见列表 (Ctrl+A)"}
        >
          <CheckSquare className="h-3.5 w-3.5" />
          <span>{allSelected ? "取消全选" : "全选"}</span>
        </Button>

        <Button
          size="sm"
          variant="ghost"
          className="h-7 px-2 text-xs gap-1 rounded-lg text-muted-foreground hover:text-foreground"
          disabled={disabled || totalCount === 0}
          onClick={onInvertSelection}
          title="反选当前可见列表"
        >
          <Shuffle className="h-3.5 w-3.5" />
          <span>反选</span>
        </Button>

        {extraActions}

        <div className="ml-auto flex items-center gap-1.5">
          <Button
            size="sm"
            variant="ghost"
            className="h-7 px-2 text-xs gap-1 rounded-lg text-muted-foreground hover:text-foreground"
            disabled={disabled}
            onClick={onClearSelection}
            title="取消选择 (Esc)"
          >
            <X className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">取消</span>
          </Button>
          <Button
            size="sm"
            variant="destructive"
            className="h-8 px-3 text-xs gap-1.5 rounded-xl font-medium shadow-sm"
            disabled={disabled || selectedCount === 0}
            onClick={onCleanup}
            title="删除所选项目 (Delete / Backspace)"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span>清理选中（{selectedCount}）</span>
            <kbd className="hidden md:inline-flex items-center rounded bg-destructive-foreground/15 px-1 py-0.5 text-[10px] font-mono leading-none">
              Del
            </kbd>
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  );
}

/** 兼容保留的紧凑工具栏组件。 */
export function BulkCleanupBar({
  totalCount,
  selectedCount,
  allSelected,
  someSelected,
  disabled,
  onToggleAll,
  onClearSelection,
  onCleanup,
}: {
  totalCount: number;
  selectedCount: number;
  allSelected: boolean;
  someSelected: boolean;
  disabled: boolean;
  onToggleAll: () => void;
  onClearSelection: () => void;
  onCleanup: () => void;
}) {
  return (
    <div
      className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card px-3 py-2.5 text-xs"
      aria-label="批量清理工具栏"
    >
      <BulkSelectAllControl
        totalCount={totalCount}
        selectedCount={selectedCount}
        allSelected={allSelected}
        someSelected={someSelected}
        disabled={disabled}
        onToggleAll={onToggleAll}
      />
      {selectedCount > 0 && (
        <div className="flex items-center gap-1.5 sm:ml-auto">
          <Button
            size="sm"
            variant="ghost"
            className="h-7 text-xs"
            disabled={disabled}
            onClick={onClearSelection}
          >
            取消选择
          </Button>
          <Button
            size="sm"
            variant="soft-destructive"
            className="h-7 gap-1.5 text-xs"
            disabled={disabled}
            onClick={onCleanup}
          >
            <Trash2 className="h-3.5 w-3.5" />
            清理选中（{selectedCount}）
          </Button>
        </div>
      )}
    </div>
  );
}

export function BulkCleanupDialog({
  targets,
  description,
  busy,
  progress,
  failureMessage,
  onClose,
  onConfirm,
}: {
  targets: CleanupTarget[] | null;
  description: string;
  busy: boolean;
  progress?: CleanupProgress | null;
  failureMessage: string;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const pct =
    progress && progress.total > 0
      ? Math.min(100, Math.round((progress.done / progress.total) * 100))
      : 0;

  return (
    <Dialog
      open={targets !== null}
      onOpenChange={(open) => {
        if (!open && !busy) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>确认清理选中的 {targets?.length ?? 0} 项？</DialogTitle>
          <DialogDescription>{description} 此操作不可撤销。</DialogDescription>
        </DialogHeader>
        <ul className="max-h-56 overflow-auto space-y-1 text-xs break-all" aria-label="本次清理项目">
          {targets?.map((target) => (
            <li
              key={target.key}
              className="rounded-lg bg-muted/50 px-2.5 py-1.5 font-mono text-[11.5px]"
            >
              {target.label}
            </li>
          ))}
        </ul>
        {busy && progress && progress.total > 0 && (
          <div className="space-y-1.5" role="status" aria-live="polite">
            <div className="flex items-center justify-between text-[11.5px] text-muted-foreground font-mono tabular-nums">
              <span>正在清理…</span>
              <span>
                {progress.done} / {progress.total} ({pct}%)
              </span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
              <div
                className="h-full bg-destructive transition-all duration-150"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        )}
        {failureMessage && (
          <p className="text-xs text-destructive" role="alert">
            {failureMessage} 已成功的项目不会重复提交。
          </p>
        )}
        <DialogFooter>
          <Button variant="outline" disabled={busy} onClick={onClose}>
            取消
          </Button>
          <Button variant="destructive" disabled={busy || !targets?.length} onClick={onConfirm}>
            {busy
              ? progress && progress.total > 0
                ? `正在清理 (${progress.done}/${progress.total})…`
                : "正在清理…"
              : failureMessage
                ? "重试失败项"
                : "确认清理"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
