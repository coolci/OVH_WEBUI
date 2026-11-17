import { useEffect, useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/http";
import { apiMessage } from "@/lib/api-error";
import { cleanupDescriptions, cleanupTargets, retainVisibleSelection, type CleanupKind, type CleanupTarget } from "@/lib/bulk-cleanup";

export function useBulkCleanup<T>(items: T[], getTarget: (item: T) => CleanupTarget, kind: CleanupKind) {
  const qc = useQueryClient();
  const targets = useMemo(() => items.map(getTarget), [items, getTarget]);
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  const [confirmation, setConfirmation] = useState<CleanupTarget[] | null>(null);
  const [failureMessage, setFailureMessage] = useState("");
  const selectedTargets = targets.filter((target) => selected.has(target.key));
  const allSelected = targets.length > 0 && selectedTargets.length === targets.length;

  useEffect(() => {
    setSelected((current) => retainVisibleSelection(current, targets));
  }, [targets]);

  const mutation = useMutation({
    mutationFn: (snapshot: CleanupTarget[]) => cleanupTargets(snapshot,
      (target) => api.delete(target.path, { params: target.params })),
    retry: false,
    onSuccess: ({ succeeded, failed }) => {
      const removed = new Set(succeeded);
      setSelected((current) => new Set([...current].filter((key) => !removed.has(key))));
      if (succeeded.length) toast.success(`已清理 ${succeeded.length} 项`);
      const message = failed.length ? `${failed.length} 项清理失败：${apiMessage(failed[0].error) || "请求失败，请重试"}` : "";
      setFailureMessage(message);
      setConfirmation(failed.length ? failed.map(({ target }) => target) : null);
      if (message) toast.error(message);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: [kind] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
  });
  const busy = mutation.isPending;
  const locked = busy || confirmation !== null;
  return {
    busy, locked,
    selectedCount: selectedTargets.length,
    totalCount: targets.length,
    allSelected,
    someSelected: selectedTargets.length > 0 && !allSelected,
    selectedKeys: selectedTargets.map((target) => target.key),
    selectKeys: (keys: string[]) => {
      if (!locked) setSelected(retainVisibleSelection(new Set(keys), targets));
    },
    isSelected: (item: T) => selected.has(getTarget(item).key),
    toggle: (item: T) => {
      if (locked) return;
      const key = getTarget(item).key;
      setSelected((current) => {
        const next = new Set(current);
        if (next.has(key)) next.delete(key); else next.add(key);
        return next;
      });
    },
    toggleAll: () => {
      if (!locked) setSelected(allSelected ? new Set() : new Set(targets.map((target) => target.key)));
    },
    clearSelection: () => { if (!locked) setSelected(new Set()); },
    requestCleanup: () => {
      if (!locked && selectedTargets.length) {
        setFailureMessage("");
        // Freeze the exact rows at confirmation time; polling cannot expand this list.
        setConfirmation(selectedTargets.map((target) => ({ ...target, params: target.params && { ...target.params } })));
      }
    },
    dialogProps: {
      targets: confirmation,
      description: cleanupDescriptions[kind],
      busy,
      failureMessage,
      onClose: () => { if (!busy) setConfirmation(null); },
      onConfirm: () => { if (confirmation?.length && !busy) mutation.mutate(confirmation); },
    },
  };
}
