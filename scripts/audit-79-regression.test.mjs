import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { createRequire } from "node:module";

const repoRoot = process.cwd();
const require = createRequire(path.join(repoRoot, "package.json"));
const ts = require("typescript");

function createTsLoader(overrides = {}) {
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
      if (spec in overrides) return overrides[spec];
      let target = spec;
      if (spec.startsWith("@/")) {
        target = path.join(repoRoot, "src", spec.slice(2));
      } else if (spec.startsWith(".")) {
        target = path.resolve(path.dirname(absPath), spec);
      } else {
        return require(spec);
      }
      const candidates = [
        target,
        `${target}.ts`,
        `${target}.tsx`,
        path.join(target, "index.ts"),
        path.join(target, "index.tsx"),
      ];
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

  return loadTsModule;
}

/** Minimal @tanstack/react-query stub; `@/lib/query` instantiates a real QueryClient at import time. */
function reactQueryStub(qc = { invalidateQueries() {}, removeQueries() {} }) {
  return {
    QueryClient: class QueryClient {},
    useQueryClient: () => qc,
    useMutation: (x) => x,
    useQuery: (x) => x,
    useQueries: (x) => x,
  };
}

/** Parse `METHOD /api/path` keys from api/openapi.yaml (server base url is /api). */
function openapiRouteKeys() {
  const lines = fs.readFileSync(path.join(repoRoot, "api/openapi.yaml"), "utf8").split(/\r?\n/);
  const keys = [];
  let inPaths = false;
  let current = null;
  for (const line of lines) {
    if (/^paths:\s*$/.test(line)) {
      inPaths = true;
      continue;
    }
    if (inPaths && /^[A-Za-z0-9_-]+:/.test(line)) break;
    if (!inPaths) continue;
    const p = line.match(/^  (\/[^:]+):\s*$/);
    if (p) {
      current = p[1].trim();
      continue;
    }
    const m = line.match(/^    (get|post|put|delete|patch):\s*$/);
    if (current && m) {
      const full = current === "/health" ? current : `/api${current}`;
      keys.push(`${m[1].toUpperCase()} ${full}`);
    }
  }
  return keys;
}

/** A frontend `{*}` segment (dynamic template expression) matches any single contract segment. */
function feKeyResolves(feKey, contractKeys) {
  const [feMethod, fePath] = feKey.split(" ");
  const feSegs = fePath.split("/");
  return contractKeys.some((ck) => {
    const [m, p] = ck.split(" ");
    if (m !== feMethod) return false;
    const segs = p.split("/");
    if (segs.length !== feSegs.length) return false;
    return feSegs.every((s, i) => s === "{*}" || /^\{[^}]+\}$/.test(segs[i]) || s === segs[i]);
  });
}

test("AUDIT-79 [FE-CORE-01 & FE-CORE-04]: Datacenter alias bidirectional mapping works across MonitorSubscribeDialog, hasStockWithOption & EditQueueDialog", () => {
  const load = createTsLoader({
    "@/i18n": { default: { t: (k) => k, exists: () => false } },
  });
  const dcMod = load("src/lib/datacenters.ts");
  const availMod = load("src/hooks/ovh/use-availability.ts");

  assert.equal(dcMod.apiDatacenterCode("mum"), "ynm");
  assert.equal(dcMod.apiDatacenterCode("tor"), "ca-east-tor-a");
  assert.equal(dcMod.apiDatacenterCode("par-a"), "eu-west-par-a");
  assert.equal(dcMod.displayDatacenterCode("ynm"), "mum");
  assert.equal(dcMod.displayDatacenterCode("ca-east-tor-a"), "tor");
  assert.equal(dcMod.displayDatacenterCode("eu-west-par-a"), "par-a");

  const monitorDlgSrc = fs.readFileSync(
    path.join(repoRoot, "src/components/monitor/MonitorSubscribeDialog.tsx"),
    "utf8"
  );
  assert.equal(monitorDlgSrc.includes("apiDatacenterCode"), true, "MonitorSubscribeDialog must call apiDatacenterCode");
  assert.equal(monitorDlgSrc.includes("displayDatacenterCode"), true, "MonitorSubscribeDialog must call displayDatacenterCode");

  const editQueueSrc = fs.readFileSync(
    path.join(repoRoot, "src/components/queue/EditQueueDialog.tsx"),
    "utf8"
  );
  assert.equal(editQueueSrc.includes("displayDatacenterCode"), true, "EditQueueDialog must map API code to display code");
  assert.equal(editQueueSrc.includes("apiDatacenterCode"), true, "EditQueueDialog must map display code to API code");

  const variants = [
    {
      fqn: "24sk50.ram-64g-ecc-2400.softraid-2x450nvme",
      planCode: "24sk50",
      datacenters: [{ datacenter: "ynm", availability: "72H" }],
    },
  ];
  assert.equal(
    availMod.hasStockWithOption(variants, {}, "memory", "ram-64g-ecc-2400", ["mum"]),
    true,
    "hasStockWithOption must match ['mum'] when OVH API reports 'ynm' in stock"
  );
});

