import { useEffect, useMemo, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { api } from "@/lib/http";
import { apiMessage } from "@/lib/api-error";
import {
  cleanupDescriptions,
  cleanupTargets,
  retainVisibleSelection,
  type CleanupKind,
  type CleanupTarget,
} from "@/lib/bulk-cleanup";
import { invertSelectionKeys, selectRangeKeys } from "@/lib/drag-selection";

export interface CleanupProgress {
  done: number;
  total: number;
}

export function useBulkCleanup<T>(items: T[], getTarget: (item: T) => CleanupTarget, kind: CleanupKind) {
  const qc = useQueryClient();
  const targets = useMemo(() => items.map(getTarget), [items, getTarget]);
  const orderedKeys = useMemo(() => targets.map((target) => target.key), [targets]);
  const [selected, setSelected] = useState<Set<string>>(() => new Set());
  const [anchorKey, setAnchorKey] = useState<string | null>(null);
  const [confirmation, setConfirmation] = useState<CleanupTarget[] | null>(null);
  const [failureMessage, setFailureMessage] = useState("");
  const [progress, setProgress] = useState<CleanupProgress | null>(null);
  const selectedTargets = targets.filter((target) => selected.has(target.key));
  const allSelected = targets.length > 0 && selectedTargets.length === targets.length;

  useEffect(() => {
    setSelected((current) => retainVisibleSelection(current, targets));
  }, [targets]);

  const mutation = useMutation({
    mutationFn: (snapshot: CleanupTarget[]) =>
      cleanupTargets(
        snapshot,
        (target) => api.delete(target.path, { params: target.params }),
        (done, total) => setProgress({ done, total }),
      ),
    retry: false,
    onSuccess: ({ succeeded, failed }) => {
      const removed = new Set(succeeded);
      setSelected((current) => new Set([...current].filter((key) => !removed.has(key))));
      if (succeeded.length) toast.success(`已清理 ${succeeded.length} 项`);
      const firstFailure = failed[0];
      const message = firstFailure
        ? `${failed.length} 项清理失败：${apiMessage(firstFailure.error) || "请求失败，请重试"}`
        : "";
      setFailureMessage(message);
      setConfirmation(failed.length ? failed.map(({ target }) => target) : null);
      if (message) toast.error(message);
    },
    onSettled: () => {
      setProgress(null);
      qc.invalidateQueries({ queryKey: [kind] });
      qc.invalidateQueries({ queryKey: ["stats"] });
    },
  });
  const busy = mutation.isPending;
  const locked = busy || confirmation !== null;

  const toggleKey = (key: string, shiftKey = false) => {
    if (locked) return;
    if (shiftKey && anchorKey) {
      setSelected((current) =>
        retainVisibleSelection(
          new Set(selectRangeKeys(orderedKeys, anchorKey, key, [...current])),
          targets,
        ),
      );
      setAnchorKey(key);
      return;
    }
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
    setAnchorKey(key);
  };

  return {
    busy,
    locked,
    selectedCount: selectedTargets.length,
    totalCount: targets.length,
    allSelected,
    someSelected: selectedTargets.length > 0 && !allSelected,
    selectedKeys: selectedTargets.map((target) => target.key),
    selectKeys: (keys: string[]) => {
      if (locked) return;
      const retained = retainVisibleSelection(new Set(keys), targets);
      setSelected(retained);
      if (retained.size === 0) setAnchorKey(null);
      else {
        const lastKey = keys[keys.length - 1];
        if (lastKey && retained.has(lastKey)) setAnchorKey(lastKey);
      }
    },
    isSelected: (item: T) => selected.has(getTarget(item).key),
    toggle: (item: T, shiftKey = false) => toggleKey(getTarget(item).key, shiftKey),
    toggleKey,
    selectAll: () => {
      if (!locked) setSelected(new Set(orderedKeys));
    },
    toggleAll: () => {
      if (locked) return;
      if (allSelected) {
        setSelected(new Set());
        setAnchorKey(null);
      } else {
        setSelected(new Set(orderedKeys));
      }
    },
    invertSelection: () => {
      if (locked) return;
      setSelected((current) => new Set(invertSelectionKeys(orderedKeys, [...current])));
    },
    clearSelection: () => {
      if (!locked) {
        setSelected(new Set());
        setAnchorKey(null);
      }
    },
    requestCleanup: () => {
      if (!locked && selectedTargets.length) {
        setFailureMessage("");
        // Freeze the exact rows at confirmation time; polling cannot expand this list.
        setConfirmation(
          selectedTargets.map((target) => ({
            ...target,
            params: target.params && { ...target.params },
          })),
        );
      }
    },
    dialogProps: {
      targets: confirmation,
      description: cleanupDescriptions[kind],
      busy,
      progress,
      failureMessage,
      onClose: () => {
        if (!busy) setConfirmation(null);
      },
      onConfirm: () => {
        if (confirmation?.length && !busy) mutation.mutate(confirmation);
      },
    },
  };
}
