const assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const base=process.env.OVH_UI_BASE||'http://127.0.0.1:18080';
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.OVH_UI_BROWSER?{executablePath:process.env.OVH_UI_BROWSER}:{})});
 const context=await browser.newContext({locale:'zh-CN',viewport:{width:1440,height:1000}});
 await context.addInitScript(()=>{localStorage.setItem('ovh_sniper_api_key','fixture');localStorage.setItem('ovh-lang','zh');});
 let mode='error';
 await context.route('**/*',async route=>{
  const req=route.request(),url=new URL(req.url());
  const json=(data,status=200,headers={})=>route.fulfill({status,contentType:'application/json',headers,body:JSON.stringify(data)});
  if(url.origin!==new URL(base).origin)return json([]);
  if(!url.pathname.startsWith('/api/')&&url.pathname!='/health')return route.continue();
  assert.equal(req.method(),'GET');
  if(url.pathname==='/api/ovh/account/email-history'){
   if(mode==='error')return json({error:'fixture detail failure'},502);
   if(mode==='partial')return json([{id:1,subject:'fixture email',date:'2026-11-06',body:'fixture'}],200,{'X-Partial-Failures':'1'});
   return json([]);
  }
  const bodies={'/api/accounts':{accounts:[{id:'fixture',name:'fixture',isDefault:true,zone:'IE',endpoint:'ovh-eu'}]},'/api/ovh/account/info':{customerCode:'fixture'},'/api/logs':[], '/api/settings':{}};
  return json(bodies[url.pathname]||{});
 });
 const page=await context.newPage();page.setDefaultTimeout(12000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
 try{
  await page.goto(base+'/account');await page.getByText('fixture detail failure',{exact:true}).waitFor();
  assert.equal(await page.getByText('暂无邮件',{exact:true}).count(),0);
  mode='partial';await page.reload();await page.getByText('fixture email',{exact:true}).waitFor();
  await page.getByText(/邮件有 1 条详情未能获取/).waitFor();
  mode='error';await page.getByRole('button',{name:'刷新',exact:true}).nth(1).click();
  await page.getByText('刷新失败，显示上次读取的记录',{exact:true}).waitFor();
  await page.getByText('fixture email',{exact:true}).waitFor();
  mode='empty';await page.reload();await page.getByText('暂无邮件',{exact:true}).waitFor();
  assert.deepEqual(errors,[]);
  console.log('PASS account records: initial error, partial data, stale-cache refresh error, genuine empty list');
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
