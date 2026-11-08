import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

function load(file, imports = {}) {
  const exports = {};
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS },
  }).outputText;
  vm.runInNewContext(js, { exports, require: id => {
    if (!(id in imports)) throw Error(`Unexpected import ${id}`);
    return imports[id];
  } }, { filename: file });
  return exports;
}
const availability = load('src/lib/availability.ts');
const { selectionDcStatus, stockOnlyParts } = load('src/lib/variant-stock.ts', { '@/lib/availability': availability });
const plain = v => JSON.parse(JSON.stringify(v));

// 2026-11-09 实测 24sk50-v1:有货的只有 2×4T HDD(BHS 32G、GRA/WAW 64G),
// 而 IE 销售目录的硬盘选项里根本没有 2×4T。
const plan = '24sk50-v1';
const v = (mem, disk, ok) => ({
  planCode: plan,
  fqn: `${plan}.${mem}.${disk}`,
  datacenters: ['gra', 'bhs', 'waw', 'rbx'].map(dc => ({ datacenter: dc, availability: ok.includes(dc) ? '1H-low' : 'unavailable' })),
});
const variants = [
  v('ram-32g-ecc-2400', 'softraid-2x1200nvme', []),
  v('ram-32g-ecc-2400', 'softraid-2x4000sa', ['bhs']),
  v('ram-64g-ecc-2400', 'softraid-2x1200nvme', []),
  v('ram-64g-ecc-2400', 'softraid-2x4000sa', ['gra', 'waw']),
];
const planLevel = { gra: '1H-low', bhs: '1H-low', waw: '1H-low', rbx: 'unavailable' };

test('机房按所选配置显示:选 32G + 2×1.2T NVMe 时全线缺货', () => {
  const sel = plain(selectionDcStatus(planLevel, variants, ['ram-32g-ecc-2400-24sk50-v1', 'softraid-2x1200nvme-24sk50-v1']));
  assert.deepEqual(sel, { gra: 'unavailable', bhs: 'unavailable', waw: 'unavailable', rbx: 'unavailable' });
});

test('所选配置有货的机房照常显示有货', () => {
  const sel = plain(selectionDcStatus(planLevel, variants, ['ram-64g-ecc-2400-24sk50-v1', 'softraid-2x4000sa-24sk50-v1']));
  assert.equal(sel.gra, '1H-low');
  assert.equal(sel.waw, '1H-low');
  assert.equal(sel.bhs, 'unavailable');
});

test('只选了内存时,任一含该内存的配置有货即可', () => {
  const sel = plain(selectionDcStatus(planLevel, variants, ['ram-32g-ecc-2400-24sk50-v1']));
  assert.equal(sel.bhs, '1H-low');
  assert.equal(sel.gra, 'unavailable');
});

test('找出有货但目录里选不到的配置段', () => {
  const catalog = ['ram-32g-ecc-2400-24sk50-v1', 'ram-64g-ecc-2400-24sk50-v1', 'softraid-3x4000sa-24sk50-v1',
    'softraid-2x2000sa-24sk50-v1', 'hardraid-3x960ssd-24sk50-v1', 'softraid-2x450nvme-24sk50-v1', 'softraid-2x1200nvme-24sk50-v1'];
  assert.deepEqual(plain(stockOnlyParts(variants, catalog)), ['softraid-2x4000sa']);
  assert.deepEqual(plain(stockOnlyParts(variants, [...catalog, 'softraid-2x4000sa-24sk50-v1'])), []);
});
