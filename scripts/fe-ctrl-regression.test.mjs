import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { createRequire } from "node:module";
import { loadHook } from "./test-hook-loader.mjs";

const repoRoot = process.cwd();
const nodeRequire = createRequire(path.join(repoRoot, "package.json"));
const ts = nodeRequire("typescript");

function defaultReactQueryMock() {
  return {
    QueryClient: class QueryClient {
      invalidateQueries() {}
      removeQueries() {}
    },
    queryOptions: (x) => x,
    useQuery: (x) => x,
    useQueries: (x) => x,
    useMutation: (x) => x,
    useQueryClient: () => ({
      invalidateQueries() {},
      removeQueries() {},
    }),
  };
}

/**
 * 轻量同域内存 TS/TSX 模块加载器（支持 @/ 别名与可选外部依赖桩），
 * 使用 vm.runInThisContext 保持与 Node 断言一致的原生对象原型链。
 */
function createModuleLoader(overrides = {}) {
  const cache = new Map();

  function loadModule(relOrAbsPath) {
    const absPath = path.isAbsolute(relOrAbsPath)
      ? relOrAbsPath
      : path.join(repoRoot, relOrAbsPath);
    if (cache.has(absPath)) return cache.get(absPath).exports;

    const source = fs.readFileSync(absPath, "utf8");
    const transpiled = ts.transpileModule(source, {
      compilerOptions: {
        module: ts.ModuleKind.CommonJS,
        target: ts.ScriptTarget.ES2022,
        esModuleInterop: true,
        jsx: ts.JsxEmit.ReactJSX,
      },
      fileName: absPath,
    }).outputText;

    const mod = { exports: {} };
    cache.set(absPath, mod);

    function customRequire(spec) {
      if (spec in overrides) {
        return typeof overrides[spec] === "function" ? overrides[spec]() : overrides[spec];
      }
      let target = spec;
      if (spec.startsWith("@/")) {
        target = path.join(repoRoot, "src", spec.slice(2));
      } else if (spec.startsWith(".")) {
        target = path.resolve(path.dirname(absPath), spec);
      } else {
        return nodeRequire(spec);
      }
      const candidates = [
        target,
        `${target}.ts`,
        `${target}.tsx`,
        path.join(target, "index.ts"),
        path.join(target, "index.tsx"),
      ];
      const resolved = candidates.find((c) => fs.existsSync(c) && fs.statSync(c).isFile());
      if (!resolved) {
        throw new Error(`Cannot resolve ${spec} from ${absPath}`);
      }
      return loadModule(resolved);
    }

    const fn = vm.runInThisContext(
      `(function(exports, require, module, __filename, __dirname) { ${transpiled}\n})`,
      { filename: absPath },
    );
    fn(mod.exports, customRequire, mod, absPath, path.dirname(absPath));
    return mod.exports;
  }

  return { loadModule };
}

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-01: 单盘组独服自定义分区 + 硬件 RAID 合并（消除恒假死分支 & 防分裂重复 diskGroupId）
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-01: single-disk-group custom partitions + hardware RAID merge into one StorageGroup without duplicate diskGroupId", async () => {
  let postedBody = null;
  const installMod = loadHook("src/hooks/ovh/server-control/use-server-install.ts", {
    post: async (_url, body) => {
      postedBody = body;
      return { data: { success: true } };
    },
  });

  // 场景 A：PartitionRow 旧态或未指定 diskGroupId (undefined)，但单盘组独服 diskGroups={ 1: ... } 且配置硬件 RAID1
  await installMod.useReinstallServer().mutationFn({
    serviceName: "ns12345.ip-1-2-3.eu",
    templateName: "debian12_64",
    customPartitions: [
      { mountpoint: "/", filesystem: "ext4", size: 50000, order: 1, type: "primary", diskGroupId: undefined },
      { mountpoint: "/home", filesystem: "xfs", size: 0, order: 2, type: "primary", diskGroupId: undefined },
    ],
    hardwareRaid: { 1: "raid1" },
    diskGroups: {
      1: { id: 1, disks: [{ capacity: 960, unit: "GB" }, { capacity: 960, unit: "GB" }], isDefault: true },
    },
  });

  assert.ok(Array.isArray(postedBody.storageConfig), "storageConfig must be built");
  assert.equal(
    postedBody.storageConfig.length,
    1,
    "Single disk group MUST NOT split into 2 StorageGroup entries (400 STORAGE_DUPLICATE_GROUP)",
  );
  assert.equal(postedBody.storageConfig[0].diskGroupId, 1);
  assert.equal(postedBody.storageConfig[0].partitioning.layout.length, 2);
  assert.deepEqual(JSON.parse(JSON.stringify(postedBody.storageConfig[0].hardwareRaid)), [
    { raidLevel: 1, disks: 2 },
  ]);

  // 场景 B：显式绑定 diskGroupId=1 时结果完全一致
  const builtExplicit = installMod.buildInstallStorageConfig({
    serviceName: "ns12345.ip-1-2-3.eu",
    templateName: "debian12_64",
    defaultDiskGroupId: 1,
    customPartitions: [
      { mountpoint: "/", filesystem: "ext4", size: 50000, order: 1, type: "primary", diskGroupId: 1 },
    ],
    hardwareRaid: { 1: "raid1" },
    diskGroups: {
      1: { id: 1, disks: [{ capacity: 960, unit: "GB" }, { capacity: 960, unit: "GB" }] },
    },
  });
  assert.equal(builtExplicit.length, 1);
  assert.equal(builtExplicit[0].diskGroupId, 1);
  assert.ok(builtExplicit[0].partitioning);
  assert.ok(builtExplicit[0].hardwareRaid);
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-02: 硬件 RAID 热备盘扣除（disks = totalDisks - spares）、多盘组隔离与 RAID50/60 arrays 透传
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-02: hardware RAID deducts hot spares from physical disks, isolates per-group spares/arrays, and passes arrays for raid50/raid60", () => {
  const installMod = loadHook("src/hooks/ovh/server-control/use-server-install.ts", {});
  const { loadModule } = createModuleLoader();
  const { HARDWARE_RAID_LEVELS } = loadModule("src/components/server-control/reinstall/constants.ts");

  // UI 常量包含 raid50 / raid60
  const raidValues = HARDWARE_RAID_LEVELS.map((r) => r.value);
  assert.ok(raidValues.includes("raid50"), "HARDWARE_RAID_LEVELS must include raid50");
  assert.ok(raidValues.includes("raid60"), "HARDWARE_RAID_LEVELS must include raid60");

  // 1) 4 块物理盘配置 RAID5 + 1 块热备盘 -> disks=3, spares=1, disks + spares === 4 <= 4
  const multiGroupCfg = installMod.buildInstallStorageConfig({
    serviceName: "ns-raid.eu",
    templateName: "debian12_64",
    diskGroups: {
      1: { id: 1, disks: [{}, {}, {}, {}], isDefault: true },
      2: { id: 2, disks: [{}, {}, {}, {}, {}, {}, {}, {}] },
    },
    hardwareRaid: { 1: "raid5", 2: "raid60" },
    // 按盘组独立隔离：组 1 配 1 块热备盘，组 2 配 0 块热备盘 + 2 个子阵列 (RAID 60)
    hwRaidSpares: { 1: 1, 2: 0 },
    hwRaidArrays: { 2: 2 },
  });

  assert.equal(multiGroupCfg.length, 2);
  const g1 = multiGroupCfg.find((g) => g.diskGroupId === 1);
  const g2 = multiGroupCfg.find((g) => g.diskGroupId === 2);
  assert.deepEqual(JSON.parse(JSON.stringify(g1.hardwareRaid[0])), {
    raidLevel: 5,
    disks: 3,
    spares: 1,
  });
  assert.equal(g1.hardwareRaid[0].disks + g1.hardwareRaid[0].spares, 4);

  // 组 2：RAID60 透传 arrays=2，spares=0 不污染，disks=8
  assert.deepEqual(JSON.parse(JSON.stringify(g2.hardwareRaid[0])), {
    raidLevel: 60,
    disks: 8,
    arrays: 2,
  });

  // RAID50 同样透传 arrays（7 盘扣 1 热备 = 6 成员盘 = 2 阵列 × 3 盘/阵列）
  const raid50Cfg = installMod.buildInstallStorageConfig({
    serviceName: "ns-raid50.eu",
    templateName: "debian12_64",
    diskGroups: { 1: { id: 1, disks: [{}, {}, {}, {}, {}, {}, {}] } },
    hardwareRaid: { 1: "raid50" },
    hwRaidSpares: { 1: 1 },
    hwRaidArrays: { 1: 2 },
  });
  assert.deepEqual(JSON.parse(JSON.stringify(raid50Cfg[0].hardwareRaid[0])), {
    raidLevel: 50,
    disks: 6,
    spares: 1,
    arrays: 2,
  });

  // 2) 越界拦截：2 盘 RAID1 配 1 热备盘只剩 1 块成员盘 (< 2)，必须在前端直接拦截报错
  assert.throws(() =>
    installMod.buildInstallStorageConfig({
      serviceName: "ns-overflow.eu",
      templateName: "debian12_64",
      diskGroups: { 1: { id: 1, disks: [{}, {}] } },
      hardwareRaid: { 1: "raid1" },
      hwRaidSpares: { 1: 1 },
    }),
  );
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-03: 跨模板定制问答隔离、number 类型转换、mandatory 必填拦截与 confirming 二次确认指纹复位
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-03: normalizeCustomizations isolates template questions, coerces numbers, enforces mandatory fields, and confirming fingerprint tracks storage mutations", () => {
  const installMod = loadHook("src/hooks/ovh/server-control/use-server-install.ts", {});
  const { loadModule } = createModuleLoader();
  const { buildReinstallConfirmFingerprint } = loadModule(
    "src/components/server-control/reinstall/reinstall-helpers.ts",
  );

  const questions = [
    { name: "sshPort", type: "number", description: "SSH Port", mandatory: true },
    { name: "adminUser", type: "string", description: "Admin Username", required: true },
    { name: "notes", type: "string", description: "Optional notes" },
  ];

  // 1) 必填项缺失时 missingMandatory 非空拦截提交
  const missingMandatory = installMod.normalizeCustomizations(questions, {
    sshPort: "2222",
    adminUser: "   ",
    staleProxmoxField: "leak",
  });
  assert.equal(missingMandatory.missingMandatory.length, 1);
  assert.equal(missingMandatory.missingMandatory[0], "Admin Username");

  // 2) 正常填写时：剔除跨模板残留字段 staleProxmoxField，且将 sshPort 从字符串 "2222" 转为 JS number 2222
  const validNorm = installMod.normalizeCustomizations(questions, {
    sshPort: "2222",
    adminUser: "debian",
    staleProxmoxField: "leak",
    notes: "",
  });
  assert.equal(validNorm.missingMandatory.length, 0);
  assert.equal(validNorm.invalidNumbers.length, 0);
  assert.deepEqual(JSON.parse(JSON.stringify(validNorm.value)), {
    sshPort: 2222,
    adminUser: "debian",
  });
  assert.equal(typeof validNorm.value.sshPort, "number");
  assert.equal("staleProxmoxField" in validNorm.value, false);

  // 3) 非法数值拦截
  const badNum = installMod.normalizeCustomizations(questions, {
    sshPort: "not-a-port",
    adminUser: "debian",
  });
  assert.equal(badNum.invalidNumbers.length, 1);

  // 4) confirming 二次确认指纹：修改 keepDiskGroups、hardwareRaid、hwRaidSpares、hwRaidArrays、customPartitions 均产生新指纹，触发 confirming 自动复位
  const baseState = {
    templateName: "debian12_64",
    storageMode: "custom",
    partitionSchemeName: "",
    zfsRaidLevel: 1,
    zfsVzSize: 0,
    keepDiskGroups: new Set([2]),
    hardwareRaid: { 1: "raid1" },
    hwRaidArrays: {},
    hwRaidSpares: { 1: "0" },
    useSoftwareRaid: false,
    softwareRaidLevel: "1",
    softwareRaidDisks: "2",
    customPartitions: [{ mountpoint: "/", filesystem: "ext4", size: 50000, diskGroupId: 1 }],
    hostname: "srv1",
    customizationAnswers: { sshPort: 2222 },
  };
  const fpBase = buildReinstallConfirmFingerprint(baseState);
  assert.notEqual(
    fpBase,
    buildReinstallConfirmFingerprint({ ...baseState, keepDiskGroups: new Set([]) }),
    "Changing keepDiskGroups must reset confirming",
  );
  assert.notEqual(
    fpBase,
    buildReinstallConfirmFingerprint({ ...baseState, hardwareRaid: { 1: "raid5" } }),
    "Changing hardwareRaid must reset confirming",
  );
  assert.notEqual(
    fpBase,
    buildReinstallConfirmFingerprint({ ...baseState, hwRaidSpares: { 1: "1" } }),
    "Changing hwRaidSpares must reset confirming",
  );
  assert.notEqual(
    fpBase,
    buildReinstallConfirmFingerprint({ ...baseState, hwRaidArrays: { 1: "2" } }),
    "Changing hwRaidArrays must reset confirming",
  );
  assert.notEqual(
    fpBase,
    buildReinstallConfirmFingerprint({
      ...baseState,
      customPartitions: [{ mountpoint: "/", filesystem: "xfs", size: 80000, diskGroupId: 1 }],
    }),
    "Changing customPartitions must reset confirming",
  );
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-04: 磁盘容量单位换算（MB/Mo/MiB 除以 1024）与默认系统盘组精确识别
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-04: diskCapacityToGB converts MB/Mo/MiB accurately without 1024x inflation and pickDefaultDiskGroup honors defaultDiskGroupId", async () => {
  const { loadModule } = createModuleLoader({
    "@tanstack/react-query": defaultReactQueryMock(),
    "@/lib/http": {
      api: {
        get: async () => ({
          data: {
            hardware: { defaultDiskGroupId: 2 },
            diskGroups: {
              "1": { id: 1, description: "Data HDD", disks: [{ capacity: 4, unit: "TB" }] },
              "2": { id: 2, description: "OS NVMe", disks: [{ capacity: 480000, unit: "MB" }, { capacity: 480000, unit: "MB" }] },
            },
          },
        }),
      },
    },
    "@/hooks/use-active-account": {
      useActiveAccount: () => ["acc-eu"],
      useActiveServerControlAccount: () => ["acc-eu"],
      useActiveVpsControlAccount: () => ["acc-eu"],
    },
    "@/hooks/use-accounts": {
      useDefaultAccount: () => ({ id: "acc-eu" }),
    },
  });

  const installMod = loadModule("src/hooks/ovh/server-control/use-server-install.ts");
  const hwMod = loadModule("src/hooks/ovh/server-control/use-server-hardware.ts");

  // 2 × 480000 MB NVMe 必须换算为 937.5 GB（绝不能当成 GB 算出 960,000 GB）
  const perDiskGb = installMod.diskCapacityToGB(480000, "MB");
  assert.equal(perDiskGb, 468.75);
  assert.equal(perDiskGb * 2, 937.5);

  assert.equal(installMod.diskCapacityToGB(512000, "Mo"), 500);
  assert.equal(installMod.diskCapacityToGB(1024, "MiB"), 1);
  assert.equal(installMod.diskCapacityToGB(960, "GB"), 960);
  assert.equal(installMod.diskCapacityToGB(960, "Go"), 960);
  assert.equal(installMod.diskCapacityToGB(2, "TB"), 2048);
  assert.equal(installMod.diskCapacityToGB(2, "To"), 2048);
  assert.equal(installMod.diskCapacityToGB(1048576, "KB"), 1);

  // useServerDiskInfo 标记 hardware.defaultDiskGroupId (2) 为 isDefault: true，pickDefaultDiskGroup 优先选中盘组 2 而非 Object.values()[0]
  const diskInfoQ = hwMod.useServerDiskInfo("ns-multi.eu", true, "acc-eu");
  const diskMap = await diskInfoQ.queryFn();
  assert.equal(diskMap["1"].isDefault, false);
  assert.equal(diskMap["2"].isDefault, true);

  const chosen = installMod.pickDefaultDiskGroup(diskMap);
  assert.equal(chosen.id, 2);
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-05: VPS 重装无 SSH 密钥时硬禁用/双重拦截 doNotSendPassword，防 410 Gone 锁死新实例
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-05: VPS reinstall blocks doNotSendPassword without a valid OVH SSH key name in both UI helper and mutation guard", async () => {
  const { loadModule } = createModuleLoader();
  const { resolveVpsReinstallSshKey, canEnableDoNotSendPassword } = loadModule(
    "src/components/vps-control/vps-reinstall-helpers.ts",
  );

  // 无密钥、空自定义密钥、原始公钥文本、多密钥逗号拼接均不可开启免密
  assert.equal(canEnableDoNotSendPassword("__none__", ""), false);
  assert.equal(canEnableDoNotSendPassword("__custom__", "   "), false);
  assert.equal(
    canEnableDoNotSendPassword("__custom__", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockKey user@host"),
    false,
  );
  assert.equal(resolveVpsReinstallSshKey("__custom__", "key1,key2").error, "multiple_keys");
  assert.equal(canEnableDoNotSendPassword("__custom__", "key1,key2"), false);

  // 合法单密钥名称才允许开启免密
  assert.equal(canEnableDoNotSendPassword("my-prod-key", ""), true);
  assert.equal(canEnableDoNotSendPassword("__custom__", "deploy-key-eu"), true);

  // Hook 层硬门禁：即使绕过 UI 直接传 doNotSendPassword: true 且无有效 sshKey，也在发请求前抛错阻断
  let apiCalls = 0;
  let lastPayload = null;
  const vpsMod = loadHook("src/hooks/ovh/use-vps-control.ts", {
    post: async (_url, body) => {
      apiCalls++;
      lastPayload = body;
      return { data: { success: true } };
    },
  });

  await assert.rejects(
    vpsMod.useReinstallVps("vps-123.ovh.net", "acc-1").mutationFn({
      templateId: "debian-12",
      doNotSendPassword: true,
    }),
  );
  await assert.rejects(
    vpsMod.useReinstallVps("vps-123.ovh.net", "acc-1").mutationFn({
      templateId: "debian-12",
      sshKey: ["ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQ..."],
      doNotSendPassword: true,
    }),
  );
  assert.equal(apiCalls, 0, "Lockout request must never hit the backend");

  // 携带有效 OVH SSH key 名时正常放行
  await vpsMod.useReinstallVps("vps-123.ovh.net", "acc-1").mutationFn({
    templateId: "debian-12",
    sshKey: ["my-prod-key"],
    doNotSendPassword: true,
  });
  assert.equal(apiCalls, 1);
  assert.deepEqual(JSON.parse(JSON.stringify(lastPayload)), {
    templateId: "debian-12",
    sshKey: ["my-prod-key"],
    doNotSendPassword: true,
  });

  // 源码级确认 VpsDetail 按实例作用域挂载 key，且 VpsReinstallDialog 在关闭/切实例/成功时重置 doNotSendPassword(false)
  const vpsDetailSrc = fs.readFileSync(path.join(repoRoot, "src/components/vps-control/VpsDetail.tsx"), "utf8");
  const vpsDlgSrc = fs.readFileSync(path.join(repoRoot, "src/components/vps-control/VpsReinstallDialog.tsx"), "utf8");
  assert.ok(vpsDetailSrc.includes("key={`${instanceAccount || \"\"}:${server.serviceName}`}"));
  assert.ok(vpsDlgSrc.includes("setDoNotSendPassword(false)"));
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-06: VPS 监控切换机型/子公司清空失效 os、retiredWarning 告警透出与全量 24 子公司列表
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-06: VPS monitor covers all 24 OVH subsidiaries, clears stale OS on model/subsidiary switch, and surfaces retiredWarning", () => {
  const { loadModule } = createModuleLoader({
    "@tanstack/react-query": defaultReactQueryMock(),
    "@/lib/http": { api: {} },
    sonner: { toast: { success() {}, warning() {}, error() {} } },
  });

  const { OVH_SUBSIDIARIES } = loadModule("src/lib/ovh-subsidiaries.ts");
  const { SUBSIDIARIES, reconcileVpsOsSelection } = loadModule("src/components/vps-monitor/types.ts");
  const { notifyVpsSubscriptionResult } = loadModule("src/hooks/ovh/use-vps-monitor.ts");

  // 1) 完整覆盖全部 24 个 OVH 子公司站点
  assert.equal(SUBSIDIARIES.length, OVH_SUBSIDIARIES.length);
  const subValues = new Set(SUBSIDIARIES.map((s) => s.value));
  for (const requiredCode of [
    "DE", "ES", "FI", "IT", "LT", "NL", "PL", "PT", "IE", "FR", "GB", "CZ",
    "MA", "SN", "TN", "CA", "QC", "WE", "WS", "SG", "AU", "ASIA", "IN", "US",
  ]) {
    assert.ok(subValues.has(requiredCode), `SUBSIDIARIES must contain ${requiredCode}`);
  }

  // 2) 切换机型或子公司后：若旧 os 不在新镜像列表中，立即归零为 ""，避免后端漏填必填 vps_os 触发结账 400
  assert.equal(
    reconcileVpsOsSelection("Windows Server 2022", ["Debian 12", "Ubuntu 24.04"], true),
    "",
  );
  assert.equal(
    reconcileVpsOsSelection("Debian 12", ["Debian 12", "Ubuntu 24.04"], true),
    "Debian 12",
  );
  assert.equal(
    reconcileVpsOsSelection("Ubuntu 22.04", [], true),
    "",
    "Switching to custom/unloaded model must clear stale OS from previous model",
  );

  // 3) 后端返回 retiredWarning 时调用 toast.warning 显著告警用户
  const warnings = [];
  const successes = [];
  notifyVpsSubscriptionResult(
    {
      status: "warning",
      message: "已订阅 vps-2025-model1",
      retiredWarning: "OVH 在 IE 站点已经不卖 vps-2025-model1，这条订阅可能永远不会补货",
    },
    "已添加订阅",
    {
      success: (m) => successes.push(m),
      warning: (m, opts) => warnings.push({ m, opts }),
    },
  );
  assert.equal(warnings.length, 1);
  assert.match(warnings[0].m, /已经不卖/);
  assert.equal(successes.length, 0);
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-07: IPMI serialOverLanSshKey 公钥输入校验与 ssh:// URI 终端指引（禁误报 JNLP 下载）
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-07: IPMI SOL SSH mode validates OpenSSH public key and classifies ssh:// URIs as SSH terminal commands instead of JNLP", () => {
  const { loadModule } = createModuleLoader();
  const { validateSolSshPublicKey, classifyIpmiConsoleResult } = loadModule(
    "src/components/server-control/ipmi-helpers.ts",
  );

  // 公钥校验
  assert.equal(validateSolSshPublicKey("   ").valid, false);
  assert.equal(validateSolSshPublicKey("not-an-ssh-key").valid, false);
  const goodKey = validateSolSshPublicKey(
    "  ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMockPublicKeyMaterial1234567890 ops@ovh  ",
  );
  assert.equal(goodKey.valid, true);
  assert.ok(goodKey.normalized.startsWith("ssh-ed25519 "));

  // ssh:// 连接串严禁落入 jnlp 分支误提示“JNLP 文件已开始下载”
  const solRes = classifyIpmiConsoleResult(
    "ssh://sol-user@ipmi-gra.ovh.net:2222",
    "serialOverLanSshKey",
  );
  assert.equal(solRes.kind, "ssh");
  assert.equal(solRes.command, "ssh -p 2222 sol-user@ipmi-gra.ovh.net");

  // 即使后端对其他模式返回 ssh:// URI，也识别为 ssh 而非 jnlp
  const genericSsh = classifyIpmiConsoleResult("ssh://root@198.51.100.10", "serialOverLanURL");
  assert.equal(genericSsh.kind, "ssh");
  assert.equal(genericSsh.command, "ssh root@198.51.100.10");

  // 真正的 kvmipJnlp 与 https KVM HTML5 保持原语义
  assert.equal(classifyIpmiConsoleResult(undefined, "kvmipJnlp").kind, "jnlp");
  const html5Res = classifyIpmiConsoleResult("https://ipmi.ovh.net/kvm?token=abc", "kvmipHtml5URL");
  assert.equal(html5Res.kind, "https");
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-08: useBiosSettings 解包后端 { success: true, bios: d } 包装对象
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-08: useBiosSettings / useServerBiosSettings unwraps { success: true, bios } and { success: true, sgx } payloads", async () => {
  const { loadModule } = createModuleLoader({
    "@tanstack/react-query": defaultReactQueryMock(),
    "@/lib/http": {
      api: {
        get: async (url) => {
          if (String(url).endsWith("/sgx")) {
            return {
              data: {
                success: true,
                sgx: { status: "enabled", prmrr: 128 },
              },
            };
          }
          return {
            data: {
              success: true,
              bios: {
                bootMode: "uefi",
                hyperThreading: true,
              },
            },
          };
        },
      },
    },
    "@/hooks/use-active-account": {
      useActiveAccount: () => ["acc-eu"],
      useActiveServerControlAccount: () => ["acc-eu"],
      useActiveVpsControlAccount: () => ["acc-eu"],
    },
    "@/hooks/use-accounts": {
      useDefaultAccount: () => ({ id: "acc-eu" }),
    },
  });

  const hwMod = loadModule("src/hooks/ovh/server-control/use-server-hardware.ts");
  const q = hwMod.useBiosSettings("ns-bios.eu", true, "acc-eu");
  const data = await q.queryFn();
  assert.equal("success" in data.settings, false, "Outer wrapper key 'success' must be stripped from settings");
  assert.equal("bios" in data.settings, false, "Outer wrapper key 'bios' must be stripped from settings");
  assert.deepEqual(data, {
    settings: {
      bootMode: "uefi",
      hyperThreading: true,
    },
    sgx: {
      status: "enabled",
      prmrr: 128,
    },
  });

  // 空 BIOS 设置对象返回 {}，从而正确进入 EmptyState 而非渲染 `{ success: true }` 假键值
  assert.deepEqual(hwMod.unwrapBiosPayload({ success: true, bios: {} }), {});
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-09: SPLA 许可证变更后立即清理 localStorage 模板缓存并失效 osTemplates Query
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-09: syncAfterSplaChange clears localStorage template cache and invalidates osTemplates + spla queries", () => {
  const { loadModule } = createModuleLoader({
    "@tanstack/react-query": defaultReactQueryMock(),
    "@/lib/http": { api: {} },
    "@/hooks/use-active-account": {
      useActiveAccount: () => ["acc-eu"],
      useActiveServerControlAccount: () => ["acc-eu"],
      useActiveVpsControlAccount: () => ["acc-eu"],
    },
    "@/hooks/use-accounts": {
      useDefaultAccount: () => ({ id: "acc-eu" }),
    },
    "@/i18n": { default: { t: (k) => k } },
  });

  const { TEMPLATES_LS_PREFIX, getServerTemplatesCacheKey } = loadModule(
    "src/hooks/ovh/server-control/use-server-install.ts",
  );
  const { syncAfterSplaChange } = loadModule("src/components/server-control/spla-helpers.ts");

  const svc = "ns-win.ip-1-2-3.eu";
  const accountId = "acc-eu";
  const lsKey = getServerTemplatesCacheKey(svc, accountId);
  assert.equal(lsKey, `${TEMPLATES_LS_PREFIX}${accountId}:${svc}`);

  const removedKeys = [];
  const invalidated = [];
  const removedQueries = [];
  const mockStorage = {
    removeItem(k) {
      removedKeys.push(k);
    },
  };
  const mockQc = {
    removeQueries({ queryKey }) {
      removedQueries.push(queryKey);
    },
    invalidateQueries({ queryKey }) {
      invalidated.push(queryKey);
    },
  };

  syncAfterSplaChange(mockQc, svc, accountId, mockStorage);

  // 验证当前账户缓存键与默认回退键均被清理
  assert.ok(removedKeys.includes(lsKey));
  assert.ok(removedKeys.includes(`${lsKey}_at`));
  assert.ok(removedKeys.includes(`${TEMPLATES_LS_PREFIX}default:${svc}`));

  // 验证 spla 与 osTemplates Query 缓存均被失效/移除（qk.serverControl.osTemplates 为 ["server-control", "os-templates", svc, accountId]）
  assert.ok(
    invalidated.some((k) => k[0] === "server-control" && k[1] === "spla" && k[2] === svc && k[3] === accountId),
    "spla query must be invalidated",
  );
  assert.ok(
    invalidated.some((k) => k[0] === "server-control" && k[1] === "os-templates" && k[2] === svc && k[3] === accountId),
    "osTemplates query must be invalidated",
  );
  assert.equal(removedQueries.length, 1);
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-10: MRTG 流量图按 timestamp 并集对齐合并上下行且单向采样为空仍正常渲染
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-10: MRTG merges download/upload points by timestamp union (never array index) and renders unidirectional series", () => {
  const { loadModule } = createModuleLoader();
  const { mergeMrtgInterfaces, mergeMrtgSeries } = loadModule(
    "src/components/server-control/mrtg-helpers.ts",
  );

  // 1) 上行接口在 T=1300 缺采样点：若按 upload.data[i] 配对会把 T=1600 的上传值错贴到 T=1300！
  const downloadPts = [
    { timestamp: 1000, value: { value: 100, unit: "bps" } },
    { timestamp: 1300, value: { value: 200, unit: "bps" } },
    { timestamp: 1600, value: { value: 300, unit: "bps" } },
  ];
  const uploadPts = [
    { timestamp: 1600, value: { value: 999, unit: "bps" } },
    { timestamp: 1000, value: { value: 50, unit: "bps" } },
  ];

  const merged = mergeMrtgSeries(downloadPts, uploadPts);
  assert.deepEqual(merged, [
    { timestamp: 1000, download: 100, upload: 50 },
    { timestamp: 1300, download: 200, upload: null },
    { timestamp: 1600, download: 300, upload: 999 },
  ]);

  // 2) 单向为空（例如 upload=[]，download 有数据）时仍返回网卡，绝不因 !upload?.length 误判无流量数据
  const nics = mergeMrtgInterfaces(
    [{ mac: "aa:bb:cc:dd:ee:ff", type: "public", data: downloadPts }],
    [],
  );
  assert.equal(nics.length, 1);
  assert.equal(nics[0].mac, "aa:bb:cc:dd:ee:ff");
  assert.equal(nics[0].download.data.length, 3);
  assert.equal(nics[0].upload.data.length, 0);
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-11: 实例级账户作用域隔离（阻断全局 localStorage 串号）与深链 ?service= 防过早清空
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-11: instance-scoped account context overrides global activeAccount across hooks and deep-link preserves wantedService until target list loads", async () => {
  const scopedAccountRef = "instance-acc-B";
  const requestedAccounts = [];

  const apiSpy = {
    get: async (_url, cfg) => {
      requestedAccounts.push(cfg?.params?.account);
      return { data: {} };
    },
    put: async (_url, _body, cfg) => {
      requestedAccounts.push(cfg?.params?.account);
      return { data: {} };
    },
    post: async (_url, _body, cfg) => {
      requestedAccounts.push(cfg?.params?.account);
      return { data: {} };
    },
  };

  const { loadModule } = createModuleLoader({
    react: {
      createContext: (def) => ({ _currentValue: def }),
      useContext: () => scopedAccountRef,
    },
    "@tanstack/react-query": defaultReactQueryMock(),
    "@/lib/http": { api: apiSpy },
    "@/hooks/use-active-account": {
      useActiveServerControlAccount: () => ["global-wrong-acc"],
      useActiveVpsControlAccount: () => ["global-wrong-acc"],
      useActiveAccount: () => ["global-wrong-acc"],
    },
    "@/hooks/use-accounts": {
      useAccounts: () => ({
        data: [
          { id: "global-wrong-acc", endpoint: "ovh-us", zone: "US" },
          { id: "instance-acc-B", endpoint: "ovh-eu", zone: "FR" },
        ],
        isPending: false,
      }),
      useDefaultAccount: () => ({ id: "global-wrong-acc", endpoint: "ovh-us", zone: "US" }),
      findAccountByID: (list, id) => (list || []).find((a) => a.id === id),
    },
    "@/i18n": { default: { t: (k) => k } },
  });

  // 加载 server-scope 自动执行 registerServerScopeHook，把页面子树内的所有 server-control hooks 锁死在 instance-acc-B
  const scopeMod = loadModule("src/components/server-control/server-scope.ts");
  const coreMod = loadModule("src/hooks/ovh/server-control/use-server-core.ts");
  const netMod = loadModule("src/hooks/ovh/server-control/use-server-network.ts");
  const featMod = loadModule("src/hooks/ovh/server-control/use-server-features.ts");
  const engageMod = loadModule("src/hooks/ovh/server-control/use-server-engagement.ts");

  // 即使不显式传 overrideAccountId，子树内 hooks 也自动取实例作用域 "instance-acc-B"，绝不串号到 "global-wrong-acc"
  await coreMod.useServerServiceInfo("ns-target.eu").queryFn();
  await coreMod.useUpdateRenewal("ns-target.eu").mutationFn({ mode: "auto" });
  await netMod.useMitigation("ns-target.eu").queryFn();
  await featMod.useServerBurst("ns-target.eu").queryFn();
  await featMod.useToggleMonitoring().mutationFn({ serviceName: "ns-target.eu", enabled: true });
  await engageMod.useRequestRetraction("ns-target.eu", false).mutationFn({ reason: "other" });

  assert.deepEqual(requestedAccounts, [
    "instance-acc-B",
    "instance-acc-B",
    "instance-acc-B",
    "instance-acc-B",
    "instance-acc-B",
    "instance-acc-B",
  ]);

  // useScopedServerEndpoint 同样按实例账户返回欧区能力（isUS === false），不被顶栏美区账户误伤
  const ep = scopeMod.useScopedServerEndpoint();
  assert.equal(ep.accountId, "instance-acc-B");
  assert.equal(ep.isUS, false);

  // 验证 ?service=ns-new.eu&account=acc-B 深链完整异步时序：
  // 阶段 1：accounts 尚未返回、activeAccount 仍是 acc-A 且列表为空 -> 保留 wantedAccount 与 wantedService
  let step = scopeMod.resolveControlDeepLink({
    wantedAccount: "acc-B",
    wantedService: "ns-new.eu",
    accounts: undefined,
    activeAccount: "acc-A",
    items: [],
    listPending: true,
    listFetching: true,
    selectedName: null,
  });
  assert.equal(step.nextWantedAccount, "acc-B");
  assert.equal(step.nextWantedService, "ns-new.eu");

  // 阶段 2：accounts 加载完成 -> 发出 switchToAccount: "acc-B"，同时继续保留 wantedService
  step = scopeMod.resolveControlDeepLink({
    wantedAccount: step.nextWantedAccount,
    wantedService: step.nextWantedService,
    accounts: [{ id: "acc-A" }, { id: "acc-B" }],
    activeAccount: "acc-A",
    items: [{ serviceName: "ns-old-a.eu", accountId: "acc-A" }],
    listPending: false,
    listFetching: false,
    selectedName: "ns-old-a.eu",
  });
  assert.equal(step.switchToAccount, "acc-B");
  assert.equal(step.nextWantedService, "ns-new.eu");

  // 阶段 3：activeAccount 已切到 acc-B，但新账户机器列表仍在拉取 (listPending=true, items=[]) -> 绝不清空 wantedService！
  step = scopeMod.resolveControlDeepLink({
    wantedAccount: step.nextWantedAccount,
    wantedService: step.nextWantedService,
    accounts: [{ id: "acc-A" }, { id: "acc-B" }],
    activeAccount: "acc-B",
    items: [],
    listPending: true,
    listFetching: true,
    selectedName: null,
  });
  assert.equal(step.nextWantedService, "ns-new.eu");
  assert.equal(step.nextSelectedName, null);

  // 阶段 4：acc-B 机器列表到达 -> 精准选中 ns-new.eu 并消费掉 wantedService
  step = scopeMod.resolveControlDeepLink({
    wantedAccount: step.nextWantedAccount,
    wantedService: step.nextWantedService,
    accounts: [{ id: "acc-A" }, { id: "acc-B" }],
    activeAccount: "acc-B",
    items: [
      { serviceName: "ns-first-b.eu", accountId: "acc-B" },
      { serviceName: "ns-new.eu", accountId: "acc-B" },
    ],
    listPending: false,
    listFetching: false,
    selectedName: null,
  });
  assert.equal(step.nextSelectedName, "ns-new.eu");
  assert.equal(step.nextWantedService, null);
});

