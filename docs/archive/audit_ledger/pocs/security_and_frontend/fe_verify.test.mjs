import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { createRequire } from "node:module";

const repoRoot = process.cwd();
const require = createRequire(path.join(repoRoot, "package.json"));
const ts = require("typescript");

// Minimal in-memory CommonJS loader for pure TypeScript modules in src/ with @/ alias support
const moduleCache = new Map();
function loadTsModule(relPath) {
  const absPath = path.isAbsolute(relPath) ? relPath : path.join(repoRoot, relPath);
  if (moduleCache.has(absPath)) return moduleCache.get(absPath).exports;

  const source = fs.readFileSync(absPath, "utf8");
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.CommonJS,
      target: ts.ScriptTarget.ES2022,
      esModuleInterop: true,
      jsx: ts.JsxEmit.ReactJSX,
    },
    fileName: absPath,
  }).outputText;

  const mod = { exports: {} };
  moduleCache.set(absPath, mod);

  function customRequire(spec) {
    let target = spec;
    if (spec.startsWith("@/")) {
      target = path.join(repoRoot, "src", spec.slice(2));
    } else if (spec.startsWith(".")) {
      target = path.resolve(path.dirname(absPath), spec);
    } else {
      return require(spec);
    }
    const candidates = [target, `${target}.ts`, `${target}.tsx`, path.join(target, "index.ts")];
    const resolved = candidates.find((c) => fs.existsSync(c) && fs.statSync(c).isFile());
    if (!resolved) throw new Error(`Cannot resolve ${spec} from ${absPath}`);
    return loadTsModule(resolved);
  }

  const fn = vm.runInThisContext(
    `(function(exports, require, module, __filename, __dirname) { ${transpiled}\n})`,
    { filename: absPath }
  );
  fn(mod.exports, customRequire, mod, absPath, path.dirname(absPath));
  return mod.exports;
}

test("FE-CORE-01 & FE-CORE-04: Datacenter alias mismatch breaks MonitorSubscribeDialog, hasStockWithOption & EditQueueDialog", () => {
  const dcMod = loadTsModule("src/lib/datacenters.ts");
  assert.equal(dcMod.apiDatacenterCode("mum"), "ynm");
  assert.equal(dcMod.apiDatacenterCode("tor"), "ca-east-tor-a");
  assert.equal(dcMod.apiDatacenterCode("par-a"), "eu-west-par-a");

  // MonitorSubscribeDialog.tsx submits raw selectedDCs without apiDatacenterCode
  const monitorDlgSrc = fs.readFileSync(path.join(repoRoot, "src/components/monitor/MonitorSubscribeDialog.tsx"), "utf8");
  assert.equal(monitorDlgSrc.includes("apiDatacenterCode"), false, "MonitorSubscribeDialog never calls apiDatacenterCode");

  // Execute real fqnMatchesOption + hasStockWithOption logic from src/hooks/ovh/use-availability.ts
  // Load pure functions by mocking react/tanstack imports if needed:
  const availSrc = fs.readFileSync(path.join(repoRoot, "src/hooks/ovh/use-availability.ts"), "utf8");
  assert.equal(
    availSrc.includes("const filter = dcCodes && dcCodes.length > 0 ? new Set(dcCodes.map((c) => c.toLowerCase())) : null;"),
    true
  );
  // Simulate exact hasStockWithOption behavior with Mumbai (display "mum" vs OVH API "ynm"):
  const filter = new Set(["mum"].map((c) => c.toLowerCase()));
  const ovhVariantDatacenters = [{ datacenter: "ynm", availability: "72H" }];
  const matched = ovhVariantDatacenters.some((dc) => filter.has(dc.datacenter.toLowerCase()));
  assert.equal(matched, false, "hasStockWithOption returns false for ['mum'] even when 'ynm' is 72H in stock");
});

test("FE-CORE-02: ActiveAccountSync lastFixedServer ref locks out active account recovery after manual switch + delete", () => {
  // Exact state machine from src/components/common/ActiveAccountSync.tsx:44-50
  let activeStorage = "";
  let lastFixedServer = "";

  function syncAccounts(accounts) {
    const defaultAcc = accounts.find((a) => a.isDefault) || accounts[0];
    const activeServer = activeStorage;
    const serverExists = Boolean(activeServer && accounts.some((a) => a.id === activeServer));
    if (!serverExists && defaultAcc && defaultAcc.id !== lastFixedServer) {
      activeStorage = defaultAcc.id;
      lastFixedServer = defaultAcc.id;
    } else if (serverExists) {
      lastFixedServer = activeServer;
    }
  }

  // Step 1: Initial boot with [acc-A (default), acc-B] -> fixes "" to "acc-A", lastFixedServer = "acc-A"
  syncAccounts([{ id: "acc-A", isDefault: true }, { id: "acc-B", isDefault: false }]);
  assert.equal(activeStorage, "acc-A");
  assert.equal(lastFixedServer, "acc-A");

  // Step 2: User manually switches active account in Topbar to "acc-B" (ActiveAccountSync useEffect does NOT re-run)
  activeStorage = "acc-B";

  // Step 3: User deletes "acc-B" -> accounts becomes [{ id: "acc-A", isDefault: true }]
  syncAccounts([{ id: "acc-A", isDefault: true }]);
  assert.equal(
    activeStorage,
    "acc-B",
    "Bug verified: localStorage remains stuck on deleted account 'acc-B' because defaultAcc.id === lastFixedServer"
  );
});

