import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { createRequire } from "node:module";
import { QueryClient } from "@tanstack/react-query";
import { jsx, jsxs, Fragment } from "react/jsx-runtime";
import { renderToStaticMarkup } from "react-dom/server";

const repoRoot = process.cwd();
const require = createRequire(path.join(repoRoot, "package.json"));
const ts = require("typescript");

function createTsLoader(overrides = {}) {
  const cache = new Map();

  function loadModule(relOrAbsPath) {
    const absPath = path.isAbsolute(relOrAbsPath)
      ? relOrAbsPath
      : path.join(repoRoot, relOrAbsPath);
    if (cache.has(absPath)) return cache.get(absPath).exports;

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
    cache.set(absPath, mod);

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
      if (!resolved) {
        throw new Error(`Cannot resolve ${spec} from ${absPath}`);
      }
      return loadModule(resolved);
    }

    const fn = vm.runInThisContext(
      `(function(exports, require, module, __filename, __dirname) { ${transpiled}\n})`,
      { filename: absPath }
    );
    fn(mod.exports, customRequire, mod, absPath, path.dirname(absPath));
    return mod.exports;
  }

  return loadModule;
}

test("BUG-CONTRACT-02: api.moveIp uses destination service in path, toggleBurst sends status, getIpmiAccess maps official enum", async () => {
  const calls = [];
  const load = createTsLoader({
    "./http": {
      apiRequest: async (endpoint, options = {}, opts) => {
        calls.push({ endpoint, options, opts });
        return { success: true };
      },
      API_KEY_STORAGE: "k",
      BACKEND_URL_STORAGE: "b",
      SERVER_CONTROL_ACCOUNT_KEY: "s",
      getApiSecretKey: () => "",
      setApiSecretKey: () => {},
      clearApiSecretKey: () => {},
      getBackendUrl: () => "",
      setBackendUrl: () => {},
      getActiveServerControlAccount: () => "",
      setActiveServerControlAccount: () => {},
      ApiError: class ApiError extends Error {},
    },
  });
  const { api } = load("src/lib/api.ts");

  await api.moveIp("ns-source.ip-1.eu", "198.51.100.42", "ns-target.ip-2.eu");
  assert.equal(calls[0].endpoint, "/api/server-control/ns-target.ip-2.eu/ip/move");
  assert.deepEqual(JSON.parse(calls[0].options.body), { ip: "198.51.100.42" });

  await api.toggleBurst("ns-1.eu", true);
  assert.equal(calls[1].endpoint, "/api/server-control/ns-1.eu/burst");
  assert.deepEqual(JSON.parse(calls[1].options.body), { status: "active" });

  await api.toggleBurst("ns-1.eu", false);
  assert.deepEqual(JSON.parse(calls[2].options.body), { status: "inactive" });

  await api.getIpmiAccess("ns-1.eu", "html5");
  assert.equal(calls[3].endpoint, "/api/server-control/ns-1.eu/console?type=kvmipHtml5URL");

  await api.getIpmiAccess("ns-1.eu", "jnlp");
  assert.equal(calls[4].endpoint, "/api/server-control/ns-1.eu/console?type=kvmipJnlp");

  await api.getIpmiAccess("ns-1.eu", "sol");
  assert.equal(calls[5].endpoint, "/api/server-control/ns-1.eu/console?type=serialOverLanURL");

  await api.getIpmiAccess("ns-1.eu", "serialOverLanSshKey", "ssh-ed25519 AAAAC3Nza");
  assert.equal(
    calls[6].endpoint,
    "/api/server-control/ns-1.eu/console?type=serialOverLanSshKey&sshKey=ssh-ed25519+AAAAC3Nza"
  );
});

