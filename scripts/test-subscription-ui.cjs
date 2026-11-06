// Isolated browser regression. Every API request is mocked; no real orders are sent.
// Start Vite on port 18080, then run with Playwright installed (or PLAYWRIGHT_MODULE set).
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.OVH_UI_BASE || 'http://127.0.0.1:18080';
const accounts = [
  { id: 'account-a', name: '测试账户 A', endpoint: 'ovh-eu', zone: 'IE', isDefault: true },
  { id: 'account-b', name: '测试账户 B', endpoint: 'ovh-eu', zone: 'FR', isDefault: false },
];
const plan = {
  planCode: 'known-plan', name: '测试服务器', cpu: 'Test CPU', memory: '64GB', storage: '1TB',
  bandwidth: '1Gbps', vrackBandwidth: '', availableOptions: [], defaultOptions: [],
  datacenters: [{ datacenter: 'gra', dcName: 'Gravelines', region: 'EU', availability: 'unavailable', countryCode: 'FR' }],
};

async function main() {
  const browser = await chromium.launch({
    headless: true,
    ...(process.env.OVH_UI_BROWSER ? { executablePath: process.env.OVH_UI_BROWSER } : {}),
  });
  const context = await browser.newContext({ viewport: { width: 1440, height: 1080 }, locale: 'zh-CN' });
  await context.addInitScript(() => {
    localStorage.setItem('ovh_sniper_api_key', 'isolated-browser-test');
    localStorage.setItem('ovh-lang', 'zh');
  });
  const writes = [];
  const errors = [];
  let subscriptions = [];
  let regionIssues = [];
  await context.route('**/*', async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const json = (body) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    // Block external traffic, including the public availability API.
    if (url.origin !== new URL(base).origin) return json([]);
    if (!url.pathname.startsWith('/api/') && url.pathname !== '/health') return route.continue();
    if (!['GET', 'HEAD'].includes(request.method())) {
      const body = request.postDataJSON();
      writes.push({ method: request.method(), path: url.pathname, query: url.searchParams.toString(), body });
      if (url.pathname === '/api/monitor/subscriptions') {
        subscriptions = [...subscriptions, { ...body, createdAt: new Date().toISOString(), lastStatus: {} }];
      }
      if (request.method() === 'PUT' && url.pathname.startsWith('/api/monitor/subscriptions/')) {
        subscriptions = subscriptions.map((s) => s.planCode === body.planCode ? { ...s, ...body } : s);
      }
      return json({ status: 'success', success: true, id: 'mock-task', message: '已保存' });
    }
    if (url.pathname === '/api/accounts') return json({ accounts });
    if (url.pathname === '/api/servers') return json({ servers: [plan] });
    if (url.pathname === '/api/monitor/subscriptions') return json(subscriptions);
    if (url.pathname === '/api/monitor/status') return json({ running: true, subscriptions_count: subscriptions.length, check_interval: 5, region_issues: regionIssues });
    if (url.pathname === '/api/telegram/verify') return json({ ok: true });
    if (url.pathname === '/api/settings') return json({ endpoint: 'ovh-eu', zone: 'IE' });
    if (url.pathname === '/api/catalog') return json({ plans: [], addons: [], products: [] });
    if (url.pathname === '/api/queue') return json([]);
    if (url.pathname === '/api/vps-monitor/subscriptions') return json([]);
    if (url.pathname === '/api/vps-monitor/models') return json({ models: [{ planCode: 'vps-known', name: '测试VPS' }] });
    if (url.pathname.endsWith('/history')) return json([]);
    return json({});
  });
  const page = await context.newPage();
  page.on('pageerror', (error) => errors.push(error.message));
  page.setDefaultTimeout(10000);
  const dialog = () => page.getByRole('dialog');
  const selectCustom = async (value, keyboard = false) => {
    await dialog().getByRole('combobox').filter({ hasText: /输入或搜索|搜索型号|vps-known/ }).click();
    const search = page.locator('[cmdk-input]');
    await search.fill(value);
    const option = page.getByRole('option').filter({ hasText: value }).last();
    await option.waitFor();
    if (keyboard) await search.press('Enter');
    else await option.click();
    await dialog().getByRole('combobox').filter({ hasText: value }).waitFor();
  };
  const waitForWrite = async (start, path) => {
    for (let attempt = 0; attempt < 100; attempt++) {
      const hit = writes.slice(start).find((item) => item.path === path);
      if (hit) return hit.body;
      await new Promise((resolve) => setTimeout(resolve, 50));
    }
    throw new Error(`No mocked request captured for ${path}`);
  };
  try {
    await page.goto(`${base}/monitor`);
    await page.getByRole('button', { name: '添加订阅', exact: true }).click();
    await selectCustom('custom-unlisted.plan');
    await dialog().getByText('有货时自动下单', { exact: true }).click();
    assert.equal(await dialog().getByText('已选 2 / 2 个账户', { exact: true }).count(), 1);
    assert.equal(await dialog().getByRole('checkbox', { name: /测试账户 A/ }).isChecked(), true);
    assert.equal(await dialog().getByRole('checkbox', { name: /测试账户 B/ }).isChecked(), true);
    assert.equal(await dialog().getByRole('checkbox', { name: /抢到后自动付款/ }).isChecked(), false);
    await dialog().getByRole('spinbutton').fill('2');
    let start = writes.length;
    await dialog().getByRole('button', { name: '确认添加', exact: true }).click();
    const created = await waitForWrite(start, '/api/monitor/subscriptions');
    assert.equal(created.planCode, 'custom-unlisted.plan');
    assert.deepEqual(created.autoOrderAccountIds, ['account-a', 'account-b']);
    assert.equal(created.quantity, 2);
    assert.equal(created.autoPay, false);
    console.log('PASS custom plan + default two-account subscription + quantity + payment default');

    // Existing single-account subscriptions must not silently expand during an edit.
    subscriptions = [{
      planCode: 'legacy-custom', datacenters: ['gra'], notifyAvailable: true, notifyUnavailable: false,
      autoOrder: true, autoOrderAccountId: 'account-b', quantity: 1, autoPay: false,
      options: ['custom-ram-addon'], lastStatus: {}, createdAt: new Date().toISOString(),
    }];
    await page.reload();
    await page.getByRole('button', { name: /编辑/ }).first().click();
    assert.equal(await dialog().getByRole('checkbox', { name: /测试账户 A/ }).isChecked(), false);
    assert.equal(await dialog().getByRole('checkbox', { name: /测试账户 B/ }).isChecked(), true);
    start = writes.length;
    await dialog().getByRole('button', { name: '保存设置', exact: true }).click();
    const edited = await waitForWrite(start, '/api/monitor/subscriptions/legacy-custom');
    assert.deepEqual(edited.autoOrderAccountIds, ['account-b']);
    assert.deepEqual(edited.options, ['custom-ram-addon']);
    console.log('PASS legacy single-account edit preserves account and custom addons');

    // Multi-account selection must survive saving a subset and re-opening it.
    subscriptions = [{ ...subscriptions[0], autoOrderAccountId: '', autoOrderAccountIds: ['account-a', 'account-b'] }];
    await page.reload();
    await page.getByRole('button', { name: /编辑/ }).first().click();
    await dialog().getByRole('checkbox', { name: /测试账户 B/ }).uncheck();
    start = writes.length;
    await dialog().getByRole('button', { name: '保存设置', exact: true }).click();
    const subset = await waitForWrite(start, '/api/monitor/subscriptions/legacy-custom');
    assert.deepEqual(subset.autoOrderAccountIds, ['account-a']);
    console.log('PASS choosing an account subset');

    subscriptions = [{ ...subscriptions[0], autoOrder: false, autoOrderAccountId: '', autoOrderAccountIds: [] }];
    await page.reload();
    await page.getByRole('button', { name: /编辑/ }).first().click();
    assert.equal(await dialog().getByRole('checkbox', { name: /有货时自动下单/ }).isChecked(), false);
    await dialog().getByText('有货时自动下单', { exact: true }).click();
    assert.equal(await dialog().getByText('已选 2 / 2 个账户', { exact: true }).count(), 1);
    await dialog().getByRole('checkbox', { name: /测试账户 A/ }).uncheck();
    await dialog().getByRole('checkbox', { name: /测试账户 B/ }).uncheck();
    assert.equal(await dialog().getByText('已选 0 / 2 个账户', { exact: true }).count(), 1);
    await dialog().getByRole('button', { name: '取消', exact: true }).click();
    console.log('PASS explicitly enabling an old notification subscription defaults to all accounts without overriding manual clearing');

    await page.goto(`${base}/queue`);
    await page.getByRole('button', { name: '新建抢购', exact: true }).click();
    await selectCustom('custom-queue-plan');
    assert.equal(await dialog().getByRole('combobox').filter({ hasText: 'custom-queue-plan' }).count(), 1);
    await dialog().getByRole('button', { name: /GRA/ }).click();
    await dialog().getByLabel('自定义配置 planCode（可选）').fill('custom-ram,custom-disk');
    start = writes.length;
    await dialog().getByRole('button', { name: /创建/ }).last().click();
    const queue = await waitForWrite(start, '/api/queue');
    assert.equal(queue.planCode, 'custom-queue-plan');
    assert.equal(queue.force, true);
    assert.deepEqual(queue.options, ['custom-ram', 'custom-disk']);
    console.log('PASS custom plan and addons submitted to queue with directory override');

    subscriptions = [];
    await page.goto(`${base}/servers`);
    await page.getByRole('button', { name: /监控/ }).last().click();
    await dialog().getByText('有货时自动下单', { exact: true }).click();
    assert.equal(await dialog().getByText('已选 2 / 2 个账户', { exact: true }).count(), 1);
    start = writes.length;
    await dialog().getByRole('button', { name: '确认添加', exact: true }).click();
    const fromServer = await waitForWrite(start, '/api/monitor/subscriptions');
    assert.equal(fromServer.planCode, 'known-plan');
    assert.deepEqual(fromServer.autoOrderAccountIds, ['account-a', 'account-b']);
    console.log('PASS server-card subscription defaults to both accounts');

    await page.goto(`${base}/monitor`);
    await page.getByRole('button', { name: '添加订阅', exact: true }).click();
    start = writes.length;
    await selectCustom('keyboard-custom-plan', true);
    assert.equal(writes.length, start, 'Enter in model picker must not submit the subscription');
    await dialog().getByRole('button', { name: '取消', exact: true }).click();
    console.log('PASS Enter selects a custom plan without submitting the parent form');

    await page.goto(`${base}/`);
    await page.getByRole('button', { name: '快速下单', exact: true }).first().click();
    await selectCustom('custom-quick-plan');
    await dialog().getByRole('button', { name: /GRA/ }).click();
    await dialog().getByLabel('自定义配置 planCode（可选，逗号分隔）').fill('quick-custom-addon');
    start = writes.length;
    await dialog().getByRole('button', { name: /创建 1 个任务/ }).click();
    const quick = await waitForWrite(start, '/api/queue');
    assert.equal(quick.planCode, 'custom-quick-plan');
    assert.equal(quick.force, true);
    assert.deepEqual(quick.options, ['quick-custom-addon']);
    console.log('PASS quick-order custom plan and addon submission');

    await page.goto(`${base}/vps-monitor`);
    await page.getByRole('button', { name: /添加订阅/ }).first().click();
    await selectCustom('vps-custom-plan', true);
    start = writes.length;
    await dialog().getByRole('button', { name: /确认添加|添加订阅/ }).click();
    const vps = await waitForWrite(start, '/api/vps-monitor/subscriptions');
    assert.equal(vps.planCode, 'vps-custom-plan');
    assert.ok(!vps.autoOrder, 'VPS monitoring must not silently enable purchases');
    console.log('PASS VPS custom plan remains selected and submits for monitoring');

    subscriptions = [
      { planCode: 'identity-plan', options: ['ram-64g'], datacenters: ['gra'], lastStatus: {}, createdAt: new Date().toISOString() },
      { planCode: 'identity-plan', options: [], datacenters: ['gra'], lastStatus: {}, createdAt: new Date().toISOString() },
    ];
    regionIssues = [{ planCode: 'identity-plan', accountId: 'account-b', region: 'EU', error: 'mock inventory unavailable' }];
    await page.goto(`${base}/monitor`);
    await page.getByText(/mock inventory unavailable/).waitFor();
    start = writes.length;
    await page.getByRole('button', { name: '暂停此订阅', exact: true }).last().click();
    const paused = await waitForWrite(start, '/api/monitor/subscriptions/identity-plan/pause');
    assert.equal(paused.optionsKey, '');
    assert.equal(paused.paused, true);
    console.log('PASS structured region error renders and empty configuration pause preserves identity');

    assert.deepEqual(errors, [], `Browser runtime errors: ${errors.join('\n')}`);
    console.log('PASS no browser runtime errors; all network writes were mocked');
  } finally {
    await browser.close();
  }
}

main().catch((error) => { console.error(error); process.exitCode = 1; });