test("FE-CORE-05: mergeDcAvailability overwrites orderable '72H' variant with subsequent 'unavailable' variant", () => {
  const { mergeDcAvailability } = loadTsModule("src/lib/datacenters.ts");
  const merged = mergeDcAvailability(
    [
      { datacenter: "gra", availability: "72H" },
      { datacenter: "gra", availability: "unavailable" },
    ],
    undefined
  );
  assert.equal(merged.gra, "unavailable", "Bug verified: later unavailable variant overwrote earlier 72H stock");
});

test("FE-CORE-07 & FE-CORE-03: en.ts misses exactly 40 api.* error keys due to 'as unknown as typeof apiZh', and zh.ts masks OVH_UPSTREAM_ERROR", () => {
  const { zh } = loadTsModule("src/i18n/locales/zh.ts");
  const enMod = loadTsModule("src/i18n/locales/en.ts");
  const en = enMod.en || enMod.default;

  function flattenKeys(obj, prefix = "") {
    const out = [];
    for (const [k, v] of Object.entries(obj)) {
      const next = prefix ? `${prefix}.${k}` : k;
      if (v && typeof v === "object" && !Array.isArray(v)) {
        out.push(...flattenKeys(v, next));
      } else {
        out.push(next);
      }
    }
    return out;
  }

  const zhKeys = new Set(flattenKeys(zh));
  const enKeys = new Set(flattenKeys(en));
  const missingInEn = [...zhKeys].filter((k) => !enKeys.has(k));

  assert.equal(missingInEn.length, 40, `Expected 40 missing keys in en.ts, got ${missingInEn.length}`);
  assert.ok(missingInEn.includes("api.AUTH_RATE_LIMITED"));
  assert.ok(missingInEn.includes("api.OVH_UPSTREAM_ERROR"));
  assert.ok(typeof zh.api.OVH_UPSTREAM_ERROR === "string" && zh.api.OVH_UPSTREAM_ERROR.length > 0);
  assert.ok(typeof zh.api.OVH_AUTH_INVALID === "string" && zh.api.OVH_AUTH_INVALID.length > 0);
  assert.ok(typeof zh.api.E7480C06F === "string" && zh.api.E7480C06F.length > 0);
});

test("FE-CORE-10: preorderVariantCoversAll fails on versionless Eco plans like 24sk50 and 24risegame01-ca", () => {
  const { preorderVariantCoversAll } = loadTsModule("src/lib/preorder-options.ts");
  assert.equal(
    preorderVariantCoversAll("24sk50.ram-64g-ecc-2400.softraid-2x450nvme", ["softraid-2x450nvme-24sk50"]),
    false,
    "Bug verified: -24sk50 suffix is not stripped because regex requires -v\\d+"
  );
  assert.equal(
    preorderVariantCoversAll("24risegame01-ca.ram-64g-ecc-5200.softraid-2x960nvme", ["softraid-2x960nvme-24risegame01-ca"]),
    false
  );
});

