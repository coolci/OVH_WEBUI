import { describe, expect, it } from "vitest";
import {
  applyDragSelection,
  clampSelectionRect,
  intersectsSelection,
  invertSelectionKeys,
  mergeDragSelection,
  selectionRect,
  selectRangeKeys,
} from "./drag-selection";

describe("drag-selection", () => {
  it("marquee selects intersecting rows in either drag direction and excludes adjacent rows", () => {
    const start = { x: 200, y: 180 };
    const end = { x: 20, y: 40 };
    const rows = [
      { left: 0, right: 250, top: 0, bottom: 40 },
      { left: 0, right: 250, top: 40, bottom: 100 },
      { left: 0, right: 250, top: 110, bottom: 170 },
      { left: 0, right: 250, top: 180, bottom: 240 },
    ];
    for (const rect of [selectionRect(start, end), selectionRect(end, start)]) {
      expect(rows.map((row) => intersectsSelection(rect, row))).toEqual([false, true, true, false]);
    }
  });

  it("normal drag replaces selection; modifier drag adds exact identities without duplicates", () => {
    expect(mergeDragSelection(["row-2", "row-3", "row-2"], ["row-1"], false)).toEqual(["row-2", "row-3"]);
    expect(mergeDragSelection(["row-2", "row-3"], ["row-1", "row-2"], true)).toEqual([
      "row-1",
      "row-2",
      "row-3",
    ]);
    expect(mergeDragSelection([], ["row-1"], false)).toEqual([]);
    expect(mergeDragSelection([], ["row-1"], true)).toEqual(["row-1"]);
  });

  it("toggle drag mode inverts hit rows while preserving untouched selections", () => {
    expect(applyDragSelection(["row-2", "row-3"], ["row-1", "row-2"], "toggle")).toEqual(["row-1", "row-3"]);
    expect(applyDragSelection(["row-1", "row-2"], ["row-1", "row-2"], "toggle")).toEqual([]);
    expect(applyDragSelection([], ["row-1"], "toggle")).toEqual(["row-1"]);
  });

  it("range selection selects contiguous rows in either direction and falls back when anchor is missing", () => {
    const ordered = ["r1", "r2", "r3", "r4", "r5"];
    expect(selectRangeKeys(ordered, "r2", "r4", ["r2"])).toEqual(["r2", "r3", "r4"]);
    expect(selectRangeKeys(ordered, "r4", "r2", ["r1", "r4"])).toEqual(["r1", "r4", "r2", "r3"]);
    expect(selectRangeKeys(ordered, null, "r3", ["r1"])).toEqual(["r1", "r3"]);
    expect(selectRangeKeys(ordered, "deleted-anchor", "r3", ["r3"])).toEqual([]);
  });

  it("inverts selection across visible rows only", () => {
    expect(invertSelectionKeys(["a", "b", "c", "d"], ["b", "d"])).toEqual(["a", "c"]);
    expect(invertSelectionKeys(["a", "b"], [])).toEqual(["a", "b"]);
    expect(invertSelectionKeys(["a", "b"], ["a", "b"])).toEqual([]);
  });

  it("clamps visual marquee rect to scroll viewport bounds", () => {
    const bounds = { left: 240, top: 56, right: 1200, bottom: 800 };
    expect(clampSelectionRect({ left: 200, top: 10, right: 500, bottom: 300 }, bounds)).toEqual({
      left: 240,
      top: 56,
      right: 500,
      bottom: 300,
    });
    expect(clampSelectionRect({ left: 10, top: 10, right: 100, bottom: 40 }, bounds)).toBeNull();
  });
});
