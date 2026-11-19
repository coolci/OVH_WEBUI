import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

function load(file, imports = {}) {
  const exports = {};
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  vm.runInNewContext(js, { exports, URL, require: id => {
    if (!(id in imports)) throw Error(`Unexpected import ${id}`);
    return imports[id];
  } }, { filename: file });
  return exports;
}

const availability = load('src/lib/availability.ts');
const { isDcInStock, monitorDcAvailability } = load('src/lib/datacenters.ts', { '@/lib/availability': availability });

// comingSoon 还没开卖:以前黑名单把它判成有货,前端亮绿点,后端判不可订,任务空转
test('isDcInStock uses the orderable whitelist', () => {
  for (const s of ['1H-low', '24H', '72H', '1H-high', 'available']) assert.equal(isDcInStock(s), true, s);
  for (const s of ['comingSoon', 'unavailable', 'unknown', 'price_check_failed', '', undefined]) {
    assert.equal(isDcInStock(s), false, String(s));
  }
});

test('monitor chips do not let a sold-out config hide one with stock', () => {
  const m = monitorDcAvailability({ 'gra|a': 'unavailable', 'gra|b': 'available', 'ynm|a': 'comingSoon' });
  assert.equal(m.gra, 'available');
  assert.equal(isDcInStock(m.mum), false);
});

const { maskSensitive } = load('src/hooks/use-hide-ip.ts', { react: { useCallback: f => f, useEffect: () => {}, useState: v => [v, () => {}] } });

// 文本里同时夹带反向 DNS 和真实 IP 时,以前命中第一种就返回,括号里的 IP 原样露出
test('maskSensitive masks every embedded form', () => {
  const out = maskSensitive('ns392029.ip-37-187-28.eu (37.187.28.10)', true);
  assert.ok(!out.includes('37.187.28.10'), out);
  assert.ok(!out.includes('392029'), out);
  assert.equal(maskSensitive('1.2.3.4', true), '1.***.***.***');
  assert.equal(maskSensitive('1.2.3.4', false), '1.2.3.4');
  const v6 = maskSensitive('gateway 2001:41d0:1:2345::1 ok', true);
  assert.ok(!v6.includes('2345::1'), v6);
  assert.equal(maskSensitive('plain text', true), 'plain text');
});

const i18nMock = { default: { t: (_k, o) => o?.defaultValue ?? _k, exists: () => false } };
const regions = load('src/lib/ovh-regions.ts', { '@/i18n': i18nMock });
const subs = load('src/lib/ovh-subsidiaries.ts', {
  '@/lib/ovh-regions': regions,
  '@/i18n': i18nMock,
});

// 价格地区必须和账户实际下单的子公司(zone)一致,只能在本大区内手动切换,且按账户记忆。
// 以前跟 OVH /me 走、全局记忆、可选任意大区:加区账户被定成 WE,换回欧区账户仍停在 WE,价格全是 "—"。
test('price subsidiary follows the account zone and stays inside its region', () => {
  const { resolvePriceSubsidiary, accountSubsidiary } = subs;
  assert.equal(accountSubsidiary('', 'ovh-ca'), 'CA');
  assert.equal(accountSubsidiary('', 'ovh-us'), 'US');
  assert.equal(accountSubsidiary('fr', 'ovh-eu'), 'FR');
  // 没手动选过:跟账户走
  assert.equal(resolvePriceSubsidiary(null, 'ca-acc', 'CA', 'ovh-ca'), 'CA');
  // 在加区账户上手动选了 WE:只对那个账户有效
  const pick = { account: 'ca-acc', code: 'WE' };
  assert.equal(resolvePriceSubsidiary(pick, 'ca-acc', 'CA', 'ovh-ca'), 'WE');
  assert.equal(resolvePriceSubsidiary(pick, 'eu-acc', 'IE', 'ovh-eu'), 'IE');
  // 同一账户但选的是别的大区的子公司(账户改了 endpoint 等):退回账户自己的
  assert.equal(resolvePriceSubsidiary({ account: 'us-acc', code: 'WE' }, 'us-acc', 'US', 'ovh-us'), 'US');
});

