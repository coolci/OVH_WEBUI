import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import type { CleanupTarget } from "@/lib/bulk-cleanup";

export function BulkCleanupBar({ totalCount, selectedCount, allSelected, someSelected, disabled, onToggleAll, onClearSelection, onCleanup }: {
  totalCount: number; selectedCount: number; allSelected: boolean; someSelected: boolean; disabled: boolean;
  onToggleAll: () => void; onClearSelection: () => void; onCleanup: () => void;
}) {
  return <div className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card p-3 text-xs" aria-label="批量清理工具栏">
    <label className="flex items-center gap-2 cursor-pointer">
      <Checkbox checked={allSelected ? true : someSelected ? "indeterminate" : false} disabled={disabled || totalCount === 0} onCheckedChange={onToggleAll} />
      全选当前列表（{totalCount}）
    </label>
    <span className="text-muted-foreground" role="status">已选 {selectedCount} 项</span>
    <Button size="sm" variant="ghost" className="h-8 text-xs" disabled={disabled || selectedCount === 0} onClick={onClearSelection}>取消选择</Button>
    <Button size="sm" variant="soft-destructive" className="h-8 gap-1.5 text-xs sm:ml-auto" disabled={disabled || selectedCount === 0} onClick={onCleanup}>
      <Trash2 className="h-3.5 w-3.5" />清理选中（{selectedCount}）
    </Button>
    <span className="w-full text-muted-foreground">鼠标左键拖动框选多条，按 Delete 或点击“清理选中”删除；Ctrl / Shift 可追加选择。</span>
  </div>;
}

export function BulkCleanupDialog({ targets, description, busy, failureMessage, onClose, onConfirm }: {
  targets: CleanupTarget[] | null; description: string; busy: boolean; failureMessage: string;
  onClose: () => void; onConfirm: () => void;
}) {
  return <Dialog open={targets !== null} onOpenChange={(open) => { if (!open && !busy) onClose(); }}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>确认清理选中的 {targets?.length ?? 0} 项？</DialogTitle>
        <DialogDescription>{description} 此操作不可撤销。</DialogDescription>
      </DialogHeader>
      <ul className="max-h-52 overflow-auto space-y-1 text-xs break-all" aria-label="本次清理项目">
        {targets?.map((target) => <li key={target.key} className="rounded bg-muted/50 px-2 py-1.5">{target.label}</li>)}
      </ul>
      {failureMessage && <p className="text-xs text-destructive" role="alert">{failureMessage} 已成功的项目不会重复提交。</p>}
      <DialogFooter>
        <Button variant="outline" disabled={busy} onClick={onClose}>取消</Button>
        <Button variant="destructive" disabled={busy || !targets?.length} onClick={onConfirm}>{busy ? "正在清理…" : failureMessage ? "重试失败项" : "确认清理"}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
}
