export interface SelectionPoint {
  x: number;
  y: number;
}

export interface SelectionRect {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

export type DragSelectMode = "replace" | "add" | "toggle";

export function selectionRect(a: SelectionPoint, b: SelectionPoint): SelectionRect {
  return {
    left: Math.min(a.x, b.x),
    top: Math.min(a.y, b.y),
    right: Math.max(a.x, b.x),
    bottom: Math.max(a.y, b.y),
  };
}

export function intersectsSelection(a: SelectionRect, b: SelectionRect): boolean {
  return a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
}

/** 将框选矩形裁剪到可视滚动容器边界内，避免选框阴影覆盖顶栏/底栏。 */
export function clampSelectionRect(rect: SelectionRect, bounds: SelectionRect): SelectionRect | null {
  const left = Math.max(rect.left, bounds.left);
  const top = Math.max(rect.top, bounds.top);
  const right = Math.min(rect.right, bounds.right);
  const bottom = Math.min(rect.bottom, bounds.bottom);
  if (right <= left || bottom <= top) return null;
  return { left, top, right, bottom };
}

export function mergeDragSelection(hits: string[], base: string[], additive: boolean): string[] {
  return applyDragSelection(hits, base, additive ? "add" : "replace");
}

/**
 * 按框选模式合并命中项：
 * - replace：普通拖动，替换当前选区
 * - add：Shift + 拖动，累加至原选区
 * - toggle：Ctrl / Cmd + 拖动，对框内项执行反选（已选的取消，未选的选中）
 */
export function applyDragSelection(hits: string[], base: string[], mode: DragSelectMode): string[] {
  if (mode === "replace") return [...new Set(hits)];
  if (mode === "add") return [...new Set([...base, ...hits])];
  const hitSet = new Set(hits);
  const baseSet = new Set(base);
  const result: string[] = [];
  for (const key of baseSet) {
    if (!hitSet.has(key)) result.push(key);
  }
  for (const key of hitSet) {
    if (!baseSet.has(key)) result.push(key);
  }
  return result;
}

/** Shift + 点击连续区间选择；若锚点不在当前可见列表中，则退化为单条切换。 */
export function selectRangeKeys(
  orderedKeys: string[],
  anchorKey: string | null,
  targetKey: string,
  base: string[],
): string[] {
  const targetIndex = orderedKeys.indexOf(targetKey);
  if (targetIndex === -1) return [...new Set(base)];
  const anchorIndex = anchorKey ? orderedKeys.indexOf(anchorKey) : -1;
  if (anchorIndex === -1) {
    const next = new Set(base);
    if (next.has(targetKey)) next.delete(targetKey);
    else next.add(targetKey);
    return [...next];
  }
  const start = Math.min(anchorIndex, targetIndex);
  const end = Math.max(anchorIndex, targetIndex);
  return [...new Set([...base, ...orderedKeys.slice(start, end + 1)])];
}

/** 反选当前可见列表。 */
export function invertSelectionKeys(orderedKeys: string[], selected: string[]): string[] {
  const current = new Set(selected);
  return orderedKeys.filter((key) => !current.has(key));
}
