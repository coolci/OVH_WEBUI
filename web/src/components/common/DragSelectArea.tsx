import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import {
  applyDragSelection,
  clampSelectionRect,
  intersectsSelection,
  selectionRect,
  type DragSelectMode,
  type SelectionPoint,
  type SelectionRect,
} from "@/lib/drag-selection";
import { cn } from "@/lib/utils";

const INTERACTIVE_SELECTOR =
  'button,a,input,textarea,select,label,[role="button"],[role="checkbox"],[role="combobox"],[role="menuitem"],[contenteditable="true"],[data-no-drag-select]';

const EDITABLE_SELECTOR = 'input,textarea,select,[contenteditable="true"]';

interface DragSession {
  pointer: number;
  start: SelectionPoint;
  current: SelectionPoint;
  base: string[];
  allowed: Set<string>;
  mode: DragSelectMode;
  active: boolean;
  scroll: HTMLElement | null;
  startRowKey: string | null;
  shiftKey: boolean;
  ctrlOrMeta: boolean;
}

interface MarqueeVisual {
  rect: SelectionRect;
  count: number;
  mode: DragSelectMode;
}

export interface DragSelectAreaProps {
  children: ReactNode;
  className?: string;
  selectedKeys: string[];
  disabled: boolean;
  onSelectionChange: (keys: string[]) => void;
  onToggleKey?: (key: string, shiftKey: boolean) => void;
  onSelectAll?: () => void;
  onDeleteSelected: () => void;
}

function findScrollContainer(element: HTMLElement): HTMLElement | null {
  let node: HTMLElement | null = element.parentElement;
  while (node) {
    if (/(auto|scroll)/.test(getComputedStyle(node).overflowY)) return node;
    node = node.parentElement;
  }
  return null;
}

function visibleViewportBounds(scroll: HTMLElement | null): SelectionRect {
  const viewport: SelectionRect = {
    left: 0,
    top: 0,
    right: window.innerWidth,
    bottom: window.innerHeight,
  };
  if (!scroll) return viewport;
  const r = scroll.getBoundingClientRect();
  return {
    left: Math.max(viewport.left, r.left),
    top: Math.max(viewport.top, r.top),
    right: Math.min(viewport.right, r.right),
    bottom: Math.min(viewport.bottom, r.bottom),
  };
}

function isEditableTarget(target: EventTarget | null): boolean {
  return target instanceof Element && Boolean(target.closest(EDITABLE_SELECTOR));
}

function hasOpenModal(): boolean {
  return Boolean(document.querySelector('[role="dialog"],[role="alertdialog"]'));
}

/**
 * 列表级鼠标框选与批量选择交互容器：
 * - 普通拖动：框选替换；Shift+拖动：追加；Ctrl/Cmd+拖动：反选；
 * - Shift+点击行：区间连选；Ctrl/Cmd+点击行：切换单行；已进入选择态时单击行本体即可切换；
 * - 双击文字保留原生选词复制；不拦截输入框和弹窗；
 * - 全局快捷键：Delete/Backspace 触发删除确认、Esc 取消拖动或清空选区、Ctrl/Cmd+A 全选当前列表。
 */