test("AUDIT-79 [FE-CORE-02]: ActiveAccountSync eliminates lastFixedServer lock and recovers active account after manual switch + delete", () => {
  const syncSrc = fs.readFileSync(
    path.join(repoRoot, "src/components/common/ActiveAccountSync.tsx"),
    "utf8"
  );
  // The doc comment may still name the removed guards; assert no live ref usage remains.
  assert.doesNotMatch(syncSrc, /lastFixed(Server|Vps)\s*=\s*useRef|lastFixed(Server|Vps)\.current/, "lastFixed* ref guards must be removed");
  assert.doesNotMatch(syncSrc, /\buseRef\b/, "ActiveAccountSync must not keep a ref-based fix-once guard");

  const load = createTsLoader({
    "@tanstack/react-query": reactQueryStub(),
    "@/lib/http": {
      api: {},
      setActiveServerControlAccount: () => {},
      setActiveVpsControlAccount: () => {},
    },
    sonner: { toast: { success() {}, error() {}, warning() {} } },
    "@/i18n": { __esModule: true, default: { t: (k) => k, exists: () => false } },
  });
  const { syncActiveAccountsWithList } = load("src/hooks/ovh/use-accounts.ts");

  let activeServer = "";
  let activeVps = "";
  const io = {
    getServer: () => activeServer,
    setServer: (v) => {
      activeServer = v;
    },
    getVps: () => activeVps,
    setVps: (v) => {
      activeVps = v;
    },
  };

  // Step 1: Initial boot with [acc-A (default), acc-B] -> fixes "" to "acc-A"
  syncActiveAccountsWithList(
    [
      { id: "acc-A", isDefault: true },
      { id: "acc-B", isDefault: false },
    ],
    io
  );
  assert.equal(activeServer, "acc-A");

  // Step 2: User manually switches active account to "acc-B"
  activeServer = "acc-B";
  activeVps = "acc-B";

  // Step 3: User deletes "acc-B" -> accounts becomes [{ id: "acc-A", isDefault: true }]
  syncActiveAccountsWithList([{ id: "acc-A", isDefault: true }], io);
  assert.equal(activeServer, "acc-A", "Active server account must recover to default acc-A");
  assert.equal(activeVps, "acc-A", "Active VPS account must recover to default acc-A");

  // Step 4: Deleting all accounts clears stale IDs
  syncActiveAccountsWithList([], io);
  assert.equal(activeServer, "");
  assert.equal(activeVps, "");
});

test("AUDIT-79 [FE-CORE-05]: mergeDcAvailability preserves orderable '72H' variant over subsequent 'unavailable' variant", () => {
  const load = createTsLoader();
  const { mergeDcAvailability } = load("src/lib/datacenters.ts");
  const merged = mergeDcAvailability(
    [
      { datacenter: "gra", availability: "72H" },
      { datacenter: "gra", availability: "unavailable" },
    ],
    undefined
  );
  assert.equal(merged.gra, "72H", "Orderable 72H stock must not be overwritten by later unavailable variant");
});

test("AUDIT-79 [FE-CORE-07 & FE-CORE-03]: en.ts has 0 missing api.* error keys without unsafe casts, and apiMessage preserves ovh.Explain details", () => {
  const load = createTsLoader();
  const { zh } = load("src/i18n/locales/zh.ts");
  const enMod = load("src/i18n/locales/en.ts");
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

  assert.equal(missingInEn.length, 0, `Expected 0 missing keys in en.ts, got: ${missingInEn.join(", ")}`);
  assert.ok(typeof en.api.AUTH_RATE_LIMITED === "string" && en.api.AUTH_RATE_LIMITED.length > 0);
  assert.ok(typeof en.api.OVH_UPSTREAM_ERROR === "string" && en.api.OVH_UPSTREAM_ERROR.length > 0);
  assert.ok(typeof en.api.OVH_AUTH_INVALID === "string" && en.api.OVH_AUTH_INVALID.length > 0);
  assert.ok(typeof en.api.ENQUEUE_REJECTED === "string" && en.api.ENQUEUE_REJECTED.length > 0);

  const enSrc = fs.readFileSync(path.join(repoRoot, "src/i18n/locales/en.ts"), "utf8");
  assert.equal(enSrc.includes("as unknown as"), false, "en.ts must not use 'as unknown as'");

  const errLoad = createTsLoader({
    "@/i18n": {
      __esModule: true,
      default: {
        exists: (k) => k.startsWith("api.") && k.slice(4) in zh.api,
        t: (k) => zh.api[k.slice(4)] || k,
      },
    },
  });
  const { apiMessage } = errLoad("src/lib/api-error.ts");
  const detailedMsg = apiMessage({
    response: {
      status: 403,
      data: {
        code: "OVH_UPSTREAM_ERROR",
        error: "OVH HTTP 403 Forbidden: missing dedicatedServer:apiovh:reboot [QueryID: eu-123]",
      },
    },
  });
  assert.ok(detailedMsg.includes("QueryID: eu-123"), "apiMessage must retain upstream ovh.Explain details");
});

