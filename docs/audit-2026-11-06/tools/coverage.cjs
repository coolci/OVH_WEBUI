const fs=require('node:fs');
const base='docs/audit-2026-11-06/';
const index=JSON.parse(fs.readFileSync(base+'evidence/source-index.json','utf8'));
const reviews=new Map();
const rank={'indexed-only':0,structural:1,partial:2,full:3};
if(fs.existsSync(base+'structural-coverage.json'))for(const r of JSON.parse(fs.readFileSync(base+'structural-coverage.json')))reviews.set(r.file,{review:'structural',owner:'AST结构核对'});
for(const owner of ['root','platform','server-support','order-monitor']){
 const file=base+owner+'-coverage.json';if(!fs.existsSync(file))continue;
 const c=JSON.parse(fs.readFileSync(file,'utf8'));
 const entries=[...(c.full||[]).map(file=>({file,review:'full'})),...(c.partial||[]).map(x=>({...x,file:x.path||x.file,review:'partial'})),...(c.files||[])];
 for(const e of entries){if(e.teamReview==='full'){e.review='full';e.team=true;}const old=reviews.get(e.file);if(!old||rank[e.review]>rank[old.review])reviews.set(e.file,{...e,owner:e.team?'platform + server-support（分段联合）':owner});}
}
const rows=index.files.map(f=>({...f,...(reviews.get(f.file)||{review:'indexed-only',owner:'AST/文件索引'})}));
const pending=rows.filter(f=>/^(src|backend)\//.test(f.file)&&/\.(go|tsx?)$/.test(f.file)&&!/_test\.go$/.test(f.file)&&!['full','structural'].includes(f.review));
fs.writeFileSync(base+'evidence/coverage.json',JSON.stringify({rows,pendingProduction:pending},null,2));
fs.writeFileSync(base+'COVERAGE.md','# 逐文件审查覆盖\n\n本表合并各审查者的实际记录。full表示完整实现已人工审读（不等于动态分支全覆盖）；structural表示已核对AST结构的类型/资源/转导出或简单UI包装，详见STRUCTURAL.md；partial保留明确区间；indexed-only只完成文件/AST提取。测试和文档不冒充逐行人工业务审查。联合完整覆盖保留各审查者的分段证据。\n\n| 文件 | 审查级别 | 记录来源 |\n|---|---|---|\n'+rows.map(f=>`| [${f.file}](../../${f.file}) | ${f.review} | ${f.owner} |`).join('\n')+'\n');
console.log(JSON.stringify({full:rows.filter(f=>f.review==='full').length,partial:rows.filter(f=>f.review==='partial').length,pendingProduction:pending.length}));
console.log(pending.map(f=>f.file).join('\n'));
