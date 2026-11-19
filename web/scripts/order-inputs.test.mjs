import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

const source = await readFile(new URL("../src/lib/order-inputs.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } });
const { accountsWhenEnablingAutoOrder, monitorAccountIds, isSinglePlanCode, customPlanCode, parseOptionCodes } = await import(
  `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`
);

test("multiple order accounts survive normalization without duplicates", () => {
  assert.deepEqual(monitorAccountIds({ autoOrderAccountIds: [" a ", "b", "a", ""] }), ["a", "b"]);
});

test("legacy single-account subscriptions stay on their original account", () => {
  assert.deepEqual(monitorAccountIds({ autoOrderAccountId: "a" }), ["a"]);
  assert.deepEqual(monitorAccountIds({ autoOrderAccountIds: null, autoOrderAccountId: "a" }), ["a"]);
});

test("an explicit cleared list never restores the old account", () => {
  assert.deepEqual(monitorAccountIds({ autoOrderAccountIds: [], autoOrderAccountId: "a" }), []);
  assert.deepEqual(monitorAccountIds({}), []);
});

test("explicitly enabling automatic orders defaults an empty selection to all existing accounts", () => {
  assert.deepEqual(accountsWhenEnablingAutoOrder(false, true, [], ["a", "b"]), ["a", "b"]);
});

test("opening, saving, clearing, and existing selections never silently expand account scope", () => {
  assert.deepEqual(accountsWhenEnablingAutoOrder(false, false, [], ["a", "b"]), []);
  assert.deepEqual(accountsWhenEnablingAutoOrder(true, true, [], ["a", "b"]), []);
  assert.deepEqual(accountsWhenEnablingAutoOrder(false, true, ["b"], ["a", "b"]), ["b"]);
});

test("custom models are available with an empty or incomplete catalog", () => {
  assert.equal(customPlanCode("  26custom-v1  ", []), "26custom-v1");
  assert.equal(customPlanCode("vps-custom-eu", [{ planCode: "vps-known" }]), "vps-custom-eu");
});

test("a catalog match uses its existing option regardless of case", () => {
  assert.equal(customPlanCode("24SK102", [{ planCode: "24sk102" }]), "");
});

test("multiple codes and descriptive search text cannot become an accidental order code", () => {
  for (const value of ["", " ", "plan-a,plan-b", "plan-a，plan-b", "plan-a、plan-b", "Intel Xeon", "plan-a\nplan-b"]) {
    assert.equal(isSinglePlanCode(value), false, value);
    assert.equal(customPlanCode(value, []), "", value);
  }
});

test("manually entered hardware addons preserve arbitrary codes and deduplicate separators", () => {
  assert.deepEqual(parseOptionCodes(" ram-custom，storage-custom, ram-custom\n network-custom "), ["ram-custom", "storage-custom", "network-custom"]);
});

// datacenters.ts 依赖 @/lib/availability(有货白名单):data: URL 里解析不了路径别名,先把它编译成 data: URL 再替换进去
const toDataUrl = (code) => `data:text/javascript;base64,${Buffer.from(code).toString("base64")}`;
const esm = { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 } };
const availSource = await readFile(new URL("../src/lib/availability.ts", import.meta.url), "utf8");
const availUrl = toDataUrl(ts.transpileModule(availSource, esm).outputText);
const dcSource = await readFile(new URL("../src/lib/datacenters.ts", import.meta.url), "utf8");
const dcCompiled = ts.transpileModule(dcSource, esm).outputText.replace('"@/lib/availability"', JSON.stringify(availUrl));
const { monitorDcAvailability } = await import(toDataUrl(dcCompiled));

test("monitor dashboard combines per-account and per-configuration stock without hiding stock", () => {
  assert.deepEqual(monitorDcAvailability({
    'account:"a"|gra|ram-64': "1H-low",
    'account:"b"|gra|ram-64': "unavailable",
    'account:"a"|rbx|ram-64': "unavailable",
    'account:"b"|rbx|ram-128': "72H",
    'account:"a"|ynm|ram-64': "unavailable",
  }), { gra: "1H-low", rbx: "72H", mum: "unavailable" });
});

test("monitor dashboard reads legacy keys and JSON-encoded account IDs", () => {
  assert.deepEqual(monitorDcAvailability({
    gra: "unknown",
    "gra|memory": "1H-low",
    [`account:${JSON.stringify('id|with"quotes')}|sbg|memory`]: "unavailable",
    "account:malformed|vin": "1H-low",
  }), { gra: "1H-low", sbg: "unavailable" });
});

test("monitor dashboard ignores saved stock baselines for deselected accounts", () => {
  assert.deepEqual(monitorDcAvailability({
    "gra|memory": "available",
    'account:"a"|gra|memory': "unavailable",
    'account:"b"|gra|memory': "1H-low",
    'account:"b"|rbx|memory': "1H-low",
  }, ["a"]), { gra: "unavailable" });
});

test("notification-only subscriptions still show legacy inventory without showing removed accounts", () => {
  assert.deepEqual(monitorDcAvailability({
    "gra|memory": "available",
    'account:"a"|rbx|memory': "available",
  }, []), { gra: "available" });
});