test("AUDIT-79 [FE-CORE-10]: preorderVariantCoversAll matches versionless Eco plans like 24sk50 and 24risegame01-ca", () => {
  const load = createTsLoader();
  const { preorderVariantCoversAll } = load("src/lib/preorder-options.ts");
  assert.equal(
    preorderVariantCoversAll("24sk50.ram-64g-ecc-2400.softraid-2x450nvme", ["softraid-2x450nvme-24sk50"]),
    true
  );
  assert.equal(
    preorderVariantCoversAll("24risegame01-ca.ram-64g-ecc-5200.softraid-2x960nvme", ["softraid-2x960nvme-24risegame01-ca"]),
    true
  );
});

test("AUDIT-79 [FE-CTRL-01, FE-CTRL-02 & FE-CTRL-04]: Reinstall merges single disk group + RAID, deducts hot spares, and converts MB->GB accurately", () => {
  const load = createTsLoader({
    "@tanstack/react-query": reactQueryStub(),
    "@/lib/http": { api: {} },
    "@/hooks/use-active-account": {
      useActiveServerControlAccount: () => ["acc-1"],
      useActiveVpsControlAccount: () => ["acc-1"],
    },
  });
  const { buildInstallStorageConfig, diskCapacityToGB, pickDefaultDiskGroup } = load(
    "src/hooks/ovh/server-control/use-server-install.ts"
  );

  // Single disk group server (diskGroupId=1, 4 physical disks) + custom partition + Hardware RAID1 + 1 hot spare
  const storageArray = buildInstallStorageConfig({
    serviceName: "ns1.ip-1-2-3.eu",
    templateName: "debian12_64",
    customPartitions: [
      { mountpoint: "/", filesystem: "ext4", size: 50000, order: 1, type: "primary", diskGroupId: undefined },
    ],
    hardwareRaid: { 1: "raid1" },
    hwRaidSpares: { 1: 1 },
    diskGroups: {
      1: { id: 1, disks: [{ capacity: 480000, unit: "MB" }, { capacity: 480000, unit: "MB" }, { capacity: 480000, unit: "MB" }, { capacity: 480000, unit: "MB" }], isDefault: true },
    },
  });

  assert.equal(storageArray.length, 1, "Single disk group must merge partitioning and hardwareRaid into 1 entry");
  assert.equal(storageArray[0].diskGroupId, 1);
  assert.equal(storageArray[0].partitioning.layout.length, 1);
  assert.equal(storageArray[0].hardwareRaid[0].disks, 3);
  assert.equal(storageArray[0].hardwareRaid[0].spares, 1);
  assert.equal(
    storageArray[0].hardwareRaid[0].disks + storageArray[0].hardwareRaid[0].spares,
    4,
    "disks(3) + spares(1) must equal 4 <= 4 physical disks"
  );

  // MB -> GB unit conversion (2 x 480000 MB = 937.5 GB, NOT 960,000 GB)
  const totalGb = diskCapacityToGB(480000, "MB") * 2;
  assert.equal(totalGb, 937.5);

  // Default disk group selection honors isDefault over Object.values(groups)[0]
  const picked = pickDefaultDiskGroup({
    1: { id: 1, isDefault: false, disks: [] },
    2: { id: 2, isDefault: true, disks: [] },
  });
  assert.equal(picked.id, 2);
});

test("AUDIT-79 [FE-CTRL-06 & FE-CORE-16]: VPS SUBSIDIARIES covers all 24 zones and resolveAccountEndpoint preserves kimsufi/soyoustart", () => {
  const load = createTsLoader({
    "@/i18n": { default: { t: (k, o) => o?.defaultValue ?? k, exists: () => false } },
  });
  const { OVH_SUBSIDIARIES, resolveAccountEndpoint } = load("src/lib/ovh-subsidiaries.ts");
  const { SUBSIDIARIES: vpsSubs } = load("src/components/vps-monitor/types.ts");

  assert.equal(OVH_SUBSIDIARIES.length, 24);
  assert.equal(vpsSubs.length, 24, "VPS SUBSIDIARIES must include all 24 OVH subsidiaries");
  assert.equal(resolveAccountEndpoint("FR", "kimsufi-eu"), "kimsufi-eu");
  assert.equal(resolveAccountEndpoint("FR", "soyoustart-eu"), "soyoustart-eu");
  assert.equal(resolveAccountEndpoint("CA", "kimsufi-ca"), "kimsufi-ca");
  assert.equal(resolveAccountEndpoint("CA", "soyoustart-ca"), "soyoustart-ca");
  assert.equal(resolveAccountEndpoint("FR", undefined), "ovh-eu");
});

