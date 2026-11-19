import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

function load(file) {
  const exports = {};
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS },
  }).outputText;
  vm.runInNewContext(js, { exports, Date }, { filename: file });
  return exports;
}
const { sortServersForSniping, isNewPlan, planYear } = load('src/lib/server-sort.ts');

const codes = list => Array.from(list, s => s.planCode);

test('预订置顶,正式销售按首次上架时间与年代新的在上', () => {
  const list = [
    { planCode: '24ska01', firstSeen: 0 },
    { planCode: '25skle01', firstSeen: 0 },
    { planCode: '26sk50b-v1', firstSeen: 0 },
    { planCode: '24sk202', firstSeen: 1_800_000_000 }, // 记录开始后才上架,即便代码年代老也排最前
    { planCode: '27skle03', prerelease: true },
    { planCode: '26sk10b-v1', firstSeen: 0 },
  ];
  assert.deepEqual(codes(sortServersForSniping(list)),
    ['27skle03', '24sk202', '26sk50b-v1', '26sk10b-v1', '25skle01', '24ska01']);
});

test('按价格排序时预订仍置顶,其余按价格', () => {
  const price = { a: 30, b: 10, c: 20 };
  const list = [
    { planCode: '24a' }, { planCode: '24b' }, { planCode: '26pre', prerelease: true }, { planCode: '24c' },
  ];
  const cmp = (x, y) => price[x.planCode.slice(2)] - price[y.planCode.slice(2)];
  assert.deepEqual(codes(sortServersForSniping(list, cmp)), ['26pre', '24b', '24c', '24a']);
});

test('不改动原数组;新上架只认 14 天内且有记录时间的', () => {
  const list = [{ planCode: '24a' }, { planCode: '26b' }];
  sortServersForSniping(list);
  assert.deepEqual(codes(list), ['24a', '26b']);
  const now = 1_800_000_000_000;
  assert.equal(isNewPlan(0, now), false);
  assert.equal(isNewPlan(undefined, now), false);
  assert.equal(isNewPlan(now / 1000 - 3 * 86400, now), true);
  assert.equal(isNewPlan(now / 1000 - 20 * 86400, now), false);
  assert.equal(planYear('26sk50b-v1'), 2026);
  assert.equal(planYear('kimsufi'), 0);
});