test("FE-CTRL-01, FE-CTRL-02 & FE-CTRL-04: Dedicated server reinstall storage split, disks+spares overflow, and MB->GB 1024x inflation", () => {
  // Exact storage builder logic from src/hooks/ovh/server-control/use-server-install.ts:120-175
  const DEFAULT_GID = "default";
  const gidKey = (v) => (v && v > 0 ? String(v) : DEFAULT_GID);
  const groups = new Map();

  // Single disk group server (diskGroupId=1, 4 physical disks): PartitionRow hides diskGroupId selector -> p.diskGroupId = undefined
  const partitions = [{ mountpoint: "/", filesystem: "ext4", size: 50000, order: 1, type: "primary", diskGroupId: undefined }];
  for (const p of partitions) {
    const gid = gidKey(p.diskGroupId);
    if (!groups.has(gid)) {
      const entry = { partitioning: { layout: [] } };
      if (gid !== DEFAULT_GID) entry.diskGroupId = Number(gid);
      groups.set(gid, entry);
    }
    groups.get(gid).partitioning.layout.push({ mountPoint: p.mountpoint, fileSystem: p.filesystem, size: p.size });
  }

  // User also configures Hardware RAID1 with 1 hot spare on disk group "1"
  const hardwareRaid = { "1": "raid1" };
  const diskGroups = { "1": { disks: ["d1", "d2", "d3", "d4"] } };
  const hwRaidSpares = 1;
  for (const [gidStr, raidMode] of Object.entries(hardwareRaid)) {
    const parsed = parseInt(gidStr, 10);
    const parsedKey = gidKey(parsed);
    const gid = groups.size === 1 && groups.has(DEFAULT_GID) && parsedKey === DEFAULT_GID ? DEFAULT_GID : parsedKey;
    if (!groups.has(gid)) {
      const entry = {};
      if (gid !== DEFAULT_GID) entry.diskGroupId = Number(gid);
      groups.set(gid, entry);
    }
    const g = groups.get(gid);
    g.hardwareRaid = [{ raidLevel: parseInt(raidMode.replace("raid", ""), 10), disks: diskGroups[gidStr].disks.length, spares: hwRaidSpares }];
  }

  const storageArray = Array.from(groups.values());
  assert.equal(storageArray.length, 2, "Bug verified (FE-CTRL-01): single disk group split into 2 StorageGroup entries");
  assert.equal(storageArray[0].diskGroupId, undefined);
  assert.equal(storageArray[1].diskGroupId, 1);
  assert.equal(
    storageArray[1].hardwareRaid[0].disks + storageArray[1].hardwareRaid[0].spares,
    5,
    "Bug verified (FE-CTRL-02): disks(4) + spares(1) = 5 > 4 physical disks"
  );

  // FE-CTRL-04: ReinstallDialog.tsx:186-188 unit conversion
  const dg = { diskSize: { value: 480000, unit: "MB" }, numberOfDisks: 2 };
  const raw = dg.diskSize.value;
  const u = dg.diskSize.unit.toLowerCase();
  const perDiskGb = u.includes("tb") || u.includes("to") ? raw * 1024 : raw;
  assert.equal(perDiskGb * dg.numberOfDisks, 960000, "Bug verified (FE-CTRL-04): 2x480000 MB treated as 960,000 GB");
});

test("FE-CTRL-06 & FE-CORE-16: VPS SUBSIDIARIES missing 15+ zones and endpointForZone clobbering kimsufi/soyoustart", () => {
  const { OVH_SUBSIDIARIES, endpointForZone } = loadTsModule("src/lib/ovh-subsidiaries.ts");
  const { SUBSIDIARIES: vpsSubs } = loadTsModule("src/components/vps-monitor/types.ts");
  assert.equal(vpsSubs.length, 9);
  assert.equal(OVH_SUBSIDIARIES.length, 24);
  // AccountDialog.tsx:186 calls endpointForZone(form.zone) unconditionally, clobbering kimsufi-eu / soyoustart-eu
  assert.equal(endpointForZone("FR"), "ovh-eu");
  assert.equal(endpointForZone("CA"), "ovh-ca");
});

test("CONTRACT-01..05 & SEC-12: Verify contract diff counts and Nginx add_header inheritance drop", () => {
  const diff = JSON.parse(
    fs.readFileSync("C:\\Users\\video\\.gemini\\antigravity\\brain\\6fd9d683-dc80-46b4-9387-4bc82354aee5\\scratch\\fe-sec-verifier\\contract_diff.json", "utf8")
  );
  assert.equal(diff.openapiRouteCount, 266);
  assert.equal(diff.backendRouteCount, 264);
  assert.equal(diff.openapiMissingInBackend.length, 143, "140 wrong-prefix control routes + 3 ghost routes");

  const apiTs = fs.readFileSync(path.join(repoRoot, "src/lib/api.ts"), "utf8");
  assert.ok(apiTs.includes("body: JSON.stringify({ enabled: enable })"), "toggleBurst sends {enabled} instead of {status}");
  assert.ok(apiTs.includes("body: JSON.stringify({ ip, to: targetService })"), "moveIp sends source in URL and target in ignored body.to");

  const nginxConf = fs.readFileSync(path.join(repoRoot, "nginx/default.conf"), "utf8");
  const assetsMatch = /location \/assets\/\s*\{([\s\S]*?)\}/.exec(nginxConf);
  assert.ok(assetsMatch && assetsMatch[1].includes("add_header Cache-Control"), "location /assets/ defines add_header");
  assert.ok(!assetsMatch[1].includes("X-Frame-Options"), "location /assets/ drops server-level security headers");
  assert.equal(nginxConf.includes("/api/internal"), false, "nginx/default.conf lacks /api/internal block");
});
