// Read-only production-source inventory; writes audit artifacts only.
const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const ts = require('typescript');
const out = path.resolve('docs/audit-2026-11-06');
fs.mkdirSync(path.join(out,'evidence'),{recursive:true});
const tracked = [...new Set(cp.execFileSync('git',['ls-files','--cached','--others','--exclude-standard'],{encoding:'utf8'}).trim().split(/\r?\n/).filter(f=>!f.startsWith('docs/audit-2026-11-06/')))];
const commit = cp.execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim();
const functions = JSON.parse(cp.execFileSync('go',['run',path.join(out,'tools/index-go.go')],{encoding:'utf8',maxBuffer:32*1024*1024}));
functions.push(...JSON.parse(cp.execFileSync('python',[path.join(out,'tools/index-python.py')],{encoding:'utf8'})));
const requests = [];
for (const file of tracked.filter(f=>/\.(tsx?|jsx?|mjs|cjs)$/.test(f))) {
 const source = fs.readFileSync(file,'utf8');
 const sf = ts.createSourceFile(file,source,ts.ScriptTarget.Latest,true);
 const line = n=>sf.getLineAndCharacterOfPosition(n.getStart(sf)).line+1;
 function visit(node) {
  const isFunction = ts.isFunctionDeclaration(node)||ts.isMethodDeclaration(node)||ts.isArrowFunction(node)||ts.isFunctionExpression(node);
  if(isFunction) {
   const p=node.parent;
   const name=node.name?.getText(sf)||(ts.isVariableDeclaration(p)||ts.isPropertyAssignment(p)?p.name.getText(sf):`<callback@${line(node)}>`);
   functions.push({file,name,line:line(node),end:sf.getLineAndCharacterOfPosition(node.end).line+1,kind:node.name||ts.isVariableDeclaration(p)?'function/component/hook':'callback'});
  }
  if(ts.isCallExpression(node)) {
   const expr=node.expression.getText(sf);
   if(/(?:api\.(?:get|post|put|delete|patch)|apiRequest|fetch)$/.test(expr)) {
    requests.push({file,line:line(node),call:expr,arg:node.arguments[0]?.getText(sf)||''});
   }
  }
  ts.forEachChild(node,visit);
 }
 visit(sf);
}
const files=tracked.map(file=>({file,bytes:fs.statSync(file).size,lines:/\.(go|tsx?|jsx?|mjs|cjs|json|ya?ml|sh|py|html|css|md)$/.test(file)?fs.readFileSync(file,'utf8').split(/\r?\n/).length:null,functions:functions.filter(f=>f.file===file).length}));
fs.writeFileSync(path.join(out,'evidence/source-index.json'),JSON.stringify({commit,files,functions,requests},null,2));
const safe=s=>s.replace(/\|/g,'\\|').replace(/\r?\n/g,' ');
const link=(f,line)=>`[${f}${line?':'+line:''}](../../${f}${line?'#L'+line:''})`;
fs.writeFileSync(path.join(out,'FUNCTION-INDEX.md'),`# 全项目函数与回调索引\n\n当前工作区（包含本次修复）；基础提交：\`${commit}\`。由 Go / TypeScript / Python AST 提取；Shell/PowerShell额外列出脚本入口和词法函数定义（结束行仅为定义行），列出命名函数、方法、组件、hook 和 JS/TS 匿名回调；Go 匿名闭包归属其所在命名函数，不另列。类型声明、样式、配置不是可执行函数，见文件清单。**本索引是导航/提取证据，不等于每个函数都已动态测试。**\n\n共 ${functions.length} 条。业务含义和缺陷见各模块报告。\n\n| 文件 / 行 | 独立函数或回调 | 类型 | 结束行 |\n|---|---|---|---|\n`+functions.map(f=>`| ${link(f.file,f.line)} | \`${safe(f.name)}\` | ${f.kind} | ${f.end} |`).join('\n')+'\n');
fs.writeFileSync(path.join(out,'FILE-INDEX.md'),`# 仓库文件清单\n\n当前工作区基础提交：\`${commit}\`；${files.length} 个受控文件及当前新增业务测试文件。包含源代码、测试、部署、接口说明和已有文档；不读取 node_modules、生成 dist、用户运行数据或密钥。目录索引不代表逐行人工审查，人工范围见 COVERAGE.md。\n\n| 文件 | 行数 | AST 函数数 | 字节 |\n|---|---|---|---|\n`+files.map(f=>`| ${link(f.file)} | ${f.lines??'二进制/其他'} | ${f.functions} | ${f.bytes} |`).join('\n')+'\n');
fs.writeFileSync(path.join(out,'API-CALL-INDEX.md'),`# 前端及脚本请求调用点\n\n静态 AST 提取 ${requests.length} 个 api.get/post/put/delete/patch、apiRequest、fetch 调用点。动态模板保留表达式；包括测试脚本，不能直接等同于真实 HTTP 路由数。路由表另见 ENDPOINTS.md。\n\n| 文件 / 行 | 调用 | URL/首参数表达式 |\n|---|---|---|\n`+requests.map(r=>`| ${link(r.file,r.line)} | \`${r.call}\` | \`${safe(r.arg)}\` |`).join('\n')+'\n');
const ledger=fs.readFileSync('docs/prd-v2/progress/ledger.csv','utf8').trim().split(/\r?\n/).slice(1).map(l=>l.split(','));
fs.writeFileSync(path.join(out,'ENDPOINTS.md'),`# 已登记后端接口清单\n\n从项目 ledger 提取 ${ledger.length} 条；本次 verify:ledger 验证登记状态/实现映射通过。这不等于 ${ledger.length} 个接口都经过真实外部 API 端到端测试。新增/未登记路径见路由源码与模块报告。\n\n| ID | 领域 | Method | 路径 | 登记状态 |\n|---|---|---|---|---|\n`+ledger.map(r=>`| ${r[0]} | ${r[1]} | ${r[3]} | \`${r[4]}\` | ${r[6]} |`).join('\n')+'\n');
const routesPath=path.join(out,'evidence/routes.json');
if(fs.existsSync(routesPath)) {
 const routes=JSON.parse(fs.readFileSync(routesPath,'utf8')).sort((a,b)=>a.path.localeCompare(b.path)||a.method.localeCompare(b.method));
 const key=(method,path)=>method+' '+path.replace(/:[A-Za-z0-9_]+/g,':param');
 const keys=new Set(ledger.map(r=>key(r[3],r[4])));
 const actual=new Set(routes.map(r=>key(r.method,r.path)));
 const extra=routes.filter(r=>!keys.has(key(r.method,r.path)));
 const missing=ledger.filter(r=>!actual.has(key(r[3],r[4])));
 fs.writeFileSync(path.join(out,'evidence/route-ledger-diff.json'),JSON.stringify({actual:routes.length,ledger:ledger.length,extra,missing},null,2));
 fs.writeFileSync(path.join(out,'ENDPOINTS.md'),`# 当前真实路由与登记表\n\n通过构造实际 Gin router（未监听端口、未调用外部服务）取得 **${routes.length}** 条路由。当前 ledger 为 ${ledger.length} 条；实际存在但登记表未列 ${extra.length} 条，登记表有但实际未列 ${missing.length} 条。\`verify:ledger\`通过仅表示其定义的检查通过，不能解释为完整接口覆盖或业务正确性。方法/路径精确比较，参数名称归一化，详见 [差异JSON](evidence/route-ledger-diff.json)。\n\n| Method | 当前路径 | Handler | ledger匹配 |\n|---|---|---|---|\n`+routes.map(r=>`| ${r.method} | \`${r.path}\` | \`${r.handler.replace('github.com/ovh-webui/server/internal/','')}\` | ${keys.has(key(r.method,r.path))?'是':'未登记'} |`).join('\n')+'\n');
 console.log(JSON.stringify({actualRoutes:routes.length,unlistedRoutes:extra.length,staleLedgerEntries:missing.length}));
}
console.log(JSON.stringify({commit,trackedFiles:files.length,goFiles:files.filter(f=>f.file.endsWith('.go')).length,tsFiles:files.filter(f=>/\.tsx?$/.test(f.file)).length,functions:functions.length,requestSites:requests.length,ledgerEndpoints:ledger.length}));