test("BUG-FE-CORE-01 & BUG-FE-CORE-04: lossless bidirectional datacenter mapping across MonitorSubscribeDialog, hasStockWithOption, EditQueueDialog & StockRadarSection", () => {
  const load = createTsLoader({
    "@/i18n": { default: { t: (k) => k, exists: () => false } },
  });
  const dcMod = load("src/lib/datacenters.ts");
  const availMod = load("src/hooks/ovh/use-availability.ts");

  const pairs = [
    ["mum", "ynm"],
    ["tor", "ca-east-tor-a"],
    ["par-a", "eu-west-par-a"],
    ["par-b", "eu-west-par-b"],
    ["par-c", "eu-west-par-c"],
    ["gra", "gra"],
  ];

  for (const [disp, apiCode] of pairs) {
    assert.equal(dcMod.apiDatacenterCode(disp), apiCode);
    assert.equal(dcMod.apiDatacenterCode(apiCode), apiCode);
    assert.equal(dcMod.displayDatacenterCode(apiCode), disp);
    assert.equal(dcMod.displayDatacenterCode(disp), disp);
  }

  // hasStockWithOption matches OVH API codes when called with display codes and vice versa
  const variants = [
    {
      fqn: "24sk50.ram-64g-ecc-2400.softraid-2x450nvme",
      planCode: "24sk50",
      datacenters: [
        { datacenter: "ynm", availability: "72H" },
        { datacenter: "ca-east-tor-a", availability: "1H-low" },
        { datacenter: "eu-west-par-b", availability: "24H" },
      ],
    },
  ];
  assert.equal(availMod.hasStockWithOption(variants, {}, "memory", "ram-64g-ecc-2400", ["mum"]), true);
  assert.equal(availMod.hasStockWithOption(variants, {}, "memory", "ram-64g-ecc-2400", ["ynm"]), true);
  assert.equal(availMod.hasStockWithOption(variants, {}, "memory", "ram-64g-ecc-2400", ["tor"]), true);
  assert.equal(availMod.hasStockWithOption(variants, {}, "memory", "ram-64g-ecc-2400", ["par-b"]), true);
  assert.equal(availMod.hasStockWithOption(variants, {}, "memory", "ram-64g-ecc-2400", ["par-a"]), false);

  // MonitorSubscribeDialog & EditQueueDialog enforce bidirectional conversion
  const monitorDlgSrc = fs.readFileSync(path.join(repoRoot, "src/components/monitor/MonitorSubscribeDialog.tsx"), "utf8");
  assert.ok(monitorDlgSrc.includes("selectedDCs.map(apiDatacenterCode)"));
  assert.ok(monitorDlgSrc.includes("displayDatacenterCode(d)"));

  const editQueueSrc = fs.readFileSync(path.join(repoRoot, "src/components/queue/EditQueueDialog.tsx"), "utf8");
  assert.ok(editQueueSrc.includes("displayDatacenterCode(item.datacenter)"));
  assert.ok(editQueueSrc.includes("apiDatacenterCode(datacenter)"));

  // StockRadarSection lights up alias DCs and never truncates the 7th+ in-stock DC
  const radarLoad = createTsLoader({
    "react/jsx-runtime": { jsx, jsxs, Fragment },
    "react-router-dom": { Link: ({ children, to }) => jsx("a", { href: to, children }) },
    "@/components/ui/card": {
      Card: ({ children }) => jsx("div", { children }),
      CardHeader: ({ children }) => jsx("div", { children }),
      CardTitle: ({ children }) => jsx("div", { children }),
      CardContent: ({ children }) => jsx("div", { children }),
    },
    "@/components/ui/button": { Button: ({ children }) => jsx("button", { children }) },
  });
  const { StockRadarSection } = radarLoad("src/components/dashboard/StockRadarSection.tsx");
  const eightInStockSub = {
    planCode: "24sk102",
    datacenters: ["ynm", "ca-east-tor-a", "eu-west-par-a", "gra", "rbx", "sbg", "bhs", "sgp"],
    notifyAvailable: true,
    notifyUnavailable: false,
    lastStatus: {
      "ynm|default": "available",
      "ca-east-tor-a|default": "available",
      "eu-west-par-a|default": "available",
      "gra|default": "available",
      "rbx|default": "available",
      "sbg|default": "available",
      "bhs|default": "available",
      "sgp|default": "available",
    },
    createdAt: "2026-11-16T00:00:00Z",
  };
  const radarHtml = renderToStaticMarkup(
    jsx(StockRadarSection, {
      monitorRunning: true,
      subscriptions: [eightInStockSub],
      isPending: false,
      isFetching: false,
      onRefetch: () => {},
      onOpenSubscribe: () => {},
      onOpenQuickOrder: () => {},
    })
  );
  // All 8 in-stock DCs (including aliased mum/tor/par-a and the 7th/8th bhs/sgp) must render with (available)
  for (const dc of ["mum", "tor", "par-a", "gra", "rbx", "sbg", "bhs", "sgp"]) {
    assert.match(
      radarHtml,
      new RegExp(`${dc}<span[^>]*>\\(available\\)</span>`, "i"),
      `Expected ${dc} to be visible and in-stock`
    );
  }
  assert.doesNotMatch(radarHtml, /\+\d+/, "No in-stock DC should be hidden behind +N truncation");
});