test("AUDIT-79 [CONTRACT-01..05, SEC-12 & MANIFEST]: OpenAPI 0 mismatch, api.ts aligned, Nginx /assets/ & /api/internal hardened, and 79/79 bugs FIXED_VERIFIED", () => {
  const diff = JSON.parse(
    fs.readFileSync(path.join(repoRoot, "audit_ledger/pocs/security_and_frontend/contract_diff.json"), "utf8")
  );
  assert.equal(diff.backendMissingInOpenapi.length, 0, "backendMissingInOpenapi must be 0");
  assert.equal(diff.openapiMissingInBackend.length, 0, "openapiMissingInBackend must be 0");
  assert.equal(diff.openapiRouteCount, diff.backendRouteCount, "OpenAPI and backend route counts must match");

  // The static extractor cannot expand fully-dynamic template segments (e.g. `${prefix}/${svc}/retraction`,
  // `/vps-monitor/${running ? "stop" : "start"}`), so it reports them as `{*}`. Every such leftover must
  // still resolve to a documented (and therefore implemented, given 0/0 parity above) contract route.
  const contractKeys = openapiRouteKeys();
  assert.equal(contractKeys.length, diff.openapiRouteCount);
  const unresolved = diff.feMissingInBackend.filter((c) => !feKeyResolves(c.normKey, contractKeys));
  assert.deepEqual(
    unresolved.map((c) => `${c.file}:${c.line} ${c.normKey}`),
    [],
    "Every frontend API call must resolve to a backend/OpenAPI route"
  );
  for (const c of diff.feMissingInBackend) {
    assert.ok(c.normKey.includes("{*}"), `Only dynamic-template calls may be unmatched statically: ${c.normKey}`);
  }

  const apiTs = fs.readFileSync(path.join(repoRoot, "src/lib/api.ts"), "utf8");
  // toggleBurst must send {status} (backend UpdateBurst); toggleFirewall legitimately keeps {enabled} (backend UpdateFirewall).
  const burstBlock = /toggleBurst:[\s\S]*?\}\),/.exec(apiTs)?.[0] || "";
  assert.ok(burstBlock.includes('status: enable ? "active" : "inactive"'), "toggleBurst must send {status}");
  assert.equal(burstBlock.includes("enabled: enable"), false, "toggleBurst must not send {enabled}");
  const moveIpBlock = /moveIp:[\s\S]*?\}\),/.exec(apiTs)?.[0] || "";
  assert.ok(moveIpBlock.includes("/api/server-control/${encodeURIComponent(targetService)}/ip/move"), "moveIp must target the destination service");
  assert.equal(moveIpBlock.includes("to: targetService"), false, "moveIp must not rely on the ignored body.to");
  assert.ok(apiTs.includes("serialOverLanSshKey"));

  const nginxConf = fs.readFileSync(path.join(repoRoot, "nginx/default.conf"), "utf8");
  const assetsMatch = /location\s+\/assets\/\s*\{([\s\S]*?)\}/.exec(nginxConf);
  assert.ok(assetsMatch && assetsMatch[1].includes("Cache-Control"), "location /assets/ must retain Cache-Control");
  // A child `add_header` drops every inherited server-level header, so each one must be re-declared here.
  for (const header of ["X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "X-Robots-Tag"]) {
    assert.ok(assetsMatch[1].includes(header), `location /assets/ must re-declare ${header}`);
  }
  assert.equal(nginxConf.includes("/api/internal"), true, "nginx/default.conf must block /api/internal");
  assert.match(nginxConf, /location\s+\^~\s+\/api\/internal[\s\S]*?return\s+403/);

  const viteCfg = fs.readFileSync(path.join(repoRoot, "vite.config.ts"), "utf8");
  assert.equal(viteCfg.includes('host: "0.0.0.0"'), false, "vite.config.ts must default to 127.0.0.1 instead of 0.0.0.0");

  const manifest = JSON.parse(
    fs.readFileSync(path.join(repoRoot, "audit_ledger/verified_bugs_manifest.json"), "utf8")
  );
  assert.equal(manifest.sourceCodeModified, true);
  assert.equal(manifest.totalVerifiedBugs, 79);
  assert.equal(manifest.bugs.length, 79);
  const unfixed = manifest.bugs.filter((b) => b.status !== "FIXED_VERIFIED");
  assert.equal(unfixed.length, 0, `All 79 bugs must be FIXED_VERIFIED, remaining: ${unfixed.map((b) => b.id).join(", ")}`);
});
