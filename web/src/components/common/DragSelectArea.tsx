import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { intersectsSelection, mergeDragSelection, selectionRect, type SelectionPoint, type SelectionRect } from "@/lib/drag-selection";

const interactive = 'button,a,input,textarea,select,label,[role="button"],[role="checkbox"],[role="combobox"],[contenteditable="true"],[data-no-drag-select]';
interface DragSession {
  pointer: number; start: SelectionPoint; current: SelectionPoint; base: string[];
  allowed: Set<string>; additive: boolean; active: boolean; scroll: HTMLElement | null;
}

/** Mouse marquee selection. Financial actions and deletion remain explicit. */
export function DragSelectArea({ children, className, selectedKeys, onSelectionChange, onDeleteSelected, disabled }: {
  children: ReactNode; className?: string; selectedKeys: string[]; disabled: boolean;
  onSelectionChange: (keys: string[]) => void; onDeleteSelected: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const session = useRef<DragSession | null>(null);
  const frame = useRef<number>(0);
  const callback = useRef(onSelectionChange);
  callback.current = onSelectionChange;
  const [box, setBox] = useState<SelectionRect | null>(null);

  const stop = useCallback((restore = false) => {
    const drag = session.current;
    session.current = null;
    cancelAnimationFrame(frame.current);
    if (drag && root.current?.hasPointerCapture(drag.pointer)) root.current.releasePointerCapture(drag.pointer);
    if (restore && drag?.active) callback.current(drag.base);
    setBox(null);
  }, []);

  useEffect(() => {
    if (disabled) stop();
  }, [disabled, stop]);
  useEffect(() => () => { cancelAnimationFrame(frame.current); session.current = null; }, []);

  const update = () => {
    const drag = session.current;
    const element = root.current;
    if (!drag?.active || !element) return;
    const bounds = element.getBoundingClientRect();
    const rectangle = selectionRect({ x: bounds.left + drag.start.x, y: bounds.top + drag.start.y }, drag.current);
    const hits: string[] = [];
    element.querySelectorAll<HTMLElement>('[data-selection-key]').forEach((row) => {
      const key = row.dataset.selectionKey;
      if (key && drag.allowed.has(key) && row.getClientRects().length && intersectsSelection(rectangle, row.getBoundingClientRect())) hits.push(key);
    });
    callback.current(mergeDragSelection(hits, drag.base, drag.additive));
    setBox(rectangle);
  };

  const tick = () => {
    const drag = session.current;
    if (!drag?.active) return;
    if (drag.scroll) {
      const bounds = drag.scroll.getBoundingClientRect();
      const top = Math.max(0, bounds.top), bottom = Math.min(window.innerHeight, bounds.bottom);
      const speed = drag.current.y < top + 40 ? -14 : drag.current.y > bottom - 40 ? 14 : 0;
      if (speed) { drag.scroll.scrollTop += speed; update(); }
    }
    frame.current = requestAnimationFrame(tick);
  };

  const start = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (disabled || event.pointerType !== "mouse" || event.button !== 0 || !(event.target instanceof Element) || event.target.closest(interactive)) return;
    const element = event.currentTarget;
    const bounds = element.getBoundingClientRect();
    let scroll: HTMLElement | null = element.parentElement;
    while (scroll && !/(auto|scroll)/.test(getComputedStyle(scroll).overflowY)) scroll = scroll.parentElement;
    const allowed = new Set(Array.from(element.querySelectorAll<HTMLElement>('[data-selection-key]')).map((row) => row.dataset.selectionKey || ""));
    session.current = { pointer: event.pointerId, start: { x: event.clientX - bounds.left, y: event.clientY - bounds.top },
      current: { x: event.clientX, y: event.clientY }, base: [...selectedKeys], allowed,
      additive: event.ctrlKey || event.metaKey || event.shiftKey, active: false, scroll };
    element.setPointerCapture(event.pointerId);
    element.focus({ preventScroll: true });
    event.preventDefault();
  };

  return <div ref={root} tabIndex={0} className={className} aria-label="支持鼠标框选的列表"
    onPointerDown={start}
    onPointerMove={(event) => {
      const drag = session.current;
      if (!drag || drag.pointer !== event.pointerId) return;
      drag.current = { x: event.clientX, y: event.clientY };
      const bounds = event.currentTarget.getBoundingClientRect();
      if (!drag.active && Math.hypot(event.clientX - bounds.left - drag.start.x, event.clientY - bounds.top - drag.start.y) >= 6) {
        drag.active = true;
        frame.current = requestAnimationFrame(tick);
      }
      update();
    }}
    onPointerUp={(event) => { if (session.current?.pointer === event.pointerId) { update(); stop(); } }}
    onPointerCancel={() => stop(true)} onLostPointerCapture={() => { if (session.current) stop(true); }}
    onKeyDown={(event) => {
      if (event.key === "Escape" && session.current) { event.preventDefault(); stop(true); }
      if (event.key === "Delete" && !disabled && !(event.target instanceof Element && event.target.closest(interactive))) {
        event.preventDefault(); onDeleteSelected();
      }
    }}>
    {children}
    {box && createPortal(<div aria-hidden="true" className="pointer-events-none fixed z-50 border border-primary bg-primary/15"
      style={{ left: box.left, top: box.top, width: box.right - box.left, height: box.bottom - box.top }} />, document.body)}
  </div>;
}
