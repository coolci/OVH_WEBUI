// Offline regression tests for corrected behavior.
// Run from repository root: node --test docs/audit-2026-11-06/server-support-proof.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

function loadHook(file, api) {
  const source = fs.readFileSync(file, 'utf8');
  const js = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText;
  const exports = {};
  const keys = new Proxy({}, { get: (_, resource) => new Proxy({}, { get: (_, method) => (...args) => [resource, method, ...args] }) });
  const require = (id) => {
    if (id === '@tanstack/react-query') return { useQuery: x => x, useMutation: x => x, useQueryClient: () => ({ invalidateQueries() {} }) };
    if (id === '@/lib/http') return { api };
    if (id === '@/lib/query') return { qk: keys };
    if (id.endsWith('use-active-account')) return { useActiveAccount: () => ['fixture'], useActiveVpsControlAccount: () => ['fixture'] };
    if (id.endsWith('use-accounts')) return { useDefaultAccount: () => ({id: 'fixture'}) };
    if (id === '@/i18n') return { default: { t: x => x } };
    // 纯函数模块,用真实实现
    if (id.endsWith('/partial-list')) return loadHook('src/hooks/ovh/partial-list.ts', api);
    throw Error(`Unexpected import ${id}`);
  };
  vm.runInNewContext(js, { exports, require, console }, { filename: file });
  return exports;
}

test('SS-01: snapshot rollback hook sends required confirmation body', async () => {
  let sent;
  const hooks = loadHook('src/hooks/ovh/use-vps-control.ts', { post: async (...args) => { sent = args; return {data: {}}; } });
  await hooks.useRevertVpsSnapshot('vps-fixture').mutationFn();
  assert.equal(sent[0], '/vps-control/vps-fixture/snapshot/revert');
  assert.equal(sent[1].confirm, true);
});

test('SS-02: owned-server query preserves all existing machines', async () => {
  const fixture = [
    { serviceName: 'healthy', state: 'ok', status: 'ok' },
    { serviceName: 'faulted', state: 'error', status: 'ok' },
    { serviceName: 'suspended', state: 'ok', status: 'suspended' },
    { serviceName: 'expired', state: 'ok', status: 'expired' },
    { serviceName: 'pending', state: 'installing', status: 'ok' },
  ];
  const hooks = loadHook('src/hooks/ovh/server-control/use-server-core.ts', { get: async () => ({ data: {success: true, servers: fixture} }) });
  const result = await hooks.useOwnedServers().queryFn();
  assert.deepEqual(Array.from(result, x => x.serviceName), fixture.map(x => x.serviceName));
});

test('SS-03: account lists distinguish failure, partial results and empty records', async () => {
  for (const [hook, body, headers, failed] of [
    ['useOrders', {orders: [], total: 2, failed: 2}, {}, true],
    ['useOrders', {orders: [{orderId: 1}], total: 2, failed: 1}, {}, false],
    ['useEmails', [], {'x-partial-failures':'2'}, true],
    ['useEmails', [{id:1}], {'x-partial-failures':'1'}, false],
    ['useRefunds', [], {}, false],
  ]) {
    const hooks = loadHook('src/hooks/ovh/use-account.ts', {get: async () => ({data:body,headers})});
    if (failed) await assert.rejects(hooks[hook]().queryFn(), /读取失败/);
    else {
      const result = await hooks[hook]().queryFn();
      assert.equal(result.failed, hook === 'useRefunds' ? 0 : 1);
      assert.equal(result.items.length, hook === 'useRefunds' ? 0 : 1);
    }
  }
});

test('SS-05: rDNS dialog preserves selected secondary IP', async () => {
  const file = 'src/components/server-control/ReverseDnsDialog.tsx';
  const js = ts.transpileModule(fs.readFileSync(file, 'utf8'), { compilerOptions: {
    module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX,
  }}).outputText;
  const state = [];
  let cursor = 0, sent, postPath;
  const networkHooks = loadHook('src/hooks/ovh/server-control/use-server-network.ts', {
    post: async (path, payload) => { postPath = path; sent = payload; return {data: {}}; },
  });
  const exports = {};
  const require = id => {
    if (id === 'react') return { useEffect() {}, useState(initial) {
      const i = cursor++;
      if (!(i in state)) state[i] = initial;
      return [state[i], next => { state[i] = typeof next === 'function' ? next(state[i]) : next; }];
    }};
    if (id === 'react/jsx-runtime') return { jsx: (type, props) => ({type, props}), jsxs: (type, props) => ({type, props}) };
    if (id === '@/hooks/use-server-control') return {
      useServerReverseDns: () => ({data: [], isPending: false}),
      useServerIps: () => ({data: [{ip:'192.0.2.1'}, {ip:'192.0.2.2'}]}),
      useSetServerReverseDns: serviceName => ({mutateAsync: networkHooks.useSetServerReverseDns(serviceName).mutationFn}),
      useDeleteServerReverseDns: () => ({}),
    };
    if (id === 'sonner') return {toast: {success() {}, error() {}}};
    if (id === '@/lib/api-error') return {errorMessage: String};
    if (id.startsWith('@/components/') || id === 'lucide-react') return new Proxy({}, {get: (_, name) => name});
    throw Error(`Unexpected import ${id}`);
  };
  vm.runInNewContext(js, {exports, require}, {filename: file});
  const render = (open, defaultIp) => {
    cursor = 0;
    return exports.ReverseDnsDialog({serviceName:'ns-fixture', open, defaultIp, onOpenChange() {}});
  };
  const nodes = root => !root || typeof root !== 'object' ? [] : Array.isArray(root)
    ? root.flatMap(nodes) : [root, ...nodes(root.props?.children)];
  render(false, ''); // The parent keeps this dialog mounted before choosing an IP.
  let tree = render(true, '192.0.2.2');
  const create = nodes(tree).find(n => n.type === 'Button' &&
    Array.isArray(n.props.children) && n.props.children.includes('设置反向 DNS'));
  assert.ok(create, 'set-rDNS button is present');
  create.props.onClick();
  tree = render(true, '192.0.2.2');
  nodes(tree).find(n => n.type === 'Input').props.onChange({target:{value:'secondary.example.test'}});
  tree = render(true, '192.0.2.2');
  const save = nodes(tree).find(n => n.type === 'Button' && Array.isArray(n.props.children) && n.props.children.includes('保存反向解析'));
  await save.props.onClick();
  assert.equal(postPath, '/server-control/ns-fixture/reverse');
  assert.equal(sent.ip, '192.0.2.2');
  assert.equal(sent.reverse, 'secondary.example.test');
});