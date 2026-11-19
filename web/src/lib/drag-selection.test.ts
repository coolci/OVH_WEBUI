import { describe, expect, it } from "vitest";
import { intersectsSelection, mergeDragSelection, selectionRect } from "./drag-selection";

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
});
