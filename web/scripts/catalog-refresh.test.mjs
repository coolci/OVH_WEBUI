import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

// Enable the real QueryObserver timer path without a browser or live OVH calls.
globalThis.window = {};
const { QueryClient, QueryObserver, focusManager } = await import('@tanstack/react-query');
function load(file, imports = {}, clock = Date) {
  const exports = {};
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  vm.runInNewContext(js, { exports, Date: clock, require: id => {
    if (!(id in imports)) throw Error(`Unexpected import ${id}`);
    return imports[id];
  } }, { filename: file });
  return exports;
}
const file = 'src/lib/catalog-refresh.ts';

test('expiry uses backend snapshot age, retries back off and reset on recovery', async () => {
  let now = 1_000_000;
  const { catalogRefresh } = load(file, {}, { now: () => now });
  const client = new QueryClient();
  let failed = false;
  let networkError = false;
  const old = now - 90_000;
  const options = catalogRefresh(client, ['servers', 'a'], async () => {
    if (networkError) throw Error('offline');
    return { data: ['a'], updatedAt: failed ? old : now - 90_000, failed };
  });
  try {
    await options.queryFn();
    assert.equal(options.refetchInterval(), 30_000, 'only 30s left on a 90s-old snapshot');
    now += 30_000;
    assert.equal(options.refetchInterval(), 1000);
    failed = true;
    for (const delay of [5000, 10000, 20000, 40000, 60000, 60000]) {
      await options.queryFn();
      assert.equal(options.refetchInterval(), delay, 'HTTP 200 stale response retries');
      assert.equal(client.getQueryData(options.statusKey).updatedAt, old);
    }
    networkError = true;
    await assert.rejects(options.queryFn(), /offline/);
    assert.equal(options.refetchInterval(), 60_000);
    assert.equal(client.getQueryData(options.statusKey).updatedAt, old);
    failed = false; networkError = false;
    await options.queryFn();
    assert.equal(client.getQueryData(options.statusKey).failures, 0);
    assert.equal(options.refetchInterval(), 30_000);
  } finally { client.clear(); }
});

test('account changes isolate refresh metadata and list requests; manual refresh is account scoped', async () => {
  const client = new QueryClient();
  const helper = load(file);
  let active = 'a';
  const requests = [];
  const hooks = load('src/hooks/ovh/use-servers.ts', {
    react: { useRef: value => ({current: value}) },
    '@/lib/catalog-refresh': helper,
    '@tanstack/react-query': { useQueryClient: () => client, useQuery: o => ({...o, refetch: o.queryFn}), useMutation: o => o },
    '@/lib/http': { api: { get: async (url, config) => {
      requests.push(config.params);
      return {data: {servers: [{planCode: config.params.account}], cacheInfo: {timestamp: 100, usingExpiredCache: config.params.account === 'a'}}};
    } } },
    '@/lib/query': { qk: {servers: {list: (show, account) => ['servers', show, account]}} },
    '@/hooks/use-accounts': { useAccounts: () => ({data: [{id:'a'}, {id:'b'}], isPending:false}), findAccountByID: (accounts, id) => accounts.find(a => a.id === id) },
    '@/hooks/use-active-account': { useActiveServerControlAccount: () => [active] },
    sonner: {}, '@/i18n': {}, '@/lib/api-error': {},
  });
  try {
    const a = hooks.useServers();
    await a.queryFn();
    assert.equal(client.getQueryData(a.statusKey).failures, 1);
    active = 'b';
    const b = hooks.useServers();
    await b.queryFn();
    assert.equal(client.getQueryData(b.statusKey).failures, 0);
    assert.equal(client.getQueryData(a.statusKey).failures, 1);
    assert.equal(requests[0].account, 'a');
    assert.equal(requests[1].account, 'b');
    await b.forceRefresh();
    assert.equal(requests[2].account, 'b');
    assert.equal(requests[2].forceRefresh, true);
    await a.queryFn();
    assert.equal(requests[3].forceRefresh, true, 'retry bypasses still-valid fallback cache');
  } finally { client.clear(); }
});

