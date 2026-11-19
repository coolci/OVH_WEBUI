export interface SelectionPoint { x: number; y: number }
export interface SelectionRect { left: number; top: number; right: number; bottom: number }

export function selectionRect(a: SelectionPoint, b: SelectionPoint): SelectionRect {
  return { left: Math.min(a.x, b.x), top: Math.min(a.y, b.y), right: Math.max(a.x, b.x), bottom: Math.max(a.y, b.y) };
}

export function intersectsSelection(a: SelectionRect, b: SelectionRect): boolean {
  return a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
}

export function mergeDragSelection(hits: string[], base: string[], additive: boolean): string[] {
  return [...new Set(additive ? [...base, ...hits] : hits)];
}
