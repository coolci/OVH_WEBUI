const fs=require('node:fs'),ts=require('typescript');
const base='docs/audit-2026-11-06/';
const files=JSON.parse(fs.readFileSync(base+'evidence/source-index.json')).files;
const result=[];
for(const {file} of files.filter(x=>x.file.startsWith('src/')&&/\.tsx?$/.test(x.file))){
 const source=fs.readFileSync(file,'utf8'),sf=ts.createSourceFile(file,source,99,true);
 const calls=new Set();let functions=0,danger=false;
 function visit(n){
  if(ts.isCallExpression(n)||ts.isNewExpression(n))calls.add(n.expression.getText(sf));
  if(ts.isFunctionDeclaration(n)||ts.isArrowFunction(n)||ts.isFunctionExpression(n))functions++;
  if(ts.isJsxAttribute(n)&&/dangerouslySetInnerHTML|on[A-Z]/.test(n.name.getText(sf)))danger=true;
  ts.forEachChild(n,visit);
 }visit(sf);
 let category='';
 if(!functions&&!calls.size)category='声明、类型、资源或转导出；无本地执行函数';
 else if(file.startsWith('src/components/ui/')&&!danger&&[...calls].every(c=>['React.forwardRef','forwardRef','cn','cva','buttonVariants','badgeVariants','alertVariants','labelVariants','navigationMenuTriggerStyle','sheetVariants','toastVariants','toggleVariants'].includes(c)))category='UI基本封装；调用仅ref转发与样式组合，事件通过props交由使用方';
 if(category)result.push({file,category,calls:[...calls],method:'AST结构核对；不冒充人工逐行或视觉测试'});
}
fs.writeFileSync(base+'structural-coverage.json',JSON.stringify(result,null,2));
fs.writeFileSync(base+'STRUCTURAL.md','# 声明与UI基本封装检查\n\n完整AST遍历确认下面文件的本地调用类别。声明/类型/字典不含独立执行函数；UI封装仅调用ref转发、样式组合和声明式子组件，实际业务事件由调用方传入。该检查覆盖结构和副作用入口，不等同于每个像素或可访问性状态都已人工验证。包含自有状态、effect、数据转换或HTML注入的组件不自动归类，另行审读。\n\n| 文件 | 类别 | 调用集合 |\n|---|---|---|\n'+result.map(r=>`| [${r.file}](../../${r.file}) | ${r.category} | ${r.calls.join(', ')} |`).join('\n')+'\n');
console.log({structurallyReviewed:result.length});
