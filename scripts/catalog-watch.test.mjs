import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
import { jsx, jsxs, Fragment } from 'react/jsx-runtime';
import { renderToStaticMarkup } from 'react-dom/server';

function load(file, imports) {
  const exports = {};
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), {compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX}}).outputText;
  vm.runInNewContext(js, {exports, require:id=>{if (!(id in imports)) throw Error(id);return imports[id];}}, {filename:file});
  return exports;
}
test('catalog watch mounting and polling never change the backend switch', async () => {
  const calls=[];
  const hooks=load('src/hooks/use-catalog-watch.ts',{
    '@tanstack/react-query':{useQuery:o=>o,useMutation:o=>o,useQueryClient:()=>({setQueryData(){}})},
    '@/lib/http':{api:{get:async path=>{calls.push(['GET',path]);return {data:{enabled:true}};},put:async(path,body)=>{calls.push(['PUT',path,body]);return {data:{enabled:body.enabled}};}}},
  });
  const q=hooks.useCatalogWatch();await q.queryFn();
  assert.equal(q.refetchInterval,2000);
  assert.deepEqual(calls,[['GET','/catalog-watch']]);
  const toggle=hooks.useSetCatalogWatch();await toggle.mutationFn(false);
  assert.equal(calls[1][2].enabled,false);
  assert.deepEqual(Object.keys(calls[1][2]),['enabled']);
});
test('server-list panel shows backend-owned state and explains closing the browser', () => {
  let status={enabled:true,accountName:'Default A',subsidiary:'IE',baselineAt:'2026-11-07T00:00:00Z',baselineCount:2,lastCheck:'',error:'',active:false,counts:{pending:2,attempting:1,completed:4,uncertain:5},jobs:[{id:'private-job',planCode:'DO-NOT-RENDER-ORDER',options:[],datacenters:['gra'],status:'completed',orderId:'private-order'}]};
  const panel=load('src/components/servers/CatalogWatchPanel.tsx',{
    'react/jsx-runtime':{jsx,jsxs,Fragment},
    '@/components/ui/switch':{Switch:({checked,disabled})=>jsx('input',{type:'checkbox',checked,disabled,readOnly:true})},
    '@/hooks/use-catalog-watch':{useCatalogWatch:()=>({data:status}),useSetCatalogWatch:()=>({isPending:false,mutate(){}})},
    '@/lib/api-error':{},sonner:{},
  });
  let html=renderToStaticMarkup(jsx(panel.CatalogWatchPanel,{}));
  assert.match(html,/checked/);assert.match(html,/默认开启/);assert.match(html,/关闭网页不会停止/);assert.match(html,/Default A/);assert.match(html,/120 秒/);
  assert.match(html,/每个 planCode × 配置 × 机房各成功 1 台/);assert.match(html,/各机房独立并发尝试/);
  assert.match(html,/最多 100 个同时执行/);assert.match(html,/本轮统计/);assert.match(html,/切换开关时归零/);assert.match(html,/后台自动查单/);
  assert.doesNotMatch(html,/最近自动下单记录|最多 30 条|<details|private-order|DO-NOT-RENDER-ORDER/);
  assert.match(html,/自动下单统计/);assert.match(html,/>3<\/div>/);assert.match(html,/>4<\/div>/);assert.match(html,/>5<\/div>/);
  status={...status,enabled:false};html=renderToStaticMarkup(jsx(panel.CatalogWatchPanel,{}));
  assert.doesNotMatch(html,/checked=""/);assert.match(html,/后台检测已关闭/);
});

test('history refreshes background purchases and orders timestamps chronologically', async () => {
  const response=[{orderId:'old',purchaseTime:'2026-11-06T23:49:00+08:00'},{orderId:'new',purchaseTime:'2026-11-06T16:54:22Z'}];
  const hooks=load('src/hooks/ovh/use-history.ts',{
    '@tanstack/react-query':{useQuery:o=>o},
    '@/lib/http':{api:{get:async()=>({data:response})}},
    '@/lib/query':{qk:{history:()=>['history']}},
    'sonner':{},'@/i18n':{},'@/lib/api-error':{},
  });
  const q=hooks.useHistory();assert.equal(q.refetchInterval,5000);assert.equal(q.refetchIntervalInBackground,false);
  for(const k of ['refetchOnMount','refetchOnWindowFocus','refetchOnReconnect'])assert.equal(q[k],'always');
  assert.equal((await q.queryFn())[0].orderId,'new');assert.equal(response[0].orderId,'old');
});
