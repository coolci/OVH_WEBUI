// Regression: assert corrected behavior with every API request intercepted.
// All API/external traffic is intercepted. No credentials, OVH orders or real writes.
const assert = require('node:assert/strict');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const base = process.env.OVH_UI_BASE || 'http://127.0.0.1:18080';
const accounts = [{id:'audit-a',name:'审计模拟账户',endpoint:'ovh-eu',zone:'IE',isDefault:true}];
const plan = {planCode:'audit-plan',name:'审计型号',cpu:'CPU',memory:'64GB',storage:'1TB',bandwidth:'1Gbps',availableOptions:[],defaultOptions:[],datacenters:[]};
const logs = Array.from({length:10},(_,i)=>({id:String(i),timestamp:'2026-11-06T01:00:00Z',level:i===0?'ERROR':'INFO',message:`audit log ${i}`,source:'audit'}));
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.OVH_UI_BROWSER?{executablePath:process.env.OVH_UI_BROWSER}:{})});
 const context=await browser.newContext({viewport:{width:1440,height:1080},locale:'zh-CN',colorScheme:'light'});
 await context.addInitScript(()=>{localStorage.setItem('ovh_sniper_api_key','audit-mock');localStorage.setItem('ovh-lang','zh');});
 const requests=[],errors=[];
 let subscriptions=[];
 let notificationMode="webhook";
 let regionIssues=[];
 await context.route('**/*', async route=>{
  const req=route.request(),url=new URL(req.url());
  const json=body=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
  if(url.origin!==new URL(base).origin) return json([]);
  if(!url.pathname.startsWith('/api/')&&url.pathname!='/health')return route.continue();
  requests.push({method:req.method(),path:url.pathname});
  const bodies={
   '/api/accounts':{accounts},'/api/servers':{servers:[plan]},'/api/settings':{endpoint:'ovh-eu',zone:'IE'},
   '/api/telegram/verify':{ok:false,reason:'仅启用 Webhook'},
   '/api/notify/channels':{anyAvailable:notificationMode !== 'none',verified:true,channels:[{name:notificationMode,configured:notificationMode !== 'none',ok:notificationMode !== 'none'}]},
   '/api/monitor/subscriptions':subscriptions, '/api/monitor/status':{running:false,subscriptions_count:subscriptions.length,check_interval:5,region_issues:regionIssues},
   '/api/vps-monitor/subscriptions':[], '/api/vps-monitor/models':{models:[{planCode:'vps-audit',name:'VPS模拟型号'}]},
   '/api/queue':[], '/api/logs':logs, '/api/catalog':{plans:[],addons:[],products:[]},
   '/api/stats':{activeQueues:0,totalServers:10,availableServers:0,purchaseSuccess:0,purchaseFailed:0,queueProcessorRunning:false,monitorRunning:false},
   '/api/system/metrics':{cpu:{cores:1,percent:0},memory:{totalBytes:1024,usedBytes:0,percent:0},disk:{totalBytes:1024,usedBytes:0,percent:0,path:'mock'},host:{hostname:'mock',uptimeSec:0}},
   '/api/version':{version:'audit'}, '/api/version/check-update':{hasUpdate:false},
  };
  if(req.method()!=='GET')throw new Error(`Unexpected mutation blocked: ${req.method()} ${url.pathname}`);
  return json(bodies[url.pathname]??{});
 });
 const page=await context.newPage();page.setDefaultTimeout(12000);page.on('pageerror',e=>errors.push(e.message));
 try {
  for (notificationMode of ['webhook','telegram','none']) {
   for(const path of ['/monitor','/vps-monitor']) {
    await page.goto(base+path);
    await page.getByRole('button',{name:'添加订阅',exact:true}).click();
    const dialog=page.getByRole('dialog');
    await page.waitForTimeout(300);
    assert.equal(await dialog.getByRole('button',{name:'确认添加',exact:true}).isDisabled(),notificationMode === 'none');
   }
   console.log(`REGRESSION notification gate: ${notificationMode}`);
  }
  notificationMode='webhook';
  assert.equal(requests.some(r=>r.path==='/api/notify/channels'),true);
  await page.goto(base+'/logs');
  await page.getByRole('button',{name:'切换主题',exact:true}).waitFor();
  await page.locator('body').click({position:{x:700,y:50}});
  for(const key of ['t','k']) {
   await page.keyboard.press('g');await page.keyboard.press(key);await page.waitForTimeout(250);
   await page.waitForURL(key === 't' ? '**/telegram-order' : '**/tickets');
  }
  await page.keyboard.press('g');await page.keyboard.press('s');
  await page.waitForURL('**/servers');
  console.log('REGRESSION G T / G K and G S navigate');
  await page.goto(base+'/settings');
  await page.getByRole('button',{name:'外观',exact:true}).click();
  const light=page.getByRole('button').filter({has:page.getByText('浅色',{exact:true})});
  await light.click();
  assert.equal(await page.locator('html').evaluate(e=>e.classList.contains('dark')),false);
  assert.equal(await page.evaluate(()=>localStorage.getItem('ovh-theme')),'light');
  await page.reload();
  await page.getByRole('button',{name:'切换主题',exact:true}).waitFor();
  assert.equal(await page.locator('html').evaluate(e=>e.classList.contains('dark')),false);
  assert.equal(await page.evaluate(()=>localStorage.getItem('ovh-theme')),'light');
  console.log('REGRESSION Light theme persists after reload');
  await page.getByRole('button',{name:'切换主题',exact:true}).click();
  await page.getByRole('menuitem',{name:'跟随系统'}).click();
  await page.emulateMedia({colorScheme:'dark'});
  await page.waitForFunction(()=>document.documentElement.classList.contains('dark'));
  await page.emulateMedia({colorScheme:'light'});
  await page.waitForFunction(()=>!document.documentElement.classList.contains('dark'));
  console.log('REGRESSION single theme provider follows system changes');

  await page.goto(base+'/logs');
  await page.getByRole('combobox').first().click();
  await page.getByRole('option',{name:'ERROR',exact:true}).click();
  await page.getByText(/显示 1$/).waitFor();
  assert.equal(await page.getByText(/内存中共 10 条匹配日志/).count(),0);
  console.log('REGRESSION Filtered logs report matching total');
  await page.goto(base+'/');
  await page.getByText('已读取本地状态',{exact:true}).waitFor();
  console.log('REGRESSION Dashboard labels local backend status');
  subscriptions=[
   {planCode:'audit-plan',serverName:'配置A',options:['ram-a'],datacenters:['gra'],notifyAvailable:true,autoOrder:false,lastStatus:{},createdAt:'2026-11-06T01:00:00Z'},
   {planCode:'audit-plan',serverName:'配置B',options:['ram-b'],datacenters:['gra'],notifyAvailable:true,autoOrder:false,lastStatus:{},createdAt:'2026-11-06T01:00:00Z'},
  ];
  await page.reload();
  await page.getByRole('button',{name:'设置',exact:true}).nth(1).click();
  assert.equal(await page.getByRole('dialog').locator('#monitor-options').inputValue(),'ram-b');
  console.log('REGRESSION Second configuration opens ram-b');
  assert.deepEqual(errors,[]);
  regionIssues=[{planCode:'audit-plan',serverName:'模拟型号',accountId:'audit-a',region:'EU',subsidiary:'IE',checkedAt:'2026-11-06T01:00:00Z',error:'模拟区域不匹配'}];
  await page.goto(base+'/monitor');
  await page.getByText(/模拟区域不匹配/).waitFor();
  assert.deepEqual(errors,[]);
  console.log('REGRESSION OM-09 fixed: structured region issue renders without React errors');
  console.log('6 UI regressions passed; OM-09 regression passed; no real API writes, external requests or render exceptions');
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});