import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

const exports = {};
vm.runInNewContext(ts.transpileModule(fs.readFileSync('src/lib/history-sort.ts', 'utf8'), {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS },
}).outputText, { exports, Date });
const { sortHistory, normalizeHistorySort } = exports;
const ids = (items) => Array.from(items, (item) => item.id);
const order = (id, orderStatus, purchaseTime, patch = {}) => ({ id, orderStatus, purchaseTime, status: 'success', ...patch });

test('only paid delivering/delivered orders are pinned, with newest first within each group', () => {
  const items = [
    order('unpaid-new', 'notPaid', '2026-11-18T12:00:00Z'),
    order('delivering-old', 'delivering', '2026-11-16T12:00:00Z'),
    order('delivered-new', 'delivered', '2026-11-17T12:00:00Z'),
    order('checking', 'checking', '2026-11-18T13:00:00Z'),
    order('retracting', 'delivered', '2026-11-18T14:00:00Z', { retractionState: 'submitted' }),
    order('failed', 'delivered', '2026-11-18T15:00:00Z', { status: 'failed' }),
  ];
  const before = ids(items);
  assert.deepEqual(ids(sortHistory(items, 'paid-first')), ['delivered-new', 'delivering-old', 'failed', 'retracting', 'checking', 'unpaid-new']);
  assert.deepEqual(ids(items), before, 'sorting must not mutate the shared query cache');
});

test('chronological choices remove status priority, compare timezones correctly and keep unknown times last', () => {
  const items = [
    order('paid-old', 'delivered', '2026-11-16T12:00:00Z'),
    order('new', 'notPaid', '2026-11-18T12:00:00Z'),
    order('middle', 'notPaid', '2026-11-18T19:00:00+08:00'),
    order('unknown', 'notPaid', 'invalid'),
  ];
  assert.deepEqual(ids(sortHistory(items, 'newest')), ['new', 'middle', 'paid-old', 'unknown']);
  assert.deepEqual(ids(sortHistory(items, 'oldest')), ['paid-old', 'middle', 'new', 'unknown']);
  for (const value of [null, undefined, '', 'stale-setting', 'paid-first']) assert.equal(normalizeHistorySort(value), 'paid-first');
  assert.equal(normalizeHistorySort('newest'), 'newest');
  assert.equal(normalizeHistorySort('oldest'), 'oldest');
});
