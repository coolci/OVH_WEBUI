import test from 'node:test';
import assert from 'node:assert/strict';
import { loadHook } from './test-hook-loader.mjs';

const base = {serviceName:'fixture', templateName:'debian12_64', keepDiskGroups:[2]};
const partition = {diskGroupId:1, mountpoint:'/', filesystem:'ext4', size:0};
for (const [name, patch] of [
  ['keep only', {}],
  ['default install target', {customPartitions:[{...partition,diskGroupId:undefined}]}],
  ['same install and preserved target', {customPartitions:[{...partition,diskGroupId:2}]}],
  ['duplicate preserved target', {customPartitions:[partition],keepDiskGroups:[2,2]}],
  ['ZFS overrides preservation', {customPartitions:[partition],useProxmox9Zfs:true}],
]) test(`SS-04 rejects ${name} before any API call`, async () => {
  let calls=0;
  const hook=loadHook('src/hooks/ovh/server-control/use-server-install.ts',{post:async()=>{calls++;return {data:{}};}});
  await assert.rejects(hook.useReinstallServer().mutationFn({...base,...patch}));
  assert.equal(calls,0);
});
test('SS-04 explicit installation and preserved data reach the request',async()=>{
  let payload;
  const hook=loadHook('src/hooks/ovh/server-control/use-server-install.ts',{post:async(path,body)=>{payload=body;return {data:{}};}});
  await hook.useReinstallServer().mutationFn({...base,customPartitions:[partition]});
  assert.deepEqual(JSON.parse(JSON.stringify(payload.storageConfig)),[
    {diskGroupId:1,partitioning:{layout:[{mountPoint:'/',fileSystem:'ext4',size:0}]}},
    {diskGroupId:2,erase:false},
  ]);
});
test('ordinary default reinstall retains its original request',async()=>{
  let payload;
  const hook=loadHook('src/hooks/ovh/server-control/use-server-install.ts',{post:async(path,body)=>{payload=body;return {data:{}};}});
  await hook.useReinstallServer().mutationFn({...base,keepDiskGroups:[]});
  assert.equal(payload.storageConfig,undefined);
});
