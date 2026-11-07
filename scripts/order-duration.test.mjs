import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/lib/order-duration.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } });
const { durationMonths, normalizeDuration, supportsDuration, isOrderDuration, isDefaultDuration, formatDuration } = await import(
  `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`
);

const t = (key, opts) => `${key}:${opts.count}`;

test("empty and legacy values mean one month, same as the backend", () => {
  assert.equal(durationMonths(undefined), 1);
  assert.equal(durationMonths(""), 1);
  assert.equal(normalizeDuration(""), "P1M");
  assert.equal(isDefaultDuration(null), true);
});

test("years and months normalize to the stored PnM form", () => {
  assert.equal(normalizeDuration("P1Y"), "P12M");
  assert.equal(normalizeDuration("p24m"), "P24M");
  assert.equal(durationMonths("P2Y"), 24);
});

test("only whole months or years are valid periods", () => {
  for (const ok of ["P1M", "P12M", "P1Y", "p3m"]) assert.equal(isOrderDuration(ok), true, ok);
  for (const bad of ["P0M", "1M", "P1D", "P1.5Y", "", null, 12]) assert.equal(isOrderDuration(bad), false, String(bad));
});

test("a yearly choice matches an OVH offer spelled either way", () => {
  assert.equal(supportsDuration(["P1M", "P1Y"], "P12M"), true);
  assert.equal(supportsDuration(["P1M"], "P12M"), false);
  // unknown offers never block saving; the backend re-checks before ordering
  assert.equal(supportsDuration(undefined, "P36M"), true);
  assert.equal(supportsDuration([], "P36M"), true);
});

test("labels pick singular/plural month and year keys", () => {
  assert.equal(formatDuration(t, "P1M"), "orderDuration.month:1");
  assert.equal(formatDuration(t, "P6M"), "orderDuration.months:6");
  assert.equal(formatDuration(t, "P12M"), "orderDuration.year:1");
  assert.equal(formatDuration(t, "P36M"), "orderDuration.years:3");
});
