import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import ts from "typescript";

test("disabled prerelease discovery returns the sales catalog without adding stock-only models", async () => {
  const source = await readFile(new URL("../src/lib/stock-plans.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } });
  const { mergeStockPlans } = await import("data:text/javascript;base64," + Buffer.from(compiled.outputText).toString("base64"));
  const plans = [{planCode: "24sk202"}];
  const stock = [{planCode: "27skle03-v1", memory: "ram-64g", storage: "softraid-2x480ssd"}];
  assert.equal(mergeStockPlans(plans, stock), plans);
  assert.deepEqual(mergeStockPlans([], stock), []);
  assert.deepEqual(plans, [{planCode: "24sk202"}]);
});