test("BUG-FE-CORE-02: syncActiveAccountsWithList recovers from manual account switch + deletion, and useDeleteAccount invalidates all dependent domains", async () => {
  const invalidated = [];
  const removed = [];
  const fakeQc = {
    invalidateQueries: ({ queryKey }) => invalidated.push(queryKey),
    removeQueries: ({ queryKey }) => removed.push(queryKey),
  };
  const load = createTsLoader({
    "@tanstack/react-query": {
      QueryClient,
      useQueryClient: () => fakeQc,
      useMutation: (opts) => opts,
      useQuery: (opts) => opts,
      useQueries: (opts) => opts,
    },
    "@/lib/http": {
      api: { delete: async () => ({ data: { ok: true } }) },
      setActiveServerControlAccount: () => {},
      setActiveVpsControlAccount: () => {},
    },
    sonner: { toast: { success: () => {}, error: () => {}, warning: () => {} } },
    "@/i18n": { __esModule: true, default: { t: (k) => k, exists: () => false } },
  });
  const accountsMod = load("src/hooks/ovh/use-accounts.ts");

  let serverAcc = "";
  let vpsAcc = "";
  const io = {
    getServer: () => serverAcc,
    setServer: (v) => { serverAcc = v; },
    getVps: () => vpsAcc,
    setVps: (v) => { vpsAcc = v; },
  };

  // Boot with [acc-A (default), acc-B]
  accountsMod.syncActiveAccountsWithList(
    [{ id: "acc-A", isDefault: true }, { id: "acc-B", isDefault: false }],
    io
  );
  assert.equal(serverAcc, "acc-A");
  assert.equal(vpsAcc, "acc-A");

  // User manually switches to acc-B, then deletes acc-B
  serverAcc = "acc-B";
  vpsAcc = "acc-B";
  const recovered = accountsMod.syncActiveAccountsWithList([{ id: "acc-A", isDefault: true }], io);
  assert.equal(recovered, true);
  assert.equal(serverAcc, "acc-A");
  assert.equal(vpsAcc, "acc-A");

  // Deleting all accounts clears residual IDs
  const cleared = accountsMod.syncActiveAccountsWithList([], io);
  assert.equal(cleared, true);
  assert.equal(serverAcc, "");
  assert.equal(vpsAcc, "");

  // useDeleteAccount invalidates monitor, catalog-watch, stats, queue, history, settings
  const delMut = accountsMod.useDeleteAccount();
  delMut.onSuccess({}, "acc-B");
  const flatTopKeys = new Set(invalidated.map((k) => k[0]));
  for (const expectedDomain of ["accounts", "queue", "history", "monitor", "vps-monitor", "catalog-watch", "stats", "settings"]) {
    assert.ok(flatTopKeys.has(expectedDomain), `Expected useDeleteAccount to invalidate ${expectedDomain}`);
  }
});

