import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

function load(file, imports = {}) {
  const exports = {};
  const compiled = ts.transpileModule(fs.readFileSync(file, 'utf8'), {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  vm.runInNewContext(compiled, { exports, require(id) {
    if (!(id in imports)) throw Error(`Unexpected import: ${id}`);
    return imports[id];
  } }, { filename: file });
  return exports;
}
const helpers = load('src/lib/bulk-cleanup.ts');
const dragHelpers = load('src/lib/drag-selection.ts');

test('cleanup targets distinguish exact local IDs, accounts and same-model configurations, including all configurations', () => {
  const history = helpers.historyCleanupTarget({ id: 'id /?&', accountId: 'a', orderId: '123', planCode: 'p', datacenter: 'gra' });
  assert.equal(history.path, '/purchase-history/id%20%2F%3F%26');
  assert.equal(helpers.queueCleanupTarget({ id: 'task /1' }).path, '/queue/task%20%2F1');
  const all = helpers.monitorCleanupTarget({ planCode: 'same/model', options: [] });
  const ram = helpers.monitorCleanupTarget({ planCode: 'same/model', options: ['ram', 'bandwidth'] });
  assert.equal(all.params.optionsKey, '', 'empty optionsKey must be sent explicitly');
  assert.equal(ram.params.optionsKey, 'bandwidth,ram');
  assert.notEqual(all.key, ram.key);
  assert.equal(all.path, '/monitor/subscriptions/same%2Fmodel');
  assert.match(helpers.cleanupDescriptions.history, /不会取消 OVH 订单/);
});

test('batch cleanup limits concurrency, reports partial failures and never deletes an unselected target', async () => {
  const targets = Array.from({ length: 13 }, (_, i) => ({ key: `id-${i}`, path: `/queue/id-${i}` }));
  let active = 0, max = 0;
  const requested = [];
  const result = await helpers.cleanupTargets([...targets, targets[0]], async (target) => {
    requested.push(target.key);
    active++;
    max = Math.max(max, active);
    await new Promise((resolve) => setImmediate(resolve));
    active--;
    if (target.key === 'id-2') throw { response: { status: 500 } };
    if (target.key === 'id-3') throw { response: { status: 404 } };
  });
  assert.equal(max, 4);
  assert.equal(requested.length, 13);
  assert.equal(result.succeeded.length, 12);
  assert.equal(result.failed.length, 1);
  assert.equal(result.failed[0].target.key, 'id-2');
  assert.ok(requested.every((id) => targets.some((target) => target.key === id)));
});

function hookHarness() {
  let cursor = 0, pending = false, options, promise;
  const values = [], effects = [], requests = [], invalidated = [];
  const failures = new Set();
  const hook = load('src/hooks/use-bulk-cleanup.ts', {
    react: {
      useMemo: (factory) => factory(),
      useEffect: (effect) => effects.push(effect),
      useState: (initial) => {
        const i = cursor++;
        if (!(i in values)) values[i] = typeof initial === 'function' ? initial() : initial;
        return [values[i], (next) => { values[i] = typeof next === 'function' ? next(values[i]) : next; }];
      },
    },
    '@tanstack/react-query': {
      useQueryClient: () => ({ invalidateQueries: ({ queryKey }) => invalidated.push(queryKey) }),
      useMutation: (config) => {
        options = config;
        return { isPending: pending, mutate(snapshot) {
          pending = true;
          promise = (async () => {
            try { config.onSuccess(await config.mutationFn(snapshot)); }
            finally { pending = false; config.onSettled(); }
          })();
        } };
      },
    },
    '@/lib/http': { api: { delete: async (path, config) => {
      requests.push({ path, config });
      if (failures.has(path)) throw { response: { status: 500 } };
      return {};
    } } },
    '@/lib/api-error': { apiMessage: () => 'offline failure' },
    '@/lib/bulk-cleanup': helpers,
    '@/lib/drag-selection': dragHelpers,
    sonner: { toast: { success() {}, error() {} } },
  });
  let items = [{ id: 'a' }, { id: 'b' }, { id: 'c' }];
  const getTarget = (item) => ({ key: item.id, label: item.id, path: `/queue/${item.id}` });
  return {
    requests, invalidated, failures,
    setItems(next) { items = next; },
    render() { cursor = 0; const state = hook.useBulkCleanup(items, getTarget, 'queue'); while (effects.length) effects.shift()(); return state; },
    finish: () => promise,
    options: () => options,
  };
}

test('single/multiple selection and select-all apply only to visible rows; new polled rows never become selected automatically', () => {
  const harness = hookHarness();
  let state = harness.render();
  state.toggle({ id: 'a' });
  state = harness.render();
  assert.equal(state.selectedCount, 1);
  assert.equal(state.someSelected, true);
  state.toggle({ id: 'b' });
  assert.equal(harness.render().selectedCount, 2);
  harness.setItems([{ id: 'b' }, { id: 'c' }]);
  state = harness.render();
  assert.equal(state.selectedCount, 1, 'hidden a cannot be cleaned');
  state.toggleAll();
  state = harness.render();
  assert.equal(state.selectedCount, 2);
  assert.equal(state.allSelected, true);
  harness.setItems([{ id: 'b' }, { id: 'c' }, { id: 'new-order' }]);
  state = harness.render();
  assert.equal(state.selectedCount, 2);
  assert.equal(state.allSelected, false);
  harness.setItems([{ id: 'a' }, { id: 'b' }, { id: 'c' }]);
  state = harness.render();
  assert.equal(state.isSelected({ id: 'a' }), false, 'returning to an old filter does not restore hidden selection');
  state.clearSelection();
  assert.equal(harness.render().selectedCount, 0);
  assert.equal(harness.requests.length, 0);
});

test('confirmation freezes exact selections across polling and disables duplicate submissions', async () => {
  const harness = hookHarness();
  let state = harness.render();
  state.toggle({ id: 'a' });
  state = harness.render();
  state.requestCleanup();
  state = harness.render();
  assert.equal(state.dialogProps.targets.length, 1);
  assert.equal(harness.requests.length, 0, 'opening confirmation is read-only');
  harness.setItems([{ id: 'b' }, { id: 'new-order' }]);
  state = harness.render();
  assert.equal(state.locked, true);
  state.toggleAll();
  state.dialogProps.onConfirm();
  assert.equal(harness.render().busy, true);
  harness.render().dialogProps.onConfirm();
  await harness.finish();
  assert.equal(harness.requests.length, 1);
  assert.equal(harness.requests[0].path, '/queue/a');
  assert.equal(harness.render().dialogProps.targets, null);
  assert.equal(harness.options().retry, false);
  assert.ok(harness.invalidated.some((key) => key[0] === 'queue'));
  assert.ok(harness.invalidated.some((key) => key[0] === 'stats'));
});

test('partial failure keeps only failed items selected and retry does not resend successful deletions', async () => {
  const harness = hookHarness();
  harness.failures.add('/queue/b');
  let state = harness.render();
  state.toggleAll();
  state = harness.render();
  state.requestCleanup();
  harness.render().dialogProps.onConfirm();
  await harness.finish();
  state = harness.render();
  assert.equal(state.selectedCount, 1);
  assert.equal(state.dialogProps.targets.length, 1);
  assert.equal(state.dialogProps.targets[0].key, 'b');
  assert.match(state.dialogProps.failureMessage, /1 项清理失败/);
  harness.failures.clear();
  state.dialogProps.onConfirm();
  await harness.finish();
  assert.equal(harness.requests.filter(({ path }) => path === '/queue/a').length, 1);
  assert.equal(harness.requests.filter(({ path }) => path === '/queue/b').length, 2);
  assert.equal(harness.requests.filter(({ path }) => path === '/queue/c').length, 1);
  assert.equal(harness.render().selectedCount, 0);
});

test('shift-click range selection, invertSelection, and deletion progress reporting work across visible rows', async () => {
  const progressEvents = [];
  const res = await helpers.cleanupTargets(
    [{ key: '1', path: '/queue/1' }, { key: '2', path: '/queue/2' }],
    async () => {},
    (done, total) => progressEvents.push([done, total]),
  );
  assert.equal(res.succeeded.length, 2);
  assert.deepEqual(progressEvents[0], [0, 2]);
  assert.deepEqual(progressEvents[progressEvents.length - 1], [2, 2]);

  const harness = hookHarness();
  harness.setItems([{ id: 'a' }, { id: 'b' }, { id: 'c' }, { id: 'd' }]);
  let state = harness.render();
  state.toggle({ id: 'a' }, false);
  state = harness.render();
  state.toggle({ id: 'c' }, true);
  state = harness.render();
  assert.deepEqual(state.selectedKeys, ['a', 'b', 'c']);
  state.invertSelection();
  state = harness.render();
  assert.deepEqual(state.selectedKeys, ['d']);
  state.selectAll();
  state = harness.render();
  assert.deepEqual(state.selectedKeys, ['a', 'b', 'c', 'd']);
});
