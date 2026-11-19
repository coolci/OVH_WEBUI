# 分册六：前端独服控制面与 VPS 控制/监控面已验证 BUG 台账（`BUG-FE-CTRL-01` ~ `BUG-FE-CTRL-12`）

- **所属总账**：`00_MASTER_HANDOFF_LEDGER.md`
- **收录范围**：独服运维控制页与 29 个重装/RAID/续费/退订/IPMI/防火墙组件（`src/pages/ServerControlPage.tsx`、`src/components/server-control/**`、`src/hooks/ovh/server-control/*`）、VPS 控制与监控面（`src/pages/VpsControlPage.tsx`、`VpsMonitorPage.tsx`、`src/components/vps-control/*`、`vps-monitor/*`、`use-vps-control.ts`、`use-vps-monitor.ts`）
- **核验结果**：共 **12 项** 100% 实证 BUG（`Critical: 1` / `High: 7` / `Medium: 4` / `Low: 0`）
- **配套动态验证套件**：`audit_ledger/pocs/security_and_frontend/fe_verify.test.mjs`

---

## BUG-FE-CTRL-01：`useInstallServer` 中 `parsedKey === DEFAULT_GID` 恒为 `false` 死分支，导致单磁盘组独服同时配置自定义分区与硬件 RAID 时分裂为两条重复 `diskGroupId` 被后端 `STORAGE_DUPLICATE_GROUP` 100% 拒绝

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/hooks/ovh/server-control/use-server-install.ts:120-128, 149-163`
  - `src/components/server-control/reinstall/PartitionRow.tsx:68`
  - `backend/internal/handlers/server_control_basic.go:1084-1102`
- **完整触发链路**：
  - 单磁盘组独服上新增分区 `diskGroupId` 为 `undefined`，归入键 `"default"`；用户同时选硬件 RAID（键 `"1"`）时，第 155 行合并守卫 `parsedKey === DEFAULT_GID`（`"1" === "default"`）恒假，生成两条记录发往后端，后端补默认 ID 后触发 `400 STORAGE_DUPLICATE_GROUP`。
- **动态验证铁证**：`✔ FE-CTRL-01, FE-CTRL-02 & FE-CTRL-04` 实测断言输出两条分裂 `StorageGroup` 通过。
- **下一代架构修复方案**：
  - 单磁盘组机器创建分区时直接绑定真实 `diskGroupIds[0]`，消除虚拟 `"default"` 键分裂。

---

## BUG-FE-CTRL-02：硬件 RAID 热备盘未从总盘数扣除（`disks + spares > 物理盘数` 必致 OVH 400）、多磁盘组共用全局 `hwRaidSpares`/`hwRaidArrays` 且漏掉 `raid50`/`raid60` 的 `arrays` 参数

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/hooks/ovh/server-control/use-server-install.ts:165-173`
  - `src/components/server-control/ReinstallDialog.tsx:66-67, 250-251`
  - `src/components/server-control/reinstall/StoragePartitionSection.tsx:371-398`
- **动态验证铁证**：`✔ FE-CTRL-01, FE-CTRL-02 & FE-CTRL-04` 实测 4 盘 + 1 热备生成 `disks(4) + spares(1) = 5 > 4` 越界载荷。
- **下一代架构修复方案**：
  - 按 `diskGroupId` 独立维护热备盘与阵列数，计算 `item.disks = diskCount - spares` 并开放 `raid50`/`raid60` 的 `arrays` 输入。

---