test("BUG-FE-CORE-03 & BUG-FE-CORE-07: en.ts has 0 missing keys without unsafe casts, and api-error preserves ovh.Explain details", () => {
  const load = createTsLoader();
  const { zh } = load("src/i18n/locales/zh.ts");
  const { en } = load("src/i18n/locales/en.ts");
  const { apiEn } = load("src/i18n/locales/api/en.ts");

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
  const extraInEn = [...enKeys].filter((k) => !zhKeys.has(k));

  assert.equal(missingInEn.length, 0, `Expected 0 missing keys in en.ts, got: ${missingInEn.join(", ")}`);
  assert.equal(extraInEn.length, 0, `Expected 0 extra keys in en.ts, got: ${extraInEn.join(", ")}`);
  assert.ok(apiEn.AUTH_RATE_LIMITED.length > 0);
  assert.ok(apiEn.OVH_UPSTREAM_ERROR.length > 0);
  assert.ok(apiEn.OVH_AUTH_INVALID.length > 0);
  assert.ok(apiEn.ENQUEUE_REJECTED.length > 0);

  for (const rel of [
    "src/i18n/locales/en.ts",
    "src/i18n/locales/api/en.ts",
    "src/i18n/locales/api-en.translated.ts",
  ]) {
    const src = fs.readFileSync(path.join(repoRoot, rel), "utf8");
    assert.equal(src.includes("as unknown as"), false, `${rel} must not contain 'as unknown as'`);
  }

  // Verify apiMessage preserves dynamic ovh.Explain troubleshooting details in both zh and en
  let currentDict = zh;
  const errLoad = createTsLoader({
    "@/i18n": {
      __esModule: true,
      default: {
        exists: (k) => k.startsWith("api.") && k.slice(4) in currentDict.api,
        t: (k) => currentDict.api[k.slice(4)] || k,
      },
    },
  });
  const { apiMessage } = errLoad("src/lib/api-error.ts");

  const upstream403 = {
    response: {
      status: 403,
      data: {
        success: false,
        code: "OVH_UPSTREAM_ERROR",
        error: "OVH API 权限不足 (HTTP 403): 缺少 POST /dedicated/server/*/reboot 权限 [QueryID: eu-req-991]",
      },
    },
  };
  const msg403 = apiMessage(upstream403);
  assert.ok(msg403.includes("HTTP 403"), msg403);
  assert.ok(msg403.includes("QueryID: eu-req-991"), msg403);
  assert.ok(!msg403.includes("请稍后重试"), "Must not mask 403 error with generic retry phrase");

  const avail429 = {
    response: {
      status: 502,
      data: {
        code: "E7480C06F",
        error: "查询可用性失败",
        message: "OVH HTTP 429 Too Many Requests [QueryID: eu-rate-77]",
      },
    },
  };
  const msg429Zh = apiMessage(avail429);
  assert.ok(msg429Zh.includes("查询可用性失败"), msg429Zh);
  assert.ok(msg429Zh.includes("429 Too Many Requests"), msg429Zh);

  currentDict = en;
  const msg429En = apiMessage(avail429);
  assert.ok(msg429En.includes("Failed to query availability"), msg429En);
  assert.ok(msg429En.includes("429 Too Many Requests"), msg429En);
});

test("BUG-FE-CORE-05: mergeDcAvailability prioritizes orderable variants over subsequent unavailable/unknown variants", () => {
  const load = createTsLoader();
  const { mergeDcAvailability, lookupDcStatus, OVH_DATACENTERS } = load("src/lib/datacenters.ts");

  const merged = mergeDcAvailability(
    [
      { datacenter: "gra", availability: "72H" },
      { datacenter: "gra", availability: "unavailable" },
      { datacenter: "rbx", availability: "unavailable" },
      { datacenter: "rbx", availability: "1H-high" },
      { datacenter: "rbx", availability: "240H" },
    ],
    undefined
  );
  assert.equal(merged.gra, "72H");
  assert.equal(merged.rbx, "1H-high");

  const mumDc = OVH_DATACENTERS.find((d) => d.code === "mum");
  const withRealtime = mergeDcAvailability(
    [{ datacenter: "mum", availability: "unavailable" }],
    { ynm: "72H" }
  );
  assert.equal(lookupDcStatus(withRealtime, mumDc), "72H");
});

