import fs from 'node:fs';
import vm from 'node:vm';
import ts from 'typescript';

export function loadHook(file, api) {
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
    throw Error(`Unexpected import ${id}`);
  };
  vm.runInNewContext(js, { exports, require, console }, { filename: file });
  return exports;
}