## BUG-FE-CTRL-03：`ReinstallDialog` 切换 OS 模板未清空 `customizationAnswers` 跨模板污染、`number` 问答以 `string` 提交、漏拦 `mandatory` 空值，且修改保留数据盘/RAID 不重置高危二次确认态

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/server-control/ReinstallDialog.tsx:69, 89-101, 178-180, 207-236`
  - `src/components/server-control/reinstall/PostInstallSection.tsx:54-125`
- **完整触发链路**：
  - 用户点击第一次“确认重装”进入 `confirming = true` 后，若取消勾选 `keepDiskGroups`（保留数据盘）或修改硬件 RAID/分区表，因复位依赖未包含这些高危状态，再点按钮将跳过二次确认直接下发格盘重装！同时切换模板残留旧问答参数且数值问答传字符串触发 OVH 400。
- **下一代架构修复方案**：
  - 切换模板重置 `customizationAnswers`，按当前模板白名单校验必填项并转换 `number` 类型；将全部磁盘/分区/保留开关纳入 `confirming` 复位依赖。

---

## BUG-FE-CTRL-04：`ReinstallDialog` 智能分区容量计算将 `MB`/`Mo` 单位直接当作 `GB` 放大 1024 倍，且前端按 `Object.values(groups)[0]` 选默认盘组

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/server-control/ReinstallDialog.tsx:182-196`（关联后端 `BUG-BE-03`）
- **动态验证铁证**：`✔ FE-CTRL-01, FE-CTRL-02 & FE-CTRL-04` 实测 `2 × 480000 MB` 算出 `960,000 GB`（960 TB）。
- **下一代架构修复方案**：
  - 补全 `MB`/`Mo`（`/ 1024`）与 `KB`/`Ko`（`/ 1048576`）单位换算，并显式读取硬件规格中的 `defaultDiskGroupId`。

---

## BUG-FE-CTRL-05：`VpsReinstallDialog` 允许无 SSH 密钥时勾选 `doNotSendPassword`（而远程改密 API 已 410 永久退役），且关闭/提交后漏重置 `doNotSendPassword` 导致跨实例重装静默锁死新机器

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/vps-control/VpsReinstallDialog.tsx:39-44, 60-97, 231-256`
  - `src/components/vps-control/VpsDetail.tsx:520-525`
  - `backend/internal/handlers/vps_control_install.go:283-340`、`backend/internal/handlers/vps_control_power.go:121-127`
- **完整触发链路**：
  1. OVH 已下线 VPS API 远程设密（`vps_control_power.go:121-127` 固定返回 `410 Gone`）。
  2. `VpsDetail` 常驻挂载 `<VpsReinstallDialog>`；提交成功后第 89-94 行重置了 `setSelectedSshKey("__none__")` 却**漏写 `setDoNotSendPassword(false)`**！
  3. 运维人员给第一台带 SSH Key 的机器勾选“不发送密码邮件”重装后，切到第二台机器重装（默认不注入密钥）时，残留的 `doNotSendPassword === true` 叠加 `selectedSshKey === "__none__"` 直接交付一台既无 SSH 公钥又无密码邮件的**永久死锁 VPS**！
- **下一代安全与架构修复方案**：
  - 前端在未选有效 SSH Key 时强制禁用并置 `doNotSendPassword = false`，在 `open`/`serviceName` 变化时全量初始化状态；后端 `ReinstallVps` 增加硬性校验拒绝“无公钥 + 禁发密码”组合（返 `400 EVPS_LOCKOUT_PREVENTED`）。

---

## BUG-FE-CTRL-06：VPS 补货监控弹窗切换机型/子公司未清空旧 `os` 致后端 `buildVPSConfig` 静默漏填必填 `vps_os` 结账报 400；且吞没 `retiredWarning`、硬编码子公司缺失 15 个站点

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/vps-monitor/AddVPSDialog.tsx:66-77, 131-153`
  - `src/components/vps-monitor/types.ts:1-11`（仅 9 项 vs `src/lib/ovh-subsidiaries.ts:15-40` 共 24 项）
  - `src/hooks/ovh/use-vps-monitor.ts:93-107`、`backend/internal/vps/purchase.go:366-378`
- **动态验证铁证**：`✔ FE-CTRL-06 & FE-CORE-16` 实测确认 `vpsSubs.length === 9` vs `OVH_SUBSIDIARIES.length === 24`（缺失 `SG`、`AU`、`ASIA`、`IN`、`NL` 等 15 个站点）。
- **下一代架构修复方案**：
  - 切换机型/子公司时重置 `os`，复用全局统一的 24 站点 `OVH_SUBSIDIARIES`，展示 `retiredWarning` 告警，并在后端对不兼容 `os` 显式报错。