// QA-P1-03: 外部 URL 必须校验 https: 协议,拦截 javascript:/data:/http: 伪协议注入
test('safeHttpsUrl only permits valid https URLs', () => {
  const { safeHttpsUrl } = load('src/lib/utils.ts', {
    clsx: { clsx: (...args) => args.join(' ') },
    'tailwind-merge': { twMerge: s => s },
  });
  assert.equal(
    safeHttpsUrl('https://eu.ovh.com/manager/#/billing/orders'),
    'https://eu.ovh.com/manager/#/billing/orders',
  );
  assert.equal(safeHttpsUrl('javascript:alert(1)'), undefined);
  assert.equal(safeHttpsUrl('data:text/html,<script>alert(1)</script>'), undefined);
  assert.equal(safeHttpsUrl('http://insecure.example.com/order.pdf'), undefined);
  assert.equal(safeHttpsUrl(''), undefined);
  assert.equal(safeHttpsUrl(null), undefined);
  assert.equal(safeHttpsUrl(undefined), undefined);
});

// QA-P1-05: api-error 必须兼容 AxiosError 与 http.ts 抛出的自定义 ApiError
test('api-error extracts status, code, body and message from both AxiosError and ApiError', () => {
  const apiErrMod = load('src/lib/api-error.ts', {
    '@/i18n': i18nMock,
    '@/i18n/translate-api': { translateApiText: s => s },
  });
  const axiosErr = {
    response: {
      status: 429,
      data: { error: 'Rate limit exceeded', code: 'RATE_LIMITED', extra: 'retry-after-5s' },
    },
    message: 'Request failed with status code 429',
  };
  assert.equal(apiErrMod.errorStatus(axiosErr), 429);
  assert.equal(apiErrMod.apiMessage(axiosErr), 'Rate limit exceeded');
  assert.equal(apiErrMod.errorField(axiosErr, 'extra'), 'retry-after-5s');
  assert.deepEqual(apiErrMod.errorBody(axiosErr), axiosErr.response.data);

  const customApiErr = {
    name: 'ApiError',
    status: 403,
    code: 'FORBIDDEN_ACTION',
    data: { error: 'Permission denied on this resource', detail: 'readonly' },
    message: 'Permission denied on this resource',
  };
  assert.equal(apiErrMod.errorStatus(customApiErr), 403);
  assert.equal(apiErrMod.apiMessage(customApiErr), 'Permission denied on this resource');
  assert.equal(apiErrMod.errorField(customApiErr, 'detail'), 'readonly');
  assert.deepEqual(apiErrMod.errorBody(customApiErr), customApiErr.data);
});

// QA-P0-02: qk.vpsControl 所有单实例查询 key 必须包含 accountId,防止跨账户缓存串号
test('qk.vpsControl scopes all per-VPS query keys by accountId', () => {
  const { qk } = load('src/lib/query.ts', {
    '@tanstack/react-query': { QueryClient: class {} },
  });
  const singleArgBuilders = [
    'info',
    'status',
    'serviceInfo',
    'ips',
    'datacenter',
    'templates',
    'currentOS',
    'tasks',
    'snapshot',
    'secondaryDns',
    'options',
    'automatedBackup',
    'engagement',
    'engagementAvailable',
    'engagementRequest',
    'mitigation',
  ];
  for (const name of singleArgBuilders) {
    const keyA = qk.vpsControl[name]('vps-123.ovh.net', 'acc-a');
    const keyB = qk.vpsControl[name]('vps-123.ovh.net', 'acc-b');
    assert.notDeepEqual(keyA, keyB, `qk.vpsControl.${name} must differ across accounts`);
    assert.ok(keyA.includes('acc-a'), `qk.vpsControl.${name} must include accountId`);
  }
  const taskKeyA = qk.vpsControl.task('vps-123.ovh.net', 42, 'acc-a');
  const taskKeyB = qk.vpsControl.task('vps-123.ovh.net', 42, 'acc-b');
  assert.notDeepEqual(taskKeyA, taskKeyB);
  assert.ok(taskKeyA.includes('acc-a'));
});