test("BUG-FE-CORE-06: TelegramOrderPage isolates localStorage errors from HTTP 200 order completion", () => {
  const tgSrc = fs.readFileSync(path.join(repoRoot, "src/pages/TelegramOrderPage.tsx"), "utf8");
  assert.ok(tgSrc.includes("function persistCommandHistory"), "persistCommandHistory helper must exist");
  assert.ok(tgSrc.includes("function readSavedCommandHistory"), "readSavedCommandHistory helper must exist");
  // Ensure api.post("/telegram/quick-order") try block returns early on network error before addToHistory runs
  const postMatches = [...tgSrc.matchAll(/try\s*\{\s*result\s*=\s*\(\s*await api\.post<TelegramActionResult>\("\/telegram\/quick-order"/g)];
  assert.equal(postMatches.length, 2, "Both raw and structured submit handlers must isolate network POST try/catch");
});

test("BUG-FE-CORE-08: usePrewarmOrder normalizes multi-DC via apiDatacenterCode, enforces 90s TTL, and requires ready === true", () => {
  const load = createTsLoader({
    "@/lib/http": { api: { post: async () => ({ data: { success: true, ready: false } }) } },
  });
  const { normalizePrewarmDatacenters, isPrewarmCacheFresh, PREWARM_READY_TTL_MS } = load("src/hooks/use-prewarm.ts");

  assert.equal(PREWARM_READY_TTL_MS, 90_000);
  assert.deepEqual(
    normalizePrewarmDatacenters(["mum", "tor", "par-a", "ynm"]),
    ["ynm", "ca-east-tor-a", "eu-west-par-a"]
  );

  const entry = { key: "acc|24sk50|ynm|||", expiresAt: 1_000 + PREWARM_READY_TTL_MS };
  assert.equal(isPrewarmCacheFresh(entry, "acc|24sk50|ynm|||", 50_000), true);
  assert.equal(isPrewarmCacheFresh(entry, "acc|24sk50|ynm|||", 92_000), false);

  const hookSrc = fs.readFileSync(path.join(repoRoot, "src/hooks/use-prewarm.ts"), "utf8");
  assert.ok(hookSrc.includes("res.data?.ready === true"), "Must check ready === true strictly");
});

test("BUG-FE-CORE-09: useServers.forceRefresh sends forceRefresh=true across co-mounted hook instances without useRef closure loss", async () => {
  const qc = new QueryClient();
  const requests = [];
  const load = createTsLoader({
    "@tanstack/react-query": {
      QueryClient,
      useQueryClient: () => qc,
      useQuery: (opts) => ({ ...opts, isRefetching: false }),
      useMutation: (opts) => opts,
    },
    "@/lib/http": {
      api: {
        get: async (_url, config) => {
          requests.push({ ...config.params });
          return {
            data: {
              servers: [{ planCode: "24sk50", force: Boolean(config.params.forceRefresh) }],
              cacheInfo: { timestamp: 200, usingExpiredCache: false },
            },
          };
        },
      },
    },
    "@/hooks/use-accounts": {
      useAccounts: () => ({ data: [{ id: "acc-1" }], isPending: false }),
      findAccountByID: (list, id) => list?.find((a) => a.id === id),
    },
    "@/hooks/use-active-account": {
      useActiveServerControlAccount: () => ["acc-1"],
    },
    sonner: { toast: { success: () => {}, error: () => {} } },
    "@/i18n": { default: { t: (k) => k, exists: () => false } },
  });

  try {
    const { useServers } = load("src/hooks/ovh/use-servers.ts");
    // Co-mount two instances sharing the same QueryClient & queryKey (e.g. ServersPage + MonitorSubscribeDialog)
    const pageHook = useServers(true);
    const dialogHook = useServers(true);

    await pageHook.queryFn();
    await dialogHook.queryFn();
    assert.equal(Boolean(requests[0].forceRefresh), false);
    assert.equal(Boolean(requests[1].forceRefresh), false);

    // Triggering forceRefresh on the first mounted instance must still pass forceRefresh=true and update cache
    const refreshed = await pageHook.forceRefresh();
    assert.equal(requests[2].forceRefresh, true);
    assert.equal(requests[2].account, "acc-1");
    assert.equal(refreshed[0].force, true);
    assert.deepEqual(qc.getQueryData(pageHook.queryKey), refreshed);
  } finally {
    qc.clear();
  }
});

test("BUG-FE-CORE-10: preorderVariantCoversAll supports versionless and regional Eco plans without stripping multi-disk suffixes", () => {
  const load = createTsLoader();
  const { preorderVariantCoversAll } = load("src/lib/preorder-options.ts");

  assert.equal(
    preorderVariantCoversAll("24sk50.ram-64g-ecc-2400.softraid-2x450nvme", ["softraid-2x450nvme-24sk50"]),
    true
  );
  assert.equal(
    preorderVariantCoversAll(
      "24risegame01-ca.ram-64g-ecc-5200.softraid-2x960nvme",
      ["softraid-2x960nvme-24risegame01-ca"]
    ),
    true
  );
  assert.equal(
    preorderVariantCoversAll(
      "24risegame01-ca.ram-64g-ecc-5200.softraid-2x960nvme",
      ["softraid-2x960nvme-24risegame01"]
    ),
    true
  );
  assert.equal(
    preorderVariantCoversAll("24sk50.softraid-2x480ssd", ["softraid-2x480ssd-12x8000sa"]),
    false
  );
});

test("BUG-FE-CORE-11..15: queue/order/monitor dialogs enforce option deselection, duration guard, selectionDcStatus & exact optionsKey lookup", () => {
  const load = createTsLoader();
  const { normalizeMonitorOptionsKey } = load("src/lib/order-inputs.ts");
  assert.equal(normalizeMonitorOptionsKey([" softraid-2x450nvme ", "ram-64g", "ram-64g"]), "ram-64g,softraid-2x450nvme");
  assert.equal(normalizeMonitorOptionsKey([]), "");
  assert.equal(normalizeMonitorOptionsKey(undefined), "");

  const editQueueSrc = fs.readFileSync(path.join(repoRoot, "src/components/queue/EditQueueDialog.tsx"), "utf8");
  assert.ok(editQueueSrc.includes("mergeStockPlans"), "EditQueueDialog must merge stock-only preorder plans");
  assert.ok(editQueueSrc.includes("preorderOptions"), "EditQueueDialog must filter CPU options for preorders");
  assert.ok(editQueueSrc.includes('id="edit-queue-custom-options"'), "EditQueueDialog must always render extraInput");

  for (const dialogRel of [
    "src/components/orders/QuickOrderDialog.tsx",
    "src/components/servers/ServerDetailDialog.tsx",
    "src/components/queue/CreateQueueDialog.tsx",
    "src/components/queue/EditQueueDialog.tsx",
    "src/components/monitor/MonitorSubscribeDialog.tsx",
  ]) {
    const src = fs.readFileSync(path.join(repoRoot, dialogRel), "utf8");
    assert.ok(src.includes("supportsDuration"), `${dialogRel} must guard submission with supportsDuration`);
  }

  for (const dialogRel of [
    "src/components/orders/QuickOrderDialog.tsx",
    "src/components/queue/CreateQueueDialog.tsx",
  ]) {
    const src = fs.readFileSync(path.join(repoRoot, dialogRel), "utf8");
    assert.ok(src.includes("selectionDcStatus"), `${dialogRel} must compute selectionDcStatus`);
    assert.ok(src.includes("availability={selectionMap}"), `${dialogRel} must pass selectionMap to DatacenterPicker`);
  }

  const quickSrc = fs.readFileSync(path.join(repoRoot, "src/components/orders/QuickOrderDialog.tsx"), "utf8");
  const detailSrc = fs.readFileSync(path.join(repoRoot, "src/components/servers/ServerDetailDialog.tsx"), "utf8");
  assert.ok(quickSrc.includes("p[g] === value ? \"\" : value"), "QuickOrderDialog must allow deselecting active option chip");
  assert.ok(detailSrc.includes("p[g] === value ? \"\" : value"), "ServerDetailDialog must allow deselecting active option chip");
  assert.ok(detailSrc.includes("useOvhCatalog(effectiveSubsidiary)"), "ServerDetailDialog must recalculate catalog price for selected account subsidiary");
  assert.ok(detailSrc.includes("onMonitor(selectedDCs, selectedValues)"), "ServerDetailDialog must forward selectedValues to onMonitor");

  const serversPageSrc = fs.readFileSync(path.join(repoRoot, "src/pages/ServersPage.tsx"), "utf8");
  assert.ok(serversPageSrc.includes("normalizeMonitorOptionsKey(s.options) === targetOptionsKey"));
});

test("BUG-FE-CORE-16: resolveAccountEndpoint preserves kimsufi/soyoustart brand endpoints, AccountDialog avoids double verify, and SettingsPage uses incremental patch", () => {
  const load = createTsLoader({
    "@/i18n": { default: { t: (k, o) => o?.defaultValue ?? k, exists: () => false } },
  });
  const { resolveAccountEndpoint } = load("src/lib/ovh-subsidiaries.ts");

  assert.equal(resolveAccountEndpoint("FR", "kimsufi-eu"), "kimsufi-eu");
  assert.equal(resolveAccountEndpoint("DE", "soyoustart-eu"), "soyoustart-eu");
  assert.equal(resolveAccountEndpoint("QC", "kimsufi-ca"), "kimsufi-ca");
  assert.equal(resolveAccountEndpoint("SG", "soyoustart-ca"), "soyoustart-ca");
  assert.equal(resolveAccountEndpoint("US", "kimsufi-eu"), "ovh-us");
  assert.equal(resolveAccountEndpoint("FR", undefined), "ovh-eu");

  const accDlgSrc = fs.readFileSync(path.join(repoRoot, "src/components/settings/AccountDialog.tsx"), "utf8");
  assert.ok(accDlgSrc.includes("resolveAccountEndpoint"));
  assert.equal(accDlgSrc.includes("useVerifyAccount"), false, "AccountDialog must not issue redundant /verify call");

  const settingsPageSrc = fs.readFileSync(path.join(repoRoot, "src/pages/SettingsPage.tsx"), "utf8");
  assert.ok(settingsPageSrc.includes("resolveAccountEndpoint"));
  assert.equal(settingsPageSrc.includes("dirtyRef"), false, "SettingsPage must use incremental patch instead of freezing stale form");
});

test("BUG-FE-CORE-17: AccountPage clears stale email on switch, shows priceWithTax on OrdersTab, guards RefundsTab optional chain, and OrderProgressButton falls back to defaultAcc", () => {
  const accountPageSrc = fs.readFileSync(path.join(repoRoot, "src/pages/AccountPage.tsx"), "utf8");
  assert.ok(accountPageSrc.includes("setSelected(null)"), "EmailsTab must reset selected email when accountId changes");
  assert.ok(accountPageSrc.includes("const hasWithTax = o.priceWithTax?.value != null || Boolean(o.priceWithTax?.text);"));
  assert.ok(accountPageSrc.includes("r.priceWithTax?.text"), "RefundsTab must use optional chaining on priceWithTax");
  assert.equal(accountPageSrc.includes("r.priceWithTax.text"), false);

  const progressBtnSrc = fs.readFileSync(path.join(repoRoot, "src/components/history/OrderProgressButton.tsx"), "utf8");
  assert.ok(progressBtnSrc.includes("const resolvedAccountId = accountId || defaultAcc?.id || \"\";"));
});

test("BUG-FE-CORE-18: useTelegramPoller separates lastPollAt from lastUpdateAt", async () => {
  let mockPayload = {
    running: true,
    configured: true,
    botUsername: "ovh_bot",
    offset: 101,
    lastPollAt: "2026-11-16T10:30:00Z",
    lastUpdateAt: "",
  };

  const load = createTsLoader({
    "@tanstack/react-query": {
      QueryClient,
      useQuery: (opts) => opts,
      useMutation: (opts) => opts,
      useQueryClient: () => ({ invalidateQueries: () => {} }),
    },
    "@/lib/http": {
      api: { get: async () => ({ data: mockPayload }) },
    },
    sonner: { toast: { success: () => {}, error: () => {} } },
    "@/i18n": { default: { t: (k) => k, exists: () => false } },
  });

  const { useTelegramPoller } = load("src/hooks/ovh/use-settings.ts");
  const query = useTelegramPoller();
  const res1 = await query.queryFn();
  assert.equal(res1.lastPollAt, "2026-11-16T10:30:00Z");
  assert.equal(res1.lastUpdateAt, undefined);
  assert.equal(res1.poller.lastPollAt, "2026-11-16T10:30:00Z");
  assert.equal(res1.poller.lastUpdateAt, undefined);

  mockPayload = {
    running: true,
    configured: true,
    botUsername: "ovh_bot",
    offset: 102,
    lastPollAt: "2026-11-16T10:35:00Z",
    lastUpdateAt: "2026-11-16T10:31:00Z",
  };
  const res2 = await query.queryFn();
  assert.equal(res2.lastPollAt, "2026-11-16T10:35:00Z");
  assert.equal(res2.lastUpdateAt, "2026-11-16T10:31:00Z");
});

test("BUG-FE-CORE-19: CoreServiceCard sanitizes update.url through safeHttpsUrl", () => {
  const load = createTsLoader({
    "react/jsx-runtime": { jsx, jsxs, Fragment },
    "@/components/ui/card": {
      Card: ({ children }) => jsx("div", { children }),
      CardContent: ({ children }) => jsx("div", { children }),
    },
  });
  const { CoreServiceCard } = load("src/components/dashboard/CoreServiceCard.tsx");

  const unsafeHtml = renderToStaticMarkup(
    jsx(CoreServiceCard, {
      version: "1.0.0",
      update: { hasUpdate: true, latest: "1.1.0", url: "javascript:alert(document.cookie)" },
    })
  );
  assert.doesNotMatch(unsafeHtml, /javascript:/i);
  assert.doesNotMatch(unsafeHtml, /<a\b/i);
  assert.match(unsafeHtml, /v1\.1\.0/);

  const safeHtml = renderToStaticMarkup(
    jsx(CoreServiceCard, {
      version: "1.0.0",
      update: { hasUpdate: true, latest: "1.1.0", url: "https://github.com/example/ovh-webui/releases/tag/v1.1.0" },
    })
  );
  assert.match(safeHtml, /<a[^>]+href="https:\/\/github\.com\/example\/ovh-webui\/releases\/tag\/v1\.1\.0"/);
});

test("BUG-FE-CORE-20: isValidHealthPayload rejects SPA HTML fallbacks, and index.html initializes theme without dark flash", () => {
  const load = createTsLoader({
    "@/lib/http": { apiRequest: async () => ({ status: "ok" }) },
  });
  const { isValidHealthPayload } = load("src/hooks/use-backend-connection.ts");

  assert.equal(isValidHealthPayload("<!doctype html><html><body>SPA</body></html>", "text/html"), false);
  assert.equal(isValidHealthPayload("<!doctype html><html><body>SPA</body></html>"), false);
  assert.equal(isValidHealthPayload({ status: "ok" }, "text/html; charset=utf-8"), false);
  assert.equal(isValidHealthPayload({}, "application/json"), false);
  assert.equal(isValidHealthPayload([], "application/json"), false);
  assert.equal(isValidHealthPayload({ status: "ok", time: "2026-11-16T00:00:00Z" }, "application/json; charset=utf-8"), true);

  const indexHtml = fs.readFileSync(path.join(repoRoot, "index.html"), "utf8");
  assert.doesNotMatch(indexHtml, /<html[^>]*class=["']dark["']/i, "index.html must not hardcode class='dark'");
  const scriptMatch = /<script>([\s\S]*?)<\/script>/.exec(indexHtml);
  assert.ok(scriptMatch, "index.html must contain synchronous inline theme script in <head>");

  function runThemeInit(storedTheme, systemDark) {
    const classes = new Set();
    let metaColor = "#080c14";
    const storage = new Map(storedTheme ? [["ovh-theme", storedTheme]] : []);
    const sandbox = {
      localStorage: {
        getItem: (k) => storage.get(k) ?? null,
        setItem: (k, v) => storage.set(k, v),
      },
      window: {
        matchMedia: () => ({ matches: systemDark }),
      },
      document: {
        documentElement: {
          classList: {
            toggle: (cls, force) => {
              if (force) classes.add(cls);
              else classes.delete(cls);
            },
          },
          style: {},
        },
        querySelector: () => ({
          setAttribute: (_k, v) => {
            metaColor = v;
          },
        }),
      },
    };
    vm.runInNewContext(scriptMatch[1], sandbox);
    return { isDark: classes.has("dark"), colorScheme: sandbox.document.documentElement.style.colorScheme, metaColor };
  }

  assert.deepEqual(runThemeInit("light", true), { isDark: false, colorScheme: "light", metaColor: "#f8fafc" });
  assert.deepEqual(runThemeInit("dark", false), { isDark: true, colorScheme: "dark", metaColor: "#080c14" });
  assert.deepEqual(runThemeInit(null, false), { isDark: false, colorScheme: "light", metaColor: "#f8fafc" });
  assert.deepEqual(runThemeInit(null, true), { isDark: true, colorScheme: "dark", metaColor: "#080c14" });
});