---

## BUG-FE-CTRL-07：独服 `IpmiDialog` 选择 `serialOverLanSshKey` 时缺失 SSH 公钥输入框，且返回 `ssh://` 连接串后被 `safeHttpsUrl` 拦截误渲染为“JNLP 已下载”假提示

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/components/server-control/IpmiDialog.tsx:133, 144-159, 215-219, 250-265`
  - `backend/internal/handlers/server_control_network.go:407, 559-563`
- **下一代架构修复方案**：
  - 为 `serialOverLanSshKey` 增加公钥输入与 `params.sshKey` 透传；对返回的 `ssh://` URI 单独提供代码块展示与一键复制终端登录指令。

---

## BUG-FE-CTRL-08：`useBiosSettings` 未解包后端 `{ success: true, bios: d }` 包装对象，导致 `BiosDialog` 将整个 `bios` 渲染为单行原始 JSON 字符串且空状态失效

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/hooks/ovh/server-control/use-server-hardware.ts:129-141`、`src/components/server-control/BiosDialog.tsx:22-24, 53-58`、`backend/internal/handlers/server_control_misc.go:760`
- **下一代架构修复方案**：`useBiosSettings` 显式解包 `res.data?.bios ?? {}`。

---

## BUG-FE-CTRL-09：`SplaDialog` 添加/注销许可证后未失效 1 小时 `osTemplates` 查询与 `localStorage` 缓存，重装弹窗长期看不到刚解锁的 Windows/SQL 镜像

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/server-control/SplaDialog.tsx:77-108`、`src/hooks/ovh/server-control/use-server-install.ts:12-55`
- **下一代架构修复方案**：SPLA 写操作成功后同步清理按 `accountId + serviceName` 隔离的模板 `localStorage` 与 Query 缓存。

---

## BUG-FE-CTRL-10：`MrtgTrafficChart` 按数组下标 `upload.data[i]` 硬配对上下行曲线（缺帧时时间轴错位），且单向采样为空时整张网卡图静默消失

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/components/server-control/MrtgTrafficChart.tsx:58-67, 131-142`
- **下一代架构修复方案**：按 `timestamp` 外连接归并排序上下行采样点并允许单向曲线独立渲染。

---

## BUG-FE-CTRL-11：`ServerControlPage` 仅向 `PowerTab` 与 `ReinstallDialog` 透传 `server.accountId`，其余 3 个高危计费/退订弹窗与 3 个功能标签页仍读全局 `localStorage` 账户致跨账户串号；且 URL 深链过早清空 `wantedService`

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/pages/ServerControlPage.tsx:60-66, 89-93, 379-429`
- **完整触发链路**：
  - 多账户环境下，`RetractionDialog`（撤单退款）、`EngagementDialog`（承诺期销毁）、`RenewalDialog`（续费）、`OverviewTab`、`MaintenanceTab`、`AdvancedTab` 均未获传 `server.accountId`，多标签页切顶栏账户后拿错误账户请求目标独服报 403/404；`?service=xxx&account=accB` 深链在旧账户列表非空时提前将 `wantedService.current` 置 `null` 导致跳转失焦。
- **下一代架构修复方案**：
  - 通过 `ServerScopeContext` 向所有子 Tab 与 Dialog 强制绑定 `server.accountId`；深链仅在目标账户列表就绪后消费 `wantedService`。

---

## BUG-FE-CTRL-12：`use-vps-control.ts` 中重启、重装、快照回滚与终止服务 5 个写操作 `onSuccess` 漏失效关联缓存，操作完成后界面长期停留旧状态诱导重复提交

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：`src/hooks/ovh/use-vps-control.ts:225-230, 298-314, 390-401, 459-481`
- **下一代架构修复方案**：封装统一的 `invalidateVpsLifecycle(qc, serviceName, accountId)` 覆盖 `detail`、`tasks`、`serviceInfo`、`list`。
