// Audit evidence only: all API and external traffic is intercepted.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.OVH_UI_BASE || 'http://127.0.0.1:18080';
(async () => {
  const browser = await chromium.launch({headless:true, ...(process.env.OVH_UI_BROWSER ? {executablePath:process.env.OVH_UI_BROWSER} : {})});
  const context = await browser.newContext({viewport:{width:1280,height:1080}, locale:'zh-CN'});
  await context.addInitScript(() => {
    localStorage.setItem('ovh_sniper_api_key', 'audit-mock');
    localStorage.setItem('ovh-lang', 'zh');
  });
  const created = [];
  await context.route('**/*', async route => {
    const req = route.request(), url = new URL(req.url());
    const json = (body, status=200) => route.fulfill({status, contentType:'application/json', body:JSON.stringify(body)});
    if (url.origin !== new URL(base).origin) return json([]);
    if (!url.pathname.startsWith('/api/') && url.pathname !== '/health') return route.continue();
    if (url.pathname === '/api/accounts' && req.method() === 'POST') {
      const account = {...req.postDataJSON(), id:'mock-saved-'+(created.length+1), isDefault:true};
      created.push(account);
      return json({success:false, account, valid:false, error:'AUDIT: account saved but remote verification rejected'},422);
    }
    if (req.method() !== 'GET') throw new Error('Unexpected mutation blocked: '+req.method()+' '+url.pathname);
    const bodies = {
      '/api/accounts':{accounts:created},
      '/api/stats':{activeQueues:0,totalServers:0,availableServers:0,purchaseSuccess:0,purchaseFailed:0,queueProcessorRunning:false,monitorRunning:false},
      '/api/settings':{zone:'IE',endpoint:'ovh-eu'}, '/api/version':{version:'audit'},
      '/api/version/check-update':{hasUpdate:false}, '/api/queue':[], '/api/logs':[],
      '/api/servers':{servers:[]}, '/api/monitor/subscriptions':[], '/api/monitor/status':{running:false},
      '/api/system/metrics':{cpu:{cores:1,percent:0},memory:{totalBytes:1,usedBytes:0,percent:0},disk:{totalBytes:1,usedBytes:0,percent:0,path:'mock'},host:{}},
    };
    return json(bodies[url.pathname] ?? {});
  });
  const page = await context.newPage();
  page.setDefaultTimeout(12000);
  const errors = [];
  page.on('pageerror',e => errors.push(e.message));
  try {
    await page.goto(base+'/');
    const submit = page.getByRole('button',{name:'创建并进入',exact:true});
    await submit.waitFor();
    await page.locator('input[type=password]').nth(0).fill('audit-app-key');
    await page.locator('input[type=password]').nth(1).fill('audit-app-secret');
    await page.locator('input[type=password]').nth(2).fill('audit-consumer');
    await submit.click();
    await submit.waitFor({state:'detached'});
    assert.equal(created.length,1);
    await page.reload();
    await page.getByRole('button',{name:'切换主题',exact:true}).waitFor();
    assert.equal(await submit.count(),0);
    assert.equal(created.length,1);
    assert.deepEqual(errors,[]);
    console.log('REGRESSION PF-07: saved 422 account exits the gate and remains a single account after reload. All API/external traffic intercepted.');
  } finally { await browser.close(); }
})().catch(e => {console.error(e);process.exitCode=1;});