export function DragSelectArea({
  children,
  className,
  selectedKeys,
  disabled,
  onSelectionChange,
  onToggleKey,
  onSelectAll,
  onDeleteSelected,
}: DragSelectAreaProps) {
  const root = useRef<HTMLDivElement>(null);
  const session = useRef<DragSession | null>(null);
  const frame = useRef<number>(0);
  const suppressClickUntil = useRef<number>(0);

  const selectedKeysRef = useRef(selectedKeys);
  selectedKeysRef.current = selectedKeys;
  const onSelectionChangeRef = useRef(onSelectionChange);
  onSelectionChangeRef.current = onSelectionChange;
  const onToggleKeyRef = useRef(onToggleKey);
  onToggleKeyRef.current = onToggleKey;
  const onSelectAllRef = useRef(onSelectAll);
  onSelectAllRef.current = onSelectAll;
  const onDeleteSelectedRef = useRef(onDeleteSelected);
  onDeleteSelectedRef.current = onDeleteSelected;

  const [visual, setVisual] = useState<MarqueeVisual | null>(null);
  const [dragging, setDragging] = useState(false);

  const detachListenersRef = useRef<(() => void) | null>(null);

  const update = useCallback(() => {
    const drag = session.current;
    const element = root.current;
    if (!drag?.active || !element) return;
    const bounds = element.getBoundingClientRect();
    const rawRect = selectionRect(
      { x: bounds.left + drag.start.x, y: bounds.top + drag.start.y },
      drag.current,
    );
    const hits: string[] = [];
    element.querySelectorAll<HTMLElement>("[data-selection-key]").forEach((row) => {
      const key = row.dataset.selectionKey;
      if (
        key &&
        drag.allowed.has(key) &&
        row.getClientRects().length > 0 &&
        intersectsSelection(rawRect, row.getBoundingClientRect())
      ) {
        hits.push(key);
      }
    });
    const merged = applyDragSelection(hits, drag.base, drag.mode);
    onSelectionChangeRef.current(merged);
    const clamped = clampSelectionRect(rawRect, visibleViewportBounds(drag.scroll));
    setVisual(clamped ? { rect: clamped, count: merged.length, mode: drag.mode } : null);
  }, []);

  const stop = useCallback((restore = false) => {
    const drag = session.current;
    session.current = null;
    cancelAnimationFrame(frame.current);
    frame.current = 0;
    detachListenersRef.current?.();
    detachListenersRef.current = null;
    if (restore && drag?.active) {
      onSelectionChangeRef.current(drag.base);
    }
    setDragging(false);
    setVisual(null);
  }, []);

  useEffect(() => {
    if (disabled) stop();
  }, [disabled, stop]);

  useEffect(() => () => stop(), [stop]);

  // 页面级键盘快捷键（独立于容器焦点，勾选 Checkbox 或点击工具栏按钮后仍可直接按 Delete / Esc / Ctrl+A）
  useEffect(() => {
    if (disabled) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (session.current?.active) {
          event.preventDefault();
          stop(true);
          return;
        }
        if (!hasOpenModal() && !isEditableTarget(event.target) && selectedKeysRef.current.length > 0) {
          event.preventDefault();
          onSelectionChangeRef.current([]);
        }
        return;
      }

      if (hasOpenModal() || isEditableTarget(event.target)) return;

      if (
        (event.key === "Delete" || event.key === "Backspace") &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey &&
        selectedKeysRef.current.length > 0
      ) {
        event.preventDefault();
        onDeleteSelectedRef.current();
        return;
      }

      if (
        (event.ctrlKey || event.metaKey) &&
        !event.altKey &&
        !event.shiftKey &&
        event.key.toLowerCase() === "a" &&
        onSelectAllRef.current &&
        root.current?.querySelector("[data-selection-key]")
      ) {
        event.preventDefault();
        onSelectAllRef.current();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [disabled, stop]);

  const start = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (
      disabled ||
      event.pointerType !== "mouse" ||
      event.button !== 0 ||
      event.detail >= 2 ||
      !(event.target instanceof Element) ||
      event.target.closest(INTERACTIVE_SELECTOR)
    ) {
      return;
    }

    stop();

    const element = event.currentTarget;
    const bounds = element.getBoundingClientRect();
    const scroll = findScrollContainer(element);
    const allowed = new Set(
      Array.from(element.querySelectorAll<HTMLElement>("[data-selection-key]"))
        .map((row) => row.dataset.selectionKey || "")
        .filter(Boolean),
    );
    const startRowKey =
      event.target.closest<HTMLElement>("[data-selection-key]")?.dataset.selectionKey ?? null;
    const ctrlOrMeta = event.ctrlKey || event.metaKey;
    const mode: DragSelectMode = ctrlOrMeta ? "toggle" : event.shiftKey ? "add" : "replace";

    // Shift+点击行时阻止浏览器原生跨行蓝底选字，普通点击保留双击选词复制能力
    if (event.shiftKey && startRowKey) {
      event.preventDefault();
    }

    session.current = {
      pointer: event.pointerId,
      start: { x: event.clientX - bounds.left, y: event.clientY - bounds.top },
      current: { x: event.clientX, y: event.clientY },
      base: [...selectedKeysRef.current],
      allowed,
      mode,
      active: false,
      scroll,
      startRowKey,
      shiftKey: event.shiftKey,
      ctrlOrMeta,
    };

    const tick = () => {
      const drag = session.current;
      if (!drag?.active) return;
      if (drag.scroll) {
        const vp = visibleViewportBounds(drag.scroll);
        const edge = 56;
        let speed = 0;
        if (drag.current.y < vp.top + edge) {
          const ratio = Math.min(1, Math.max(0, (vp.top + edge - drag.current.y) / edge));
          speed = -Math.round(4 + 18 * ratio);
        } else if (drag.current.y > vp.bottom - edge) {
          const ratio = Math.min(1, Math.max(0, (drag.current.y - (vp.bottom - edge)) / edge));
          speed = Math.round(4 + 18 * ratio);
        }
        if (speed !== 0) {
          const prevTop = drag.scroll.scrollTop;
          drag.scroll.scrollTop += speed;
          if (drag.scroll.scrollTop !== prevTop) update();
        }
      }
      frame.current = requestAnimationFrame(tick);
    };

    const onWindowMove = (moveEvent: PointerEvent) => {
      const drag = session.current;
      const el = root.current;
      if (!drag || drag.pointer !== moveEvent.pointerId || !el) return;
      drag.current = { x: moveEvent.clientX, y: moveEvent.clientY };
      drag.mode =
        moveEvent.ctrlKey || moveEvent.metaKey
          ? "toggle"
          : moveEvent.shiftKey
            ? "add"
            : "replace";
      const curBounds = el.getBoundingClientRect();
      if (
        !drag.active &&
        Math.hypot(
          moveEvent.clientX - curBounds.left - drag.start.x,
          moveEvent.clientY - curBounds.top - drag.start.y,
        ) >= 6
      ) {
        drag.active = true;
        setDragging(true);
        window.getSelection()?.removeAllRanges();
        frame.current = requestAnimationFrame(tick);
      }
      if (drag.active) update();
    };

    const onWindowUp = (upEvent: PointerEvent) => {
      const drag = session.current;
      if (!drag || drag.pointer !== upEvent.pointerId) return;
      if (drag.active) {
        update();
        suppressClickUntil.current = performance.now() + 80;
        stop();
        return;
      }
      stop();

      // 若用户通过原生选区选中了文本，不干扰复制
      if ((window.getSelection()?.toString() || "").length > 0) return;

      if (drag.startRowKey) {
        if (drag.shiftKey) {
          onToggleKeyRef.current?.(drag.startRowKey, true);
        } else if (drag.ctrlOrMeta || drag.base.length > 0) {
          onToggleKeyRef.current?.(drag.startRowKey, false);
        }
      } else if (!drag.shiftKey && !drag.ctrlOrMeta && drag.base.length > 0) {
        onSelectionChangeRef.current([]);
      }
    };

    const onWindowCancel = () => stop(true);
    const onContainerScroll = () => {
      if (session.current?.active) update();
    };

    window.addEventListener("pointermove", onWindowMove);
    window.addEventListener("pointerup", onWindowUp);
    window.addEventListener("pointercancel", onWindowCancel);
    scroll?.addEventListener("scroll", onContainerScroll, { passive: true });
    detachListenersRef.current = () => {
      window.removeEventListener("pointermove", onWindowMove);
      window.removeEventListener("pointerup", onWindowUp);
      window.removeEventListener("pointercancel", onWindowCancel);
      scroll?.removeEventListener("scroll", onContainerScroll);
    };
  };

  const modeHint = visual?.mode === "toggle" ? "反选" : visual?.mode === "add" ? "追加" : "";

  return (
    <div
      ref={root}
      className={cn(
        "relative outline-none",
        dragging && "select-none cursor-crosshair",
        selectedKeys.length > 0 && "[&_[data-selection-key]]:cursor-pointer",
        className,
      )}
      aria-label="支持鼠标框选的列表"
      onPointerDown={start}
      onClickCapture={(event) => {
        if (performance.now() < suppressClickUntil.current) {
          event.preventDefault();
          event.stopPropagation();
        }
      }}
    >
      {children}
      {visual &&
        createPortal(
          <div
            aria-hidden="true"
            className="pointer-events-none fixed z-50 rounded-md border border-primary/80 bg-primary/15 shadow-[inset_0_0_0_1px_rgba(255,255,255,0.08)]"
            style={{
              left: visual.rect.left,
              top: visual.rect.top,
              width: visual.rect.right - visual.rect.left,
              height: visual.rect.bottom - visual.rect.top,
            }}
          >
            <div className="absolute right-1.5 bottom-1.5 inline-flex items-center gap-1 rounded-md bg-primary px-1.5 py-0.5 text-[11px] font-medium text-primary-foreground shadow-sm whitespace-nowrap">
              <span>已选 {visual.count} 项</span>
              {modeHint && <span className="opacity-85">· {modeHint}</span>}
            </div>
          </div>,
          document.body,
        )}
    </div>
  );
}
