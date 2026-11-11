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
  vm.runInNewContext(js, { exports, require: id => {
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

const { maskSensitive } = load('src/hooks/ovh/use-hide-ip.ts', { react: { useCallback: f => f, useEffect: () => {}, useState: v => [v, () => {}] } });

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

const i18nMock = { default: { t: (_k, o) => o?.defaultValue ?? _k } };
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