const wait = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(check) {
  for (let i = 0; i < 100; i++) { if (check()) return; await wait(10); }
  assert.fail('automatic refresh did not run');
}
test('real QueryObserver refreshes, retries network and stale failures, pauses hidden and cleans up', async () => {
  const client = new QueryClient();
  const { catalogRefresh } = load(file);
  let calls = 0;
  const options = catalogRefresh(client, ['live'], async () => {
    calls++;
    if (calls === 2) throw Error('temporary network failure');
    return {data: [calls], updatedAt: Date.now(), failed: calls === 3};
  });
  focusManager.setFocused(true);
  const observer = new QueryObserver(client, {
    queryKey: ['live'], ...options,
    // Speed up timers only; production expiry/backoff calculations are tested above.
    refetchInterval: () => options.refetchInterval() / 1000,
  });
  const unsubscribe = observer.subscribe(() => {});
  try {
    await until(() => calls >= 4 && !client.getQueryData(options.statusKey)?.failures);
    assert.equal(observer.getCurrentResult().data[0], 4);
    focusManager.setFocused(false);
    const hiddenCalls = calls;
    await wait(180);
    assert.equal(calls, hiddenCalls, 'hidden page must not poll');
    focusManager.setFocused(true);
    await until(() => calls > hiddenCalls);
    unsubscribe();
    const stoppedCalls = calls;
    await wait(180);
    assert.equal(calls, stoppedCalls, 'unmount clears the timer');
  } finally { unsubscribe(); client.clear(); focusManager.setFocused(undefined); }
});

test('price catalog retries stale HTTP 200 and respects upstream cache age', async () => {
  const client = new QueryClient();
  const helper = load(file);
  let stale = true;
  const hooks = load('src/hooks/ovh/use-availability.ts', {
    '@/lib/catalog-refresh': helper,
    '@tanstack/react-query': {useQueryClient: () => client, useQuery: x => x},
    '@/lib/availability': {}, '@/lib/variant-stock': {}, axios: {}, '@/lib/query': {},
    '@/components/common/active-endpoint': {}, '@/lib/ovh-regions': {}, '@/lib/money': {}, '@/i18n': {},
    '@/lib/http': {api: {get: async (url, {params}) => {
      assert.equal(params.subsidiary, 'US');
      return {data: {plans: []}, headers: {'x-cache-age-seconds': '90', ...(stale ? {'x-cache-warning': 'stale'} : {})}};
    }}},
  });
  try {
    const options = hooks.useOvhCatalog('US');
    await options.queryFn();
    assert.equal(client.getQueryData(options.statusKey).failures, 1);
    assert.ok(options.refetchInterval() <= 5000);
    stale = false;
    await options.queryFn();
    assert.equal(client.getQueryData(options.statusKey).failures, 0);
    assert.ok(options.refetchInterval() > 29000 && options.refetchInterval() <= 30000);
  } finally {client.clear();}
});

test('badge distinguishes last successful update, retry and refreshing states', async () => {
  const jsx = await import('react/jsx-runtime');
  const {renderToStaticMarkup} = await import('react-dom/server');
  const now = Date.now();
  const {CacheBadge} = load('src/components/servers/CacheBadge.tsx', {
    react: {useEffect: () => {}, useState: () => [now]},
    'react/jsx-runtime': jsx,
  });
  const status = {updatedAt: now - 3 * 3600_000, expiresAt: now - 1, failures: 1};
  let html = renderToStaticMarkup(CacheBadge({status, fetching:false, failed:false}));
  assert.match(html, /刷新失败，自动重试中/);
  assert.match(html, /最后成功更新：3 小时/);
  assert.doesNotMatch(html, /点刷新|手动/);
  html = renderToStaticMarkup(CacheBadge({status, fetching:true, failed:false}));
  assert.match(html, /正在刷新/);
  html = renderToStaticMarkup(CacheBadge({fetching:false, failed:true}));
  assert.match(html, /尚无成功更新记录/);
});
