import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import ts from "typescript";

async function load(path, replaceImports = (s) => s) {
  const source = replaceImports(await readFile(new URL(path, import.meta.url), "utf8"));
  const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } });
  return import("data:text/javascript;base64," + Buffer.from(compiled.outputText).toString("base64"));
}

// stock-plans.ts imports describeOptionCodes; stub it so the module loads on its own.
const { mergeStockPlans, stockPlanName } = await load("../src/lib/stock-plans.ts", (s) =>
  s.replace(/import \{ describeOptionCodes \} from "@\/lib\/option-groups";/, "const describeOptionCodes = (v) => v.join(' / ');")
    .replace(/^import type .*$/gm, ""));
const { preorderOptions, preorderVariantCoversAll } = await load("../src/lib/preorder-options.ts");

test("stock-only Eco models become preorder candidates; catalog models stay untouched", () => {
  const plans = [{ planCode: "24sk202" }];
  const stock = [
    { planCode: "27skle03-v1", server: "", memory: "ram-64g", storage: "softraid-2x480ssd", fqn: "27skle03-v1.ram-64g.softraid-2x480ssd" },
    { planCode: "27skle03-v1", server: "", memory: "ram-32g", storage: "softraid-2x480ssd", fqn: "27skle03-v1.ram-32g.softraid-2x480ssd" },
    { planCode: "24sk202", server: "", memory: "ram-16g", storage: "", fqn: "24sk202.ram-16g" },
    { planCode: "25scale01-v1", server: "", memory: "ram-1t", storage: "", fqn: "25scale01-v1.ram-1t" },
  ];
  const merged = mergeStockPlans(plans, stock);
  assert.equal(merged.length, 2);
  assert.deepEqual(merged[0], { planCode: "24sk202" });
  const pre = merged[1];
  assert.equal(pre.planCode, "27skle03-v1");
  assert.equal(pre.prerelease, true);
  assert.equal(pre.name, stockPlanName("27skle03-v1"));
  assert.deepEqual(pre.availableOptions.filter((o) => o.family === "memory").map((o) => o.value).sort(), ["ram-32g", "ram-64g"]);
  // nothing to add: the original array is returned unchanged
  assert.equal(mergeStockPlans(plans, [stock[2]]), plans);
});

test("preorders never submit CPU, which is not a matching condition", () => {
  assert.deepEqual(preorderOptions(["cpu-intel-x", "ram-64g", "processor-a", "softraid-2x480ssd"]), ["ram-64g", "softraid-2x480ssd"]);
  assert.deepEqual(preorderOptions(["xeon-e5", "ram-64g"], ["xeon-e5"]), ["ram-64g"]);
});

test("form feedback flags a configuration absent from the stock feed", () => {
  const fqn = "27skle03-v1.ram-64g-ddr4-ecc-2933.softraid-2x480ssd";
  assert.equal(preorderVariantCoversAll(fqn, ["ram-64g-ddr4-ecc-2933-27skle03-v1", "softraid-2x480ssd"]), true);
  assert.equal(preorderVariantCoversAll(fqn, ["ram-64g-ddr4-ecc-2666"]), false);
  assert.equal(preorderVariantCoversAll(fqn, ["softraid-2x480ssd-2x8000sa"]), false);
});
