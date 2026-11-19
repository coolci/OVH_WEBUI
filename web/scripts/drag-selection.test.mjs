import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';
const exports = {};
vm.runInNewContext(ts.transpileModule(fs.readFileSync('src/lib/drag-selection.ts','utf8'), {
  compilerOptions:{target:ts.ScriptTarget.ES2020,module:ts.ModuleKind.CommonJS},
}).outputText,{exports});
const {selectionRect,intersectsSelection,mergeDragSelection} = exports;

test('marquee selects intersecting rows in either drag direction and excludes adjacent rows',()=>{
  const start={x:200,y:180},end={x:20,y:40};
  const rows=[{left:0,right:250,top:0,bottom:40},{left:0,right:250,top:40,bottom:100},{left:0,right:250,top:110,bottom:170},{left:0,right:250,top:180,bottom:240}];
  for(const rect of [selectionRect(start,end),selectionRect(end,start)]){
    assert.deepEqual(rows.map(row=>intersectsSelection(rect,row)),[false,true,true,false]);
  }
});

test('normal drag replaces selection; modifier drag adds exact identities without duplicates',()=>{
  assert.deepEqual(Array.from(mergeDragSelection(['row-2','row-3','row-2'],['row-1'],false)),['row-2','row-3']);
  assert.deepEqual(Array.from(mergeDragSelection(['row-2','row-3'],['row-1','row-2'],true)),['row-1','row-2','row-3']);
  assert.deepEqual(Array.from(mergeDragSelection([],['row-1'],false)),[]);
  assert.deepEqual(Array.from(mergeDragSelection([],['row-1'],true)),['row-1']);
});