/* ──────────────────────────────────────────────────────────────────────────
 * BUG-FE-CTRL-12: VPS 全部生命周期写操作（启停/重启/重装/快照回滚/终止/取消策略）完整失效关联缓存
 * ────────────────────────────────────────────────────────────────────────── */
test("BUG-FE-CTRL-12: VPS lifecycle mutations invalidate list, info, status, serviceInfo, tasks, currentOS, and snapshot caches", () => {
  const invalidatedKeys = [];
  const { loadModule } = createModuleLoader({
    "@tanstack/react-query": {
      ...defaultReactQueryMock(),
      useQueryClient: () => ({
        invalidateQueries: ({ queryKey }) => {
          invalidatedKeys.push(queryKey);
        },
      }),
    },
    "@/lib/http": {
      api: {
        get: async () => ({ data: {} }),
        post: async () => ({ data: {} }),
        put: async () => ({ data: {} }),
        delete: async () => ({ data: {} }),
      },
    },
    "@/hooks/use-active-account": {
      useActiveVpsControlAccount: () => ["acc-vps"],
      useActiveServerControlAccount: () => ["acc-vps"],
    },
    "@/hooks/use-accounts": {
      useDefaultAccount: () => ({ id: "acc-vps" }),
    },
  });

  const vpsHooks = loadModule("src/hooks/ovh/use-vps-control.ts");
  const svc = "vps-abc.ovh.net";
  const acc = "acc-vps";

  const cases = [
    ["useVpsStart", () => vpsHooks.useVpsStart(svc, acc).onSuccess()],
    ["useVpsStop", () => vpsHooks.useVpsStop(svc, acc).onSuccess()],
    ["useVpsReboot", () => vpsHooks.useVpsReboot(svc, acc).onSuccess()],
    ["useReinstallVps", () => vpsHooks.useReinstallVps(svc, acc).onSuccess()],
    ["useRevertVpsSnapshot", () => vpsHooks.useRevertVpsSnapshot(svc, acc).onSuccess()],
    ["useUpdateVpsTerminationPolicy", () => vpsHooks.useUpdateVpsTerminationPolicy(svc, acc).onSuccess()],
    ["useTerminateVps", () => vpsHooks.useTerminateVps(acc).onSuccess(undefined, { serviceName: svc })],
    [
      "useConfirmTerminateVps",
      () => vpsHooks.useConfirmTerminateVps(acc).onSuccess(undefined, { serviceName: svc, token: "tok" }),
    ],
  ];

  for (const [hookName, triggerSuccess] of cases) {
    invalidatedKeys.length = 0;
    triggerSuccess();

    const keyNames = invalidatedKeys.map((k) => `${k[0]}:${k[1]}`);
    for (const expected of [
      "vps-control:list",
      "vps-control:info",
      "vps-control:status",
      "vps-control:service-info",
      "vps-control:tasks",
      "vps-control:current-os",
      "vps-control:snapshot",
    ]) {
      assert.ok(
        keyNames.includes(expected),
        `${hookName}.onSuccess must invalidate ${expected} (got: ${keyNames.join(", ")})`,
      );
    }
  }
});
